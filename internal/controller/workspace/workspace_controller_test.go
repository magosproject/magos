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

package workspace

import (
	"testing"

	"github.com/magosproject/magos/types/magosproject/v1alpha1"
	"github.com/stretchr/testify/assert"
)

func TestResolveWorkspacePVCSizeUsesWorkspaceSpec(t *testing.T) {
	reconciler := WorkspaceReconciler{
		DefaultWorkspacePVCSize: "2Gi",
	}
	workspace := &v1alpha1.Workspace{
		Spec: v1alpha1.WorkspaceSpec{
			PVCSize: "7Gi",
		},
	}

	assert.Equal(t, "7Gi", reconciler.resolveWorkspacePVCSize(workspace))
}

func TestResolveWorkspacePVCSizeUsesControllerDefault(t *testing.T) {
	reconciler := WorkspaceReconciler{
		DefaultWorkspacePVCSize: "3Gi",
	}

	assert.Equal(t, "3Gi", reconciler.resolveWorkspacePVCSize(&v1alpha1.Workspace{}))
}

func TestResolveWorkspacePVCSizeFallsBackToBuiltInDefault(t *testing.T) {
	reconciler := WorkspaceReconciler{}

	assert.Equal(t, DefaultWorkspacePVCSize, reconciler.resolveWorkspacePVCSize(&v1alpha1.Workspace{}))
}
