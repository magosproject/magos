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

	"github.com/magosproject/magos/types/magosproject/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// reconcileApplyJob handles Steps 7 and 8 of reconcileWorkspace.
//
// Step 7: Run "terraform apply" (requires approval).
// The Plan succeeded so the .tfplan file is available on the PVC. Before we
// create the Apply Job we need to verify that someone actually approved it.
// There are two ways approval can happen: the Workspace has spec.autoApply
// set to true, which means every successful plan is applied automatically,
// or someone (a human or an external system) set the ApprovedAnnotation on
// the Workspace to "true" to explicitly approve this specific plan.
//
// If neither of those is the case we park the Workspace in the Planned
// phase and wait. Once approval comes in, the annotation change triggers a
// new reconcile and we pick up here again.
//
// Step 8: The Apply succeeded. Record the result and release the execution lock.
func (r *WorkspaceReconciler) reconcileApplyJob(
	ctx context.Context,
	workspace *v1alpha1.Workspace,
	rc runContext,
) error {
	logger := log.FromContext(ctx)

	if rc.applyJobErr != nil {
		if errors.IsNotFound(rc.applyJobErr) {
			return r.createApplyJobIfApproved(ctx, workspace, rc)
		}
		return rc.applyJobErr
	}

	if rc.applyJob.Status.Failed > 0 {
		logger.Info("Apply Job failed", "job", rc.applyJobName)
		if err := r.archiveRunLogs(ctx, workspace, &rc.applyJob, v1alpha1.RunPhaseApply, v1alpha1.RunLogResultFailed); err != nil {
			logger.Error(err, "Failed to archive apply logs", "job", rc.applyJobName)
		}
		r.updateStatus(ctx, workspace, v1alpha1.PhaseFailed, "ApplyFailed", "Terraform Apply execution failed", metav1.ConditionFalse)

		// Same as the plan failure path in Step 6: release the execution lock
		// so the Rollout controller can continue with the next Workspace.
		//
		// If this run was triggered by a manual reconcile request, consume that
		// annotation too so retries follow the sync interval backoff rather than
		// re-triggering immediately.
		if err := r.deleteAnnotations(ctx, workspace,
			v1alpha1.WorkspaceExecutionAllowedAnnotation,
			v1alpha1.WorkspaceReconcileRequestAnnotation,
		); err != nil {
			logger.Error(err, "Failed to consume execution annotations via Patch on failure")
			return err
		}
		return nil
	}

	if rc.applyJob.Status.Succeeded == 0 {
		logger.Info("Apply Job is currently running", "job", rc.applyJobName)
		r.updateStatus(ctx, workspace, v1alpha1.PhaseApplying, "Applying", "Terraform Apply execution is running", metav1.ConditionUnknown)
		return nil
	}

	return r.handleApplySuccess(ctx, workspace, rc)
}

// createApplyJobIfApproved checks for plan approval (autoApply or annotation)
// and creates the Apply Job if approved. Parks the Workspace in Planned
// phase otherwise and waits for an approval annotation to trigger a new reconcile.
//
// When we do have approval we remove the annotation before creating the
// Job. This is important because annotations persist across reconciles. If
// we left it in place and the spec changed later (producing a new plan),
// that stale "approved" annotation would cause the new plan to be applied
// without anyone actually reviewing it.
func (r *WorkspaceReconciler) createApplyJobIfApproved(
	ctx context.Context,
	workspace *v1alpha1.Workspace,
	rc runContext,
) error {
	logger := log.FromContext(ctx)

	isApproved := workspace.Spec.AutoApply
	if workspace.Annotations != nil && workspace.Annotations[v1alpha1.WorkspaceApprovedAnnotation] == v1alpha1.AnnotationValueTrue {
		isApproved = true
	}

	if !isApproved {
		logger.Info("Workspace has planned successfully, but is pending approval to apply", "workspace", workspace.Name)
		if workspace.Status.Phase != v1alpha1.PhasePlanned {
			if err := r.archiveRunLogs(ctx, workspace, &rc.planJob, v1alpha1.RunPhasePlan, v1alpha1.RunLogResultSucceeded); err != nil {
				logger.Error(err, "Failed to archive plan logs", "job", rc.planJobName)
			}
		}
		r.updateStatus(ctx, workspace, v1alpha1.PhasePlanned, "PlanSucceeded", "Terraform Plan succeeded. Waiting for manual approval to Apply.", metav1.ConditionTrue)
		return nil
	}

	// Remove the approval annotation before creating the Job. See the
	// comment above for why leaving it around would be dangerous.
	if workspace.Annotations != nil && workspace.Annotations[v1alpha1.WorkspaceApprovedAnnotation] != "" {
		patch := client.MergeFrom(workspace.DeepCopy())
		delete(workspace.Annotations, v1alpha1.WorkspaceApprovedAnnotation)
		if err := r.Patch(ctx, workspace, patch); err != nil {
			logger.Error(err, "Failed to consume approval annotation via Patch")
			return err
		}
	}

	// Archive the completed plan logs before moving on to apply so
	// both phases end up in the same reconcile run record.
	if err := r.archiveRunLogs(ctx, workspace, &rc.planJob, v1alpha1.RunPhasePlan, v1alpha1.RunLogResultSucceeded); err != nil {
		logger.Error(err, "Failed to archive plan logs", "job", rc.planJobName)
	}

	logger.Info("Creating a new Apply Job", "job", rc.applyJobName)
	runID := ensureRunMetadata(workspace)
	newJob, err := r.constructJobForWorkspace(ctx, workspace, rc, jobTypeApply, runID)
	if err != nil {
		return err
	}
	if err := r.Create(ctx, newJob); err != nil {
		return err
	}
	r.updateStatus(ctx, workspace, v1alpha1.PhaseApplying, "ApplyJobCreated", "Terraform Apply job created", metav1.ConditionUnknown)
	return nil
}

// handleApplySuccess handles Step 8: recording the apply result, stamping the
// observed revision, and releasing the execution lock.
//
// This is the final step in the Workspace lifecycle and the point where the
// detected-revision annotation is consumed. The annotation flows through
// three controllers. The RefWatcher writes it with the commit SHA when it
// discovers that a branch or tag moved. Step 3 sees the annotation, resets
// the Workspace to Pending, and intentionally preserves the annotation so
// the SHA survives the plan and apply run. Here in Step 8 we read the SHA
// from the annotation and record it as status.observedRevision, then delete
// the annotation so it does not trigger another reset. The Rollout
// controller's workspaceFullyApplied() also checks for the absence of this
// annotation, so deleting it signals that the Workspace has fully processed
// the new commit.
//
// If the RefWatcher did not trigger this cycle (e.g. periodic drift
// detection or a manual reconcile request), the annotation will not be
// present and we fall back to spec.source.targetRevision (the branch or tag
// name).
//
// After recording the revision we remove the execution-allowed,
// detected-revision, and any manual reconcile-request annotations to hand
// control back to the Rollout controller, completing this Workspace's turn
// in the rollout sequence.
func (r *WorkspaceReconciler) handleApplySuccess(
	ctx context.Context,
	workspace *v1alpha1.Workspace,
	rc runContext,
) error {
	logger := log.FromContext(ctx)
	logger.Info("Apply Job completed successfully", "job", rc.applyJobName)

	if err := r.archiveRunLogs(ctx, workspace, &rc.applyJob, v1alpha1.RunPhaseApply, v1alpha1.RunLogResultSucceeded); err != nil {
		logger.Error(err, "Failed to archive apply logs", "job", rc.applyJobName)
	}

	// Record apply job duration if both start and completion times are
	// available.
	if rc.applyJob.Status.StartTime != nil && rc.applyJob.Status.CompletionTime != nil {
		duration := rc.applyJob.Status.CompletionTime.Time.Sub(rc.applyJob.Status.StartTime.Time).Seconds()
		jobDurationSeconds.WithLabelValues(workspace.Namespace, workspace.Name, jobTypeApply).Observe(duration)
	}

	// Record the observed revision before the status update so it is included
	// in the same write. When the RefWatcher triggered this cycle the
	// detected-revision annotation carries the full 40 character commit SHA.
	// Otherwise we fall back to the branch or tag name from the spec.
	if workspace.Annotations != nil {
		if sha := workspace.Annotations[v1alpha1.WorkspaceDetectedRevisionAnnotation]; sha != "" {
			workspace.Status.ObservedRevision = sha
		} else {
			workspace.Status.ObservedRevision = workspace.Spec.Source.TargetRevision
		}
	} else {
		workspace.Status.ObservedRevision = workspace.Spec.Source.TargetRevision
	}
	r.updateStatus(ctx, workspace, v1alpha1.PhaseApplied, "ApplySucceeded", "Terraform Apply completed successfully", metav1.ConditionTrue)

	// Remove the execution-allowed, detected-revision, and any manual
	// reconcile-request annotations now that the cycle is complete. We use
	// Patch rather than Update because the status update above may have bumped
	// the resourceVersion, and a full Update would conflict. Deleting
	// execution-allowed tells the Rollout controller that this Workspace is
	// done with its turn. Deleting detected-revision tells both the Rollout
	// controller (via workspaceFullyApplied) and Step 3 (via the phase guarded
	// reset check) that the new commit has been fully processed. Deleting
	// reconcile-request consumes a one-shot manual trigger so it does not
	// immediately start another cycle.
	if err := r.deleteAnnotations(ctx, workspace,
		v1alpha1.WorkspaceExecutionAllowedAnnotation,
		v1alpha1.WorkspaceDetectedRevisionAnnotation,
		v1alpha1.WorkspaceReconcileRequestAnnotation,
	); err != nil {
		logger.Error(err, "Failed to consume execution annotations via Patch")
		return err
	}

	return nil
}
