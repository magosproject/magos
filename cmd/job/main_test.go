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

package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func TestCloneRepositoryUsesNativeGitForUnknownScheme(t *testing.T) {
	revisions := []string{
		"feature",
		"v1.2.3",
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}
	for _, revision := range revisions {
		t.Run(revision, func(t *testing.T) {
			binDir := t.TempDir()
			argsFile := filepath.Join(t.TempDir(), "args")
			gitPath := filepath.Join(binDir, "git")
			script := `#!/bin/sh
echo "$*" >> "$TEST_GIT_ARGS"
exit 0
`
			if err := os.WriteFile(gitPath, []byte(script), 0o755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("TEST_GIT_ARGS", argsFile)

			dest := filepath.Join(t.TempDir(), "repo")
			cfg := &Config{RepoURL: "codecommit://profile@repo", TargetRevision: revision}
			if err := cloneRepository(context.Background(), cfg, dest); err != nil {
				t.Fatal(err)
			}

			args, err := os.ReadFile(argsFile)
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{
				"clone --no-checkout codecommit://profile@repo " + dest,
				"-C " + dest + " checkout " + revision,
			} {
				if !strings.Contains(string(args), want) {
					t.Errorf("command log %q does not contain %q", string(args), want)
				}
			}
		})
	}
}

func TestCloneRepositoryKeepsKnownTransportOnGoGit(t *testing.T) {
	source := filepath.Join(t.TempDir(), "source")
	repo, err := git.PlainInit(source, false)
	if err != nil {
		t.Fatal(err)
	}
	worktree, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "main.tf"), []byte("terraform {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := worktree.Add("main.tf"); err != nil {
		t.Fatal(err)
	}
	hash, err := worktree.Commit("initial", &git.CommitOptions{Author: &object.Signature{
		Name: "Magos Test", Email: "test@example.com", When: time.Now(),
	}})
	if err != nil {
		t.Fatal(err)
	}

	binDir := t.TempDir()
	gitPath := filepath.Join(binDir, "git")
	if err := os.WriteFile(gitPath, []byte("#!/bin/sh\necho native git must not run >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	dest := filepath.Join(t.TempDir(), "clone")
	cfg := &Config{RepoURL: "file://" + source, TargetRevision: hash.String()}
	if err := cloneRepository(context.Background(), cfg, dest); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dest, "main.tf")); err != nil {
		t.Fatalf("cloned file: %v", err)
	}
}
