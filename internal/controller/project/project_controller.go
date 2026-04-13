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

package project

import (
	"context"
	"fmt"
	"time"

	"github.com/magosproject/magos/types/magosproject/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
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
)

// ProjectReconciler reconciles a Project object
type ProjectReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=magosproject.io,resources=projects,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=magosproject.io,resources=projects/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=magosproject.io,resources=projects/finalizers,verbs=update
// +kubebuilder:rbac:groups=magosproject.io,resources=rollouts,verbs=get;list;watch
// +kubebuilder:rbac:groups=magosproject.io,resources=workspaces,verbs=get;list;watch;update;patch

// Reconcile is the top-level entry point invoked by controller-runtime whenever
// a Project or one of its watched dependents (Workspaces, Rollouts) changes.
func (r *ProjectReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the Project instance
	project := &v1alpha1.Project{}
	if err := r.Get(ctx, req.NamespacedName, project); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Project resource not found, ignoring")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get Project")
		return ctrl.Result{}, err
	}

	// Ensure a finalizer is present so Kubernetes delays actual deletion until
	// we explicitly remove it. This guarantees the controller gets a chance to
	// run handleDeletion before the object disappears, even if someone deletes
	// the Project manually via kubectl.
	if controllerutil.AddFinalizer(project, v1alpha1.ProjectFinalizerName) {
		if err := r.Update(ctx, project); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if !project.DeletionTimestamp.IsZero() {
		finished, err := r.handleDeletion(ctx, project)
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

	err := r.reconcileProject(ctx, project)
	if err != nil {
		reconcileTotal.WithLabelValues(req.Namespace, req.Name, "error").Inc()
		r.updateStatus(ctx, project, v1alpha1.PhaseFailed, "ReconcileError", err.Error(), metav1.ConditionFalse)
		return ctrl.Result{}, err
	}

	reconcileTotal.WithLabelValues(req.Namespace, req.Name, "success").Inc()
	return ctrl.Result{}, nil
}

// handleDeletion removes the finalizer from a Project that is being deleted.
// Projects serve strictly as logical grouping boundaries and do not provision
// external infrastructure. Resource cleanup is delegated to the child
// Workspaces via standard Kubernetes garbage collection and OwnerReferences, so
// all we need to do here is remove our finalizer so that Kubernetes can proceed
// with the actual deletion.
func (r *ProjectReconciler) handleDeletion(ctx context.Context, project *v1alpha1.Project) (bool, error) {
	logger := log.FromContext(ctx)
	logger.Info("Handling project deletion")

	r.updateStatus(ctx, project, v1alpha1.PhaseDeleting, "Deleting", "Project is being deleted", metav1.ConditionFalse)

	if controllerutil.ContainsFinalizer(project, v1alpha1.ProjectFinalizerName) {
		logger.Info("Removing finalizer")
		controllerutil.RemoveFinalizer(project, v1alpha1.ProjectFinalizerName)
		if err := r.Update(ctx, project); err != nil {
			return false, err
		}
	}
	return true, nil
}

func (r *ProjectReconciler) reconcileProject(ctx context.Context, project *v1alpha1.Project) error {
	logger := log.FromContext(ctx)
	logger.V(1).Info("Reconciling project")

	// Step 1: Evaluate Rollout delegation.
	//
	// By default a Project executes all associated Workspaces in parallel.
	// However, if a Rollout exists with a matching name of the Project, the
	// Project defers all execution orchestration to the Rollout's defined
	// strategy.
	//
	// The Rollout name is expected to match the Project name exactly (1-to-1
	// mapping).
	rollout := &v1alpha1.Rollout{}
	err := r.Get(ctx, types.NamespacedName{Name: project.Name, Namespace: project.Namespace}, rollout)
	hasRollout := true
	if err != nil {
		if errors.IsNotFound(err) {
			hasRollout = false
		} else {
			logger.Error(err, "Failed to check for existing Rollout")
			return err
		}
	}

	// Step 2: Defer to explicit Rollout strategy.
	//
	// If a Rollout exists, we defer all Workspace orchestration to it. The
	// Project status is updated to reflect this delegation, and the Rollout
	// controller takes over managing execution permissions, setting the allowed
	// annotation on Workspaces according to its defined strategy.
	if hasRollout {
		logger.V(1).Info("Project is managed by a Rollout. Deferring workspace orchestration.", "rollout", rollout.Name)
		managedByRollout.WithLabelValues(project.Namespace, project.Name).Set(1)
		r.updateStatus(ctx, project, v1alpha1.PhaseReady, "ManagedByRollout", fmt.Sprintf("Project orchestration is deferred to Rollout %s", rollout.Name), metav1.ConditionTrue)

		// Since the Rollout controller is now responsible for granting
		// execution permissions to Workspaces, we need to revoke any default
		// parallel execution permissions that may have been granted previously.
		//
		// TODO: https://github.com/magosproject/magos/issues/7
		return nil
	}

	managedByRollout.WithLabelValues(project.Namespace, project.Name).Set(0)
	r.updateStatus(ctx, project, v1alpha1.PhaseReady, "DefaultParallel", "Project is orchestrating workspaces in parallel", metav1.ConditionTrue)

	// Step 3: Enforce default parallel execution.
	//
	// In the absence of a Rollout, the Project acts as the default
	// orchestrator. It discovers all Workspaces in the same namespace that
	// reference this Project and universally grants them execution permission
	// by setting the execution-allowed annotation, allowing Terraform
	// operations to proceed concurrently.
	var workspaces v1alpha1.WorkspaceList
	if err := r.List(ctx, &workspaces, client.InNamespace(project.Namespace)); err != nil {
		logger.Error(err, "Failed to list workspaces")
		return err
	}

	// Count workspaces that reference this project for the gauge.
	var wsCount float64
	for i := range workspaces.Items {
		if workspaces.Items[i].Spec.ProjectRef.Name == project.Name {
			wsCount++
		}
	}
	workspaceCount.WithLabelValues(project.Namespace, project.Name).Set(wsCount)

	for i := range workspaces.Items {
		ws := &workspaces.Items[i]
		if ws.Spec.ProjectRef.Name != project.Name {
			continue
		}

		// Skip Workspaces that already have execution permission. Re-applying
		// the permission would not change anything, but it would still result
		// in an unnecessary API update and another reconcile of the Workspace.
		isAllowed := false
		if ws.Annotations != nil {
			isAllowed = ws.Annotations[v1alpha1.WorkspaceExecutionAllowedAnnotation] == v1alpha1.AnnotationValueTrue
		}

		if isAllowed {
			continue
		}

		// Determine whether this Workspace has pending work that warrants
		// starting a new execution cycle. We only grant execution permission
		// when the Workspace is new/pending, when the RefWatcher detected a
		// new commit, or when a manual reconcile was requested via annotation.
		hasPendingWork := false
		if ws.Status.Phase == "" || ws.Status.Phase == v1alpha1.PhasePending {
			hasPendingWork = true
		} else if ws.Annotations != nil && ws.Annotations[v1alpha1.WorkspaceDetectedRevisionAnnotation] != "" {
			hasPendingWork = true
		} else if ws.Annotations != nil && ws.Annotations[v1alpha1.WorkspaceReconcileRequestAnnotation] != "" {
			hasPendingWork = true
		}

		if hasPendingWork {
			logger.Info("No Rollout detected. Granting default parallel execution permission to Workspace.", "workspace", ws.Name)

			// Fetch the latest Workspace before updating annotations to avoid
			// conflicts. Other controllers (e.g., Workspace controller) may have
			// updated its status, which increments ResourceVersion, so using a
			// stale object would cause the Update to fail.
			latestWS := &v1alpha1.Workspace{}
			if err := r.Get(ctx, client.ObjectKeyFromObject(ws), latestWS); err == nil {
				if latestWS.Annotations == nil {
					latestWS.Annotations = make(map[string]string)
				}
				latestWS.Annotations[v1alpha1.WorkspaceExecutionAllowedAnnotation] = v1alpha1.AnnotationValueTrue
				if err := r.Update(ctx, latestWS); err != nil {
					logger.Error(err, "Failed to grant execution permission to workspace", "workspace", ws.Name)
				}
			}
		}
	}

	return nil
}

// updateStatus writes the phase, reason, message, and Ready condition to the
// Project's status subresource. To avoid conflicts from concurrent updates,
// it always fetches the latest version of the Project before writing.
//
// After a successful update, the Project object passed into updateStatus is
// updated in-place with the new resourceVersion and status. This ensures that
// any subsequent logic in the same reconcile loop operates on the latest state.
func (r *ProjectReconciler) updateStatus(ctx context.Context, project *v1alpha1.Project, phase v1alpha1.Phase, reason, message string, status metav1.ConditionStatus) {
	// Fetch the latest version of the project to avoid conflict errors
	latest := &v1alpha1.Project{}
	if err := r.Get(ctx, client.ObjectKeyFromObject(project), latest); err != nil {
		log.FromContext(ctx).Error(err, "Failed to get latest project for status update")
		return
	}

	needsUpdate := false

	if latest.Status.Phase != phase || latest.Status.Reason != reason || latest.Status.Message != message {
		latest.Status.Phase = phase
		latest.Status.Reason = reason
		latest.Status.Message = message
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
		return
	}

	latest.Status.LastReconcileTime = &now

	if err := r.Status().Update(ctx, latest); err != nil {
		log.FromContext(ctx).Error(err, "Failed to update project status")
		return
	}

	// Update the original object so the caller has the latest state
	project.Status = latest.Status
	project.ResourceVersion = latest.ResourceVersion
}

// findProjectsForWorkspace maps Workspace watch events to Project reconcile
// requests.
//
// Workspaces reference their parent Project via spec.projectRef.name. When a
// Workspace changes (e.g. its status phase transitions from Planning to
// Planned), the Project controller needs to re-evaluate whether to grant
// execution permission to other Workspaces. This mapper ensures that any
// Workspace change triggers a reconcile of the Project it belongs to.
func (r *ProjectReconciler) findProjectsForWorkspace(ctx context.Context, o client.Object) []reconcile.Request {
	ws, ok := o.(*v1alpha1.Workspace)
	if !ok {
		return nil
	}
	return []reconcile.Request{
		{
			NamespacedName: types.NamespacedName{
				Name:      ws.Spec.ProjectRef.Name,
				Namespace: ws.Namespace,
			},
		},
	}
}

// findProjectsForRollout maps Rollout watch events to Project reconcile
// requests.
//
// Rollout names strictly map 1-to-1 to Project names. When a Rollout is
// created, updated, or deleted, the corresponding Project needs to re-evaluate
// its orchestration strategy. For example, if a Rollout is deleted, the Project
// should fall back to its default parallel execution mode. If a Rollout is
// created, the Project should stop granting execution permissions directly and
// defer to the Rollout controller instead.
func (r *ProjectReconciler) findProjectsForRollout(ctx context.Context, o client.Object) []reconcile.Request {
	ro, ok := o.(*v1alpha1.Rollout)
	if !ok {
		return nil
	}
	return []reconcile.Request{
		{
			NamespacedName: types.NamespacedName{
				Name:      ro.Name,
				Namespace: ro.Namespace,
			},
		},
	}
}

// SetupWithManager registers the Project controller with the Manager and
// configures the watches that trigger reconciliation.
func (r *ProjectReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Project{}).
		Watches( // Watch for changes to Workspaces that reference a Project
			&v1alpha1.Workspace{},
			handler.EnqueueRequestsFromMapFunc(r.findProjectsForWorkspace),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		).
		Watches( // Watch for Rollout creation/deletion to toggle orchestration mode
			&v1alpha1.Rollout{},
			handler.EnqueueRequestsFromMapFunc(r.findProjectsForRollout),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		).
		Named("project").
		Complete(r)
}
