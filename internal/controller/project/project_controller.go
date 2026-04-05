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

package project

import (
	"context"
	"time"

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

	magosprojectiov1alpha1 "github.com/magosproject/magos/api/v1alpha1"
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

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here: -
// https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.22.1/pkg/reconcile
func (r *ProjectReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the Project instance
	project := &magosprojectiov1alpha1.Project{}
	if err := r.Get(ctx, req.NamespacedName, project); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Project resource not found, ignoring")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get Project")
		return ctrl.Result{}, err
	}

	if controllerutil.AddFinalizer(project, magosprojectiov1alpha1.ProjectFinalizerName) {
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
		// Finalizer already removed but project is still there
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	err := r.reconcileProject(ctx, project)
	if err != nil {
		r.updateStatus(ctx, project, magosprojectiov1alpha1.PhaseFailed, "ReconcileError", err.Error(), metav1.ConditionFalse)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *ProjectReconciler) handleDeletion(ctx context.Context, project *magosprojectiov1alpha1.Project) (bool, error) {
	logger := log.FromContext(ctx)
	logger.Info("Handling project deletion")

	r.updateStatus(ctx, project, magosprojectiov1alpha1.PhaseDeleting, "Deleting", "Project is being deleted", metav1.ConditionFalse)

	// Since Projects are a pure grouping mechanism and don't execute logic,
	// there typically aren't external resources to clean up. Downstream
	// workspaces should handle their own destruction when deleted.

	if controllerutil.ContainsFinalizer(project, magosprojectiov1alpha1.ProjectFinalizerName) {
		logger.Info("Removing finalizer")
		controllerutil.RemoveFinalizer(project, magosprojectiov1alpha1.ProjectFinalizerName)
		if err := r.Update(ctx, project); err != nil {
			return false, err
		}
	}
	return true, nil
}

func (r *ProjectReconciler) reconcileProject(ctx context.Context, project *magosprojectiov1alpha1.Project) error {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling project")

	// Fast track to ready
	r.updateStatus(ctx, project, magosprojectiov1alpha1.PhaseReady, "ProjectReady", "Project grouping is available", metav1.ConditionTrue)

	// 1. Check if a Rollout exists for this Project (1-to-1 mapping via name)
	rollout := &magosprojectiov1alpha1.Rollout{}
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

	// 2. If a Rollout exists, we defer all execution orchestration to it.
	if hasRollout {
		logger.Info("Project is managed by a Rollout. Deferring workspace orchestration.", "rollout", rollout.Name)
		return nil
	}

	// 3. Default Parallel Execution: No Rollout exists.
	// We act as the default orchestrator and grant execution permission to all workspaces that need to reconcile.
	var workspaces magosprojectiov1alpha1.WorkspaceList
	if err := r.List(ctx, &workspaces, client.InNamespace(project.Namespace)); err != nil {
		logger.Error(err, "Failed to list workspaces")
		return err
	}

	for i := range workspaces.Items {
		ws := &workspaces.Items[i]
		if ws.Spec.ProjectRef.Name != project.Name {
			continue
		}

		isAllowed := false
		if ws.Annotations != nil {
			isAllowed = ws.Annotations[magosprojectiov1alpha1.WorkspaceAllowedReconcileAnnotation] == "true"
		}

		if isAllowed {
			continue // Already has permission
		}

		needsPermission := false
		if ws.Status.Phase == "" || ws.Status.Phase == magosprojectiov1alpha1.PhasePending {
			needsPermission = true // New workspace or waiting for permission
		} else if ws.Status.ObservedRevision != ws.Spec.Source.TargetRevision {
			needsPermission = true // Target revision changed
		} else if ws.Annotations != nil && ws.Annotations[magosprojectiov1alpha1.WorkspaceReconcileRequestAnnotation] != "" {
			needsPermission = true // Manual forced reconcile requested
		}

		if needsPermission {
			logger.Info("No Rollout detected. Granting default parallel execution permission to Workspace.", "workspace", ws.Name)

			// We need to fetch a fresh copy of the Workspace right before applying the annotation.
			// Since the Workspace controller is constantly updating its status (which bumps the
			// ResourceVersion), the version we grabbed from our initial List call is probably
			// stale. If we try to update the cached copy, we'll hit an Optimistic Concurrency
			// Control (OCC) conflict and trigger a requeue storm.
			latestWS := &magosprojectiov1alpha1.Workspace{}
			if err := r.Get(ctx, client.ObjectKeyFromObject(ws), latestWS); err == nil {
				if latestWS.Annotations == nil {
					latestWS.Annotations = make(map[string]string)
				}
				latestWS.Annotations[magosprojectiov1alpha1.WorkspaceAllowedReconcileAnnotation] = "true"
				if err := r.Update(ctx, latestWS); err != nil {
					logger.Error(err, "Failed to grant execution permission to workspace", "workspace", ws.Name)
				}
			}
		}
	}

	return nil
}

func (r *ProjectReconciler) updateStatus(ctx context.Context, project *magosprojectiov1alpha1.Project, phase magosprojectiov1alpha1.Phase, reason, message string, status metav1.ConditionStatus) {
	// Fetch the latest version of the project to avoid conflict errors
	latest := &magosprojectiov1alpha1.Project{}
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
		log.FromContext(ctx).Error(err, "Failed to update project status")
		return
	}

	// Update the original object so the caller has the latest state
	project.Status = latest.Status
	project.ResourceVersion = latest.ResourceVersion
}

func (r *ProjectReconciler) findProjectsForWorkspace(ctx context.Context, o client.Object) []reconcile.Request {
	ws, ok := o.(*magosprojectiov1alpha1.Workspace)
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

func (r *ProjectReconciler) findProjectsForRollout(ctx context.Context, o client.Object) []reconcile.Request {
	ro, ok := o.(*magosprojectiov1alpha1.Rollout)
	if !ok {
		return nil
	}
	// Rollout names strictly map 1-to-1 to Project names
	return []reconcile.Request{
		{
			NamespacedName: types.NamespacedName{
				Name:      ro.Name,
				Namespace: ro.Namespace,
			},
		},
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *ProjectReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&magosprojectiov1alpha1.Project{}).
		Watches(
			&magosprojectiov1alpha1.Workspace{},
			handler.EnqueueRequestsFromMapFunc(r.findProjectsForWorkspace),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		).
		Watches(
			&magosprojectiov1alpha1.Rollout{},
			handler.EnqueueRequestsFromMapFunc(r.findProjectsForRollout),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		).
		Named("project").
		Complete(r)
}
