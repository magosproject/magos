package controller

import (
	"context"
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	magosprojectiov1alpha1 "github.com/magosproject/magos/api/v1alpha1"
)

// WorkspaceReconciler reconciles a Workspace object
type WorkspaceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=magosproject.io,resources=workspaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=magosproject.io,resources=workspaces/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=magosproject.io,resources=workspaces/finalizers,verbs=update
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

func (r *WorkspaceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Fetch the Workspace instance
	var workspace magosprojectiov1alpha1.Workspace
	if err := r.Get(ctx, req.NamespacedName, &workspace); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log.Info("Reconciling Workspace", "name", workspace.Name, "namespace", workspace.Namespace)

	// Deterministic Job name for this specific Generation
	// We append "plan" and the generation to ensure each spec change gets a fresh run
	jobName := fmt.Sprintf("%s-plan-%d", workspace.Name, workspace.Generation)

	// Check if the Job already exists
	var job batchv1.Job
	err := r.Get(ctx, types.NamespacedName{Name: jobName, Namespace: workspace.Namespace}, &job)

	// If the Job does not exist, create it
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			log.Error(err, "Failed to get Job")
			return ctrl.Result{}, err
		}

		log.Info("Creating a new Job", "job", jobName)
		newJob, err := r.constructJobForWorkspace(ctx, &workspace, jobName)
		if err != nil {
			log.Error(err, "Failed to construct Job")
			return ctrl.Result{}, err
		}

		if err := r.Create(ctx, newJob); err != nil {
			log.Error(err, "Failed to create a Job")
			return ctrl.Result{}, err
		}

		// Job created successfully, exit and wait for it to run (we watch Jobs)
		return ctrl.Result{}, nil
	}

	// If the Job exists, monitor its status
	if job.Status.Succeeded > 0 {
		log.Info("Job completed successfully", "job", jobName)
		// TODO: Update Workspace status condition here to "PlanSucceeded"
	} else if job.Status.Failed > 0 {
		log.Info("Job failed", "job", jobName)
		// TODO: Update Workspace status condition here to "PlanFailed"
	} else {
		log.Info("Job is currently running", "job", jobName)
		// TODO: Update Workspace status condition here to "Planning"
	}

	return ctrl.Result{}, nil
}

func (r *WorkspaceReconciler) constructJobForWorkspace(ctx context.Context, ws *magosprojectiov1alpha1.Workspace, jobName string) (*batchv1.Job, error) {
	envVars := []corev1.EnvVar{
		{Name: "REPO_URL", Value: ws.Spec.Source.RepoURL},
		{Name: "TARGET_REVISION", Value: ws.Spec.Source.TargetRevision},
		{Name: "TF_VERSION", Value: ws.Spec.Terraform.Version},
	}

	if ws.Spec.Source.Path != "" {
		envVars = append(envVars, corev1.EnvVar{Name: "TF_PATH", Value: ws.Spec.Source.Path})
	}
	if ws.Spec.Terraform.TfvarsPath != "" {
		envVars = append(envVars, corev1.EnvVar{Name: "TF_VAR_FILE", Value: ws.Spec.Terraform.TfvarsPath})
	}

	// Pass Git credentials if SecretRef is provided
	if ws.Spec.Source.SecretRef != nil {
		// Instead of fetching the secret in the Reconciler, we mount it directly into the Job
		// using EnvVar sources! This is much more secure as the controller never sees the credentials.
		envVars = append(envVars,
			corev1.EnvVar{
				Name: "GIT_USERNAME",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: *ws.Spec.Source.SecretRef,
						Key:                  "username",
						Optional:             func(b bool) *bool { return &b }(true),
					},
				},
			},
			corev1.EnvVar{
				Name: "GIT_PASSWORD",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: *ws.Spec.Source.SecretRef,
						Key:                  "password",
						Optional:             func(b bool) *bool { return &b }(true),
					},
				},
			},
		)
	}

	var backoffLimit int32 = 0 // Don't retry blindly, especially if terraform fails

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: ws.Namespace,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backoffLimit,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:            "job",
							Image:           "magos-job:local",
							ImagePullPolicy: corev1.PullIfNotPresent,
							Env:             envVars,
						},
					},
				},
			},
		},
	}

	// Set the Workspace as the owner of the Job.
	// This ensures the Job is deleted when the Workspace is deleted,
	// and triggers a Reconcile when the Job status changes.
	if err := ctrl.SetControllerReference(ws, job, r.Scheme); err != nil {
		return nil, err
	}

	return job, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *WorkspaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&magosprojectiov1alpha1.Workspace{}).
		Owns(&batchv1.Job{}). // Watch for changes to Jobs owned by the Workspace
		Named("workspace").
		Complete(r)
}
