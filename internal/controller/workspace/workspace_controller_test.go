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
	"context"
	"testing"

	"github.com/magosproject/magos/types/magosproject/v1alpha1"
	"github.com/stretchr/testify/assert"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestResolveWorkspacePVCSizeUsesWorkspaceSpec(t *testing.T) {
	t.Setenv(envWorkspacePVCSizeDefault, "2Gi")
	reconciler := WorkspaceReconciler{}
	workspace := &v1alpha1.Workspace{
		Spec: v1alpha1.WorkspaceSpec{
			PVCSize: "7Gi",
		},
	}

	assert.Equal(t, "7Gi", reconciler.resolveWorkspacePVCSize(workspace))
}

func TestResolveWorkspacePVCSizeUsesEnvDefault(t *testing.T) {
	t.Setenv(envWorkspacePVCSizeDefault, "3Gi")
	reconciler := WorkspaceReconciler{}

	assert.Equal(t, "3Gi", reconciler.resolveWorkspacePVCSize(&v1alpha1.Workspace{}))
}

func TestResolveWorkspacePVCSizeFallsBackToBuiltInDefault(t *testing.T) {
	t.Setenv(envWorkspacePVCSizeDefault, "")
	reconciler := WorkspaceReconciler{}

	assert.Equal(t, DefaultWorkspacePVCSize, reconciler.resolveWorkspacePVCSize(&v1alpha1.Workspace{}))
}

func TestResolveWorkspaceJobResourcesUsesBuiltInDefaults(t *testing.T) {
	t.Setenv(envWorkspaceJobCPURequest, "")
	t.Setenv(envWorkspaceJobMemoryRequest, "")
	t.Setenv(envWorkspaceJobCPULimit, "")
	t.Setenv(envWorkspaceJobMemoryLimit, "")

	reconciler := WorkspaceReconciler{}
	resources, err := reconciler.resolveWorkspaceJobResources()
	if err != nil {
		t.Fatalf("resolveWorkspaceJobResources() error = %v", err)
	}

	assert.Equal(t, DefaultWorkspaceJobCPURequest, resources.Requests.Cpu().String())
	assert.Equal(t, DefaultWorkspaceJobMemoryRequest, resources.Requests.Memory().String())
	assert.Equal(t, DefaultWorkspaceJobCPULimit, resources.Limits.Cpu().String())
	assert.Equal(t, DefaultWorkspaceJobMemoryLimit, resources.Limits.Memory().String())
}

func TestResolveWorkspaceJobResourcesUsesEnvOverrides(t *testing.T) {
	t.Setenv(envWorkspaceJobCPURequest, "300m")
	t.Setenv(envWorkspaceJobMemoryRequest, "384Mi")
	t.Setenv(envWorkspaceJobCPULimit, "750m")
	t.Setenv(envWorkspaceJobMemoryLimit, "768Mi")

	reconciler := WorkspaceReconciler{}
	resources, err := reconciler.resolveWorkspaceJobResources()
	if err != nil {
		t.Fatalf("resolveWorkspaceJobResources() error = %v", err)
	}

	assert.Equal(t, "300m", resources.Requests.Cpu().String())
	assert.Equal(t, "384Mi", resources.Requests.Memory().String())
	assert.Equal(t, "750m", resources.Limits.Cpu().String())
	assert.Equal(t, "768Mi", resources.Limits.Memory().String())
}

func TestConstructJobForWorkspaceSetsContainerResources(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := v1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme() error = %v", err)
	}
	if err := batchv1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme() error = %v", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme() error = %v", err)
	}

	t.Setenv(envWorkspaceJobCPURequest, "300m")
	t.Setenv(envWorkspaceJobMemoryRequest, "384Mi")
	t.Setenv(envWorkspaceJobCPULimit, "750m")
	t.Setenv(envWorkspaceJobMemoryLimit, "768Mi")

	reconciler := WorkspaceReconciler{
		Client:   fake.NewClientBuilder().WithScheme(scheme).Build(),
		Scheme:   scheme,
		JobImage: "ghcr.io/magosproject/magos/job:test",
	}

	workspace := &v1alpha1.Workspace{}
	workspace.Name = "demo"
	workspace.Namespace = "default"
	workspace.Spec.Source.RepoURL = "https://github.com/magosproject/demo"
	workspace.Spec.Source.TargetRevision = "main"
	workspace.Spec.Terraform.Version = "1.9.0"
	workspace.Spec.ProjectRef.Name = "platform"

	job, err := reconciler.constructJobForWorkspace(
		context.Background(),
		workspace,
		runContext{
			planJobName:  "demo-plan-1234abcd",
			applyJobName: "demo-apply-1234abcd",
			planFile:     "/workspace-data/run-1234abcd.tfplan",
			pvcName:      "demo-data",
		},
		jobTypePlan,
		"20260519T120000-deadbeef",
	)
	if err != nil {
		t.Fatalf("constructJobForWorkspace() error = %v", err)
	}

	if len(job.Spec.Template.Spec.Containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(job.Spec.Template.Spec.Containers))
	}

	resources := job.Spec.Template.Spec.Containers[0].Resources
	assert.Equal(t, "300m", resources.Requests.Cpu().String())
	assert.Equal(t, "384Mi", resources.Requests.Memory().String())
	assert.Equal(t, "750m", resources.Limits.Cpu().String())
	assert.Equal(t, "768Mi", resources.Limits.Memory().String())
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
