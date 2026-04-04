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

// Phase represents the current lifecycle phase of a resource.
// +kubebuilder:validation:Enum=Pending;Reconciling;Ready;Idle;Planning;Planned;Applying;Applied;Failed;Deleting
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

	// PhaseDeleting indicates the resource is being deleted and cleanup is in progress
	PhaseDeleting Phase = "Deleting"
)

const (
	// ConditionTypeReady indicates the resource is ready and fully operational
	ConditionTypeReady = "Ready"
)
