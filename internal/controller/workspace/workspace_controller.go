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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/magosproject/magos/types/magosproject/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	// DefaultReconciliationInterval is the fallback duration between scheduled
	// reconciliations
	DefaultReconciliationInterval = 3 * time.Minute

	// Label used to identify repository credential secrets
	RepoSecretLabelKey   = "magosproject.io/secret-type"
	RepoSecretLabelValue = "repository"

	// Keys expected in the Secret's data map
	SecretKeyRepoURL       = "repoURL"
	SecretKeyUsername      = "username"
	SecretKeyPassword      = "password"
	SecretKeySSHPrivateKey = "sshPrivateKey"
)

// WorkspaceReconciler reconciles a Workspace object
type WorkspaceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// getRepoCredentials finds the Git credential Secret for a given repository
// URL. Magos uses a convention where credential Secrets are labeled with
// magosproject.io/secret-type=repository and contain a "repoURL" data key that
// identifies which repository they belong to. This function lists all such
// Secrets in the namespace and returns the first one whose repoURL matches
// targetRepoURL. Returns (nil, nil) when no matching Secret exists, which is
// fine because not every repository requires authentication.
func (r *WorkspaceReconciler) getRepoCredentials(ctx context.Context, namespace, targetRepoURL string) (*corev1.Secret, error) {
	var secretList corev1.SecretList

	// List secrets in the namespace with the specific label
	err := r.List(ctx, &secretList,
		client.InNamespace(namespace),
		client.MatchingLabels{RepoSecretLabelKey: RepoSecretLabelValue},
	)
	if err != nil {
		return nil, err
	}

	// Find the secret that matches the requested RepoURL
	for i := range secretList.Items {
		secret := &secretList.Items[i]
		if string(secret.Data[SecretKeyRepoURL]) == targetRepoURL {
			return secret, nil
		}
	}

	return nil, nil
}

// findWorkspacesForSecret maps Secret watch events to Workspace reconcile
// requests.
//
// We need this because repository credential Secrets are not owned by any
// Workspace. Without this mapper, updates to a Secret, such as SSH private key
// rotation, would not automatically trigger a reconcile of the Workspaces that
// use it. By mapping Secrets to the Workspaces referencing the same repoURL, we
// ensure that any change in credentials properly propagates, allowing the
// controller to react (e.g., by re-running jobs that may have failed due to Git
// auth issues).
func (r *WorkspaceReconciler) findWorkspacesForSecret(ctx context.Context, o client.Object) []reconcile.Request {
	secret, ok := o.(*corev1.Secret)
	if !ok {
		return nil
	}

	if secret.Labels == nil || secret.Labels[RepoSecretLabelKey] != RepoSecretLabelValue {
		return nil
	}

	repoURL, ok := secret.Data[SecretKeyRepoURL]
	if !ok {
		return nil
	}

	var workspaces v1alpha1.WorkspaceList
	if err := r.List(ctx, &workspaces, client.InNamespace(secret.Namespace)); err != nil {
		log.FromContext(ctx).Error(err, "Failed to list workspaces for secret change")
		return nil
	}

	// For each workspace in the same namespace, if its Spec.Source.RepoURL
	// matches the repoURL from the secret, enqueue a reconcile request for that
	// workspace.
	var requests []reconcile.Request
	for _, ws := range workspaces.Items {
		if ws.Spec.Source.RepoURL == string(repoURL) {
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      ws.Name,
					Namespace: ws.Namespace,
				},
			})
		}
	}
	return requests
}

// +kubebuilder:rbac:groups=magosproject.io,resources=workspaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=magosproject.io,resources=workspaces/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=magosproject.io,resources=workspaces/finalizers,verbs=update
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete

// Reconcile is the top-level entry point invoked by controller-runtime whenever
// a Workspace or one of its watched dependents (Jobs, PVCs, Secrets) changes.
func (r *WorkspaceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the Workspace instance
	workspace := &v1alpha1.Workspace{}
	if err := r.Get(ctx, req.NamespacedName, workspace); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Workspace resource not found, ignoring")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get Workspace")
		return ctrl.Result{}, err
	}

	// Ensure a finalizer is present so Kubernetes delays actual deletion until
	// we explicitly remove it. This guarantees the controller gets a chance to
	// run handleDeletion before the object disappears, even if someone deletes
	// the Workspace manually via kubectl.
	if controllerutil.AddFinalizer(workspace, v1alpha1.WorkspaceFinalizerName) {
		if err := r.Update(ctx, workspace); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if !workspace.DeletionTimestamp.IsZero() {
		finished, err := r.handleDeletion(ctx, workspace)
		if err != nil {
			return ctrl.Result{}, err
		}
		if finished {
			return ctrl.Result{}, nil
		}
		// Finalizer was removed but the object hasn't been garbage-collected
		// yet. Requeue briefly so we don't spin on every event in the meantime.
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	res, err := r.reconcileWorkspace(ctx, workspace)
	if err != nil {
		r.updateStatus(ctx, workspace, v1alpha1.PhaseFailed, "ReconcileError", err.Error(), metav1.ConditionFalse)
		return ctrl.Result{}, err
	}

	// Always requeue on the sync interval so we periodically re-plan even when
	// nothing in the cluster changes. This is how we detect infrastructure
	// drift that happened outside of Magos.
	if res.RequeueAfter == 0 {
		res.RequeueAfter = r.getSyncInterval(workspace)
	}

	r.updateNextReconcileTime(ctx, workspace, res.RequeueAfter)

	return res, nil
}

// handleDeletion removes the finalizer from a Workspace that is being deleted.
// Since all Jobs and PVCs are owned by the Workspace (via OwnerReferences),
// Kubernetes garbage collection automatically deletes them once the Workspace
// itself is removed. All we need to do here is remove our finalizer so that
// Kubernetes can proceed with the actual deletion.
func (r *WorkspaceReconciler) handleDeletion(ctx context.Context, workspace *v1alpha1.Workspace) (bool, error) {
	logger := log.FromContext(ctx)
	logger.Info("Handling workspace deletion")

	r.updateStatus(ctx, workspace, v1alpha1.PhaseDeleting, "Deleting", "Workspace is being deleted", metav1.ConditionFalse)

	// Since Jobs and PVCs are owned by the Workspace (via OwnerReferences),
	// Kubernetes garbage collection will automatically clean them up. We don't
	// need to manually delete them.

	if controllerutil.ContainsFinalizer(workspace, v1alpha1.WorkspaceFinalizerName) {
		logger.Info("Removing finalizer")
		controllerutil.RemoveFinalizer(workspace, v1alpha1.WorkspaceFinalizerName)
		if err := r.Update(ctx, workspace); err != nil {
			return false, err
		}
	}
	return true, nil
}

// getSpecHash produces a short, deterministic hash of the Workspace spec. This
// hash is used as a suffix on Job names (e.g. "myworkspace-plan-a1b2c3d4") so
// that a spec change naturally creates new Jobs while leaving old ones to be
// cleaned up by Step 2. The approval annotation is deliberately excluded from
// the hash so that approving a plan does not invalidate the existing Plan Job.
//
// We also fold the reconcile-request annotation into the hash so that setting
// that annotation (a manual "re-run" trigger) forces a new plan/apply cycle
// even when the spec itself hasn't changed.
func (r *WorkspaceReconciler) getSpecHash(ws *v1alpha1.Workspace) string {
	data, _ := json.Marshal(ws.Spec)

	if ws.Annotations != nil {
		if req, ok := ws.Annotations[v1alpha1.WorkspaceReconcileRequestAnnotation]; ok {
			data = append(data, []byte(req)...)
		}
	}

	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])[:8] // Short 8-character hash
}

// getSyncInterval returns the reconciliation interval for this Workspace. If
// the user set the magosproject.io/reconcile-interval annotation to a valid
// Go duration (e.g. "5m", "1h"), we use that. Otherwise we fall back to
// DefaultReconciliationInterval (3 minutes). This interval controls how often
// we re-plan for drift detection and how long we wait before retrying after a
// failure.
func (r *WorkspaceReconciler) getSyncInterval(ws *v1alpha1.Workspace) time.Duration {
	if ws.Annotations != nil {
		if val, ok := ws.Annotations[v1alpha1.WorkspaceReconcileIntervalAnnotation]; ok {
			if d, err := time.ParseDuration(val); err == nil {
				return d
			}
		}
	}
	return DefaultReconciliationInterval
}

func (r *WorkspaceReconciler) reconcileWorkspace(ctx context.Context, workspace *v1alpha1.Workspace) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling Workspace", "name", workspace.Name, "namespace", workspace.Namespace)

	// Step 1: Build Job names from a hash of the Workspace spec.
	//
	// Each Workspace reconciliation produces a Plan Job and an Apply Job. The
	// Kubernetes jobs are suffixed with  a short hash (specHash) so that a
	// given Apply always runs against the exact plan file that was generated
	// for the same spec. When someone changes the spec, the hash changes and we
	// get a new pair of Jobs. Importantly, approving a plan does not change the
	// hash (the approval annotation is not part of the spec) so the Apply
	// Job is guaranteed to execute the plan that was reviewed and approved.
	specHash := r.getSpecHash(workspace)
	planJobName := fmt.Sprintf("%s-plan-%s", workspace.Name, specHash)
	applyJobName := fmt.Sprintf("%s-apply-%s", workspace.Name, specHash)
	planFile := fmt.Sprintf("/workspace-data/run-%s.tfplan", specHash)

	// Step 2: Clean up Jobs left over from a previous spec version.
	//
	// When the Workspace spec changes (e.g. a new targetRevision), the specHash
	// changes too, so the old Plan/Apply Jobs no longer match. We find any Jobs
	// still owned by this Workspace that don't match the current specHash and
	// delete them to avoid leaving stale resources in the cluster.
	//
	// IMPORTANT: We only clean up orphaned Jobs when the Workspace is NOT
	// actively planning or applying. Deleting a running Job mid-execution
	// (especially during terraform apply) could corrupt Terraform state. We
	// wait for the active operation to finish before cleaning up.
	if workspace.Status.Phase != v1alpha1.PhasePlanning && workspace.Status.Phase != v1alpha1.PhaseApplying {
		var childJobs batchv1.JobList
		if err := r.List(ctx, &childJobs, client.InNamespace(workspace.Namespace)); err == nil {
			for _, j := range childJobs.Items {
				isOwned := false
				for _, owner := range j.OwnerReferences {
					if owner.UID == workspace.UID {
						isOwned = true
						break
					}
				}
				// Delete Jobs that belong to this Workspace but were created for an
				// older specHash.
				if isOwned && j.Name != planJobName && j.Name != applyJobName {
					logger.Info("Cleaning up orphaned job from previous run", "job", j.Name)
					_ = r.Delete(ctx, &j, client.PropagationPolicy(metav1.DeletePropagationBackground))
				}
			}
		}
	}

	// Look up the current Plan and Apply Jobs. A NotFound error is normal and
	// just means the Job hasn't been created yet for this specHash.
	var planJob batchv1.Job
	planJobGetErr := r.Get(ctx, types.NamespacedName{Name: planJobName, Namespace: workspace.Namespace}, &planJob)

	var applyJob batchv1.Job
	applyJobGetErr := r.Get(ctx, types.NamespacedName{Name: applyJobName, Namespace: workspace.Namespace}, &applyJob)

	// Step 3: Decide whether we need to start a fresh Plan/Apply cycle.
	//
	// After a Workspace finishes (successfully or not), its Jobs stick around
	// until the next sync interval elapses. Once enough time has passed we
	// delete the old Jobs and reset the phase to Pending, which kicks off a new
	// cycle. This handles three scenarios:
	//   - Periodic reconciliation: re-plan after a successful apply to detect drift.
	//   - Retry on failure: re-plan after a failed apply or plan.
	//   - Spec change: the specHash shifted so the old Jobs no longer exist.
	//
	// This logic must run before Step 4. Step 4 evaluates the Rollout execution
	// lock annotation (magosproject.io/execution-allowed). The Rollout controller
	// adds that annotation to allow a Workspace to execute, and removes it again
	// once the Workspace finishes.
	//
	// If we checked the execution lock first, a completed Workspace could appear
	// "not allowed" and we would never reach this reset path. That would leave the
	// Workspace stuck in a terminal(?) phase with no way to clean up old Jobs or
	// start a new cycle.
	syncInterval := r.getSyncInterval(workspace)
	needsReset := false
	resetReason := ""
	resetMessage := ""
	var exactRequeue time.Duration

	// No Jobs exist for the current specHash, yet the Workspace thinks it's
	// past Pending. This means the spec changed (new hash) or someone manually
	// deleted the Jobs. Either way, go back to Pending to start fresh.
	//
	// We skip the reset when the phase is Planning or Applying. The Jobs for
	// the *previous* specHash may still be running (their names no longer match
	// the current hash, so the GETs above return NotFound). Resetting now would
	// delete those running Jobs via the cleanup block below and could corrupt
	// Terraform state. Instead we let the running Job finish; once it completes,
	// the phase will move to a terminal state and the next reconcile will reset.
	if planJobGetErr != nil && errors.IsNotFound(planJobGetErr) && applyJobGetErr != nil && errors.IsNotFound(applyJobGetErr) {
		if workspace.Status.Phase != "" && workspace.Status.Phase != v1alpha1.PhasePending &&
			workspace.Status.Phase != v1alpha1.PhasePlanning && workspace.Status.Phase != v1alpha1.PhaseApplying {
			needsReset = true
			resetReason = "ConfigurationChanged"
			resetMessage = "Workspace spec was modified or jobs were deleted, triggering fresh execution"
		}
	}

	// Check whether the Apply Job finished (succeeded or failed)
	var applyFinishedTime time.Time
	var applySucceeded bool
	if applyJobGetErr == nil {
		if applyJob.Status.CompletionTime != nil {
			applyFinishedTime = applyJob.Status.CompletionTime.Time
			applySucceeded = true
		} else if applyJob.Status.Failed > 0 {
			for _, cond := range applyJob.Status.Conditions {
				if cond.Type == batchv1.JobFailed && cond.Status == corev1.ConditionTrue {
					applyFinishedTime = cond.LastTransitionTime.Time
					applySucceeded = false
					break
				}
			}
		}
	}

	// If the Apply Job already finished we don't want to act on it again right
	// away. We wait for the sync interval to elapse first. On success that
	// gives us periodic drift detection, on failure it acts as a backoff before
	// retrying. When the interval hasn't fully elapsed yet we requeue for
	// exactly the remaining duration to avoid waking up on every reconcile
	// loop in the meantime.
	if !applyFinishedTime.IsZero() {
		elapsed := time.Since(applyFinishedTime)
		if elapsed >= syncInterval {
			needsReset = true
			if applySucceeded {
				resetReason = "ScheduledReconcile"
				resetMessage = "Starting scheduled reconciliation"
			} else {
				resetReason = "RetryApply"
				resetMessage = "Retrying failed apply starting from new plan"
			}
		} else {
			exactRequeue = syncInterval - elapsed
		}
	} else if planJobGetErr == nil && planJob.Status.Failed > 0 {
		// We never got to Apply because the Plan itself failed. We use the same
		// sync-interval cooldown here to avoid hammering a plan that keeps
		// failing (e.g. bad credentials, broken HCL) on every reconcile loop.
		var failedTime time.Time
		for _, cond := range planJob.Status.Conditions {
			if cond.Type == batchv1.JobFailed && cond.Status == corev1.ConditionTrue {
				failedTime = cond.LastTransitionTime.Time
				break
			}
		}
		if !failedTime.IsZero() {
			elapsed := time.Since(failedTime)
			if elapsed >= syncInterval {
				needsReset = true
				resetReason = "RetryPlan"
				resetMessage = "Retrying failed plan"
			} else {
				exactRequeue = syncInterval - elapsed
			}
		}
	}

	if needsReset {
		logger.Info("Sync interval reached. Cleaning up jobs to trigger a fresh run.", "reason", resetReason)
		if planJobGetErr == nil {
			_ = r.Delete(ctx, &planJob, client.PropagationPolicy(metav1.DeletePropagationBackground))
		}
		if applyJobGetErr == nil {
			_ = r.Delete(ctx, &applyJob, client.PropagationPolicy(metav1.DeletePropagationBackground))
		}
		// Delete old Jobs and go back to Pending so the Rollout controller can
		// re-queue this Workspace for a fresh run.
		r.updateStatus(ctx, workspace, v1alpha1.PhasePending, resetReason, resetMessage, metav1.ConditionUnknown)
		return ctrl.Result{}, nil
	}

	// Step 4: Check whether the Rollout controller has granted us permission to
	// execute.
	//
	// A Rollout groups multiple Workspaces and controls the order they run in
	// (e.g. "dev must succeed before prod starts"). It does this by setting the
	// execution-allowed annotation on each Workspace when it is that Workspace's
	// turn. If the annotation is absent or not "true", it means the Rollout
	// controller hasn't reached that Workspace yet, so we stay in Pending and
	// return early. The Rollout controller will trigger a new reconcile once it
	// sets the annotation.
	isAllowed := false
	if workspace.Annotations != nil {
		isAllowed = workspace.Annotations[v1alpha1.WorkspaceExecutionAllowedAnnotation] == "true"
	}

	if !isAllowed {
		logger.Info("Workspace execution is not allowed. Waiting for rollout controller to grant permission.", "workspace", workspace.Name)
		if workspace.Status.Phase == "" {
			r.updateStatus(ctx, workspace, v1alpha1.PhasePending, "AwaitingRollout", "Waiting for the Rollout controller to schedule this Workspace for execution", metav1.ConditionUnknown)
		}
		if exactRequeue > 0 {
			return ctrl.Result{RequeueAfter: exactRequeue}, nil
		}
		return ctrl.Result{}, nil
	}

	// Step 5: Create a PersistentVolumeClaim for this Workspace if one doesn't
	// exist yet.
	//
	// Terraform's plan and apply are two separate operations that run as
	// independent Kubernetes Jobs. The Plan Job writes a .tfplan binary to
	// disk, and the Apply Job needs to read that exact file back. We create a
	// PVC per Workspace and mount it into both Jobs so the plan file can be
	// accessed between Jobs. The PVC is owned by the Workspace, so Kubernetes
	// garbage collection will clean it up automatically when the Workspace is
	// deleted.
	pvcName := fmt.Sprintf("%s-data", workspace.Name)
	if err := r.ensurePVC(ctx, workspace, pvcName); err != nil {
		logger.Error(err, "Failed to ensure PVC exists")
		return ctrl.Result{}, err
	}

	// Step 6: Run "terraform plan".
	//
	// If the Plan Job doesn't exist yet we create it. If it already exists we
	// look at its status. A still running Job means we return early and wait
	// for the next reconcile when the Job finishes. A failed Job means we mark
	// the Workspace as Failed and release the execution lock (the annotation
	// from Step 4) so the Rollout controller knows this Workspace is done and
	// can move on. A succeeded Job means the plan file is ready on the PVC and
	// we fall through to Step 7 to decide whether to apply it.
	if planJobGetErr != nil {
		if errors.IsNotFound(planJobGetErr) {
			logger.Info("Creating a new Plan Job", "job", planJobName)
			newJob, err := r.constructJobForWorkspace(ctx, workspace, planJobName, "plan", planFile, pvcName)
			if err != nil {
				return ctrl.Result{}, err
			}
			if err := r.Create(ctx, newJob); err != nil {
				return ctrl.Result{}, err
			}
			r.updateStatus(ctx, workspace, v1alpha1.PhasePlanning, "PlanJobCreated", "Terraform Plan job created", metav1.ConditionUnknown)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, planJobGetErr
	}

	if planJob.Status.Failed > 0 {
		logger.Info("Plan Job failed", "job", planJobName)
		r.updateStatus(ctx, workspace, v1alpha1.PhaseFailed, "PlanFailed", "Terraform Plan execution failed", metav1.ConditionFalse)

		// Release the execution lock so the Rollout controller knows this
		// Workspace is done with its turn, even though it failed. Without this
		// the Rollout would keep waiting for us and never advance to the next
		// Workspace in the sequence.
		if workspace.Annotations != nil && workspace.Annotations[v1alpha1.WorkspaceExecutionAllowedAnnotation] != "" {
			patch := client.MergeFrom(workspace.DeepCopy())
			delete(workspace.Annotations, v1alpha1.WorkspaceExecutionAllowedAnnotation)
			if err := r.Patch(ctx, workspace, patch); err != nil {
				logger.Error(err, "Failed to consume execution annotations via Patch on plan failure")
				return ctrl.Result{}, err
			}
		}

		return ctrl.Result{}, nil
	} else if planJob.Status.Succeeded == 0 {
		logger.Info("Plan Job is currently running", "job", planJobName)
		r.updateStatus(ctx, workspace, v1alpha1.PhasePlanning, "Planning", "Terraform Plan execution is running", metav1.ConditionUnknown)
		return ctrl.Result{}, nil
	}

	// Step 7: Run "terraform apply" (requires approval).
	//
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
	// When we do have approval we remove the annotation before creating the
	// Job. This is important because annotations persist across reconciles. If
	// we left it in place and the spec changed later (producing a new plan),
	// that stale "approved" annotation would cause the new plan to be applied
	// without anyone actually reviewing it.
	if applyJobGetErr != nil {
		if errors.IsNotFound(applyJobGetErr) {
			// Apply job doesn't exist yet, check if we have approval to proceed
			isApproved := workspace.Spec.AutoApply
			if workspace.Annotations != nil && workspace.Annotations[v1alpha1.WorkspaceApprovedAnnotation] == "true" {
				isApproved = true
			}

			if !isApproved {
				logger.Info("Workspace has planned successfully, but is pending approval to apply", "workspace", workspace.Name)
				r.updateStatus(ctx, workspace, v1alpha1.PhasePlanned, "PlanSucceeded", "Terraform Plan succeeded. Waiting for manual approval to Apply.", metav1.ConditionTrue)
				return ctrl.Result{}, nil
			}

			// Remove the approval annotation before creating the Job. See the
			// comment above for why leaving it around would be dangerous.
			if workspace.Annotations != nil && workspace.Annotations[v1alpha1.WorkspaceApprovedAnnotation] != "" {
				patch := client.MergeFrom(workspace.DeepCopy())
				delete(workspace.Annotations, v1alpha1.WorkspaceApprovedAnnotation)
				if err := r.Patch(ctx, workspace, patch); err != nil {
					logger.Error(err, "Failed to consume approval annotation via Patch")
					return ctrl.Result{}, err
				}
			}

			logger.Info("Creating a new Apply Job", "job", applyJobName)
			newJob, err := r.constructJobForWorkspace(ctx, workspace, applyJobName, "apply", planFile, pvcName)
			if err != nil {
				return ctrl.Result{}, err
			}
			if err := r.Create(ctx, newJob); err != nil {
				return ctrl.Result{}, err
			}
			r.updateStatus(ctx, workspace, v1alpha1.PhaseApplying, "ApplyJobCreated", "Terraform Apply job created", metav1.ConditionUnknown)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, applyJobGetErr
	}

	if applyJob.Status.Failed > 0 {
		logger.Info("Apply Job failed", "job", applyJobName)
		r.updateStatus(ctx, workspace, v1alpha1.PhaseFailed, "ApplyFailed", "Terraform Apply execution failed", metav1.ConditionFalse)

		// Same as the plan failure path in Step 6: release the execution lock
		// so the Rollout controller can continue with the next Workspace.
		if workspace.Annotations != nil && workspace.Annotations[v1alpha1.WorkspaceExecutionAllowedAnnotation] != "" {
			patch := client.MergeFrom(workspace.DeepCopy())
			delete(workspace.Annotations, v1alpha1.WorkspaceExecutionAllowedAnnotation)
			if err := r.Patch(ctx, workspace, patch); err != nil {
				logger.Error(err, "Failed to consume execution annotations via Patch on failure")
				return ctrl.Result{}, err
			}
		}

		return ctrl.Result{}, nil
	} else if applyJob.Status.Succeeded == 0 {
		logger.Info("Apply Job is currently running", "job", applyJobName)
		r.updateStatus(ctx, workspace, v1alpha1.PhaseApplying, "Applying", "Terraform Apply execution is running", metav1.ConditionUnknown)
		return ctrl.Result{}, nil
	}

	// Step 8: The Apply succeeded. Record the result and release the execution lock.
	//
	// We write the Git revision that was just applied into the status so that
	// external consumers (Magos UI, the Rollout controller, etc)
	// can see exactly which revision is live for this Workspace. After that we
	// remove the execution-allowed annotation to hand control back to the Rollout
	// controller, completing this Workspace's execution cycle. The next cycle
	// will start when Step 3's reset evaluation fires after the sync interval.
	logger.Info("Apply Job completed successfully", "job", applyJobName)

	// Record the observed revision before the status update so it's included
	// in the same write.
	workspace.Status.ObservedRevision = workspace.Spec.Source.TargetRevision
	r.updateStatus(ctx, workspace, v1alpha1.PhaseApplied, "ApplySucceeded", "Terraform Apply completed successfully", metav1.ConditionTrue)

	// Remove the execution lock. We use Patch rather than Update because the
	// status update above may have bumped the resourceVersion, and a full
	// Update would conflict.
	if workspace.Annotations != nil && workspace.Annotations[v1alpha1.WorkspaceExecutionAllowedAnnotation] != "" {
		patch := client.MergeFrom(workspace.DeepCopy())
		delete(workspace.Annotations, v1alpha1.WorkspaceExecutionAllowedAnnotation)
		if err := r.Patch(ctx, workspace, patch); err != nil {
			logger.Error(err, "Failed to consume execution annotations via Patch")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// ensurePVC checks whether the PVC for this Workspace already exists and
// creates it if not. The PVC uses ReadWriteOnce access mode because only one
// Job at a time needs to write to it (Plan writes, then Apply reads). We set
// the Workspace as the owner so the PVC is automatically deleted when the
// Workspace is removed.
//
// TODO: Have @fayusohenson verify the security model here.
func (r *WorkspaceReconciler) ensurePVC(ctx context.Context, ws *v1alpha1.Workspace, pvcName string) error {
	pvc := &corev1.PersistentVolumeClaim{}
	err := r.Get(ctx, types.NamespacedName{Name: pvcName, Namespace: ws.Namespace}, pvc)

	if err != nil && errors.IsNotFound(err) {
		log.FromContext(ctx).Info("Creating PVC for Workspace", "pvc", pvcName)

		newPVC := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pvcName,
				Namespace: ws.Namespace,
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("1Gi"),
					},
				},
			},
		}

		// Set the Workspace as the owner of this PVC. When the Workspace is
		// deleted, Kubernetes garbage collection will remove the PVC too.
		if err := ctrl.SetControllerReference(ws, newPVC, r.Scheme); err != nil {
			return err
		}

		return r.Create(ctx, newPVC)
	}
	return err
}

// constructJobForWorkspace builds a Kubernetes Job spec for either a "plan" or
// "apply" operation. The Job runs the magos-job container image which knows how
// to clone a Git repo, install the right Terraform version, and execute the
// requested operation.
//
// We pass all configuration to the container through environment variables.
// Plain values (repo URL, revision, terraform version, etc.) are set as
// literal env vars. Sensitive values (Git credentials) are injected via
// secretKeyRef so that Kubernetes resolves them at Pod startup from the
// referenced Secret, and we never have to copy secret data into the Job spec.
//
// The Job mounts the Workspace's shared PVC at /workspace-data. The Plan Job
// writes the .tfplan file there, and the Apply Job reads it back from the same
// path.
//
// We set backoffLimit to 0 so Kubernetes does not automatically retry a failed
// Job. Terraform failures (bad HCL, provider errors, state locks) are unlikely
// to resolve on a blind retry, and Step 3 in reconcileWorkspace already
// handles retries after a cooldown period.
//
// The Job is owned by the Workspace via SetControllerReference, so Kubernetes
// garbage collection will delete it when the Workspace is removed.
func (r *WorkspaceReconciler) constructJobForWorkspace(ctx context.Context, ws *v1alpha1.Workspace, jobName, jobType, planFile, pvcName string) (*batchv1.Job, error) {
	// The below map holds configuration that every Job needs: where to clone from, which
	// revision to check out, which Terraform version to use, and whether this
	// is a "plan" or "apply" run.
	envVars := []corev1.EnvVar{
		{Name: "REPO_URL", Value: ws.Spec.Source.RepoURL},
		{Name: "TARGET_REVISION", Value: ws.Spec.Source.TargetRevision},
		{Name: "TF_VERSION", Value: ws.Spec.Terraform.Version},
		{Name: "PROJECT_REF", Value: ws.Spec.ProjectRef.Name},
		{Name: "MAGOS_JOB_TYPE", Value: jobType},
		{Name: "MAGOS_PLAN_FILE", Value: planFile},
	}

	// Optional paths that narrow which Terraform directory to run in and which
	// .tfvars file to use. Only set when the Workspace spec provides them.
	if ws.Spec.Source.Path != "" {
		envVars = append(envVars, corev1.EnvVar{Name: "TF_PATH", Value: ws.Spec.Source.Path})
	}
	if ws.Spec.Terraform.TfvarsPath != "" {
		envVars = append(envVars, corev1.EnvVar{Name: "TF_VAR_FILE", Value: ws.Spec.Terraform.TfvarsPath})
	}

	// Look up Git credentials for this repo URL. If a matching Secret exists
	// in the namespace we inject its values via secretKeyRef. This means the
	// actual secret data never appears in the Job spec; Kubernetes resolves
	// it at Pod startup.
	authSecret, err := r.getRepoCredentials(ctx, ws.Namespace, ws.Spec.Source.RepoURL)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve repository credentials: %w", err)
	}

	if authSecret != nil {
		// SSH authentication
		if _, ok := authSecret.Data[SecretKeySSHPrivateKey]; ok {
			envVars = append(envVars,
				corev1.EnvVar{
					Name: "GIT_SSH_PRIVATE_KEY",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: authSecret.Name},
							Key:                  SecretKeySSHPrivateKey,
						},
					},
				},
			)
		} else if _, ok := authSecret.Data[SecretKeyUsername]; ok {
			// HTTPS authentication
			envVars = append(envVars,
				corev1.EnvVar{
					Name: "GIT_USERNAME",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: authSecret.Name},
							Key:                  SecretKeyUsername,
						},
					},
				},
				corev1.EnvVar{
					Name: "GIT_PASSWORD",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: authSecret.Name},
							Key:                  SecretKeyPassword,
						},
					},
				},
			)
		}
	}

	var backoffLimit int32 = 0

	// Merge shared annotations with per-phase overrides (phase wins on conflict).
	var podAnnotations map[string]string
	if len(ws.Spec.Annotations) > 0 {
		podAnnotations = make(map[string]string, len(ws.Spec.Annotations))
		for k, v := range ws.Spec.Annotations {
			podAnnotations[k] = v
		}
	}
	var overrides map[string]string
	switch jobType {
	case "plan":
		if ws.Spec.Plan != nil {
			overrides = ws.Spec.Plan.Annotations
		}
	case "apply":
		if ws.Spec.Apply != nil {
			overrides = ws.Spec.Apply.Annotations
		}
	}
	if len(overrides) > 0 {
		if podAnnotations == nil {
			podAnnotations = make(map[string]string, len(overrides))
		}
		for k, v := range overrides {
			podAnnotations[k] = v
		}
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: ws.Namespace,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backoffLimit,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: podAnnotations,
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Volumes: []corev1.Volume{
						{
							Name: "workspace-data",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: pvcName,
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:            "job",
							Image:           "magos-job:local",
							ImagePullPolicy: corev1.PullIfNotPresent,
							Env:             envVars,
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "workspace-data",
									MountPath: "/workspace-data",
								},
							},
						},
					},
				},
			},
		},
	}

	// Set the Workspace as the owner of this Job so Kubernetes garbage
	// collection deletes it when the Workspace is removed.
	if err := ctrl.SetControllerReference(ws, job, r.Scheme); err != nil {
		return nil, err
	}

	return job, nil
}

// updateStatus writes the phase, reason, message, and Ready condition to the
// Workspace's status subresource. To prevent conflicts from concurrent updates,
// it always fetches the latest version of the Workspace before writing.
//
// After a successful update, the Workspace object passed into updateStatus is
// updated in-place with the new resourceVersion and status. This guarantees
// that any subsequent logic in the same reconcile loop sees the latest state
// and avoids operating on stale data.
func (r *WorkspaceReconciler) updateStatus(ctx context.Context, workspace *v1alpha1.Workspace, phase v1alpha1.Phase, reason, message string, status metav1.ConditionStatus) {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// Fetch the latest version to avoid conflict errors
		latest := &v1alpha1.Workspace{}
		if err := r.Get(ctx, client.ObjectKeyFromObject(workspace), latest); err != nil {
			return err
		}

		needsUpdate := false

		if latest.Status.Phase != phase || latest.Status.Reason != reason || latest.Status.Message != message {
			latest.Status.Phase = phase
			latest.Status.Reason = reason
			latest.Status.Message = message
			needsUpdate = true
		}

		// Preserve observed revision if it was set
		if workspace.Status.ObservedRevision != "" && latest.Status.ObservedRevision != workspace.Status.ObservedRevision {
			latest.Status.ObservedRevision = workspace.Status.ObservedRevision
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
		workspace.Status = latest.Status
		workspace.ResourceVersion = latest.ResourceVersion
		return nil
	})

	if err != nil {
		log.FromContext(ctx).Error(err, "Failed to update workspace status")
	}
}

// updateNextReconcileTime writes the expected next reconciliation time into
// the Workspace status so that the UI can display when the next sync will
// happen.
func (r *WorkspaceReconciler) updateNextReconcileTime(ctx context.Context, workspace *v1alpha1.Workspace, requeueAfter time.Duration) {
	next := metav1.NewTime(time.Now().Add(requeueAfter))

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		latest := &v1alpha1.Workspace{}
		if err := r.Get(ctx, client.ObjectKeyFromObject(workspace), latest); err != nil {
			return err
		}

		latest.Status.NextReconcileTime = &next
		if err := r.Status().Update(ctx, latest); err != nil {
			return err
		}

		workspace.Status = latest.Status
		workspace.ResourceVersion = latest.ResourceVersion
		return nil
	})

	if err != nil {
		log.FromContext(ctx).Error(err, "Failed to update next reconcile time")
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *WorkspaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Workspace{}).
		Owns(&batchv1.Job{}).                  // Watch for changes to Jobs owned by the Workspace
		Owns(&corev1.PersistentVolumeClaim{}). // Watch PVCs
		Watches(
			&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(r.findWorkspacesForSecret),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		).
		Named("workspace").
		Complete(r)
}
