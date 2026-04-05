/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package rollout

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	magosprojectiov1alpha1 "github.com/magosproject/magos/api/v1alpha1"
)

// RolloutReconciler reconciles a Rollout object
type RolloutReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=magosproject.io,resources=rollouts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=magosproject.io,resources=rollouts/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=magosproject.io,resources=rollouts/finalizers,verbs=update
// +kubebuilder:rbac:groups=magosproject.io,resources=workspaces,verbs=get;list;watch;update;patch

func (r *RolloutReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	rollout := &magosprojectiov1alpha1.Rollout{}
	if err := r.Get(ctx, req.NamespacedName, rollout); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get Rollout")
		return ctrl.Result{}, err
	}

	if !rollout.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	err := r.reconcileRollout(ctx, rollout)
	if err != nil {
		r.updateStatus(ctx, rollout, magosprojectiov1alpha1.PhaseFailed, "ReconcileError", err.Error(), metav1.ConditionFalse)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *RolloutReconciler) reconcileRollout(ctx context.Context, rollout *magosprojectiov1alpha1.Rollout) error {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling Rollout", "name", rollout.Name)

	if len(rollout.Spec.Strategy.Steps) == 0 {
		r.updateStatus(ctx, rollout, magosprojectiov1alpha1.PhaseReady, "NoSteps", "No steps defined in strategy", metav1.ConditionTrue)
		return nil
	}

	if rollout.Status.Phase == magosprojectiov1alpha1.PhaseFailed {
		// Rollout is failed, halt execution.
		return nil
	}

	// We iterate through steps sequentially to find the *first* step that has pending work.
	// This makes the Rollout a continuous orchestrator instead of a one-shot script.
	currentActiveStep := -1
	var activeStepName string

	for i, step := range rollout.Spec.Strategy.Steps {
		selector, err := metav1.LabelSelectorAsSelector(&step.Selector)
		if err != nil {
			logger.Error(err, "Invalid label selector in step", "step", step.Name)
			return err
		}

		var workspaces magosprojectiov1alpha1.WorkspaceList
		if err := r.List(ctx, &workspaces, client.InNamespace(rollout.Namespace), client.MatchingLabelsSelector{Selector: selector}); err != nil {
			logger.Error(err, "Failed to list workspaces for step")
			return err
		}

		// Filter workspaces by project reference
		var targetWorkspaces []magosprojectiov1alpha1.Workspace
		for _, ws := range workspaces.Items {
			if ws.Spec.ProjectRef.Name == rollout.Spec.ProjectRef {
				targetWorkspaces = append(targetWorkspaces, ws)
			}
		}

		if len(targetWorkspaces) == 0 {
			// If no workspaces are found for this step yet, we should reflect that it's waiting/pending
			rollout.Status.CurrentStep = i
			r.updateStatus(ctx, rollout, magosprojectiov1alpha1.PhasePending, "WaitingForWorkspaces", "No workspaces found matching selector for step "+step.Name, metav1.ConditionUnknown)
			return nil
		}

		// Evaluate the workspaces in this step
		stepNeedsWork := false
		anyFailed := false

		for _, ws := range targetWorkspaces {
			if ws.Status.Phase == magosprojectiov1alpha1.PhaseFailed {
				anyFailed = true
				logger.Info("Workspace failed, halting rollout", "workspace", ws.Name)
				break
			}

			// Work needs to be done if it hasn't applied successfully, or if its target revision has changed,
			// or if a manual reconcile was requested.
			isFullyApplied := ws.Status.Phase == magosprojectiov1alpha1.PhaseApplied && ws.Status.ObservedRevision == ws.Spec.Source.TargetRevision
			hasReconcileRequest := ws.Annotations != nil && ws.Annotations[magosprojectiov1alpha1.WorkspaceReconcileRequestAnnotation] != ""

			if !isFullyApplied || hasReconcileRequest || ws.Status.Phase == "" || ws.Status.Phase == magosprojectiov1alpha1.PhasePending {
				stepNeedsWork = true
			}
		}

		if anyFailed {
			rollout.Status.CurrentStep = i
			r.updateStatus(ctx, rollout, magosprojectiov1alpha1.PhaseFailed, "StepFailed", "One or more workspaces failed in step "+step.Name, metav1.ConditionFalse)
			return nil
		}

		if stepNeedsWork {
			// This is our current active step!
			currentActiveStep = i
			activeStepName = step.Name

			// Grant execution permissions to pending workspaces in this step
			for _, ws := range targetWorkspaces {
				isFullyApplied := ws.Status.Phase == magosprojectiov1alpha1.PhaseApplied && ws.Status.ObservedRevision == ws.Spec.Source.TargetRevision
				hasReconcileRequest := ws.Annotations != nil && ws.Annotations[magosprojectiov1alpha1.WorkspaceReconcileRequestAnnotation] != ""

				if !isFullyApplied || hasReconcileRequest || ws.Status.Phase == "" || ws.Status.Phase == magosprojectiov1alpha1.PhasePending {
					hasPermission := false
					if ws.Annotations != nil {
						hasPermission = ws.Annotations[magosprojectiov1alpha1.WorkspaceAllowedReconcileAnnotation] == "true"
					}

					if !hasPermission {
						logger.Info("Granting execution permission to workspace", "workspace", ws.Name, "step", step.Name)
						// Fetch latest to avoid OCC
						latestWS := &magosprojectiov1alpha1.Workspace{}
						if err := r.Get(ctx, client.ObjectKeyFromObject(&ws), latestWS); err == nil {
							if latestWS.Annotations == nil {
								latestWS.Annotations = make(map[string]string)
							}
							latestWS.Annotations[magosprojectiov1alpha1.WorkspaceAllowedReconcileAnnotation] = "true"
							if err := r.Update(ctx, latestWS); err != nil {
								logger.Error(err, "Failed to update workspace with execution permission", "workspace", ws.Name)
							}
						}
					}
				}
			}

			break // Stop evaluating further steps since this one blocks the pipeline
		}
	}

	if currentActiveStep == -1 {
		// All steps evaluated and none need work!
		rollout.Status.CurrentStep = len(rollout.Spec.Strategy.Steps)
		r.updateStatus(ctx, rollout, magosprojectiov1alpha1.PhaseApplied, "RolloutCompleted", "All steps completed successfully", metav1.ConditionTrue)
		return nil
	}

	rollout.Status.CurrentStep = currentActiveStep
	r.updateStatus(ctx, rollout, magosprojectiov1alpha1.PhaseReconciling, "StepProgressing", "Executing step "+activeStepName, metav1.ConditionUnknown)
	return nil
}

func (r *RolloutReconciler) updateStatus(ctx context.Context, rollout *magosprojectiov1alpha1.Rollout, phase magosprojectiov1alpha1.Phase, reason, message string, status metav1.ConditionStatus) {
	latest := &magosprojectiov1alpha1.Rollout{}
	if err := r.Get(ctx, client.ObjectKeyFromObject(rollout), latest); err != nil {
		log.FromContext(ctx).Error(err, "Failed to get latest rollout for status update")
		return
	}

	needsUpdate := false

	if latest.Status.Phase != phase || latest.Status.Reason != reason || latest.Status.Message != message {
		latest.Status.Phase = phase
		latest.Status.Reason = reason
		latest.Status.Message = message
		needsUpdate = true
	}

	if latest.Status.CurrentStep != rollout.Status.CurrentStep {
		latest.Status.CurrentStep = rollout.Status.CurrentStep
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
		log.FromContext(ctx).Error(err, "Failed to update rollout status")
		return
	}

	rollout.Status = latest.Status
	rollout.ResourceVersion = latest.ResourceVersion
}

// findRolloutsForWorkspace finds rollouts in the same namespace that target this workspace's project
func (r *RolloutReconciler) findRolloutsForWorkspace(ctx context.Context, o client.Object) []reconcile.Request {
	ws, ok := o.(*magosprojectiov1alpha1.Workspace)
	if !ok {
		return nil
	}

	var rollouts magosprojectiov1alpha1.RolloutList
	if err := r.List(ctx, &rollouts, client.InNamespace(ws.Namespace)); err != nil {
		log.FromContext(ctx).Error(err, "Failed to list rollouts for workspace change")
		return nil
	}

	var requests []reconcile.Request
	for _, ro := range rollouts.Items {
		if ro.Spec.ProjectRef == ws.Spec.ProjectRef.Name {
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      ro.Name,
					Namespace: ro.Namespace,
				},
			})
		}
	}
	return requests
}

// SetupWithManager sets up the controller with the Manager.
func (r *RolloutReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&magosprojectiov1alpha1.Rollout{}).
		Watches(
			&magosprojectiov1alpha1.Workspace{},
			handler.EnqueueRequestsFromMapFunc(r.findRolloutsForWorkspace),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		).
		Named("rollout").
		Complete(r)
}
