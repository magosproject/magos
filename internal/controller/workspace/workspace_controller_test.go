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
	t.Setenv("MAGOS_WORKSPACE_PVC_SIZE_DEFAULT", "2Gi")
	reconciler := WorkspaceReconciler{}
	workspace := &v1alpha1.Workspace{
		Spec: v1alpha1.WorkspaceSpec{
			PVCSize: "7Gi",
		},
	}

	assert.Equal(t, "7Gi", reconciler.resolveWorkspacePVCSize(workspace))
}

func TestResolveWorkspacePVCSizeUsesEnvDefault(t *testing.T) {
	t.Setenv("MAGOS_WORKSPACE_PVC_SIZE_DEFAULT", "3Gi")
	reconciler := WorkspaceReconciler{}

	assert.Equal(t, "3Gi", reconciler.resolveWorkspacePVCSize(&v1alpha1.Workspace{}))
}

func TestResolveWorkspacePVCSizeFallsBackToBuiltInDefault(t *testing.T) {
	t.Setenv("MAGOS_WORKSPACE_PVC_SIZE_DEFAULT", "")
	reconciler := WorkspaceReconciler{}

	assert.Equal(t, DefaultWorkspacePVCSize, reconciler.resolveWorkspacePVCSize(&v1alpha1.Workspace{}))
}

func TestNormalizeRepoURLStripsGitSuffix(t *testing.T) {
	assert.Equal(t, "https://github.com/foo/bar", normalizeRepoURL("https://github.com/foo/bar.git"))
}

func TestNormalizeRepoURLLeavesPlainURLUnchanged(t *testing.T) {
	assert.Equal(t, "https://github.com/foo/bar", normalizeRepoURL("https://github.com/foo/bar"))
}

func TestNormalizeRepoURLStripsTrailingSlash(t *testing.T) {
	assert.Equal(t, "https://github.com/foo/bar", normalizeRepoURL("https://github.com/foo/bar/"))
}

func TestNormalizeRepoURLStripsTrailingSlashAfterGitSuffix(t *testing.T) {
	assert.Equal(t, "https://github.com/foo/bar", normalizeRepoURL("https://github.com/foo/bar.git/"))
}

func TestNormalizeRepoURLStripsSurroundingWhitespace(t *testing.T) {
	assert.Equal(t, "https://github.com/foo/bar", normalizeRepoURL("  https://github.com/foo/bar.git  "))
}

func TestNormalizeRepoURLHandlesSSHForm(t *testing.T) {
	assert.Equal(t, "git@github.com:foo/bar", normalizeRepoURL("git@github.com:foo/bar.git"))
}

func TestNormalizeRepoURLDoesNotStripGitInsidePath(t *testing.T) {
	assert.Equal(t, "https://github.com/foo/bar.gitlab", normalizeRepoURL("https://github.com/foo/bar.gitlab"))
}
