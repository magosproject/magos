/*
Copyright 2026. The Magos Authors.

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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// VariableSetFinalizerName is the finalizer added to VariableSet resources.
	// It lets the controller observe deletions before the object disappears so
	// that consumer Workspaces can be enqueued for a re-plan.
	VariableSetFinalizerName = "magosproject.io/finalizer"
)

// VariableSourceKindSecret and VariableSourceKindConfigMap identify which kind
// of source backs a variable's value when ValueFrom is set. They are used
// purely for status reporting (UnresolvedReference.Kind) so users can tell at
// a glance whether a missing reference is a Secret or a ConfigMap.
const (
	VariableSourceKindSecret    = "Secret"
	VariableSourceKindConfigMap = "ConfigMap"
)

// Reason codes surfaced on VariableSetStatus.UnresolvedReferences. They are
// CamelCase so that they read well in `kubectl describe` and in the UI.
const (
	// ReasonResourceNotFound is set when the referenced Secret or ConfigMap
	// does not exist in the VariableSet's namespace.
	ReasonResourceNotFound = "ResourceNotFound"

	// ReasonKeyNotFound is set when the referenced resource exists but does
	// not contain the requested key.
	ReasonKeyNotFound = "KeyNotFound"

	// ReasonForbidden is set when the controller lacks permission to read
	// the referenced resource. This usually means the chart's RBAC is out of
	// sync with the deployed CRD, so it is worth surfacing explicitly rather
	// than collapsing it into a generic error.
	ReasonForbidden = "Forbidden"
)

// VariableSetSpec defines the desired state of VariableSet.
//
// A VariableSet is a reusable bundle of Terraform input variables that is
// composed onto Workspaces by reference. Values can be literal strings or
// drawn from a Kubernetes Secret or ConfigMap in the same namespace. The
// workspace controller is what actually injects the resolved values into the
// plan and apply Job pods as TF_VAR_<name> environment variables; this
// controller is only responsible for validating references and surfacing
// their status.
type VariableSetSpec struct {
	// Description is a human-readable purpose for this VariableSet. Optional
	// and not consumed by any controller; it exists so that operators can
	// document intent in the same place as the data.
	// +optional
	Description string `json:"description,omitempty"`

	// Variables is the ordered list of Terraform variables this set
	// contributes. Each entry becomes TF_VAR_<name> on the plan and apply
	// pods of any Workspace that references this set. Order is preserved on
	// the wire so that consumers can rely on a stable iteration order when
	// computing hashes, but ordering does not change semantics within a
	// single set: duplicate names are rejected.
	// +listType=map
	// +listMapKey=name
	// +kubebuilder:validation:MinItems=1
	Variables []Variable `json:"variables"`
}

// Variable is a single Terraform input variable definition. Exactly one of
// Value or ValueFrom must be set; the CEL rule on the type enforces that so
// the API server rejects misconfigured objects before they ever reach the
// controller.
//
// +kubebuilder:validation:XValidation:rule="(has(self.value) ? 1 : 0) + (has(self.valueFrom) ? 1 : 0) == 1",message="exactly one of value or valueFrom must be set"
type Variable struct {
	// Name is the Terraform variable name. The workspace controller exposes
	// it on the pod as TF_VAR_<name>, so it must be a valid Terraform
	// identifier: letters, digits, and underscores, not starting with a
	// digit.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Pattern=`^[a-zA-Z_][a-zA-Z0-9_]*$`
	Name string `json:"name"`

	// Value is an inline literal. Use this only for non-sensitive data
	// because the value is stored verbatim in the CR and is visible to
	// anyone with read access to VariableSets in the namespace. For
	// sensitive material use ValueFrom.SecretKeyRef instead, which keeps
	// the secret bytes out of the CR and out of controller process memory.
	// +optional
	Value *string `json:"value,omitempty"`

	// ValueFrom sources the variable's value from another resource in the
	// same namespace. Mutually exclusive with Value.
	// +optional
	ValueFrom *VariableSource `json:"valueFrom,omitempty"`
}

// VariableSource describes where a variable's value should be read from.
// Exactly one of SecretKeyRef or ConfigMapKeyRef must be set.
//
// +kubebuilder:validation:XValidation:rule="(has(self.secretKeyRef) ? 1 : 0) + (has(self.configMapKeyRef) ? 1 : 0) == 1",message="exactly one of secretKeyRef or configMapKeyRef must be set"
type VariableSource struct {
	// SecretKeyRef reads the value from a key inside a Kubernetes Secret in
	// the same namespace as the VariableSet. The kubelet resolves the value
	// at pod startup time when the workspace controller wires it into the
	// plan or apply Job, so the controller process never holds the secret
	// bytes in memory.
	// +optional
	SecretKeyRef *KeySelector `json:"secretKeyRef,omitempty"`

	// ConfigMapKeyRef reads the value from a key inside a ConfigMap in the
	// same namespace. Useful for non-sensitive shared settings (region,
	// account IDs, feature flags) that you still want versioned alongside
	// your application configuration.
	// +optional
	ConfigMapKeyRef *KeySelector `json:"configMapKeyRef,omitempty"`
}

// KeySelector references a specific key inside a Secret or ConfigMap.
type KeySelector struct {
	// Name of the Secret or ConfigMap.
	// +required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// Key inside the resource's data map. Must be present on the source
	// object unless Optional is true.
	// +required
	// +kubebuilder:validation:MinLength=1
	Key string `json:"key"`

	// Optional, when true, treats a missing resource or missing key as a
	// no-op rather than an error. The variable is omitted from the pod
	// environment, and the missing reference is not reported in
	// VariableSetStatus.UnresolvedReferences. Useful for variables that are
	// only present in some environments (e.g. an optional staging-only
	// override).
	// +optional
	Optional bool `json:"optional,omitempty"`
}

// UnresolvedReference describes a single variable whose source could not be
// read at the last reconcile. Optional references that resolved to "missing"
// are not reported here; only hard failures are surfaced so that consumers
// can rely on UnresolvedReferences being a true error list.
type UnresolvedReference struct {
	// Variable is the .spec.variables[].name that could not be resolved.
	// +required
	Variable string `json:"variable"`

	// Kind is "Secret" or "ConfigMap" depending on which valueFrom source
	// was used.
	// +required
	Kind string `json:"kind"`

	// Name is the referenced resource's metadata.name.
	// +required
	Name string `json:"name"`

	// Key is the missing key within the referenced resource. Set even when
	// the resource itself was not found, so that operators can see at a
	// glance which variable is affected without correlating two list
	// entries.
	// +required
	Key string `json:"key"`

	// Reason is a CamelCase code: ResourceNotFound, KeyNotFound, Forbidden.
	// +required
	Reason string `json:"reason"`
}

// VariableSetStatus defines the observed state of VariableSet.
//
// Resolved values never appear in status. The status only records whether
// each reference could be resolved and how many consumers (Projects and
// Workspaces) currently use the set; the values themselves stay on the source
// Secrets and ConfigMaps and are read by the kubelet at pod start.
type VariableSetStatus struct {
	// Phase reflects the high-level lifecycle of the VariableSet. Mirrors
	// the Pending/Ready/Failed pattern used by the other Magos CRDs so that
	// `kubectl get variablesets` is uniform across the project.
	// +optional
	Phase Phase `json:"phase,omitempty"`

	// Reason is a brief CamelCase string explaining the current phase.
	// +optional
	Reason string `json:"reason,omitempty"`

	// Message is a human-readable explanation of the current phase.
	// +optional
	Message string `json:"message,omitempty"`

	// ObservedGeneration is the .metadata.generation observed on the last
	// reconcile. Lets consumers distinguish a stale "Ready" condition (from
	// before a spec change) from a current one.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// ResolvedVariables is the number of variables whose source was readable
	// at the last reconcile. Inline values always count as resolved.
	// Optional missing references do not count, which makes this a useful
	// quick check for "how many TF_VAR_* will a workspace receive".
	// +optional
	ResolvedVariables int32 `json:"resolvedVariables,omitempty"`

	// UnresolvedReferences lists references that could not be read at the
	// last reconcile. Empty when every required reference is satisfied.
	// Optional references that resolved to "missing" are intentionally not
	// reported here.
	// +listType=atomic
	// +optional
	UnresolvedReferences []UnresolvedReference `json:"unresolvedReferences,omitempty"`

	// LastReconcileTime is the timestamp of the last reconciliation. Useful
	// for spotting controllers that have stopped reconciling without
	// crashing.
	// +optional
	LastReconcileTime *metav1.Time `json:"lastReconcileTime,omitempty"`

	// conditions represent the current state of the VariableSet resource.
	//
	// Standard condition types:
	// - "Ready": every required reference resolved and the set is safe to
	//   consume.
	//
	// The status of each condition is one of True, False, or Unknown.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Resolved",type=integer,JSONPath=`.status.resolvedVariables`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// VariableSet is the Schema for the variablesets API
type VariableSet struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of VariableSet
	// +required
	Spec VariableSetSpec `json:"spec"`

	// status defines the observed state of VariableSet
	// +optional
	Status VariableSetStatus `json:"status,omitempty,omitzero"`
}

// +kubebuilder:object:root=true

// VariableSetList contains a list of VariableSet
type VariableSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VariableSet `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VariableSet{}, &VariableSetList{})
}
