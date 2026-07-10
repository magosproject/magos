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

package refwatcher

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/magosproject/magos/types/magosproject/v1alpha1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestResolveRefKeepsKnownSchemeOnGoGit(t *testing.T) {
	repoDir := filepath.Join(t.TempDir(), "repo.git")
	repo, err := gogit.PlainInit(repoDir, true)
	assert.NoError(t, err)
	const want = "cccccccccccccccccccccccccccccccccccccccc"
	assert.NoError(t, repo.Storer.SetReference(plumbing.NewHashReference(
		plumbing.NewBranchReferenceName("main"),
		plumbing.NewHash(want),
	)))

	got, err := resolveRef(context.Background(), "file://"+repoDir, "main")
	assert.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestResolveRefUsesNativeGitForUnknownScheme(t *testing.T) {
	binDir := t.TempDir()
	gitPath := filepath.Join(binDir, "git")
	script := `#!/bin/sh
if [ "$1" != "ls-remote" ] || [ "$2" != "bgit://example" ]; then
  echo "unexpected arguments: $*" >&2
  exit 1
fi
printf 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\tHEAD\n'
printf 'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\trefs/heads/main\n'
`
	if err := os.WriteFile(gitPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	got, err := resolveRef(context.Background(), "bgit://example", "main")
	assert.NoError(t, err)
	assert.Equal(t, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", got)
}

func TestResolveRefNativeGitReturnsRefNotFoundForMissingOrMalformedOutput(t *testing.T) {
	binDir := t.TempDir()
	gitPath := filepath.Join(binDir, "git")
	script := `#!/bin/sh
printf 'malformed output\n'
printf 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\trefs/heads/main\n'
`
	if err := os.WriteFile(gitPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	for _, ref := range []string{"missing", "refs/heads/also-missing"} {
		_, err := resolveRef(context.Background(), "bgit://example", ref)
		var notFound *RefNotFoundError
		if !errors.As(err, &notFound) {
			t.Fatalf("resolveRef(%q) error = %v, want RefNotFoundError", ref, err)
		}
	}
}

func TestResolveRefNativeGitHonorsCallerTimeout(t *testing.T) {
	binDir := t.TempDir()
	gitPath := filepath.Join(binDir, "git")
	if err := os.WriteFile(gitPath, []byte("#!/bin/sh\nwhile :; do :; done\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	started := time.Now()

	_, err := resolveRef(ctx, "bgit://example", "main")
	assert.Error(t, err)
	assert.Less(t, time.Since(started), time.Second)
}

func TestPollRemotePatchesDetectedRevisionForHelperTransport(t *testing.T) {
	binDir := t.TempDir()
	gitPath := filepath.Join(binDir, "git")
	const oldSHA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	const newSHA = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	script := `#!/bin/sh
printf 'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\trefs/heads/main\n'
`
	if err := os.WriteFile(gitPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	scheme := runtime.NewScheme()
	assert.NoError(t, v1alpha1.AddToScheme(scheme))
	ws := &v1alpha1.Workspace{
		ObjectMeta: metav1.ObjectMeta{Name: "ws", Namespace: "default"},
		Spec: v1alpha1.WorkspaceSpec{
			AutoApply: true,
			Source: v1alpha1.SourceSpec{
				RepoURL:        "bgit://example",
				TargetRevision: "main",
			},
		},
		Status: v1alpha1.WorkspaceStatus{ObservedRevision: oldSHA},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(ws).Build()
	r := New(cli, time.Minute, 1, 10)
	key := types.NamespacedName{Namespace: "default", Name: "ws"}
	r.upsertEntry(key, ws.Spec.Source.RepoURL, ws.Spec.Source.TargetRevision, time.Minute, oldSHA)

	r.pollRemote(context.Background(), key)

	got := &v1alpha1.Workspace{}
	assert.NoError(t, cli.Get(context.Background(), key, got))
	assert.Equal(t, newSHA, got.Annotations[v1alpha1.WorkspaceDetectedRevisionAnnotation])
	r.registry.mu.RLock()
	assert.Equal(t, newSHA, r.registry.entries[key].lastSHA)
	r.registry.mu.RUnlock()
}

func TestPollRemoteFailurePreservesSHAAndUsesRetryBackoff(t *testing.T) {
	binDir := t.TempDir()
	gitPath := filepath.Join(binDir, "git")
	script := `#!/bin/sh
echo "git: 'remote-bgit' is not a git command" >&2
exit 1
`
	if err := os.WriteFile(gitPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	const oldSHA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	r := New(nil, time.Minute, 1, 10)
	key := types.NamespacedName{Namespace: "default", Name: "ws"}
	r.upsertEntry(key, "bgit://example", "main", 2*time.Minute, oldSHA)
	started := time.Now()

	r.pollRemote(context.Background(), key)

	r.registry.mu.RLock()
	entry := *r.registry.entries[key]
	r.registry.mu.RUnlock()
	assert.Equal(t, oldSHA, entry.lastSHA)
	assert.WithinDuration(t, started.Add(maxRetryBackoff), entry.nextPollAt, time.Second)
}

func TestIsApprovalPendingGate(t *testing.T) {
	scheme := runtime.NewScheme()
	assert.NoError(t, v1alpha1.AddToScheme(scheme))

	ws := &v1alpha1.Workspace{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "ws",
			Namespace:   "default",
			Annotations: map[string]string{},
		},
		Spec:   v1alpha1.WorkspaceSpec{AutoApply: false},
		Status: v1alpha1.WorkspaceStatus{Phase: v1alpha1.PhasePlanned},
	}

	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(ws).Build()

	got := &v1alpha1.Workspace{}
	assert.NoError(t, cli.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: "ws"}, got))
	assert.True(t, v1alpha1.IsApprovalPending(got), "parked workspace should be approval-pending")

	if got.Annotations == nil {
		got.Annotations = map[string]string{}
	}
	got.Annotations[v1alpha1.WorkspaceApprovalDecisionAnnotation] = v1alpha1.ApprovalDecisionApproved
	assert.False(t, v1alpha1.IsApprovalPending(got), "decision in flight should not be approval-pending")
}
