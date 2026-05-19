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
	"os"
	"strings"
	"time"

	"github.com/magosproject/magos/internal/controller/variableset"
	"github.com/magosproject/magos/internal/logstore"
	"github.com/magosproject/magos/types/magosproject/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
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

// normalizeRepoURL trims whitespace, a trailing slash, and a single .git
// suffix so URLs that differ only in style match as the same repository.
func normalizeRepoURL(u string) string {
	u = strings.TrimSpace(u)
	u = strings.TrimSuffix(u, "/")
	u = strings.TrimSuffix(u, ".git")
	u = strings.TrimSuffix(u, "/")
	return u
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

	err := r.List(ctx, &secretList,
		client.InNamespace(namespace),
		client.MatchingLabels{RepoSecretLabelKey: RepoSecretLabelValue},
	)
	if err != nil {
		return nil, err
	}

	target := normalizeRepoURL(targetRepoURL)
	for i := range secretList.Items {
		secret := &secretList.Items[i]
		if normalizeRepoURL(string(secret.Data[SecretKeyRepoURL])) == target {
			return secret, nil
		}
	}

	return nil, nil
}

// findWorkspacesForSecret maps Secret watch events to Workspace reconcile
// requests.
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

	target := normalizeRepoURL(string(repoURL))
	requests := make([]reconcile.Request, 0, len(workspaces.Items))
	for _, ws := range workspaces.Items {
		if normalizeRepoURL(ws.Spec.Source.RepoURL) == target {
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
// +kubebuilder:rbac:groups=magosproject.io,resources=projects,verbs=get;list;watch
// +kubebuilder:rbac:groups=magosproject.io,resources=variablesets,verbs=get;list;watch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch
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
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	res, err := r.reconcileWorkspace(ctx, workspace)
	if err != nil {
		reconcileTotal.WithLabelValues(req.Namespace, req.Name, "error").Inc()
		r.updateStatus(ctx, workspace, v1alpha1.PhaseFailed, "ReconcileError", err.Error(), metav1.ConditionFalse)
		return ctrl.Result{}, err
	}

	reconcileTotal.WithLabelValues(req.Namespace, req.Name, "success").Inc()

	nextReconcileTime, reconcileInterval, _ := computeNextReconcileTime(workspace, workspace.Status.NextReconcileTime)
	r.updateNextReconcileTime(ctx, workspace, nextReconcileTime, reconcileInterval)
	res = withScheduledRequeue(res, nextReconcileTime.Time)

	return res, nil
}

// handleDeletion removes the finalizer from a Workspace that is being deleted.
func (r *WorkspaceReconciler) handleDeletion(ctx context.Context, workspace *v1alpha1.Workspace) (bool, error) {
	logger := log.FromContext(ctx)
	logger.Info("Handling workspace deletion")

	r.updateStatus(ctx, workspace, v1alpha1.PhaseDeleting, "Deleting", "Workspace is being deleted", metav1.ConditionFalse)

	if controllerutil.ContainsFinalizer(workspace, v1alpha1.WorkspaceFinalizerName) {
		logger.Info("Removing finalizer")
		controllerutil.RemoveFinalizer(workspace, v1alpha1.WorkspaceFinalizerName)
		if err := r.Update(ctx, workspace); err != nil {
			return false, err
		}
	}
	return true, nil
}

// runContext holds the derived names and current job state for a single
// reconcile iteration.
type runContext struct {
	planJobName   string
	applyJobName  string
	planFile      string
	pvcName       string
	planJob       *batchv1.Job
	applyJob      *batchv1.Job
	resolvedVars  []variableset.ResolvedVariable
	variablesHash string
}

func (r *WorkspaceReconciler) newRunContext(ctx context.Context, workspace *v1alpha1.Workspace) (runContext, error) {
	specHash := r.getSpecHash(workspace)
	rc := runContext{
		planJobName:  fmt.Sprintf("%s-plan-%s", workspace.Name, specHash),
		applyJobName: fmt.Sprintf("%s-apply-%s", workspace.Name, specHash),
		planFile:     fmt.Sprintf("/workspace-data/run-%s.tfplan", specHash),
		pvcName:      fmt.Sprintf("%s-data", workspace.Name),
	}
	var err error
	rc.planJob, err = r.getJobOrNil(ctx, rc.planJobName, workspace.Namespace)
	if err != nil {
		return rc, err
	}
	rc.applyJob, err = r.getJobOrNil(ctx, rc.applyJobName, workspace.Namespace)
	if err != nil {
		return rc, err
	}
	return rc, nil
}

func (r *WorkspaceReconciler) getJobOrNil(ctx context.Context, name, namespace string) (*batchv1.Job, error) {
	var job batchv1.Job
	if err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, &job); err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &job, nil
}

func (r *WorkspaceReconciler) reconcileWorkspace(ctx context.Context, workspace *v1alpha1.Workspace) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling Workspace", "name", workspace.Name, "namespace", workspace.Namespace)

	defer r.trackActiveWorkspaces(ctx)

	rc, err := r.newRunContext(ctx, workspace)
	if err != nil {
		return ctrl.Result{}, err
	}
	r.cleanupOrphanedJobs(ctx, workspace, rc)

	resolvedVars, unresolved, err := r.resolveWorkspaceVariables(ctx, workspace)
	if err != nil {
		return ctrl.Result{}, err
	}
	if len(unresolved) > 0 {
		message := fmt.Sprintf("%d variable reference(s) unresolved: %s", len(unresolved), formatUnresolved(unresolved))
		r.updateStatus(ctx, workspace, v1alpha1.PhaseFailed, "UnresolvedVariables", message, metav1.ConditionFalse)
		return ctrl.Result{}, nil
	}
	rc.resolvedVars = resolvedVars
	rc.variablesHash = variableset.Fingerprint(resolvedVars)

	d := r.checkCycleNeeded(workspace, rc)
	if d.start {
		return ctrl.Result{}, r.startFreshCycle(ctx, workspace, rc, d.reason, d.message)
	}

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

	if err := r.ensurePVC(ctx, workspace, rc.pvcName); err != nil {
		logger.Error(err, "Failed to ensure PVC exists")
		return ctrl.Result{}, err
	}

	res, planDone, err := r.reconcilePlanJob(ctx, workspace, rc)
	if err != nil || !planDone {
		return res, err
	}

	return ctrl.Result{}, r.reconcileApplyJob(ctx, workspace, rc)
}

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
	if defaultPVCSize := os.Getenv(envWorkspacePVCSizeDefault); defaultPVCSize != "" {
		if _, err := resource.ParseQuantity(defaultPVCSize); err != nil {
			return fmt.Errorf("invalid %s %q: %w", envWorkspacePVCSizeDefault, defaultPVCSize, err)
		}
	}
	if _, err := r.resolveWorkspaceJobResources(); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Workspace{}).
		Owns(&batchv1.Job{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Watches(
			&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(r.findWorkspacesForSecret),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		).
		Watches(
			&v1alpha1.VariableSet{},
			handler.EnqueueRequestsFromMapFunc(r.findWorkspacesForVariableSet),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		).
		Named("workspace").
		Complete(r)
}
