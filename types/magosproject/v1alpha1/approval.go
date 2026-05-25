/*
Copyright 2026. The Magos Authors.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use
this file except in compliance with the License. You may obtain a copy of the
License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package v1alpha1

// IsApprovalPending reports whether the workspace is parked after a successful
// plan and has no decision in flight yet. Used as the gate both for the apply
// path in the workspace controller and for the patch path in the RefWatcher.
func IsApprovalPending(ws *Workspace) bool {
	if ws == nil {
		return false
	}
	if ws.Spec.AutoApply {
		return false
	}
	if ws.Status.Phase != PhasePlanned {
		return false
	}
	if _, ok := ws.Annotations[WorkspaceApprovalDecisionAnnotation]; ok {
		return false
	}
	return true
}
