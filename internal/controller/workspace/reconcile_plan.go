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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// reconcilePlanJob handles Step 6 of reconcileWorkspace: creating the Plan Job
// when it does not exist yet and observing the outcome of an existing one.
//
// Returns (result, planSucceeded, error):
//   - error != nil: something went wrong, stop reconciling.
//   - planSucceeded == false: job is running, failed, waiting on an older job,
//     or was just created. result may request a requeue.
//   - planSucceeded == true: the plan finished successfully, the caller may
//     proceed to Step 7.
func (r *WorkspaceReconciler) reconcilePlanJob(
	ctx context.Context,
	workspace *v1alpha1.Workspace,
	rc runContext,
) (ctrl.Result, bool, error) {
	logger := log.FromContext(ctx)

	// If the Plan Job doesn't exist yet we create it. If it already exists we
	// look at its status. A still running Job means we return early and wait
	// for the next reconcile when the Job finishes. A failed Job means we mark
	// the Workspace as Failed and release the execution lock (the annotation
	// from Step 4) so the Rollout controller knows this Workspace is done and
	// can move on. A succeeded Job means the plan file is ready on the PVC and
	// we fall through to Step 7 to decide whether to apply it.
	if rc.planJob == nil {
		var ownedJobs batchv1.JobList
		if err := r.List(ctx, &ownedJobs,
			client.InNamespace(workspace.Namespace),
			client.MatchingLabels{workspaceLabelKey: workspace.Name},
		); err != nil {
			return ctrl.Result{}, false, err
		}
		for _, job := range ownedJobs.Items {
			if job.Name == rc.planJobName || job.Name == rc.applyJobName || job.Status.Active == 0 {
				continue
			}
			for _, owner := range job.OwnerReferences {
				if owner.UID == workspace.UID {
					logger.Info("Waiting for previous Job to finish")
					return ctrl.Result{RequeueAfter: 10 * time.Second}, false, nil
				}
			}
		}

		logger.Info("Creating a new Plan Job", "job", rc.planJobName)
		runID := ensureRunMetadata(workspace)
		newJob, err := r.constructJobForWorkspace(ctx, workspace, rc, jobTypePlan, runID)
		if err != nil {
			return ctrl.Result{}, false, err
		}
		if err := r.Create(ctx, newJob); err != nil {
			return ctrl.Result{}, false, err
		}
		r.updateStatus(ctx, workspace, v1alpha1.PhasePlanning, "PlanJobCreated", "Terraform Plan job created", metav1.ConditionUnknown)
		return ctrl.Result{}, false, nil
	}

	if rc.planJob.Status.Failed > 0 {
		logger.Info("Plan Job failed", "job", rc.planJobName)

		// Check whether the failure was due to policy validation. The plan job
		// emits a MAGOS_RESULT line when kyverno-json evaluation runs. If we
		// find violations in the pod logs, this is a policy failure (not a
		// terraform error) and we surface it as ValidationFailed with the
		// specific rule violations in the status.
		phase := v1alpha1.PhaseFailed
		reason := "PlanFailed"
		message := "Terraform Plan execution failed"

		if r.Clientset != nil {
			violations, err := r.readPolicyViolations(ctx, workspace.Namespace, rc.planJobName)
			if err != nil {
				logger.Error(err, "Failed to read policy violations from pod logs")
			} else if len(violations) > 0 {
				phase = v1alpha1.PhaseValidationFailed
				reason = "PolicyViolation"
				message = fmt.Sprintf("Plan violated %d policy rule(s)", len(violations))
				workspace.Status.PolicyViolations = violations
			}
		}

		if err := r.archiveRunLogs(ctx, workspace, rc.planJob, v1alpha1.RunPhasePlan, v1alpha1.RunLogResultFailed); err != nil {
			logger.Error(err, "Failed to archive plan logs", "job", rc.planJobName)
		}
		r.updateStatus(ctx, workspace, phase, reason, message, metav1.ConditionFalse)

		// Release the execution lock so the Rollout controller knows this
		// Workspace is done with its turn, even though it failed. Without this
		// the Rollout would keep waiting for us and never advance to the next
		// Workspace in the sequence.
		//
		// If this run was triggered by a manual reconcile request, consume that
		// annotation too so the Project or Rollout controller does not
		// immediately grant execution again and bypass Step 3's retry cooldown.
		if err := r.deleteAnnotations(ctx, workspace,
			v1alpha1.WorkspaceExecutionAllowedAnnotation,
			v1alpha1.WorkspaceReconcileRequestAnnotation,
		); err != nil {
			logger.Error(err, "Failed to consume execution annotations via Patch on plan failure")
			return ctrl.Result{}, false, err
		}
		return ctrl.Result{}, false, nil
	}

	if rc.planJob.Status.Succeeded == 0 {
		logger.Info("Plan Job is currently running", "job", rc.planJobName)
		r.updateStatus(ctx, workspace, v1alpha1.PhasePlanning, "Planning", "Terraform Plan execution is running", metav1.ConditionUnknown)
		return ctrl.Result{}, false, nil
	}

	// Record plan job duration if both start and completion times are
	// available.
	if rc.planJob.Status.StartTime != nil && rc.planJob.Status.CompletionTime != nil {
		duration := rc.planJob.Status.CompletionTime.Time.Sub(rc.planJob.Status.StartTime.Time).Seconds()
		jobDurationSeconds.WithLabelValues(workspace.Namespace, workspace.Name, jobTypePlan).Observe(duration)
	}

	return ctrl.Result{}, true, nil
}
