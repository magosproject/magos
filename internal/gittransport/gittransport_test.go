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

package gittransport

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestModeForURL(t *testing.T) {
	tests := []struct {
		name   string
		url    string
		want   Mode
		helper string
	}{
		{name: "https", url: "https://github.com/example/repo.git", want: GoGit},
		{name: "http", url: "http://example.com/repo.git", want: GoGit},
		{name: "ssh", url: "ssh://git@example.com/repo.git", want: GoGit},
		{name: "scp ssh", url: "git@example.com:org/repo.git", want: GoGit},
		{name: "git", url: "git://example.com/repo.git", want: GoGit},
		{name: "file", url: "file:///repos/example.git", want: GoGit},
		{name: "absolute path", url: "/repos/example.git", want: GoGit},
		{name: "relative path", url: "../repos/example.git", want: GoGit},
		{name: "malformed URL", url: "https://example.com/%zz", want: GoGit},
		{name: "bgit", url: "bgit://example", want: NativeGit, helper: "git-remote-bgit"},
		{name: "codecommit", url: "codecommit://profile@repo", want: NativeGit, helper: "git-remote-codecommit"},
		{name: "explicit helper", url: "bgit::https://example.com/repo.git", want: NativeGit, helper: "git-remote-bgit"},
		{name: "uppercase scheme", url: "BGIT://example", want: NativeGit, helper: "git-remote-bgit"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ModeForURL(tt.url); got != tt.want {
				t.Fatalf("ModeForURL(%q) = %v, want %v", tt.url, got, tt.want)
			}
			if tt.helper != "" {
				if got := HelperName(tt.url); got != tt.helper {
					t.Fatalf("HelperName(%q) = %q, want %q", tt.url, got, tt.helper)
				}
			}
		})
	}
}

func TestParseAndResolveLSRemote(t *testing.T) {
	const branchSHA = "1111111111111111111111111111111111111111"
	const tagSHA = "2222222222222222222222222222222222222222"
	const headSHA = "3333333333333333333333333333333333333333"
	refs := ParseLSRemote(strings.Join([]string{
		headSHA + "\tHEAD",
		branchSHA + "\trefs/heads/main",
		tagSHA + "\trefs/tags/v1.0.0",
		"malformed",
		"",
	}, "\n"))

	tests := []struct {
		name string
		ref  string
		want string
		ok   bool
	}{
		{name: "branch", ref: "main", want: branchSHA, ok: true},
		{name: "tag", ref: "v1.0.0", want: tagSHA, ok: true},
		{name: "qualified", ref: "refs/heads/main", want: branchSHA, ok: true},
		{name: "head", ref: "HEAD", want: headSHA, ok: true},
		{name: "empty means head", ref: "", want: headSHA, ok: true},
		{name: "pinned sha", ref: tagSHA, want: tagSHA, ok: true},
		{name: "missing", ref: "missing", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ResolveRef(refs, tt.ref)
			if got != tt.want || ok != tt.ok {
				t.Fatalf("ResolveRef(%q) = (%q, %v), want (%q, %v)", tt.ref, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestNativeGitCommandsInheritEnvironment(t *testing.T) {
	binDir := t.TempDir()
	argsFile := filepath.Join(t.TempDir(), "args")
	envFile := filepath.Join(t.TempDir(), "env")
	gitPath := filepath.Join(binDir, "git")
	script := `#!/bin/sh
echo "$*" >> "$TEST_GIT_ARGS"
echo "$HELPER_AUTH_TOKEN" > "$TEST_GIT_ENV"
if [ "$1" = "ls-remote" ]; then
  printf 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\tHEAD\n'
  printf 'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\trefs/heads/main\n'
fi
exit 0
`
	if err := os.WriteFile(gitPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TEST_GIT_ARGS", argsFile)
	t.Setenv("TEST_GIT_ENV", envFile)
	t.Setenv("HELPER_AUTH_TOKEN", "inherited")

	dest := filepath.Join(t.TempDir(), "repo")
	if err := Clone(context.Background(), "bgit://example", "main", dest); err != nil {
		t.Fatal(err)
	}
	refs, err := ListRemote(context.Background(), "bgit://example")
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := ResolveRef(refs, "main"); !ok || got != "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" {
		t.Fatalf("ResolveRef(main) = (%q, %v)", got, ok)
	}

	args, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatal(err)
	}
	wantCommands := []string{
		"clone --no-checkout bgit://example " + dest,
		"-C " + dest + " checkout main",
		"ls-remote bgit://example",
	}
	for _, want := range wantCommands {
		if !strings.Contains(string(args), want) {
			t.Errorf("command log %q does not contain %q", string(args), want)
		}
	}
	env, err := os.ReadFile(envFile)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(env)) != "inherited" {
		t.Fatalf("helper environment = %q, want inherited", string(env))
	}
}

func TestListRemoteErrorNamesExpectedHelper(t *testing.T) {
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

	_, err := ListRemote(context.Background(), "bgit://example")
	if err == nil {
		t.Fatal("ListRemote() error = nil")
	}
	for _, want := range []string{"git-remote-bgit", "remote-bgit"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err, want)
		}
	}
}

func TestCloneErrorNamesExpectedHelperAndPreservesStderr(t *testing.T) {
	binDir := t.TempDir()
	gitPath := filepath.Join(binDir, "git")
	script := `#!/bin/sh
echo "helper authentication failed" >&2
exit 1
`
	if err := os.WriteFile(gitPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	err := Clone(context.Background(), "bgit://example", "main", filepath.Join(t.TempDir(), "repo"))
	if err == nil {
		t.Fatal("Clone() error = nil")
	}
	for _, want := range []string{"git-remote-bgit", "helper authentication failed"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err, want)
		}
	}
}

func TestListRemoteHonorsContextCancellation(t *testing.T) {
	binDir := t.TempDir()
	gitPath := filepath.Join(binDir, "git")
	script := "#!/bin/sh\nwhile :; do :; done\n"
	if err := os.WriteFile(gitPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err := ListRemote(ctx, "bgit://example")
	if err == nil {
		t.Fatal("ListRemote() error = nil")
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("ListRemote() took %s after context cancellation", elapsed)
	}
}
