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

package rollout

import (
	"context"

	"github.com/magosproject/magos/types/magosproject/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// RolloutReconciler reconciles a Rollout object
type RolloutReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=magosproject.io,resources=rollouts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=magosproject.io,resources=rollouts/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=magosproject.io,resources=rollouts/finalizers,verbs=update
// +kubebuilder:rbac:groups=magosproject.io,resources=workspaces,verbs=get;list;watch;update;patch

// Reconcile is the top-level entry point invoked by controller-runtime whenever
// a Rollout or one of its watched dependents (Workspaces) changes.
func (r *RolloutReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the Rollout instance
	rollout := &v1alpha1.Rollout{}
	if err := r.Get(ctx, req.NamespacedName, rollout); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get Rollout")
		return ctrl.Result{}, err
	}

	// If the Rollout is being deleted, there is nothing to clean up. Rollouts
	// do not own any external resources directly; they only set annotations on
	// Workspaces. Once the Rollout is gone, the Project controller falls back
	// to default parallel execution and re-grants execution permissions.
	if !rollout.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	err := r.reconcileRollout(ctx, rollout)
	if err != nil {
		reconcileTotal.WithLabelValues(req.Namespace, req.Name, "error").Inc()
		r.updateStatus(ctx, rollout, v1alpha1.PhaseFailed, "ReconcileError", err.Error(), metav1.ConditionFalse)
		return ctrl.Result{}, err
	}

	reconcileTotal.WithLabelValues(req.Namespace, req.Name, "success").Inc()
	return ctrl.Result{}, nil
}

func (r *RolloutReconciler) reconcileRollout(ctx context.Context, rollout *v1alpha1.Rollout) error {
	logger := log.FromContext(ctx)
	logger.V(1).Info("Reconciling Rollout", "name", rollout.Name)

	// Track active rollout count at the end of reconciliation based on phase.
	defer func() {
		// Count all rollouts in non-terminal phases across the namespace.
		var allRollouts v1alpha1.RolloutList
		if err := r.List(ctx, &allRollouts); err == nil {
			var active float64
			for _, ro := range allRollouts.Items {
				if ro.Status.Phase == v1alpha1.PhaseReconciling {
					active++
				}
			}
			activeCount.Set(active)
		}
	}()

	// If no steps are defined, there is nothing to orchestrate. Mark the
	// Rollout as Ready so the Project controller knows it is active but idle.
	if len(rollout.Spec.Strategy.Steps) == 0 {
		r.updateStatus(ctx, rollout, v1alpha1.PhaseReady, "NoSteps", "No steps defined in strategy", metav1.ConditionTrue)
		return nil
	}

	// Step 1: Compute execution levels.
	//
	// Steps are the user-facing unit of ordering, but execution happens at
	// the level of "levels". Consecutive steps that resolve to the exact
	// same set of Workspace UIDs are merged into a single level and their
	// Workspaces execute in parallel. A level boundary is created whenever
	// the resolved workspace set changes. This is effectively Kahn's
	// algorithm applied to a linear dependency chain: steps with identical
	// inputs have no data dependency between them and can safely run
	// concurrently, while a change in the workspace set implies a new
	// dependency and requires the previous level to complete first.
	//
	// For example, given the following steps:
	//   step 0 "deploy-dev"     env:dev  → {ws-dev, ws-staging}
	//   step 1 "deploy-staging" env:dev  → {ws-dev, ws-staging}
	//   step 2 "deploy-prod"    env:prod → {ws-prod}
	//
	// Steps 0 and 1 resolve to the same workspace set, so they collapse
	// into level 0. Step 2 resolves to a different set, forming level 1.
	// Level 0 runs ws-dev and ws-staging in parallel; level 1 only starts
	// after both have reached Applied.
	type level struct {
		startIdx   int
		endIdx     int
		names      string
		workspaces []v1alpha1.Workspace
		wsUIDs     map[types.UID]struct{}
	}

	var levels []level
	var prevUIDs map[types.UID]struct{}

	for i, step := range rollout.Spec.Strategy.Steps {
		// Convert the step's LabelSelector to a labels.Selector so we can
		// use it with the client.MatchingLabelsSelector list option.
		selector, err := metav1.LabelSelectorAsSelector(&step.Selector)
		if err != nil {
			logger.Error(err, "Invalid label selector in step", "step", step.Name)
			return err
		}

		// List all Workspaces in the same namespace that match the step's
		// label selector. This is a broad query; we filter by project below.
		var wsList v1alpha1.WorkspaceList
		if err := r.List(ctx, &wsList, client.InNamespace(rollout.Namespace), client.MatchingLabelsSelector{Selector: selector}); err != nil {
			logger.Error(err, "Failed to list workspaces for step", "step", step.Name)
			return err
		}

		// Filter Workspaces to only those belonging to this Rollout's
		// Project. The label selector alone might match Workspaces from
		// other Projects that happen to share the same labels.
		var target []v1alpha1.Workspace
		uids := make(map[types.UID]struct{})
		for _, ws := range wsList.Items {
			if ws.Spec.ProjectRef.Name == rollout.Spec.ProjectRef {
				target = append(target, ws)
				uids[ws.UID] = struct{}{}
			}
		}

		// If no Workspaces match this step's selector, the Rollout cannot
		// make progress. This is treated as a failure to prevent the Rollout
		// from silently skipping an environment. Common causes include a
		// Workspace YAML that failed to apply (e.g. a typo), misconfigured
		// labels, or a Workspace that was deleted. The Rollout will
		// automatically recover once matching Workspaces appear, since any
		// Workspace change triggers re-evaluation via the watch.
		if len(target) == 0 {
			rollout.Status.CurrentStep = i
			r.updateStatus(ctx, rollout, v1alpha1.PhaseFailed, "NoWorkspacesFound",
				"No workspaces found matching selector for step "+step.Name+"; halting to prevent skipping environments", metav1.ConditionFalse)
			return nil
		}

		// Merge into the current level if the resolved workspace set is
		// identical to the previous step's set. Otherwise start a new level.
		sameSet := len(uids) == len(prevUIDs)
		if sameSet {
			for uid := range uids {
				if _, ok := prevUIDs[uid]; !ok {
					sameSet = false
					break
				}
			}
		}

		if sameSet && len(levels) > 0 {
			cur := &levels[len(levels)-1]
			cur.endIdx = i
			cur.names += ", " + step.Name
		} else {
			levels = append(levels, level{
				startIdx:   i,
				endIdx:     i,
				names:      step.Name,
				workspaces: target,
				wsUIDs:     uids,
			})
		}
		prevUIDs = uids
	}

	// Step 2: Find the earliest level that still has pending work.
	//
	// Rather than trusting the persisted CurrentStep index, we always scan
	// levels from the beginning. This handles forward progress (the first
	// incomplete level is the next one to execute), rewinds (an earlier
	// level gained new work from a RefWatcher commit or manual reconcile
	// request), and stale state (CurrentStep carried a value from a
	// previous controller version or a completed rollout cycle). If no
	// level has pending work, the rollout has completed.
	var currentLevel *level
	var currentLevelIdx int
	for li := range levels {
		lvl := &levels[li]
		for _, ws := range lvl.workspaces {
			isFullyApplied := workspaceFullyApplied(&ws)
			hasReconcileRequest := ws.Annotations != nil && ws.Annotations[v1alpha1.WorkspaceReconcileRequestAnnotation] != ""
			if !isFullyApplied || hasReconcileRequest || ws.Status.Phase == "" || ws.Status.Phase == v1alpha1.PhasePending {
				currentLevel = lvl
				currentLevelIdx = li
				break
			}
		}
		if currentLevel != nil {
			break
		}
	}
	// If no level has pending work, all steps have completed successfully.
	if currentLevel == nil {
		rollout.Status.CurrentStep = len(rollout.Spec.Strategy.Steps)
		r.updateStatus(ctx, rollout, v1alpha1.PhaseApplied, "RolloutCompleted", "All steps completed successfully", metav1.ConditionTrue)
		return nil
	}

	// Update CurrentStep to reflect the level we are actually evaluating.
	// This may differ from the stored value if we rewound to an earlier
	// level.
	previousStep := rollout.Status.CurrentStep
	rollout.Status.CurrentStep = currentLevel.startIdx

	currentLevelMetric.WithLabelValues(rollout.Namespace, rollout.Name).Set(float64(currentLevelIdx))
	if currentLevel.startIdx != previousStep {
		levelTransitionsTotal.WithLabelValues(rollout.Namespace, rollout.Name).Inc()
	}

	// Step 3: Check for failures in the current level.
	//
	// A single failed Workspace halts the entire Rollout. This is a
	// deliberate safety mechanism: applying later steps on top of a broken
	// earlier step could compound failures.
	for _, ws := range currentLevel.workspaces {
		if ws.Status.Phase == v1alpha1.PhaseFailed {
			logger.Info("Workspace failed, halting rollout", "workspace", ws.Name)
			r.updateStatus(ctx, rollout, v1alpha1.PhaseFailed, "StepFailed",
				"One or more workspaces failed in level ["+currentLevel.names+"]", metav1.ConditionFalse)
			return nil
		}
	}

	// Step 4: Grant execution permission to Workspaces in the current level.
	//
	// The Workspace controller checks for WorkspaceExecutionAllowedAnnotation
	// before starting a plan/apply cycle. Without it, the Workspace stays in
	// Pending. We only grant permission to Workspaces that actually need work
	// to avoid unnecessary API updates and redundant reconciles.
	for _, ws := range currentLevel.workspaces {
		isFullyApplied := workspaceFullyApplied(&ws)
		hasReconcileRequest := ws.Annotations != nil && ws.Annotations[v1alpha1.WorkspaceReconcileRequestAnnotation] != ""

		if !isFullyApplied || hasReconcileRequest || ws.Status.Phase == "" || ws.Status.Phase == v1alpha1.PhasePending {
			hasPermission := ws.Annotations != nil && ws.Annotations[v1alpha1.WorkspaceExecutionAllowedAnnotation] == v1alpha1.AnnotationValueTrue
			if !hasPermission {
				logger.Info("Granting execution permission to workspace", "workspace", ws.Name, "level", currentLevel.names)

				// Fetch the latest version of the Workspace before updating to
				// avoid concurrency conflicts. Workspaces frequently update their
				// own status, which increments the ResourceVersion rapidly. Using
				// a stale version would cause the Update to fail with a conflict
				// error.
				latestWS := &v1alpha1.Workspace{}
				if err := r.Get(ctx, client.ObjectKeyFromObject(&ws), latestWS); err == nil {
					if latestWS.Annotations == nil {
						latestWS.Annotations = make(map[string]string)
					}
					latestWS.Annotations[v1alpha1.WorkspaceExecutionAllowedAnnotation] = v1alpha1.AnnotationValueTrue
					if err := r.Update(ctx, latestWS); err != nil {
						logger.Error(err, "Failed to grant execution permission", "workspace", ws.Name)
					}
				}
			}
		}
	}

	// Step 5: Revoke execution permission from Workspaces in later levels.
	//
	// Without this, Workspaces that were granted permission in a previous
	// rollout cycle would retain the annotation and run out of order on
	// the next cycle. We skip Workspaces that also appear in the active
	// level to prevent a grant/revoke fight when selectors overlap.
	for li := currentLevelIdx + 1; li < len(levels); li++ {
		for _, ws := range levels[li].workspaces {
			if _, ok := currentLevel.wsUIDs[ws.UID]; ok {
				continue
			}
			if ws.Annotations != nil && ws.Annotations[v1alpha1.WorkspaceExecutionAllowedAnnotation] == v1alpha1.AnnotationValueTrue {
				logger.Info("Revoking execution permission from workspace in later level", "workspace", ws.Name, "level", levels[li].names)
				latestWS := &v1alpha1.Workspace{}
				if err := r.Get(ctx, client.ObjectKeyFromObject(&ws), latestWS); err == nil {
					delete(latestWS.Annotations, v1alpha1.WorkspaceExecutionAllowedAnnotation)
					if err := r.Update(ctx, latestWS); err != nil {
						logger.Error(err, "Failed to revoke execution permission", "workspace", ws.Name)
					}
				}
			}
		}
	}

	r.updateStatus(ctx, rollout, v1alpha1.PhaseReconciling, "LevelProgressing",
		"Executing level ["+currentLevel.names+"]", metav1.ConditionUnknown)
	return nil
}

// updateStatus writes phase, reason, message, and a Ready condition to the
// Rollout's status subresource. It always re-fetches the latest version of the
// Rollout before writing to avoid conflict errors caused by concurrent status
// updates. After a successful write, the caller's rollout object is updated
// in-place so subsequent logic in the same reconcile pass sees the fresh
// resourceVersion and status.
func (r *RolloutReconciler) updateStatus(ctx context.Context, rollout *v1alpha1.Rollout, phase v1alpha1.Phase, reason, message string, status metav1.ConditionStatus) {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// Fetch the latest version of the rollout to avoid conflict errors
		latest := &v1alpha1.Rollout{}
		if err := r.Get(ctx, client.ObjectKeyFromObject(rollout), latest); err != nil {
			return err
		}

		needsUpdate := false

		if latest.Status.Phase != phase || latest.Status.Reason != reason || latest.Status.Message != message {
			latest.Status.Phase = phase
			latest.Status.Reason = reason
			latest.Status.Message = message
			needsUpdate = true
		}

		// Sync the currentStep from the caller's rollout object, which may have
		// been updated during reconcileRollout before this function was called.
		if latest.Status.CurrentStep != rollout.Status.CurrentStep {
			latest.Status.CurrentStep = rollout.Status.CurrentStep
			needsUpdate = true
		}

		now := metav1.Now()
		condition := metav1.Condition{
			Type:               v1alpha1.ConditionTypeReady,
			Status:             status,
			Reason:             reason,
			Message:            message,
			LastTransitionTime: now,
		}

		if meta.SetStatusCondition(&latest.Status.Conditions, condition) {
			needsUpdate = true
		}

		if !needsUpdate {
			return nil
		}

		latest.Status.LastReconcileTime = &now

		if err := r.Status().Update(ctx, latest); err != nil {
			return err
		}

		// Update the original object so the caller has the latest state
		rollout.Status = latest.Status
		rollout.ResourceVersion = latest.ResourceVersion
		return nil
	})

	if err != nil {
		log.FromContext(ctx).Error(err, "Failed to update rollout status")
	}
}

// findRolloutsForWorkspace maps Workspace watch events to Rollout reconcile
// requests.
//
// Rollouts orchestrate Workspaces that belong to a specific Project. When a
// Workspace changes (e.g. its status phase transitions from Planning to
// Applied), the Rollout controller needs to re-evaluate whether the current
// step has completed and whether to advance to the next step. This mapper finds
// all Rollouts in the same namespace whose projectRef matches the Workspace's
// projectRef, and enqueues them for reconciliation.
func (r *RolloutReconciler) findRolloutsForWorkspace(ctx context.Context, o client.Object) []reconcile.Request {
	ws, ok := o.(*v1alpha1.Workspace)
	if !ok {
		return nil
	}

	var rollouts v1alpha1.RolloutList
	if err := r.List(ctx, &rollouts, client.InNamespace(ws.Namespace)); err != nil {
		log.FromContext(ctx).Error(err, "Failed to list rollouts for workspace change")
		return nil
	}

	// For each Rollout in the same namespace, if its projectRef matches the
	// Workspace's projectRef, enqueue a reconcile request for that Rollout.
	var requests []reconcile.Request
	for _, ro := range rollouts.Items {
		if ro.Spec.ProjectRef == ws.Spec.ProjectRef.Name {
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      ro.Name,
					Namespace: ro.Namespace,
				},
			})
		}
	}
	return requests
}

// workspaceFullyApplied returns true when a Workspace has completed its
// plan/apply cycle and there is no newer revision waiting to be applied.
//
// This function is the Rollout controller's primary signal for deciding
// whether a level has finished. It checks two things:
//
//  1. The Workspace must be in PhaseApplied, meaning its most recent apply
//     Job succeeded.
//  2. The detected-revision annotation must be absent. The RefWatcher sets
//     this annotation when it discovers that a branch or tag now points to
//     a new commit. The Workspace controller preserves the annotation
//     throughout the plan/apply cycle and only deletes it in Step 8 after
//     recording the commit SHA in status.observedRevision. So the presence
//     of detected-revision means either the Workspace hasn't started
//     processing the new commit yet, or it is still mid-cycle.
//
// The ordering of writes in the Workspace controller matters here. When
// the Workspace controller resets for a new commit, it writes PhasePending
// to the status subresource before clearing any annotations. This ensures
// we never observe a transient state of PhaseApplied + no annotation, which
// would cause us to incorrectly report the Workspace as fully applied and
// advance the Rollout to the next level.
func workspaceFullyApplied(ws *v1alpha1.Workspace) bool {
	if ws.Status.Phase != v1alpha1.PhaseApplied {
		return false
	}
	if ws.Annotations != nil {
		if _, ok := ws.Annotations[v1alpha1.WorkspaceDetectedRevisionAnnotation]; ok {
			return false
		}
	}
	return true
}

// SetupWithManager registers the Rollout controller with the Manager and
// configures the watches that trigger reconciliation.
func (r *RolloutReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Rollout{}).
		Watches( // Watch for changes to Workspaces so we can re-evaluate step completion
			&v1alpha1.Workspace{},
			handler.EnqueueRequestsFromMapFunc(r.findRolloutsForWorkspace),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		).
		Named("rollout").
		Complete(r)
}
