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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

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

	// A project is not an active executor, but rather a foundational grouping
	// layer that your Rollouts and Workspaces reference. It holds structural
	// metadata but delegates all active reconciliation to the downstream
	// controllers.

	// We can validate that the referenced VariableSets exist (optional layer of
	// safety) but generally we mark the project as ready since it acts as a
	// passive grouping mechanism.

	// Fast track to ready
	r.updateStatus(ctx, project, magosprojectiov1alpha1.PhaseReady, "ProjectReady", "Project grouping is available", metav1.ConditionTrue)

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

// SetupWithManager sets up the controller with the Manager.
func (r *ProjectReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&magosprojectiov1alpha1.Project{}).
		Named("project").
		Complete(r)
}
