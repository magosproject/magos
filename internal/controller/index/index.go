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

// Package index defines the controller-runtime cache field indexes shared
// across the Magos controllers. Registering an index lets a List call use a
// field selector that the cache resolves from a prebuilt map, turning what
// would otherwise be an O(all objects) in-memory scan into an O(matches)
// lookup. This matters when a namespace holds many Workspaces and several
// controllers (Project, Rollout) repeatedly need just the ones belonging to a
// single Project.
package index

import (
	"context"

	"github.com/magosproject/magos/types/magosproject/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// WorkspaceProjectRefField indexes Workspaces by their spec.projectRef.name.
// It is the field key passed to client.MatchingFields when listing the
// Workspaces that reference a given Project.
const WorkspaceProjectRefField = "spec.projectRef.name"

// workspaceProjectRef extracts the index value(s) for a single Workspace.
// Workspaces without a projectRef are left out of the index entirely so a
// field-selector list never returns them.
func workspaceProjectRef(o client.Object) []string {
	ws, ok := o.(*v1alpha1.Workspace)
	if !ok || ws.Spec.ProjectRef.Name == "" {
		return nil
	}
	return []string{ws.Spec.ProjectRef.Name}
}

// AddWorkspaceProjectRef registers the WorkspaceProjectRefField index on the
// given FieldIndexer (the manager's cache). It must be called once before the
// manager starts, and exactly once per manager: registering the same field key
// twice on the same type returns an error.
func AddWorkspaceProjectRef(ctx context.Context, fi client.FieldIndexer) error {
	return fi.IndexField(ctx, &v1alpha1.Workspace{}, WorkspaceProjectRefField, workspaceProjectRef)
}
