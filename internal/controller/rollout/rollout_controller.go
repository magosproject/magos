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

package rollout

import (
	"context"

	"github.com/magosproject/magos/types/v1alpha1"
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

// Reconcile is the top-level entry point invoked by controller-runtime whenever
// a Rollout or one of its watched dependents (Workspaces) changes.
func (r *RolloutReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the Rollout instance
	rollout := &v1alpha1.Rollout{}
	if err := r.Get(ctx, req.NamespacedName, rollout); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get Rollout")
		return ctrl.Result{}, err
	}

	// If the Rollout is being deleted, there is nothing to clean up. Rollouts
	// do not own any external resources directly; they only set annotations on
	// Workspaces. Once the Rollout is gone, the Project controller falls back
	// to default parallel execution and re-grants execution permissions.
	if !rollout.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	err := r.reconcileRollout(ctx, rollout)
	if err != nil {
		r.updateStatus(ctx, rollout, v1alpha1.PhaseFailed, "ReconcileError", err.Error(), metav1.ConditionFalse)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *RolloutReconciler) reconcileRollout(ctx context.Context, rollout *v1alpha1.Rollout) error {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling Rollout", "name", rollout.Name)

	// If no steps are defined, there is nothing to orchestrate. Mark the
	// Rollout as Ready so the Project controller knows it is active but idle.
	if len(rollout.Spec.Strategy.Steps) == 0 {
		r.updateStatus(ctx, rollout, v1alpha1.PhaseReady, "NoSteps", "No steps defined in strategy", metav1.ConditionTrue)
		return nil
	}

	// Walk through steps sequentially to find the first step that has pending
	// work. This linear scan is intentional: earlier steps must complete before
	// later ones are evaluated, enforcing the sequential execution guarantee.
	currentActiveStep := -1
	var activeStepName string

	for i, step := range rollout.Spec.Strategy.Steps {
		// Convert the step's LabelSelector to a labels.Selector so we can use
		// it with the client.MatchingLabelsSelector list option.
		selector, err := metav1.LabelSelectorAsSelector(&step.Selector)
		if err != nil {
			logger.Error(err, "Invalid label selector in step", "step", step.Name)
			return err
		}

		// List all Workspaces in the same namespace that match the step's label
		// selector. This is a broad query; we filter by project below.
		var workspaces v1alpha1.WorkspaceList
		if err := r.List(ctx, &workspaces, client.InNamespace(rollout.Namespace), client.MatchingLabelsSelector{Selector: selector}); err != nil {
			logger.Error(err, "Failed to list workspaces for step")
			return err
		}

		// Filter Workspaces to only those belonging to this Rollout's Project.
		// The label selector alone might match Workspaces from other Projects
		// that happen to share the same labels.
		var targetWorkspaces []v1alpha1.Workspace
		for _, ws := range workspaces.Items {
			if ws.Spec.ProjectRef.Name == rollout.Spec.ProjectRef {
				targetWorkspaces = append(targetWorkspaces, ws)
			}
		}

		// If no Workspaces match this step's selector yet, the Rollout cannot
		// make progress. This typically happens during initial setup when
		// Workspaces haven't been created yet, or if labels were misconfigured.
		// We park the Rollout in Pending and wait for a Workspace watch event
		// to trigger re-evaluation.
		if len(targetWorkspaces) == 0 {
			rollout.Status.CurrentStep = i
			r.updateStatus(ctx, rollout, v1alpha1.PhasePending, "WaitingForWorkspaces", "No workspaces found matching selector for step "+step.Name, metav1.ConditionUnknown)
			return nil
		}

		// Evaluate the Workspaces in this step to determine whether the step
		// still has pending work or whether any Workspace has failed.
		stepNeedsWork := false
		anyFailed := false

		for _, ws := range targetWorkspaces {
			// A single failed Workspace halts the entire Rollout. This is a
			// deliberate safety mechanism: applying later steps on top of a
			// broken earlier step could compound failures, and probably
			// conflicts with what
			if ws.Status.Phase == v1alpha1.PhaseFailed {
				anyFailed = true
				logger.Info("Workspace failed, halting rollout", "workspace", ws.Name)
				break
			}

			// A Workspace needs work if it hasn't applied successfully, if its
			// target revision has changed (meaning there's new infrastructure
			// to plan), or if a manual reconcile was requested via annotation.
			isFullyApplied := ws.Status.Phase == v1alpha1.PhaseApplied && ws.Status.ObservedRevision == ws.Spec.Source.TargetRevision
			hasReconcileRequest := ws.Annotations != nil && ws.Annotations[v1alpha1.WorkspaceReconcileRequestAnnotation] != ""

			if !isFullyApplied || hasReconcileRequest || ws.Status.Phase == "" || ws.Status.Phase == v1alpha1.PhasePending {
				stepNeedsWork = true
			}
		}

		if anyFailed {
			rollout.Status.CurrentStep = i
			r.updateStatus(ctx, rollout, v1alpha1.PhaseFailed, "StepFailed", "One or more workspaces failed in step "+step.Name, metav1.ConditionFalse)
			return nil
		}

		if stepNeedsWork {
			// This is our current active step. We stop evaluating further steps
			// because the pipeline is blocked until this one completes.
			currentActiveStep = i
			activeStepName = step.Name

			// Grant execution permission to Workspaces in this step that still
			// need work. The Workspace controller checks for this annotation
			// (WorkspaceExecutionAllowedAnnotation) before starting a
			// plan/apply cycle. Without it, the Workspace stays in Pending.
			for _, ws := range targetWorkspaces {
				isFullyApplied := ws.Status.Phase == v1alpha1.PhaseApplied && ws.Status.ObservedRevision == ws.Spec.Source.TargetRevision
				hasReconcileRequest := ws.Annotations != nil && ws.Annotations[v1alpha1.WorkspaceReconcileRequestAnnotation] != ""

				if !isFullyApplied || hasReconcileRequest || ws.Status.Phase == "" || ws.Status.Phase == v1alpha1.PhasePending {
					hasPermission := false
					if ws.Annotations != nil {
						hasPermission = ws.Annotations[v1alpha1.WorkspaceExecutionAllowedAnnotation] == "true"
					}

					if !hasPermission {
						logger.Info("Granting execution permission to workspace", "workspace", ws.Name, "step", step.Name)

						// Fetch the latest version of the Workspace before
						// updating to concurrency conflicts. Workspaces
						// frequently update their own status, which increments
						// the ResourceVersion rapidly. Using a stale version
						// would cause the Update to fail with a conflict error.
						latestWS := &v1alpha1.Workspace{}
						if err := r.Get(ctx, client.ObjectKeyFromObject(&ws), latestWS); err == nil {
							if latestWS.Annotations == nil {
								latestWS.Annotations = make(map[string]string)
							}
							latestWS.Annotations[v1alpha1.WorkspaceExecutionAllowedAnnotation] = "true"
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
		// All steps evaluated and none need work. The entire pipeline has
		// completed successfully.
		rollout.Status.CurrentStep = len(rollout.Spec.Strategy.Steps)
		r.updateStatus(ctx, rollout, v1alpha1.PhaseApplied, "RolloutCompleted", "All steps completed successfully", metav1.ConditionTrue)
		return nil
	}

	rollout.Status.CurrentStep = currentActiveStep
	r.updateStatus(ctx, rollout, v1alpha1.PhaseReconciling, "StepProgressing", "Executing step "+activeStepName, metav1.ConditionUnknown)
	return nil
}

// updateStatus writes phase, reason, message, and a Ready condition to the
// Rollout's status subresource. It always re-fetches the latest version of the
// Rollout before writing to avoid conflict errors caused by concurrent status
// updates. After a successful write, the caller's rollout object is updated
// in-place so subsequent logic in the same reconcile pass sees the fresh
// resourceVersion and status.
func (r *RolloutReconciler) updateStatus(ctx context.Context, rollout *v1alpha1.Rollout, phase v1alpha1.Phase, reason, message string, status metav1.ConditionStatus) {
	// Fetch the latest version of the rollout to avoid conflict errors
	latest := &v1alpha1.Rollout{}
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

	// Sync the currentStep from the caller's rollout object, which may have
	// been updated during reconcileRollout before this function was called.
	if latest.Status.CurrentStep != rollout.Status.CurrentStep {
		latest.Status.CurrentStep = rollout.Status.CurrentStep
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
		log.FromContext(ctx).Error(err, "Failed to update rollout status")
		return
	}

	// Update the original object so the caller has the latest state
	rollout.Status = latest.Status
	rollout.ResourceVersion = latest.ResourceVersion
}

// findRolloutsForWorkspace maps Workspace watch events to Rollout reconcile
// requests.
//
// Rollouts orchestrate Workspaces that belong to a specific Project. When a
// Workspace changes (e.g. its status phase transitions from Planning to
// Applied), the Rollout controller needs to re-evaluate whether the current
// step has completed and whether to advance to the next step. This mapper finds
// all Rollouts in the same namespace whose projectRef matches the Workspace's
// projectRef, and enqueues them for reconciliation.
func (r *RolloutReconciler) findRolloutsForWorkspace(ctx context.Context, o client.Object) []reconcile.Request {
	ws, ok := o.(*v1alpha1.Workspace)
	if !ok {
		return nil
	}

	var rollouts v1alpha1.RolloutList
	if err := r.List(ctx, &rollouts, client.InNamespace(ws.Namespace)); err != nil {
		log.FromContext(ctx).Error(err, "Failed to list rollouts for workspace change")
		return nil
	}

	// For each Rollout in the same namespace, if its projectRef matches the
	// Workspace's projectRef, enqueue a reconcile request for that Rollout.
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

// SetupWithManager registers the Rollout controller with the Manager and
// configures the watches that trigger reconciliation.
func (r *RolloutReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Rollout{}).
		Watches( // Watch for changes to Workspaces so we can re-evaluate step completion
			&v1alpha1.Workspace{},
			handler.EnqueueRequestsFromMapFunc(r.findRolloutsForWorkspace),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		).
		Named("rollout").
		Complete(r)
}
