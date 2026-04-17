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

// ValidationSpec configures how Terraform plans are evaluated against
// ValidatingPolicy resources before apply. The same shape is used on both
// Project (where it defines the default for member Workspaces) and Workspace
// (where it overrides the Project default).
type ValidationSpec struct {
	// PolicySelector selects ValidatingPolicy resources (json.kyverno.io) by
	// label. When nil, no policy validation is performed. An empty selector
	// ({}) matches every ValidatingPolicy in the cluster. On a Project this
	// acts as the default for any Workspace that does not set its own
	// Validation; on a Workspace this fully overrides the Project default.
	// +optional
	PolicySelector *metav1.LabelSelector `json:"policySelector,omitempty"`
}

// Phase represents the current lifecycle phase of a resource.
// +kubebuilder:validation:Enum=Pending;Reconciling;Ready;Idle;Planning;Planned;Applying;Applied;Failed;ValidationFailed;Deleting
type Phase string

const (
	// PhasePending indicates the resource is waiting to be processed
	PhasePending Phase = "Pending"

	// PhaseReconciling indicates the resource is being reconciled
	PhaseReconciling Phase = "Reconciling"

	// PhaseReady indicates the resource is ready and operational
	PhaseReady Phase = "Ready"

	// PhaseIdle indicates the workspace is idle
	PhaseIdle Phase = "Idle"

	// PhasePlanning indicates the workspace is planning
	PhasePlanning Phase = "Planning"

	// PhasePlanned indicates the workspace has a successful plan and is waiting for approval
	PhasePlanned Phase = "Planned"

	// PhaseApplying indicates the workspace is applying
	PhaseApplying Phase = "Applying"

	// PhaseApplied indicates the workspace successfully applied
	PhaseApplied Phase = "Applied"

	// PhaseFailed indicates the resource has failed
	PhaseFailed Phase = "Failed"

	// PhaseValidationFailed indicates that the plan violated one or more
	// ValidatingPolicy rules. Apply is blocked until the violations are
	// resolved and a new plan cycle succeeds.
	PhaseValidationFailed Phase = "ValidationFailed"

	// PhaseDeleting indicates the resource is being deleted and cleanup is in progress
	PhaseDeleting Phase = "Deleting"
)

const (
	// ConditionTypeReady indicates the resource is ready and fully operational
	ConditionTypeReady = "Ready"
)
