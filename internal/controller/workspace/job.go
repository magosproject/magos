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
	"maps"

	"github.com/magosproject/magos/types/magosproject/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// getSpecHash produces a short, deterministic hash of the Workspace spec. This
// hash is used as a suffix on Job names (e.g. "myworkspace-plan-a1b2c3d4") so
// that a spec change naturally creates new Jobs while leaving old ones to be
// cleaned up by Step 2. The approval annotation is deliberately excluded from
// the hash so that approving a plan does not invalidate the existing Plan Job.
func (r *WorkspaceReconciler) getSpecHash(ws *v1alpha1.Workspace) string {
	data, _ := json.Marshal(ws.Spec)
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])[:8] // Short 8-character hash
}

// ensurePVC checks whether the PVC for this Workspace already exists and
// creates it if not. The PVC uses ReadWriteOnce access mode because only one
// Job at a time needs to write to it (Plan writes, then Apply reads). We set
// the Workspace as the owner so the PVC is automatically deleted when the
// Workspace is removed.
//
// TODO: Have @fayusohenson verify the security model here.
// TODO: Look into having shared PVC for provider caching
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

// resolveEffectivePolicySelector determines the label selector string for
// ValidatingPolicy resources. The Workspace-level validation block takes
// precedence over the Project-level default. Returns an empty string when no
// policy validation should be performed.
func (r *WorkspaceReconciler) resolveEffectivePolicySelector(ctx context.Context, ws *v1alpha1.Workspace) string {
	if ws.Spec.Validation != nil && ws.Spec.Validation.PolicySelector != nil {
		sel, err := metav1.LabelSelectorAsSelector(ws.Spec.Validation.PolicySelector)
		if err != nil {
			log.FromContext(ctx).Error(err, "Invalid workspace validation.policySelector, skipping validation")
			return ""
		}
		return sel.String()
	}

	// Fall back to the parent Project's default.
	project := &v1alpha1.Project{}
	if err := r.Get(ctx, types.NamespacedName{Name: ws.Spec.ProjectRef.Name, Namespace: ws.Namespace}, project); err != nil {
		if !errors.IsNotFound(err) {
			log.FromContext(ctx).Error(err, "Failed to get parent Project for policy selector")
		}
		return ""
	}

	if project.Spec.Validation != nil && project.Spec.Validation.PolicySelector != nil {
		sel, err := metav1.LabelSelectorAsSelector(project.Spec.Validation.PolicySelector)
		if err != nil {
			log.FromContext(ctx).Error(err, "Invalid project validation.policySelector, skipping validation")
			return ""
		}
		return sel.String()
	}

	return ""
}

// constructJobForWorkspace builds a Kubernetes Job spec for either a "plan" or
// "apply" operation. The Job runs the magos-job container image which knows how
// to clone a Git repo, install the right Terraform version, and execute the
// requested operation.
//
// We pass all configuration to the container through environment variables.
// Plain values (repo URL, revision, terraform version, etc.) are set as literal
// env vars. Sensitive values (Git credentials) are injected via secretKeyRef so
// that Kubernetes resolves them at Pod startup from the referenced Secret, and
// we never have to copy secret data into the Job spec.
//
// The Job mounts the Workspace's shared PVC at /workspace-data. The Plan Job
// writes the .tfplan file there, and the Apply Job reads it back from the same
// path.
//
// We set backoffLimit to 0 so Kubernetes does not automatically retry a failed
// Job. Terraform failures (bad HCL, provider errors, state locks) are unlikely
// to resolve on a blind retry, and Step 3 in reconcileWorkspace already handles
// retries after a cooldown period.
//
// The Job is owned by the Workspace via SetControllerReference, so Kubernetes
// garbage collection will delete it when the Workspace is removed.
func (r *WorkspaceReconciler) constructJobForWorkspace(ctx context.Context, ws *v1alpha1.Workspace, jobName, jobType, planFile, pvcName, runID string) (*batchv1.Job, error) {
	// The below map holds configuration that every Job needs: where to clone
	// from, which revision to check out, which Terraform version to use, and
	// whether this is a "plan" or "apply" run.
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
	if logLevel := ws.Annotations[v1alpha1.WorkspaceTFLogLevelAnnotation]; logLevel != "" {
		envVars = append(envVars, corev1.EnvVar{Name: "MAGOS_TF_LOG_LEVEL", Value: logLevel})
	}

	// For plan jobs, resolve and pass the policy selector so the job can list
	// matching ValidatingPolicy resources and evaluate them against the plan.
	if jobType == jobTypePlan {
		if policySelector := r.resolveEffectivePolicySelector(ctx, ws); policySelector != "" {
			envVars = append(envVars, corev1.EnvVar{Name: "MAGOS_POLICY_SELECTOR", Value: policySelector})
		}
	}

	// Look up Git credentials for this repo URL. If a matching Secret exists in
	// the namespace we inject its values via secretKeyRef. This means the
	// actual secret data never appears in the Job spec; Kubernetes resolves it
	// at Pod startup.
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

	// Resolve the Job timeout. Per-phase TimeoutSeconds takes precedence,
	// otherwise fall back to the global default.
	timeout := DefaultJobTimeoutSeconds
	switch jobType {
	case jobTypePlan:
		if ws.Spec.Plan != nil && ws.Spec.Plan.TimeoutSeconds != nil {
			timeout = *ws.Spec.Plan.TimeoutSeconds
		}
	case jobTypeApply:
		if ws.Spec.Apply != nil && ws.Spec.Apply.TimeoutSeconds != nil {
			timeout = *ws.Spec.Apply.TimeoutSeconds
		}
	}

	// Merge shared annotations with per-phase overrides (phase wins on
	// conflict).
	var podAnnotations map[string]string
	if len(ws.Spec.Annotations) > 0 {
		podAnnotations = make(map[string]string, len(ws.Spec.Annotations))
		maps.Copy(podAnnotations, ws.Spec.Annotations)
	}
	var overrides map[string]string
	switch jobType {
	case jobTypePlan:
		if ws.Spec.Plan != nil {
			overrides = ws.Spec.Plan.Annotations
		}
	case jobTypeApply:
		if ws.Spec.Apply != nil {
			overrides = ws.Spec.Apply.Annotations
		}
	}
	if len(overrides) > 0 {
		if podAnnotations == nil {
			podAnnotations = make(map[string]string, len(overrides))
		}
		maps.Copy(podAnnotations, overrides)
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: ws.Namespace,
			Labels: map[string]string{
				"magosproject.io/workspace": ws.Name,
				"magosproject.io/job-type":  jobType,
				runIDLabelKey:               runID,
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:          &backoffLimit,
			ActiveDeadlineSeconds: &timeout,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: podAnnotations,
					Labels: map[string]string{
						"magosproject.io/workspace": ws.Name,
						"magosproject.io/job-type":  jobType,
						runIDLabelKey:               runID,
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy:      corev1.RestartPolicyNever,
					ServiceAccountName: ws.Spec.ServiceAccountName,
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
							Image:           r.JobImage,
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
