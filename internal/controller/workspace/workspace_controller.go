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

	"github.com/magosproject/magos/internal/logstore"
	"github.com/magosproject/magos/types/magosproject/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
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
	Scheme      *runtime.Scheme
	JobImage    string
	Clientset   kubernetes.Interface // for reading pod logs
	LogStore    logstore.Store
	RunRecorder RunRecorder
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
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=pods/log,verbs=get

// Reconcile is the top-level entry point invoked by controller-runtime whenever
// a Workspace or one of its watched dependents (Jobs, PVCs, Secrets) changes.
func (r *WorkspaceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

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
		reconcileTotal.WithLabelValues(req.Namespace, req.Name, "error").Inc()
		r.updateStatus(ctx, workspace, v1alpha1.PhaseFailed, "ReconcileError", err.Error(), metav1.ConditionFalse)
		return ctrl.Result{}, err
	}

	reconcileTotal.WithLabelValues(req.Namespace, req.Name, "success").Inc()

	// Always requeue on the periodic schedule so we periodically re-plan even
	// when nothing in the cluster changes. This is how we detect
	// infrastructure drift that happened outside of Magos.
	nextReconcileTime, reconcileInterval, _ := computeNextReconcileTime(workspace, workspace.Status.NextReconcileTime)
	r.updateNextReconcileTime(ctx, workspace, nextReconcileTime, reconcileInterval)
	// Calculate RequeueAfter after status writes. controller-runtime starts the
	// timer only after Reconcile returns, so doing this earlier would add status
	// update latency to every scheduled cycle.
	res = withScheduledRequeue(res, nextReconcileTime.Time)

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

// reconcileWorkspace is the main reconciliation loop for a single Workspace. It
// is called by Reconcile after the object is fetched and finalizer is ensured.
//
// The function is structured as a linear sequence of steps:
//
//  1. Build deterministic Job names from a hash of the Workspace spec.
//  2. Clean up Jobs left over from a previous spec version.
//  3. Decide whether to start a fresh plan/apply cycle (spec change, new revision,
//     manual request, or scheduled reconciliation).
//  4. Check whether the Rollout controller has granted execution permission.
//  5. Ensure the shared PVC exists.
//  6. Run "terraform plan".
//  7. Run "terraform apply" (requires approval).
//  8. Record the apply result and release the execution lock.
func (r *WorkspaceReconciler) reconcileWorkspace(ctx context.Context, workspace *v1alpha1.Workspace) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling Workspace", "name", workspace.Name, "namespace", workspace.Namespace)

	defer r.trackActiveWorkspaces(ctx)

	// Step 1: Build Job names from a hash of the Workspace spec.
	//
	// Each Workspace reconciliation produces a Plan Job and an Apply Job. The
	// Kubernetes jobs are suffixed with a short hash (specHash) so that a
	// given Apply always runs against the exact plan file that was generated
	// for the same spec. When someone changes the spec, the hash changes and we
	// get a new pair of Jobs. Importantly, approving a plan does not change the
	// hash (the approval annotation is not part of the spec) so the Apply Job
	// is guaranteed to execute the plan that was reviewed and approved.
	specHash := r.getSpecHash(workspace)
	planJobName := fmt.Sprintf("%s-plan-%s", workspace.Name, specHash)
	applyJobName := fmt.Sprintf("%s-apply-%s", workspace.Name, specHash)
	planFile := fmt.Sprintf("/workspace-data/run-%s.tfplan", specHash)

	// Step 2: Clean up Jobs left over from a previous spec version.
	r.cleanupOrphanedJobs(ctx, workspace, planJobName, applyJobName)

	// Look up the current Plan and Apply Jobs. A NotFound error is normal and
	// just means the Job hasn't been created yet for this specHash.
	var planJob batchv1.Job
	planJobGetErr := r.Get(ctx, types.NamespacedName{Name: planJobName, Namespace: workspace.Namespace}, &planJob)

	var applyJob batchv1.Job
	applyJobGetErr := r.Get(ctx, types.NamespacedName{Name: applyJobName, Namespace: workspace.Namespace}, &applyJob)

	// Step 3: Decide whether we need to start a fresh Plan/Apply cycle.
	//
	// This logic must run before Step 4. Step 4 evaluates the Rollout execution
	// lock annotation (magosproject.io/execution-allowed). The Rollout
	// controller adds that annotation to allow a Workspace to execute, and
	// removes it again once the Workspace finishes.
	//
	// If we checked the execution lock first, a completed Workspace could
	// appear "not allowed" and we would never reach this reset path. That would
	// leave the Workspace stuck in a terminal(?) phase with no way to clean up
	// old Jobs or start a new cycle.
	d := r.checkCycleNeeded(workspace, &planJob, &applyJob, planJobGetErr, applyJobGetErr)
	if d.start {
		return r.startFreshCycle(ctx, workspace, &planJob, &applyJob, planJobGetErr, applyJobGetErr, d.reason, d.message)
	}

	// Step 4: Check whether the Rollout controller has granted us permission to
	// execute.
	//
	// A Rollout groups multiple Workspaces and controls the order they run in
	// (e.g. "dev must succeed before prod starts"). It does this by setting the
	// execution-allowed annotation on each Workspace when it is that
	// Workspace's turn. If the annotation is absent or not "true", it means the
	// Rollout controller hasn't reached that Workspace yet, so we stay in
	// Pending and return early. The Rollout controller will trigger a new
	// reconcile once it sets the annotation.
	if !isExecutionAllowedByRolloutController(workspace) {
		logger.Info("Workspace execution is not allowed. Waiting for rollout controller to grant permission.", "workspace", workspace.Name)
		if workspace.Status.Phase == "" {
			r.updateStatus(ctx, workspace, v1alpha1.PhasePending, "AwaitingRollout", "Waiting for the Rollout controller to schedule this Workspace for execution", metav1.ConditionUnknown)
		}
		if d.requeue > 0 {
			return ctrl.Result{RequeueAfter: d.requeue}, nil
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
	// deleted. The creation of the PVC happens only once in the lifetime of a
	// workspace via ensurePVC.
	pvcName := fmt.Sprintf("%s-data", workspace.Name)
	if err := r.ensurePVC(ctx, workspace, pvcName); err != nil {
		logger.Error(err, "Failed to ensure PVC exists")
		return ctrl.Result{}, err
	}

	// Step 6: Run "terraform plan".
	planResult, planDone, err := r.reconcilePlanJob(ctx, workspace, planJobName, planFile, pvcName, planJobGetErr, &planJob)
	if err != nil || !planDone {
		return planResult, err
	}

	// Steps 7+8: Run "terraform apply" (requires approval) and handle completion.
	// TODO: implement approval feature
	return r.reconcileApplyJob(ctx, workspace, applyJobName, planJobName, planFile, pvcName, applyJobGetErr, &planJob, &applyJob)
}

// trackActiveWorkspaces updates the Prometheus gauge for the number of
// Workspaces currently in the Planning or Applying phase. Called as a deferred
// function at the end of every reconcileWorkspace call.
func (r *WorkspaceReconciler) trackActiveWorkspaces(ctx context.Context) {
	var allWorkspaces v1alpha1.WorkspaceList
	if err := r.List(ctx, &allWorkspaces); err == nil {
		var active float64
		for _, ws := range allWorkspaces.Items {
			if ws.Status.Phase == v1alpha1.PhasePlanning || ws.Status.Phase == v1alpha1.PhaseApplying {
				active++
			}
		}
		activeCount.Set(active)
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
