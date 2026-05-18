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

package workspace

import (
	"context"
	"fmt"
	"time"

	"github.com/magosproject/magos/types/magosproject/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// cycleDecision communicates the outcome of checkCycleNeeded.
// When start is true the caller must invoke startFreshCycle.
// When start is false and requeue > 0 the caller should requeue
// after that duration rather than at the default scheduled interval.
type cycleDecision struct {
	start   bool
	reason  string
	message string
	requeue time.Duration
}

// cleanupOrphanedJobs deletes Jobs owned by this Workspace whose names no
// longer match the current specHash. We skip cleanup while a Job is actively
// running (Planning or Applying) because deleting a mid-flight terraform apply
// could corrupt Terraform state. The next reconcile after the Job finishes will
// clean up any leftover Jobs from the previous spec.
//
// See Step 2 in reconcileWorkspace for the full rationale.
func (r *WorkspaceReconciler) cleanupOrphanedJobs(ctx context.Context, workspace *v1alpha1.Workspace, rc runContext) {
	if workspace.Status.Phase == v1alpha1.PhasePlanning || workspace.Status.Phase == v1alpha1.PhaseApplying {
		return
	}

	logger := log.FromContext(ctx)
	var childJobs batchv1.JobList
	if err := r.List(ctx, &childJobs, client.InNamespace(workspace.Namespace)); err != nil {
		return
	}
	for _, j := range childJobs.Items {
		isOwned := false
		for _, owner := range j.OwnerReferences {
			if owner.UID == workspace.UID {
				isOwned = true
				break
			}
		}
		// Delete Jobs that belong to this Workspace but were created
		// for an older specHash.
		if isOwned && j.Name != rc.planJobName && j.Name != rc.applyJobName {
			logger.Info("Cleaning up orphaned job from previous run", "job", j.Name)
			_ = r.Delete(ctx, &j, client.PropagationPolicy(metav1.DeletePropagationBackground))
		}
	}
}

// jobFinishedState returns the finish time and whether the job succeeded.
// Returns zero time when the job has not finished or does not exist.
func jobFinishedState(job *batchv1.Job) (time.Time, bool) {
	if job == nil {
		return time.Time{}, false
	}
	if job.Status.CompletionTime != nil {
		return job.Status.CompletionTime.Time, true
	}
	if job.Status.Failed > 0 {
		for _, cond := range job.Status.Conditions {
			if cond.Type == batchv1.JobFailed && cond.Status == corev1.ConditionTrue {
				return cond.LastTransitionTime.Time, false
			}
		}
	}
	return time.Time{}, false
}

// checkCycleNeeded inspects the current state of the Workspace and its Jobs and
// decides whether a fresh plan/apply cycle should start.
//
// It covers four scenarios (see Step 3 in reconcileWorkspace for the full
// reasoning behind each one):
//
//  1. Both Jobs are missing – the spec changed or they were manually deleted.
//  2. The Apply (or Plan) Job finished and the scheduled sync interval has elapsed.
//  3. The RefWatcher detected a new commit on the tracked branch or tag.
//  4. A manual reconcile request annotation was set on the Workspace.
//
// When no reset is needed but the caller should wait for the next scheduled
// reconcile, exactRequeue is set to the remaining duration.
//
//nolint:gocyclo // Intentional orchestration logic kept in one place for readability.
func (r *WorkspaceReconciler) checkCycleNeeded(
	workspace *v1alpha1.Workspace,
	rc runContext,
) cycleDecision {
	nextScheduled, _, scheduledDue := computeNextReconcileTime(workspace, workspace.Status.NextReconcileTime)

	// Both Job names are derived from the current specHash. A nil on both
	// means either the spec changed (new hash → new names → old Jobs no longer
	// exist) or Jobs were manually deleted. In either case we reset, with one
	// exception: if the phase is Planning or Applying a Job from the previous
	// specHash may still be running. Deleting it mid-flight could corrupt
	// Terraform state, so we leave it and let the next reconcile handle cleanup
	// once it finishes.
	if rc.planJob == nil && rc.applyJob == nil {
		if workspace.Status.Phase != "" && workspace.Status.Phase != v1alpha1.PhasePending &&
			workspace.Status.Phase != v1alpha1.PhasePlanning && workspace.Status.Phase != v1alpha1.PhaseApplying {
			return cycleDecision{
				start:   true,
				reason:  "ConfigurationChanged",
				message: "Workspace spec was modified or jobs were deleted, triggering fresh execution",
			}
		}
	}

	var requeue time.Duration

	// If the Apply Job already finished and the Workspace has already recorded
	// that terminal result, we wait for the sync interval to elapse before
	// starting the next run. A just-finished Job can be observed before the
	// Workspace status has moved from Applying to Applied/Failed; in that case
	// we deliberately skip reset here so the handler can archive logs and
	// record the terminal status first.
	applyFinished, applySucceeded := jobFinishedState(rc.applyJob)
	if !applyFinished.IsZero() && terminalWorkspaceRunRecorded(workspace.Status.Phase) {
		if scheduledDue {
			if applySucceeded {
				return cycleDecision{start: true, reason: scheduledReconcileReason, message: "Starting scheduled reconciliation"}
			}
			return cycleDecision{start: true, reason: "RetryApply", message: "Retrying failed apply starting from new plan"}
		}
		requeue = time.Until(nextScheduled.Time)
	} else if rc.planJob != nil && rc.planJob.Status.Failed > 0 && terminalWorkspaceRunRecorded(workspace.Status.Phase) {
		// We never got to Apply because the Plan itself failed. We use the same
		// sync-interval cooldown here to avoid hammering a plan that keeps
		// failing (e.g. bad credentials, broken HCL) on every reconcile loop.
		// As with finished Apply Jobs above, reset is only allowed after the
		// Workspace has already recorded the failed terminal status, because the
		// first reconcile that observes the failed Job still needs to archive logs.
		var failedTime time.Time
		for _, cond := range rc.planJob.Status.Conditions {
			if cond.Type == batchv1.JobFailed && cond.Status == corev1.ConditionTrue {
				failedTime = cond.LastTransitionTime.Time
				break
			}
		}
		if !failedTime.IsZero() {
			if scheduledDue {
				return cycleDecision{start: true, reason: "RetryPlan", message: "Retrying failed plan"}
			}
			requeue = time.Until(nextScheduled.Time)
		}
	}

	// The RefWatcher controller runs in a separate goroutine and polls Git
	// remotes on a configurable interval. When it discovers that a branch or
	// tag (e.g. "main") now points to a different commit, it patches the
	// detected-revision annotation on the Workspace with the new SHA. That
	// annotation is the handoff signal between the two controllers: the
	// RefWatcher writes it, and we consume it here by starting a fresh
	// plan and apply run immediately rather than waiting for the sync interval.
	//
	// The phase guard below is critical. The reset path deliberately preserves
	// the detected-revision annotation so that Step 8 can read the exact commit
	// SHA and record it as status.observedRevision after a successful apply. If
	// we allowed the check to fire from in-progress phases (Pending, Planning,
	// Planned, Applying) the annotation would trigger a reset on every
	// reconcile, creating an infinite loop because it is never cleared until
	// Step 8. By restricting to recorded terminal phases and the initial empty
	// phase, we guarantee the reset fires exactly once per new commit: the
	// Workspace resets, progresses through its plan and apply run, and only then
	// is the annotation consumed.
	if workspace.Annotations != nil && (workspace.Status.Phase == "" || terminalWorkspaceRunRecorded(workspace.Status.Phase)) {
		if detected, ok := workspace.Annotations[v1alpha1.WorkspaceDetectedRevisionAnnotation]; ok && detected != workspace.Status.ObservedRevision {
			return cycleDecision{
				start:   true,
				reason:  "NewRevisionDetected",
				message: fmt.Sprintf("RefWatcher detected new revision %s", detected),
			}
		}
	}

	// Manual reconcile requests behave similarly, but unlike
	// detected-revision they do not carry data that must survive the cycle.
	// The same phase guard ensures the request triggers exactly one fresh
	// plan and apply run and does not keep resetting an in-progress run.
	if workspace.Annotations != nil && (workspace.Status.Phase == "" || terminalWorkspaceRunRecorded(workspace.Status.Phase)) {
		if req := workspace.Annotations[v1alpha1.WorkspaceReconcileRequestAnnotation]; req != "" {
			return cycleDecision{
				start:   true,
				reason:  "ManualReconcileRequested",
				message: fmt.Sprintf("Manual reconcile requested at %s", req),
			}
		}
	}

	// A change in the effective VariableSet composition is treated like a
	// new revision detection: the resolved hash differs from the last hash
	// we stamped, so the previous plan no longer represents what we should
	// apply. The same phase guard applies, so a Secret rotation that lands
	// while a Plan or Apply Job is already in flight does not interrupt
	// the run; the next reconcile after that run terminates picks it up.
	if rc.variablesHash != workspace.Status.VariablesHash &&
		(workspace.Status.Phase == "" || terminalWorkspaceRunRecorded(workspace.Status.Phase)) {
		return cycleDecision{
			start:   true,
			reason:  "VariablesChanged",
			message: "Effective VariableSet composition changed since the last run",
		}
	}

	return cycleDecision{requeue: requeue}
}

// startFreshCycle deletes the current plan and apply jobs, stamps a fresh run ID
// and trigger, and resets the Workspace phase to Pending. It must only be
// called when checkCycleNeeded returns a decision with start==true.
//
// See Step 3 in reconcileWorkspace for the full ordering rationale – in
// particular why Pending must be written before annotations are cleared.
func (r *WorkspaceReconciler) startFreshCycle(
	ctx context.Context,
	workspace *v1alpha1.Workspace,
	rc runContext,
	reason, message string,
) error {
	logger := log.FromContext(ctx)
	logger.Info("Cleaning up jobs to trigger a fresh run.", "reason", reason)

	if rc.planJob != nil {
		_ = r.Delete(ctx, rc.planJob, client.PropagationPolicy(metav1.DeletePropagationBackground))
	}
	if rc.applyJob != nil {
		_ = r.Delete(ctx, rc.applyJob, client.PropagationPolicy(metav1.DeletePropagationBackground))
	}

	// The status update to Pending must happen before we clear any
	// annotations. The Rollout controller watches Workspace objects and
	// uses workspaceFullyApplied() to decide whether a level has completed.
	// That function returns true when phase is Applied and the
	// detected-revision annotation is absent. If we cleared annotations
	// first, there would be a brief window where the Workspace is still in
	// PhaseApplied but has no detected-revision. The Rollout would observe
	// that state, conclude the Workspace is done, and advance to the next
	// level, granting execution permission to later Workspaces (e.g. prod)
	// before earlier ones (e.g. dev) have even started their new cycle.
	// Writing Pending first closes that window: the Rollout sees
	// PhasePending and knows the Workspace still has work to do.
	// Stamp a fresh run ID and trigger reason before writing Pending.
	// Both values are shared across the plan job and the apply job that
	// follows it so the log store can group both into a single run record
	// and report what originally caused this plan and apply run to start.
	workspace.Status.CurrentRunID = newRunID()
	workspace.Status.CurrentRunTrigger = runTriggerFromReason(reason)
	now := metav1.Now()
	workspace.Status.LastRunStartedAt = &now
	// Stamp the hash that this run is about to be planned against.
	// Persisting it here prevents a failed plan from looping on
	// VariablesChanged: the hash matches what we last attempted, so the
	// run waits for the normal retry cooldown instead.
	workspace.Status.VariablesHash = rc.variablesHash
	r.updateStatus(ctx, workspace, v1alpha1.PhasePending, reason, message, metav1.ConditionUnknown)

	if reason == scheduledReconcileReason && r.RunRecorder != nil {
		run := v1alpha1.Run{
			ID:          workspace.Status.CurrentRunID,
			Trigger:     workspace.Status.CurrentRunTrigger,
			ScheduledAt: workspace.Status.NextReconcileTime,
		}
		if err := r.RunRecorder.RecordRun(ctx, workspace.Namespace, workspace.Name, run); err != nil {
			logger.Error(err, "failed to record scheduled run")
		}
	}

	// Clear execution-allowed so the Rollout controller must re-grant
	// permission before this Workspace can proceed. The Rollout decides
	// when each Workspace runs based on level ordering; without clearing
	// this annotation the Workspace would skip the gate in Step 4 and start
	// planning immediately, ignoring the rollout sequence.
	//
	// We intentionally keep detected-revision alive through the cycle. The
	// RefWatcher wrote the exact commit SHA into that annotation, and Step
	// 8 reads it after a successful apply to populate
	// status.observedRevision with the SHA (e.g. "a1b2c3d") instead of the
	// branch name (e.g. "main"). Clearing it here would discard the SHA and
	// Step 8 would fall back to spec.source.targetRevision. The phase guard
	// on the detected-revision check above prevents the annotation from
	// re-triggering a reset while the Workspace progresses through Pending,
	// Planning, and Applying.
	if err := r.deleteAnnotations(ctx, workspace, v1alpha1.WorkspaceExecutionAllowedAnnotation); err != nil {
		return err
	}
	return nil
}

// isExecutionAllowedByRolloutController returns true when the Rollout controller has granted
// this Workspace permission to run its plan and apply cycle.
func isExecutionAllowedByRolloutController(workspace *v1alpha1.Workspace) bool {
	if workspace.Annotations == nil {
		return false
	}
	return workspace.Annotations[v1alpha1.WorkspaceExecutionAllowedAnnotation] == v1alpha1.AnnotationValueTrue
}

// deleteAnnotations removes the given annotation keys from the workspace via a
// server-side Patch. Using Patch rather than Update avoids resourceVersion
// conflicts that can arise when a status write has already bumped the version
// in the same reconcile loop.
func (r *WorkspaceReconciler) deleteAnnotations(ctx context.Context, workspace *v1alpha1.Workspace, keys ...string) error {
	if workspace.Annotations == nil {
		return nil
	}
	patch := client.MergeFrom(workspace.DeepCopy())
	changed := false
	for _, key := range keys {
		if workspace.Annotations[key] != "" {
			delete(workspace.Annotations, key)
			changed = true
		}
	}
	if !changed {
		return nil
	}
	return r.Patch(ctx, workspace, patch)
}
