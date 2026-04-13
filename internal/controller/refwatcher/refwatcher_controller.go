/*
Copyright 2026. The Magos Authors.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use
this file except in compliance with the License. You may obtain a copy of the
License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed
under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR
CONDITIONS OF ANY KIND, either express or implied. See the License for the
specific language governing permissions and limitations under the License.
*/

package refwatcher

import (
	"container/heap"
	"context"
	"math/rand"
	"sync"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/magosproject/magos/types/magosproject/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	// gitTimeout is the context deadline applied to every go-git remote.List
	// call. 10 seconds is generous for an ls-remote against most Git hosts.
	// If a remote consistently exceeds this, it will surface as "error" in the
	// refwatcher_poll_total metric rather than blocking a worker indefinitely.
	gitTimeout = 10 * time.Second

	// maxRetryBackoff caps how long we wait after a failed poll before trying
	// again. On error we reschedule at min(entry.interval, maxRetryBackoff),
	// so workspaces with very long poll intervals don't wait an unreasonable
	// amount of time before retrying a transient failure.
	maxRetryBackoff = 30 * time.Second
)

// registryEntry tracks the polling state for a single Workspace. Each field
// mirrors a piece of the Workspace spec or status that the scheduler and
// workers need without re-reading the API server.
//
// We use sync.RWMutex (not sync.Map) because every poll cycle writes back
// lastSHA and nextPollAt. sync.Map is optimized for stable keys with rare
// writes, which is the opposite of this access pattern.
type registryEntry struct {
	url        string
	ref        string
	interval   time.Duration
	nextPollAt time.Time
	lastSHA    string
}

// Registry holds the set of Workspaces that RefWatcher is actively polling.
// Protected by a RWMutex so workers can read entries concurrently while
// Reconcile and reschedule hold a write lock only for mutations.
type Registry struct {
	mu      sync.RWMutex
	entries map[types.NamespacedName]*registryEntry
}

// heapEntry is a lightweight element in the min-heap scheduler. It only
// carries the key and the scheduled time; the full polling state lives in
// the Registry so we don't duplicate mutable data.
type heapEntry struct {
	key        types.NamespacedName
	nextPollAt time.Time
}

// pollHeap implements container/heap.Interface, ordered by nextPollAt ascending.
// The scheduler sleeps until heap[0].nextPollAt, giving O(log n) enqueue and
// O(1) peek — far cheaper than iterating every entry on a fixed tick when
// thousands of Workspaces are registered.
type pollHeap []heapEntry

func (h pollHeap) Len() int           { return len(h) }
func (h pollHeap) Less(i, j int) bool { return h[i].nextPollAt.Before(h[j].nextPollAt) }
func (h pollHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *pollHeap) Push(x any)        { *h = append(*h, x.(heapEntry)) }
func (h *pollHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[:n-1]
	return item
}

// RefWatcherReconciler watches Workspace resources and polls their Git remotes
// for new commits. When a branch or tag resolves to a different SHA than the
// last known value, RefWatcher patches the magosproject.io/detected-revision
// annotation on the Workspace. The main Workspace controller picks up that
// annotation change and starts a fresh plan/apply cycle.
//
// This design keeps Git I/O completely out of the Reconcile path. Reconcile
// only maintains an in-memory registry of what to poll; the actual network
// calls happen in a separate worker pool fed by a min-heap scheduler. This
// means a slow or unreachable Git remote can never block the controller-runtime
// work queue.
type RefWatcherReconciler struct {
	client.Client

	DefaultPollInterval time.Duration
	WorkerCount         int
	WorkQueueSize       int

	registry Registry
	workCh   chan types.NamespacedName
	updateCh chan struct{}

	// heapMu protects the scheduler heap independently from the registry. The
	// heap is mutated by both the scheduler goroutine (pop) and by registry
	// mutations (push), so it needs its own lock.
	heapMu sync.Mutex
	ph     pollHeap
}

// +kubebuilder:rbac:groups=magosproject.io,resources=workspaces,verbs=get;list;watch;patch

// Reconcile is the entry point invoked by controller-runtime whenever a
// Workspace is created, updated, or deleted. Its only job is to keep the
// in-memory polling registry in sync with the cluster state. It performs zero
// Git I/O — all remote calls happen asynchronously in the worker pool.
//
// On create or update the entry is upserted with the Workspace's current
// spec.source.repoURL, spec.source.targetRevision, and the parsed
// magosproject.io/git-poll-interval annotation. On delete, or when
// DeletionTimestamp is set, the entry is removed from the registry and the
// scheduler will skip it on its next wake.
func (r *RefWatcherReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	workspace := &v1alpha1.Workspace{}
	if err := r.Get(ctx, req.NamespacedName, workspace); err != nil {
		if errors.IsNotFound(err) {
			r.removeEntry(req.NamespacedName)
			logger.Info("Workspace deleted, removed from registry")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get Workspace")
		return ctrl.Result{}, err
	}

	// If the workspace is being deleted, remove it from the registry so the
	// scheduler stops polling. We don't need a finalizer here because
	// RefWatcher holds no external state that requires cleanup.
	if workspace.DeletionTimestamp != nil {
		r.removeEntry(req.NamespacedName)
		logger.Info("Workspace being deleted, removed from registry")
		return ctrl.Result{}, nil
	}

	url := workspace.Spec.Source.RepoURL
	ref := workspace.Spec.Source.TargetRevision
	interval := r.parsePollInterval(workspace)

	r.upsertEntry(req.NamespacedName, url, ref, interval, workspace.Status.ObservedRevision)

	return ctrl.Result{}, nil
}

// parsePollInterval reads the magosproject.io/git-poll-interval annotation and
// parses it as a Go duration. If the annotation is missing, empty, or
// malformed, the controller's --default-poll-interval flag value is used
// instead. This lets teams with high-frequency deployments poll every few
// seconds while keeping a conservative default for the majority of Workspaces.
func (r *RefWatcherReconciler) parsePollInterval(ws *v1alpha1.Workspace) time.Duration {
	if ws.Annotations != nil {
		if v, ok := ws.Annotations[v1alpha1.WorkspaceGitPollIntervalAnnotation]; ok {
			if d, err := time.ParseDuration(v); err == nil && d > 0 {
				return d
			}
		}
	}
	return r.DefaultPollInterval
}

// upsertEntry adds or updates a registry entry and signals the scheduler to
// re-evaluate its sleep.
//
// For a brand new entry, lastSHA is seeded from status.observedRevision. If
// the Workspace has already completed at least one apply cycle, this will be
// the commit SHA or branch name that was recorded in Step 8 of the Workspace
// controller, so the first poll will not see a diff and will not patch the
// annotation. If the Workspace is brand new (observedRevision is empty),
// lastSHA starts as "" and the first poll will resolve the branch to a real
// SHA. In that case, pollRemote detects the empty baseline and silently seeds
// the SHA without patching, so the Workspace controller can run its initial
// plan/apply cycle using the branch name from the spec rather than a
// RefWatcher injected commit SHA.
//
// nextPollAt is set to now + rand(0, interval) to spread the initial polls
// across the interval window and prevent a thundering herd when a new leader
// reconciles all Workspaces simultaneously.
//
// For an existing entry where url, ref, and interval are all unchanged, the
// method is a no-op. This is the most common path: an unrelated Workspace
// field changed (e.g. autoApply was toggled) and we must not reset the poll
// timer or clear lastSHA. If the url or ref did change, nextPollAt is reset to
// now so the next scheduler wake polls immediately. Changing the repo URL or
// branch is a strong signal that the user wants to track a different commit,
// and waiting for the remaining interval would feel sluggish.
func (r *RefWatcherReconciler) upsertEntry(key types.NamespacedName, url, ref string, interval time.Duration, observedRevision string) {
	r.registry.mu.Lock()
	defer r.registry.mu.Unlock()

	now := time.Now()

	existing, exists := r.registry.entries[key]
	if exists {
		// If url/ref/interval are unchanged, preserve lastSHA and nextPollAt.
		// This is the most common path — an unrelated Workspace update (e.g.
		// a status change or annotation tweak) should not disturb the poll
		// schedule.
		if existing.url == url && existing.ref == ref && existing.interval == interval {
			return
		}
		// url or ref changed — reset to poll immediately so the user gets
		// fast feedback after switching branches or repos.
		if existing.url != url || existing.ref != ref {
			existing.url = url
			existing.ref = ref
			existing.interval = interval
			existing.nextPollAt = now
			r.signalScheduler(key, now)
			return
		}
		// Only interval changed — update it but keep the current schedule.
		// Changing the poll interval should not trigger an immediate poll.
		existing.interval = interval
		return
	}

	// New entry: seed lastSHA from status.observedRevision. For Workspaces
	// that have already applied at least once this prevents a spurious
	// annotation patch on leader startup. For brand new Workspaces the seed
	// is "" and pollRemote handles the first poll as a silent baseline (see
	// the empty lastSHA guard in pollRemote). Apply jitter to nextPollAt so
	// that a mass re-sync (e.g. leader failover reconciling hundreds of
	// Workspaces) does not slam the Git host with concurrent ls-remote calls.
	entry := &registryEntry{
		url:        url,
		ref:        ref,
		interval:   interval,
		nextPollAt: now.Add(time.Duration(rand.Int63n(int64(interval)))),
		lastSHA:    observedRevision,
	}
	r.registry.entries[key] = entry
	registrySize.Set(float64(len(r.registry.entries)))

	r.signalScheduler(key, entry.nextPollAt)
}

// removeEntry deletes a Workspace from the registry. Stale heap entries for
// this key may still exist in the pollHeap; the scheduler's dispatchDue method
// skips any entry whose key is no longer in the registry, so we don't need to
// eagerly remove them from the heap.
func (r *RefWatcherReconciler) removeEntry(key types.NamespacedName) {
	r.registry.mu.Lock()
	defer r.registry.mu.Unlock()

	if _, exists := r.registry.entries[key]; !exists {
		return
	}
	delete(r.registry.entries, key)
	registrySize.Set(float64(len(r.registry.entries)))

	// Signal the scheduler to re-evaluate its sleep in case the deleted entry
	// was the next one due.
	r.notifyScheduler()
}

// signalScheduler pushes a new heap entry and wakes the scheduler goroutine.
// Because the heap may contain duplicate keys (from successive reschedules),
// dispatchDue validates each popped key against the registry before enqueuing
// it into the work channel.
func (r *RefWatcherReconciler) signalScheduler(key types.NamespacedName, nextPollAt time.Time) {
	r.heapMu.Lock()
	heap.Push(&r.ph, heapEntry{key: key, nextPollAt: nextPollAt})
	r.heapMu.Unlock()
	r.notifyScheduler()
}

// notifyScheduler sends a non-blocking signal on updateCh to wake the
// scheduler goroutine. If a signal is already pending (channel is full), the
// second send is dropped — the scheduler will re-evaluate the heap on the
// existing signal, which is sufficient.
func (r *RefWatcherReconciler) notifyScheduler() {
	select {
	case r.updateCh <- struct{}{}:
	default:
	}
}

// Start is called by the manager after leader election is won. It launches the
// worker pool and runs the scheduler loop until the context is cancelled (i.e.
// the manager is shutting down or leadership is lost).
//
// The registry does not need to be explicitly rebuilt on leader startup. The
// informer cache re-syncs all Workspace resources and fires a Reconcile for
// each one, which populates the registry through upsertEntry. The jitter
// applied to new entries prevents all of those initial polls from firing at
// the same instant.
func (r *RefWatcherReconciler) Start(ctx context.Context) error {
	logger := log.FromContext(ctx).WithName("refwatcher-scheduler")

	var wg sync.WaitGroup
	for i := 0; i < r.WorkerCount; i++ {
		wg.Go(func() {
			r.worker(ctx)
		})
	}

	// Scheduler loop: sleep until the earliest entry is due, pop all due
	// entries, enqueue them into the work channel, and repeat. On registry
	// mutations the updateCh signal interrupts the sleep so the scheduler
	// can immediately re-evaluate the heap (e.g. when a new Workspace with
	// nextPollAt = now is added).
	go func() {
		var timer *time.Timer
		defer func() {
			if timer != nil {
				timer.Stop()
			}
		}()

		for {
			r.heapMu.Lock()
			var sleepDur time.Duration
			if r.ph.Len() == 0 {
				// No entries — sleep for a long time. The scheduler will be
				// woken by updateCh when the first Workspace is registered.
				sleepDur = time.Hour
			} else {
				sleepDur = max(time.Until(r.ph[0].nextPollAt), 0)
			}
			r.heapMu.Unlock()

			if timer == nil {
				timer = time.NewTimer(sleepDur)
			} else {
				timer.Reset(sleepDur)
			}

			select {
			case <-ctx.Done():
				return
			case <-r.updateCh:
				// A registry mutation occurred (new entry, deleted entry, or
				// ref/url change). Drain the timer and re-evaluate the heap
				// immediately — the new earliest entry may be due sooner than
				// what we were sleeping for.
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				r.dispatchDue(logger)
			case <-timer.C:
				r.dispatchDue(logger)
			}
		}
	}()

	// Block until shutdown. Closing workCh signals workers to drain and exit.
	<-ctx.Done()
	close(r.workCh)
	wg.Wait()
	return nil
}

// dispatchDue pops all heap entries whose nextPollAt has passed and enqueues
// their keys into the work channel. Entries that no longer exist in the
// registry (deleted Workspaces) are silently discarded. If the work channel is
// full, the entry is skipped with a warning — the scheduler will naturally
// retry it on the next wake because the registry entry's nextPollAt was
// already set when the entry was created/rescheduled.
func (r *RefWatcherReconciler) dispatchDue(logger interface{ Info(string, ...any) }) {
	now := time.Now()
	r.heapMu.Lock()
	defer r.heapMu.Unlock()

	for r.ph.Len() > 0 && !r.ph[0].nextPollAt.After(now) {
		entry := heap.Pop(&r.ph).(heapEntry)

		// Validate against the registry — the Workspace may have been deleted
		// since this heap entry was pushed.
		r.registry.mu.RLock()
		_, exists := r.registry.entries[entry.key]
		r.registry.mu.RUnlock()
		if !exists {
			continue
		}

		select {
		case r.workCh <- entry.key:
		default:
			logger.Info("Work channel full, skipping workspace", "workspace", entry.key)
		}
	}

	workerQueueDepth.Set(float64(len(r.workCh)))
}

// worker consumes NamespacedName keys from the work channel and polls the
// corresponding Git remote. Each worker runs in its own goroutine; the number
// of concurrent workers is controlled by --worker-count (default 20).
func (r *RefWatcherReconciler) worker(ctx context.Context) {
	for key := range r.workCh {
		if ctx.Err() != nil {
			return
		}
		r.pollRemote(ctx, key)
	}
}

// pollRemote performs a git ls-remote for a single Workspace and, if the
// resolved SHA differs from lastSHA, patches the magosproject.io/detected-revision
// annotation on the Workspace object. The Workspace controller watches for
// changes to this annotation and triggers a fresh plan/apply cycle when the
// value diverges from status.observedRevision.
//
// When the poll succeeds the entry is rescheduled at now + interval regardless
// of whether the SHA actually changed. On error the entry is rescheduled at
// now + min(interval, 30s) so transient failures like network timeouts or
// temporary auth issues are retried quickly without waiting for the full poll
// interval. Importantly, lastSHA is never updated on error — if a poll fails
// after the remote actually moved, the next successful poll will still detect
// the change and patch the annotation.
func (r *RefWatcherReconciler) pollRemote(ctx context.Context, key types.NamespacedName) {
	logger := log.FromContext(ctx).WithValues("workspace", key)

	// Snapshot the entry state under a read lock. Workers must not hold the
	// lock during the network call — a slow remote would block all registry
	// mutations.
	r.registry.mu.RLock()
	entry, exists := r.registry.entries[key]
	if !exists {
		r.registry.mu.RUnlock()
		return
	}
	url := entry.url
	ref := entry.ref
	lastSHA := entry.lastSHA
	interval := entry.interval
	r.registry.mu.RUnlock()

	start := time.Now()
	newSHA, err := resolveRef(ctx, url, ref)
	elapsed := time.Since(start)
	pollDuration.WithLabelValues(key.Namespace, key.Name).Observe(elapsed.Seconds())

	if err != nil {
		logger.Error(err, "Failed to poll git remote", "url", url, "ref", ref)
		pollTotal.WithLabelValues(key.Namespace, key.Name, "error").Inc()
		r.reschedule(key, minDuration(interval, maxRetryBackoff))
		return
	}

	if newSHA == lastSHA {
		pollTotal.WithLabelValues(key.Namespace, key.Name, "unchanged").Inc()
		r.reschedule(key, interval)
		return
	}

	// When lastSHA is empty the Workspace has never completed an apply cycle.
	// This is the normal path for a brand new Workspace whose
	// spec.source.targetRevision is a branch name like "main". The Workspace
	// controller's Step 8 has not run yet, so status.observedRevision is blank
	// and upsertEntry seeded lastSHA as "". The first poll resolves "main" to
	// a real commit SHA and sees a diff, but this is not a genuine ref change:
	// the user just created the Workspace and expects the Workspace controller
	// to run its initial plan/apply cycle using the branch name from the spec.
	//
	// If we patched the detected-revision annotation here, the Workspace
	// controller would pick up the SHA in its reset path and Step 8 would
	// record the SHA as observedRevision instead of the branch name. Worse,
	// the Rollout controller would see the annotation and treat the Workspace
	// as having pending work before it has even started its first cycle.
	//
	// Instead we silently record the resolved SHA as the baseline so that
	// subsequent polls have a real commit to compare against. The next time
	// the branch moves to a different commit the diff will be genuine and we
	// will patch the annotation normally.
	if lastSHA == "" {
		logger.Info("Seeding initial SHA for new workspace", "sha", newSHA)
		pollTotal.WithLabelValues(key.Namespace, key.Name, "seeded").Inc()
		r.registry.mu.Lock()
		if e, ok := r.registry.entries[key]; ok {
			e.lastSHA = newSHA
		}
		r.registry.mu.Unlock()
		r.reschedule(key, interval)
		return
	}

	// The remote ref resolved to a different SHA than what we last recorded.
	// Patch the detected-revision annotation so the Workspace controller sees
	// the divergence and starts a new plan/apply cycle.
	logger.Info("Detected new revision", "old", lastSHA, "new", newSHA)

	workspace := &v1alpha1.Workspace{}
	if err := r.Get(ctx, key, workspace); err != nil {
		logger.Error(err, "Failed to get workspace for patching")
		pollTotal.WithLabelValues(key.Namespace, key.Name, "error").Inc()
		r.reschedule(key, minDuration(interval, maxRetryBackoff))
		return
	}

	// Use MergeFrom so we only send the annotation diff, not the entire
	// object. This avoids conflicts with concurrent updates to unrelated
	// fields (e.g. the Workspace controller updating status).
	patch := client.MergeFrom(workspace.DeepCopy())
	if workspace.Annotations == nil {
		workspace.Annotations = map[string]string{}
	}
	workspace.Annotations[v1alpha1.WorkspaceDetectedRevisionAnnotation] = newSHA
	if err := r.Patch(ctx, workspace, patch); err != nil {
		logger.Error(err, "Failed to patch detected-revision annotation")
		pollTotal.WithLabelValues(key.Namespace, key.Name, "error").Inc()
		// Do not update lastSHA on patch failure — the next poll will detect
		// the same change and retry the patch.
		r.reschedule(key, minDuration(interval, maxRetryBackoff))
		return
	}

	pollTotal.WithLabelValues(key.Namespace, key.Name, "changed").Inc()

	// Record the new SHA only after a successful patch. If we updated lastSHA
	// before patching and the patch failed, we'd lose track of the change and
	// never retry.
	r.registry.mu.Lock()
	if e, ok := r.registry.entries[key]; ok {
		e.lastSHA = newSHA
	}
	r.registry.mu.Unlock()

	r.reschedule(key, interval)
}

// reschedule sets the entry's nextPollAt to now + interval and pushes a fresh
// heap entry so the scheduler picks it up on its next wake.
func (r *RefWatcherReconciler) reschedule(key types.NamespacedName, interval time.Duration) {
	now := time.Now()
	next := now.Add(interval)

	r.registry.mu.Lock()
	if e, ok := r.registry.entries[key]; ok {
		e.nextPollAt = next
	}
	r.registry.mu.Unlock()

	r.signalScheduler(key, next)
}

// resolveRef performs a git ls-remote against the given URL and resolves the
// named ref to a commit SHA. We use go-git's in-memory remote to avoid
// requiring a git binary in the container image and to avoid the overhead of
// spawning a child process on every poll.
//
// The ref is first matched as a branch (refs/heads/<ref>), then as a tag
// (refs/tags/<ref>), then as a fully qualified ref name (e.g.
// refs/pull/123/head). If none of those match and the ref looks like a
// 40-character hex string, we assume it is a pinned commit SHA and return it
// directly — the ls-remote already ran for the earlier candidates, but a bare
// SHA will not appear as a ref name. Finally, if the ref is empty or the
// literal string "HEAD", we resolve the remote's HEAD.
func resolveRef(ctx context.Context, url, ref string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, gitTimeout)
	defer cancel()

	remote := gogit.NewRemote(memory.NewStorage(), &config.RemoteConfig{
		Name: "origin",
		URLs: []string{url},
	})

	refs, err := remote.ListContext(ctx, &gogit.ListOptions{})
	if err != nil {
		return "", err
	}

	// Try branch, then tag, then the ref as a fully qualified name.
	candidates := []string{
		"refs/heads/" + ref,
		"refs/tags/" + ref,
		ref,
	}

	for _, candidate := range candidates {
		for _, r := range refs {
			if r.Name() == plumbing.ReferenceName(candidate) {
				return r.Hash().String(), nil
			}
		}
	}

	// A 40-character hex ref is likely a pinned commit SHA. It won't appear
	// as a ref name in ls-remote output, but we already have it.
	if len(ref) == 40 {
		return ref, nil
	}

	// Fall back to HEAD for empty or literal "HEAD" refs.
	if ref == "" || ref == "HEAD" {
		for _, r := range refs {
			if r.Name() == plumbing.HEAD {
				return r.Hash().String(), nil
			}
		}
	}

	return "", &RefNotFoundError{Ref: ref, URL: url}
}

// RefNotFoundError is returned when resolveRef cannot match the requested ref
// against any remote reference. This is typically a configuration error (typo
// in spec.source.targetRevision) rather than a transient failure.
type RefNotFoundError struct {
	Ref string
	URL string
}

func (e *RefNotFoundError) Error() string {
	return "ref " + e.Ref + " not found in remote " + e.URL
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

// SetupWithManager registers the RefWatcher controller and the scheduler
// Runnable with the manager. The controller watches Workspace resources and
// feeds the in-memory registry; the Runnable (Start) launches the scheduler
// goroutine and worker pool after leader election is won.
//
// No persistent state is needed. When a new leader starts, the informer
// re-syncs all Workspace resources, firing Reconcile for each one, which
// repopulates the registry from scratch. The jitter applied to fresh entries
// in upsertEntry prevents the resulting burst of polls from overwhelming the
// Git host.
func (r *RefWatcherReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.registry = Registry{
		entries: make(map[types.NamespacedName]*registryEntry),
	}
	r.workCh = make(chan types.NamespacedName, r.WorkQueueSize)
	r.updateCh = make(chan struct{}, 1)
	r.ph = make(pollHeap, 0)

	// Register as a manager Runnable so the scheduler and worker pool
	// lifecycle is tied to the manager's context. Because NeedLeaderElection
	// returns true, Start is only called on the elected leader.
	if err := mgr.Add(r); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Workspace{}).
		Named("refwatcher").
		Complete(r)
}

// NeedLeaderElection implements the LeaderElectionRunnable interface. The
// scheduler and worker pool must only run on the elected leader — running
// on multiple replicas would produce duplicate annotation patches and
// unnecessary load on the Git host.
func (r *RefWatcherReconciler) NeedLeaderElection() bool {
	return true
}
