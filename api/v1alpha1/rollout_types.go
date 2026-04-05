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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// RolloutStep defines a single step in a rollout strategy
type RolloutStep struct {
	// Name of the step
	// +required
	Name string `json:"name"`

	// Selector to match workspaces for this step
	// +required
	Selector metav1.LabelSelector `json:"selector"`
}

// RolloutStrategy defines how the rollout should proceed
type RolloutStrategy struct {
	// Steps defines the sequential stages of the rollout
	// +required
	Steps []RolloutStep `json:"steps"`
}

// RolloutSpec defines the desired state of Rollout
type RolloutSpec struct {
	// ProjectRef is a reference to the project this rollout orchestrates
	// +required
	ProjectRef string `json:"projectRef"`

	// Strategy defines the rollout execution plan
	// +required
	Strategy RolloutStrategy `json:"strategy"`
}

// RolloutStatus defines the observed state of Rollout.
type RolloutStatus struct {
	// Phase represents the current phase of the Rollout
	// +optional
	Phase Phase `json:"phase,omitempty"`

	// CurrentStep is the index of the currently executing step
	// +optional
	CurrentStep int `json:"currentStep"`

	// Reason is a brief CamelCase string explaining the current phase
	// +optional
	Reason string `json:"reason,omitempty"`

	// Message is a human-readable explanation of the current phase
	// +optional
	Message string `json:"message,omitempty"`

	// LastReconcileTime is the timestamp of the last reconciliation
	// +optional
	LastReconcileTime *metav1.Time `json:"lastReconcileTime,omitempty"`

	// conditions represent the current state of the Rollout resource.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Project",type=string,JSONPath=`.spec.projectRef`
// +kubebuilder:printcolumn:name="Step",type=integer,JSONPath=`.status.currentStep`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// +kubebuilder:validation:XValidation:rule="self.metadata.name == self.spec.projectRef",message="Rollout name must exactly match the projectRef it targets to ensure 1-to-1 mapping"
// Rollout is the Schema for the rollouts API
type Rollout struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of Rollout
	// +required
	Spec RolloutSpec `json:"spec"`

	// status defines the observed state of Rollout
	// +optional
	Status RolloutStatus `json:"status,omitempty,omitzero"`
}

// +kubebuilder:object:root=true

// RolloutList contains a list of Rollout
type RolloutList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Rollout `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Rollout{}, &RolloutList{})
}
