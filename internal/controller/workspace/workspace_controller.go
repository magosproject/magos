/*
Copyright 2026.

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

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	magosprojectiov1alpha1 "github.com/magosproject/magos/api/v1alpha1"
)

const (
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

// getRepoCredentials looks for a Secret in the given namespace labeled with
// magosproject.io/secret-type: repository that matches the target repoURL.
func (r *WorkspaceReconciler) getRepoCredentials(ctx context.Context, namespace, targetRepoURL string) (*corev1.Secret, error) {
	var secretList corev1.SecretList

	// List secrets in the namespace with the specific label
	err := r.Client.List(ctx, &secretList,
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

	return nil, nil // No credentials found
}

// findWorkspacesForSecret finds all workspaces in the secret's namespace that use the secret's repository URL
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

	var workspaces magosprojectiov1alpha1.WorkspaceList
	if err := r.List(ctx, &workspaces, client.InNamespace(secret.Namespace)); err != nil {
		log.FromContext(ctx).Error(err, "Failed to list workspaces for secret change")
		return nil
	}

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

func (r *WorkspaceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the Workspace instance
	workspace := &magosprojectiov1alpha1.Workspace{}
	if err := r.Get(ctx, req.NamespacedName, workspace); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Workspace resource not found, ignoring")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get Workspace")
		return ctrl.Result{}, err
	}

	if controllerutil.AddFinalizer(workspace, magosprojectiov1alpha1.WorkspaceFinalizerName) {
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
		// Finalizer already removed but workspace is still there
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	err := r.reconcileWorkspace(ctx, workspace)
	if err != nil {
		r.updateStatus(ctx, workspace, magosprojectiov1alpha1.PhaseFailed, "ReconcileError", err.Error(), metav1.ConditionFalse)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *WorkspaceReconciler) handleDeletion(ctx context.Context, workspace *magosprojectiov1alpha1.Workspace) (bool, error) {
	logger := log.FromContext(ctx)
	logger.Info("Handling workspace deletion")

	r.updateStatus(ctx, workspace, magosprojectiov1alpha1.PhaseDeleting, "Deleting", "Workspace is being deleted", metav1.ConditionFalse)

	// Since Jobs and PVCs are owned by the Workspace (via OwnerReferences),
	// Kubernetes garbage collection will automatically clean them up. We don't
	// need to manually delete them.

	if controllerutil.ContainsFinalizer(workspace, magosprojectiov1alpha1.WorkspaceFinalizerName) {
		logger.Info("Removing finalizer")
		controllerutil.RemoveFinalizer(workspace, magosprojectiov1alpha1.WorkspaceFinalizerName)
		if err := r.Update(ctx, workspace); err != nil {
			return false, err
		}
	}
	return true, nil
}

// getRunID returns a deterministic hash of the Workspace Spec. This allows the
// controller to reuse the plan job when only the Approval status changes.
func (r *WorkspaceReconciler) getRunID(ws *magosprojectiov1alpha1.Workspace) string {
	data, _ := json.Marshal(ws.Spec)

	// Include reconcile-request annotation in the hash to allow forced re-runs
	if ws.Annotations != nil {
		if req, ok := ws.Annotations[magosprojectiov1alpha1.WorkspaceReconcileRequestAnnotation]; ok {
			data = append(data, []byte(req)...)
		}
	}

	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])[:8] // Short 8-character hash
}

func (r *WorkspaceReconciler) reconcileWorkspace(ctx context.Context, workspace *magosprojectiov1alpha1.Workspace) error {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling Workspace", "name", workspace.Name, "namespace", workspace.Namespace)

	// Check if the workspace is allowed to reconcile
	isAllowed := false
	if workspace.Annotations != nil {
		isAllowed = workspace.Annotations[magosprojectiov1alpha1.WorkspaceAllowedReconcileAnnotation] == "true"
	}

	if !isAllowed {
		logger.Info("Workspace execution is not allowed. Waiting for rollout controller to grant permission.", "workspace", workspace.Name)

		// If it's already finished (applied or failed), we just keep its state.
		// If it's just created, mark it as Pending.
		if workspace.Status.Phase == "" {
			r.updateStatus(ctx, workspace, magosprojectiov1alpha1.PhasePending, "PendingPermission", "Waiting for execution permission from Rollout orchestrator", metav1.ConditionUnknown)
		}
		return nil
	}

	// 1. Ensure PVC exists to transfer plan to apply securely
	pvcName := fmt.Sprintf("%s-data", workspace.Name)
	if err := r.ensurePVC(ctx, workspace, pvcName); err != nil {
		logger.Error(err, "Failed to ensure PVC exists")
		return err
	}

	// The run ID ensures that a plan is reused when only the Approved flag
	// changes
	runID := r.getRunID(workspace)

	// The plan file we will write/read across both jobs
	planFile := fmt.Sprintf("/workspace-data/run-%s.tfplan", runID)

	// Deterministic Job names for this specific Configuration
	planJobName := fmt.Sprintf("%s-plan-%s", workspace.Name, runID)
	applyJobName := fmt.Sprintf("%s-apply-%s", workspace.Name, runID)

	// 2. Evaluate Plan Job
	var planJob batchv1.Job
	err := r.Get(ctx, types.NamespacedName{Name: planJobName, Namespace: workspace.Namespace}, &planJob)

	// Create Plan Job if it doesn't exist
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Creating a new Plan Job", "job", planJobName)
			newJob, err := r.constructJobForWorkspace(ctx, workspace, planJobName, "plan", planFile, pvcName)
			if err != nil {
				return err
			}
			if err := r.Create(ctx, newJob); err != nil {
				return err
			}
			r.updateStatus(ctx, workspace, magosprojectiov1alpha1.PhasePlanning, "PlanJobCreated", "Terraform Plan job created", metav1.ConditionUnknown)
			return nil
		}
		return err
	}

	// Check Plan Job Status
	if planJob.Status.Failed > 0 {
		logger.Info("Plan Job failed", "job", planJobName)
		r.updateStatus(ctx, workspace, magosprojectiov1alpha1.PhaseFailed, "PlanFailed", "Terraform Plan execution failed", metav1.ConditionFalse)
		return nil
	} else if planJob.Status.Succeeded == 0 {
		logger.Info("Plan Job is currently running", "job", planJobName)
		r.updateStatus(ctx, workspace, magosprojectiov1alpha1.PhasePlanning, "Planning", "Terraform Plan execution is running", metav1.ConditionUnknown)
		return nil
	}

	// 3. Evaluate Apply Job
	var applyJob batchv1.Job
	err = r.Get(ctx, types.NamespacedName{Name: applyJobName, Namespace: workspace.Namespace}, &applyJob)

	// Create Apply Job if it doesn't exist
	if err != nil {
		if errors.IsNotFound(err) {
			// Apply job doesn't exist yet, check if we have approval to proceed
			isApproved := workspace.Spec.AutoApply
			if workspace.Annotations != nil && workspace.Annotations[magosprojectiov1alpha1.WorkspaceApprovedAnnotation] == "true" {
				isApproved = true
			}

			if !isApproved {
				logger.Info("Workspace has planned successfully, but is pending approval to apply", "workspace", workspace.Name)
				r.updateStatus(ctx, workspace, magosprojectiov1alpha1.PhasePlanned, "PlanSucceeded", "Terraform Plan succeeded. Waiting for manual approval to Apply.", metav1.ConditionTrue)
				return nil
			}

			// Consume the approval annotation FIRST so it doesn't accidentally
			// auto-apply future runs
			if workspace.Annotations != nil {
				delete(workspace.Annotations, magosprojectiov1alpha1.WorkspaceApprovedAnnotation)
				if err := r.Update(ctx, workspace); err != nil {
					logger.Error(err, "Failed to consume approval annotation")
					return err
				}
			}

			logger.Info("Creating a new Apply Job", "job", applyJobName)
			newJob, err := r.constructJobForWorkspace(ctx, workspace, applyJobName, "apply", planFile, pvcName)
			if err != nil {
				return err
			}
			if err := r.Create(ctx, newJob); err != nil {
				return err
			}
			r.updateStatus(ctx, workspace, magosprojectiov1alpha1.PhaseApplying, "ApplyJobCreated", "Terraform Apply job created", metav1.ConditionUnknown)
			return nil
		}
		return err
	}

	// 4. Check Apply Job Status
	if applyJob.Status.Failed > 0 {
		logger.Info("Apply Job failed", "job", applyJobName)
		r.updateStatus(ctx, workspace, magosprojectiov1alpha1.PhaseFailed, "ApplyFailed", "Terraform Apply execution failed", metav1.ConditionFalse)
		return nil
	} else if applyJob.Status.Succeeded == 0 {
		logger.Info("Apply Job is currently running", "job", applyJobName)
		r.updateStatus(ctx, workspace, magosprojectiov1alpha1.PhaseApplying, "Applying", "Terraform Apply execution is running", metav1.ConditionUnknown)
		return nil
	}

	// 5. Apply Succeeded
	logger.Info("Apply Job completed successfully", "job", applyJobName)
	workspace.Status.ObservedRevision = workspace.Spec.Source.TargetRevision

	// Consume the allowed annotation upon successful completion
	if workspace.Annotations != nil {
		if _, ok := workspace.Annotations[magosprojectiov1alpha1.WorkspaceAllowedReconcileAnnotation]; ok {
			delete(workspace.Annotations, magosprojectiov1alpha1.WorkspaceAllowedReconcileAnnotation)
			if err := r.Update(ctx, workspace); err != nil {
				logger.Error(err, "Failed to consume allowed annotation")
				return err
			}
		}
	}

	r.updateStatus(ctx, workspace, magosprojectiov1alpha1.PhaseApplied, "ApplySucceeded", "Terraform Apply completed successfully", metav1.ConditionTrue)

	return nil
}

func (r *WorkspaceReconciler) ensurePVC(ctx context.Context, ws *magosprojectiov1alpha1.Workspace, pvcName string) error {
	pvc := &corev1.PersistentVolumeClaim{}
	err := r.Get(ctx, types.NamespacedName{Name: pvcName, Namespace: ws.Namespace}, pvc)

	if err != nil && errors.IsNotFound(err) {
		log.FromContext(ctx).Info("Creating PVC for Workspace state persistence", "pvc", pvcName)

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

		if err := ctrl.SetControllerReference(ws, newPVC, r.Scheme); err != nil {
			return err
		}

		return r.Create(ctx, newPVC)
	}
	return err
}

func (r *WorkspaceReconciler) constructJobForWorkspace(ctx context.Context, ws *magosprojectiov1alpha1.Workspace, jobName, jobType, planFile, pvcName string) (*batchv1.Job, error) {
	envVars := []corev1.EnvVar{
		{Name: "REPO_URL", Value: ws.Spec.Source.RepoURL},
		{Name: "TARGET_REVISION", Value: ws.Spec.Source.TargetRevision},
		{Name: "TF_VERSION", Value: ws.Spec.Terraform.Version},
		{Name: "PROJECT_REF", Value: ws.Spec.ProjectRef.Name},
		{Name: "MAGOS_JOB_TYPE", Value: jobType}, // "plan" or "apply"
		{Name: "MAGOS_PLAN_FILE", Value: planFile},
	}

	if ws.Spec.Source.Path != "" {
		envVars = append(envVars, corev1.EnvVar{Name: "TF_PATH", Value: ws.Spec.Source.Path})
	}
	if ws.Spec.Terraform.TfvarsPath != "" {
		envVars = append(envVars, corev1.EnvVar{Name: "TF_VAR_FILE", Value: ws.Spec.Terraform.TfvarsPath})
	}

	// Pass Git credentials if available
	authSecret, err := r.getRepoCredentials(ctx, ws.Namespace, ws.Spec.Source.RepoURL)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve repository credentials: %w", err)
	}

	if authSecret != nil {
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

	var backoffLimit int32 = 0                // Don't retry blindly, especially if terraform fails
	var ttlSecondsAfterFinished int32 = 86400 // 24 hours

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: ws.Namespace,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoffLimit,
			TTLSecondsAfterFinished: &ttlSecondsAfterFinished,
			Template: corev1.PodTemplateSpec{
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

	// Set the Workspace as the owner of the Job.
	if err := ctrl.SetControllerReference(ws, job, r.Scheme); err != nil {
		return nil, err
	}

	return job, nil
}

func (r *WorkspaceReconciler) updateStatus(ctx context.Context, workspace *magosprojectiov1alpha1.Workspace, phase magosprojectiov1alpha1.Phase, reason, message string, status metav1.ConditionStatus) {
	// Fetch the latest version to avoid conflict errors
	latest := &magosprojectiov1alpha1.Workspace{}
	if err := r.Get(ctx, client.ObjectKeyFromObject(workspace), latest); err != nil {
		log.FromContext(ctx).Error(err, "Failed to get latest workspace for status update")
		return
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
		Type:               magosprojectiov1alpha1.ConditionTypeReady,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: now,
	}

	if meta.SetStatusCondition(&latest.Status.Conditions, condition) {
		needsUpdate = true
	}

	if !needsUpdate {
		return
	}

	latest.Status.LastReconcileTime = &now

	if err := r.Status().Update(ctx, latest); err != nil {
		log.FromContext(ctx).Error(err, "Failed to update workspace status")
		return
	}

	// Update the original object so the caller has the latest state
	workspace.Status = latest.Status
	workspace.ResourceVersion = latest.ResourceVersion
}

// SetupWithManager sets up the controller with the Manager.
func (r *WorkspaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&magosprojectiov1alpha1.Workspace{}).
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
