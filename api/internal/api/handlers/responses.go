package handlers

import v1alpha1 "github.com/magosproject/magos/types/magosproject/v1alpha1"

// ErrorResponse is the standard error response body.
type ErrorResponse struct {
	Error string `json:"error"`
}

// Type aliases so swag can resolve CRD types from handler annotations
// without requiring each handler to import the v1alpha1 package directly.
type (
	Project     = v1alpha1.Project
	Workspace   = v1alpha1.Workspace
	Rollout     = v1alpha1.Rollout
	VariableSet = v1alpha1.VariableSet
)

