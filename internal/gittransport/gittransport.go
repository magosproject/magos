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
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"strings"
)

// Mode identifies the implementation used for a repository URL.
type Mode int

const (
	// GoGit keeps the existing in-process implementation for transports that
	// go-git understands natively.
	GoGit Mode = iota
	// NativeGit delegates to the git CLI so git-remote-* helpers can handle the
	// URL's transport.
	NativeGit
)

var knownGoGitSchemes = map[string]struct{}{
	"":      {},
	"file":  {},
	"git":   {},
	"http":  {},
	"https": {},
	"ssh":   {},
}

// RemoteRef is one ref returned by git ls-remote or go-git's equivalent.
type RemoteRef struct {
	Hash string
	Name string
}

// ModeForURL keeps known go-git transports in process and delegates unknown
// URL schemes to native Git. Scheme-less paths and SCP-like SSH addresses are
// intentionally treated as go-git URLs to preserve existing behavior.
func ModeForURL(repoURL string) Mode {
	scheme := Scheme(repoURL)
	if _, ok := knownGoGitSchemes[scheme]; ok {
		return GoGit
	}
	return NativeGit
}

// Scheme returns a normalized URL scheme. Values that url.Parse cannot parse
// are left on the existing go-git path and therefore return an empty scheme.
func Scheme(repoURL string) string {
	parsed, err := url.Parse(repoURL)
	if err != nil {
		return ""
	}
	return strings.ToLower(parsed.Scheme)
}

// HelperName returns the executable Git resolves for an unknown URL scheme.
func HelperName(repoURL string) string {
	scheme := Scheme(repoURL)
	if scheme == "" {
		return "git-remote helper"
	}
	return "git-remote-" + scheme
}

// Clone uses native Git for a helper-backed URL. A full clone is intentional:
// custom helpers vary in shallow-fetch support, while a full clone reliably
// makes branches, tags, and pinned commit SHAs available for checkout.
func Clone(ctx context.Context, repoURL, targetRevision, dest string) error {
	if err := run(ctx, "clone", "--no-checkout", repoURL, dest); err != nil {
		return fmt.Errorf("clone through %s: %w", HelperName(repoURL), err)
	}
	if err := run(ctx, "-C", dest, "checkout", targetRevision); err != nil {
		return fmt.Errorf("checkout revision %q: %w", targetRevision, err)
	}
	return nil
}

// ListRemote runs git ls-remote for a helper-backed URL and parses its refs.
func ListRemote(ctx context.Context, repoURL string) ([]RemoteRef, error) {
	cmd := exec.CommandContext(ctx, "git", "ls-remote", repoURL)
	cmd.Env = os.Environ()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message != "" {
			return nil, fmt.Errorf("git ls-remote through %s failed: %w: %s", HelperName(repoURL), err, message)
		}
		return nil, fmt.Errorf("git ls-remote through %s failed: %w", HelperName(repoURL), err)
	}
	return ParseLSRemote(stdout.String()), nil
}

// ParseLSRemote converts the standard '<hash>\t<ref>' output into refs. Git
// may print blank or informational lines; malformed lines are ignored.
func ParseLSRemote(output string) []RemoteRef {
	refs := make([]RemoteRef, 0)
	for line := range strings.Lines(output) {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		refs = append(refs, RemoteRef{Hash: fields[0], Name: fields[1]})
	}
	return refs
}

// ResolveRef applies Magos' branch, tag, fully-qualified-ref, pinned-SHA, and
// HEAD lookup order to a remote ref list.
func ResolveRef(refs []RemoteRef, ref string) (string, bool) {
	candidates := []string{
		"refs/heads/" + ref,
		"refs/tags/" + ref,
		ref,
	}
	for _, candidate := range candidates {
		for _, remoteRef := range refs {
			if remoteRef.Name == candidate {
				return remoteRef.Hash, true
			}
		}
	}

	if len(ref) == 40 {
		return ref, true
	}
	if ref == "" || ref == "HEAD" {
		for _, remoteRef := range refs {
			if remoteRef.Name == "HEAD" {
				return remoteRef.Hash, true
			}
		}
	}
	return "", false
}

func run(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Env = os.Environ()
	var stderr bytes.Buffer
	cmd.Stdout = os.Stdout
	cmd.Stderr = io.MultiWriter(os.Stderr, &stderr)
	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message != "" {
			return fmt.Errorf("git %s failed: %w: %s", strings.Join(args, " "), err, message)
		}
		return fmt.Errorf("git %s failed: %w", strings.Join(args, " "), err)
	}
	return nil
}
