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
package variableset

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/magosproject/magos/types/magosproject/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// VariableSetReconciler reconciles a VariableSet object.
//
// The reconciler does not create or own any Kubernetes resources. Its sole
// responsibility is to validate that every reference inside a VariableSet
// resolves and to surface the current resolution state on .status. The
// workspace controller is what actually wires resolved values into plan and
// apply Job pods.
type VariableSetReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// MaxConcurrentReconciles bounds how many VariableSets this controller
	// reconciles in parallel. Values below 1 are treated as 1.
	MaxConcurrentReconciles int
}

// +kubebuilder:rbac:groups=magosproject.io,resources=variablesets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=magosproject.io,resources=variablesets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=magosproject.io,resources=variablesets/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch

// Reconcile is the top-level entry point invoked by controller-runtime
// whenever a VariableSet or one of its watched dependents (Secrets,
// ConfigMaps, Projects, Workspaces) changes.
func (r *VariableSetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the VariableSet instance
	vs := &v1alpha1.VariableSet{}
	if err := r.Get(ctx, req.NamespacedName, vs); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("VariableSet resource not found, ignoring")
			// Clean up gauges so deleted sets do not linger in the
			// metrics endpoint forever.
			unresolvedReferences.DeleteLabelValues(req.Namespace, req.Name)
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get VariableSet")
		return ctrl.Result{}, err
	}

	// Ensure a finalizer is present so Kubernetes delays actual deletion
	// until we explicitly remove it. We do not own any Kubernetes
	// resources, but holding the finalizer for the brief window of
	// reconciliation lets the controller observe the deletion event and
	// kick consumer Workspaces toward a re-plan that no longer references
	// this set.
	if controllerutil.AddFinalizer(vs, v1alpha1.VariableSetFinalizerName) {
		if err := r.Update(ctx, vs); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if !vs.DeletionTimestamp.IsZero() {
		finished, err := r.handleDeletion(ctx, vs)
		if err != nil {
			return ctrl.Result{}, err
		}
		if finished {
			return ctrl.Result{}, nil
		}
		// Finalizer was removed but the object hasn't been
		// garbage-collected yet. Requeue briefly so we don't spin on
		// every event in the meantime.
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	if err := r.reconcileVariableSet(ctx, vs); err != nil {
		reconcileTotal.WithLabelValues(req.Namespace, req.Name, "error").Inc()
		r.updateStatus(ctx, vs, v1alpha1.PhaseFailed, "ReconcileError", err.Error(), metav1.ConditionFalse, nil, 0)
		return ctrl.Result{}, err
	}

	reconcileTotal.WithLabelValues(req.Namespace, req.Name, "success").Inc()
	return ctrl.Result{}, nil
}

// handleDeletion removes the finalizer from a VariableSet that is being
// deleted. The controller owns no external resources, so all we need to do is
// drop the finalizer and let garbage collection proceed.
func (r *VariableSetReconciler) handleDeletion(ctx context.Context, vs *v1alpha1.VariableSet) (bool, error) {
	logger := log.FromContext(ctx)
	logger.Info("Handling variableset deletion")

	r.updateStatus(ctx, vs, v1alpha1.PhaseDeleting, "Deleting", "VariableSet is being deleted", metav1.ConditionFalse, nil, 0)

	if controllerutil.ContainsFinalizer(vs, v1alpha1.VariableSetFinalizerName) {
		logger.Info("Removing finalizer")
		controllerutil.RemoveFinalizer(vs, v1alpha1.VariableSetFinalizerName)
		if err := r.Update(ctx, vs); err != nil {
			return false, err
		}
	}
	unresolvedReferences.DeleteLabelValues(vs.Namespace, vs.Name)
	return true, nil
}

func (r *VariableSetReconciler) reconcileVariableSet(ctx context.Context, vs *v1alpha1.VariableSet) error {
	logger := log.FromContext(ctx)
	logger.V(1).Info("Reconciling variableset")

	// Step 1: Resolve every variable. Resolve only returns an error on
	// unexpected API failures; missing references are returned in-line on
	// ResolutionResult.Unresolved so they can be folded into status
	// without surfacing as a hard reconcile error (which would loop the
	// controller against the same missing Secret forever).
	resolution, err := Resolve(ctx, r.Client, vs)
	if err != nil {
		return fmt.Errorf("resolve variableset: %w", err)
	}

	// Step 2: Translate the resolution result into the user-facing status.
	// ResolvedVariables counts every variable that produced a usable
	// env-var source, which means inline values count, optional-missing
	// values do not, and hard-missing values do not.
	resolved := int32(0)
	for _, v := range resolution.Variables {
		if v.Inline != nil || v.SecretRef != nil || v.ConfigMapRef != nil {
			resolved++
		}
	}

	phase := v1alpha1.PhaseReady
	reason := "AllReferencesResolved"
	message := "All variable references are resolved"
	condStatus := metav1.ConditionTrue
	if len(resolution.Unresolved) > 0 {
		phase = v1alpha1.PhaseFailed
		reason = "UnresolvedReferences"
		message = fmt.Sprintf("%d variable reference(s) could not be resolved", len(resolution.Unresolved))
		condStatus = metav1.ConditionFalse
	}

	r.updateStatus(ctx, vs, phase, reason, message, condStatus, resolution.Unresolved, resolved)

	// Step 4: Mirror the unresolved-reference count onto its gauge. The
	// status-side number is the source of truth so the gauge tracks it
	// directly rather than recomputing.
	unresolvedReferences.WithLabelValues(vs.Namespace, vs.Name).Set(float64(len(resolution.Unresolved)))

	return nil
}

// updateStatus writes the phase, reason, message, Ready condition, the
// unresolved-reference list, and the resolved count to the VariableSet
// status subresource. To avoid conflicts from concurrent updates, it always
// fetches the latest version of the object before writing.
//
// After a successful update, vs is updated in-place with the new
// resourceVersion and status so subsequent logic in the same reconcile loop
// sees the current state.
func (r *VariableSetReconciler) updateStatus(ctx context.Context, vs *v1alpha1.VariableSet, phase v1alpha1.Phase, reason, message string, status metav1.ConditionStatus, unresolved []v1alpha1.UnresolvedReference, resolved int32) {
	latest := &v1alpha1.VariableSet{}
	if err := r.Get(ctx, client.ObjectKeyFromObject(vs), latest); err != nil {
		log.FromContext(ctx).Error(err, "Failed to get latest variableset for status update")
		return
	}

	needsUpdate := false

	if latest.Status.Phase != phase || latest.Status.Reason != reason || latest.Status.Message != message {
		latest.Status.Phase = phase
		latest.Status.Reason = reason
		latest.Status.Message = message
		needsUpdate = true
	}

	if latest.Status.ObservedGeneration != vs.Generation {
		latest.Status.ObservedGeneration = vs.Generation
		needsUpdate = true
	}

	if latest.Status.ResolvedVariables != resolved {
		latest.Status.ResolvedVariables = resolved
		needsUpdate = true
	}

	if !reflect.DeepEqual(latest.Status.UnresolvedReferences, unresolved) {
		latest.Status.UnresolvedReferences = unresolved
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
		log.FromContext(ctx).Error(err, "Failed to update variableset status")
		return
	}

	vs.Status = latest.Status
	vs.ResourceVersion = latest.ResourceVersion
}

// findVariableSetsForSecret maps Secret watch events to VariableSet reconcile
// requests.
//
// VariableSets are not owners of the Secrets they reference, so without this
// mapper a rotated Secret would not refresh the resolution status until the
// next periodic reconcile. We list every VariableSet in the Secret's
// namespace and enqueue the ones that reference it. The mapper is namespaced
// so we never list across the cluster.
func (r *VariableSetReconciler) findVariableSetsForSecret(ctx context.Context, o client.Object) []reconcile.Request {
	secret, ok := o.(*corev1.Secret)
	if !ok {
		return nil
	}
	return r.findVariableSetsReferencing(ctx, secret.Namespace, secret.Name, v1alpha1.VariableSourceKindSecret)
}

// findVariableSetsForConfigMap is the ConfigMap-side mirror of
// findVariableSetsForSecret. It exists separately rather than as a generic
// helper because the controller-runtime mapper signature is per-type.
func (r *VariableSetReconciler) findVariableSetsForConfigMap(ctx context.Context, o client.Object) []reconcile.Request {
	cm, ok := o.(*corev1.ConfigMap)
	if !ok {
		return nil
	}
	return r.findVariableSetsReferencing(ctx, cm.Namespace, cm.Name, v1alpha1.VariableSourceKindConfigMap)
}

// findVariableSetsReferencing returns reconcile requests for every
// VariableSet in the given namespace that has at least one variable sourcing
// from the named Secret or ConfigMap. We accept the kind as a string so the
// same loop body serves both watch sources.
func (r *VariableSetReconciler) findVariableSetsReferencing(ctx context.Context, namespace, name, kind string) []reconcile.Request {
	var sets v1alpha1.VariableSetList
	if err := r.List(ctx, &sets, client.InNamespace(namespace)); err != nil {
		log.FromContext(ctx).Error(err, "Failed to list VariableSets for source change", "kind", kind, "name", name)
		return nil
	}

	var requests []reconcile.Request
	for i := range sets.Items {
		vs := &sets.Items[i]
		if variableSetReferences(vs, name, kind) {
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      vs.Name,
					Namespace: vs.Namespace,
				},
			})
		}
	}
	return requests
}

// variableSetReferences reports whether vs has a variable whose ValueFrom
// points at the named Secret or ConfigMap. Optional refs count too, because
// an optional ref becoming available is still a state change the controller
// should observe.
func variableSetReferences(vs *v1alpha1.VariableSet, name, kind string) bool {
	for i := range vs.Spec.Variables {
		v := &vs.Spec.Variables[i]
		if v.ValueFrom == nil {
			continue
		}
		switch kind {
		case v1alpha1.VariableSourceKindSecret:
			if v.ValueFrom.SecretKeyRef != nil && v.ValueFrom.SecretKeyRef.Name == name {
				return true
			}
		case v1alpha1.VariableSourceKindConfigMap:
			if v.ValueFrom.ConfigMapKeyRef != nil && v.ValueFrom.ConfigMapKeyRef.Name == name {
				return true
			}
		}
	}
	return false
}

// SetupWithManager registers the VariableSet controller with the Manager and
// configures the watches that trigger reconciliation. The full watch list is:
//
//   - VariableSet itself, the primary resource.
//   - Secrets and ConfigMaps in the same namespace, so rotating an
//     externally managed source updates resolution status promptly.
//
// We deliberately do not Watch Projects or Workspaces. Their
// .spec.variableSetRef lists do not affect this controller's outputs:
// resolution status is purely a function of the VariableSet's own spec and
// the Secrets/ConfigMaps it references. ResourceVersionChangedPredicate on
// the secret and configmap watches keeps that traffic constrained to
// actual data updates.
func (r *VariableSetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.VariableSet{}).
		Watches(
			&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(r.findVariableSetsForSecret),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		).
		Watches(
			&corev1.ConfigMap{},
			handler.EnqueueRequestsFromMapFunc(r.findVariableSetsForConfigMap),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		).
		WithOptions(controller.Options{MaxConcurrentReconciles: max(1, r.MaxConcurrentReconciles)}).
		Named("variableset").
		Complete(r)
}
