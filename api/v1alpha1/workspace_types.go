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
	// WorkspaceFinalizerName is the finalizer added to Workspace resources
	WorkspaceFinalizerName = "magosproject.io/finalizer"

	// WorkspaceApprovedAnnotation is the annotation used to approve a workspace run
	WorkspaceApprovedAnnotation = "magosproject.io/approved"

	// WorkspaceExecutionAllowedAnnotation is set to "true" by the Rollout
	// controller when it is this Workspace's turn to execute. The Workspace
	// controller removes it once execution finishes (success or failure).
	WorkspaceExecutionAllowedAnnotation = "magosproject.io/execution-allowed"

	// WorkspaceReconcileRequestAnnotation is used to force a reconciliation (e.g., drift correction)
	WorkspaceReconcileRequestAnnotation = "magosproject.io/reconcile-request"

	// WorkspaceReconcileIntervalAnnotation overrides the default drift detection interval
	WorkspaceReconcileIntervalAnnotation = "magosproject.io/reconcile-interval"
)

// ProjectReference references a Project resource
type ProjectReference struct {
	// Name of the Project.
	// +required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
}

// SourceSpec defines the Git repository configuration
type SourceSpec struct {
	// RepoURL is the URL of the Git repository to clone
	// +required
	RepoURL string `json:"repoURL"`

	// TargetRevision defines the commit, tag, or branch to checkout
	// +required
	TargetRevision string `json:"targetRevision"`

	// Path is the directory path within the Git repository
	// +optional
	// +kubebuilder:default="."
	Path string `json:"path,omitempty"`
}

// TerraformSpec defines the Terraform or OpenTofu configuration
type TerraformSpec struct {
	// Version is the Terraform or OpenTofu version to use
	// +required
	Version string `json:"version"`

	// TfvarsPath is the path to the tfvars file within the repository
	// +optional
	TfvarsPath string `json:"tfvarsPath,omitempty"`
}

// WorkspaceAnnotations defines annotations to propagate to plan and/or apply Jobs.
// Common annotations are applied to both job types. Plan- and Apply-specific
// annotations are merged on top of common, with the specific annotations winning
// on conflict.
type WorkspaceAnnotations struct {
	// Common annotations propagated to both plan and apply Jobs.
	// +optional
	Common map[string]string `json:"common,omitempty"`

	// Plan-specific annotations, merged with common (plan wins on conflict).
	// +optional
	Plan map[string]string `json:"plan,omitempty"`

	// Apply-specific annotations, merged with common (apply wins on conflict).
	// +optional
	Apply map[string]string `json:"apply,omitempty"`
}

// WorkspaceSpec defines the desired state of Workspace
type WorkspaceSpec struct {
	// ProjectRef is a reference to the project this workspace belongs to
	// +required
	ProjectRef ProjectReference `json:"projectRef"`

	// AutoApply dictates whether the workspace should automatically apply after a successful plan
	// +optional
	// +kubebuilder:default=true
	AutoApply bool `json:"autoApply"`

	// Annotations to propagate to the plan and/or apply Jobs.
	// +optional
	Annotations *WorkspaceAnnotations `json:"annotations,omitempty"`

	// Source defines the Git repository configuration
	// +required
	Source SourceSpec `json:"source"`

	// Terraform defines the Terraform or OpenTofu configuration
	// +required
	Terraform TerraformSpec `json:"terraform"`
}

// WorkspaceStatus defines the observed state of Workspace.
type WorkspaceStatus struct {
	// Phase represents the current phase of the Workspace
	// +optional
	Phase Phase `json:"phase,omitempty"`

	// Reason is a brief CamelCase string explaining the current phase
	// +optional
	Reason string `json:"reason,omitempty"`

	// Message is a human-readable explanation of the current phase
	// +optional
	Message string `json:"message,omitempty"`

	// ObservedRevision is the git revision that was most recently observed/applied
	// +optional
	ObservedRevision string `json:"observedRevision,omitempty"`

	// LastReconcileTime is the timestamp of the last reconciliation
	// +optional
	LastReconcileTime *metav1.Time `json:"lastReconcileTime,omitempty"`

	// conditions represent the current state of the Workspace resource.
	// Each condition has a unique type and reflects the status of a specific aspect of the resource.
	//
	// Standard condition types include:
	// - "Available": the resource is fully functional
	// - "Progressing": the resource is being created or updated
	// - "Degraded": the resource failed to reach or maintain its desired state
	//
	// The status of each condition is one of True, False, or Unknown.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Project",type=string,JSONPath=`.spec.projectRef.name`
// +kubebuilder:printcolumn:name="Revision",type=string,JSONPath=`.status.observedRevision`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Workspace is the Schema for the workspaces API
type Workspace struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of Workspace
	// +required
	Spec WorkspaceSpec `json:"spec"`

	// status defines the observed state of Workspace
	// +optional
	Status WorkspaceStatus `json:"status,omitempty,omitzero"`
}

// +kubebuilder:object:root=true

// WorkspaceList contains a list of Workspace
type WorkspaceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Workspace `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Workspace{}, &WorkspaceList{})
}
