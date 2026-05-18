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

package variableset

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"

	"github.com/magosproject/magos/types/magosproject/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ResolutionResult is the resolved view of a single VariableSet at a point in
// time. It is shared between this controller (which uses it to derive status)
// and the workspace controller (which uses it to build the pod env). Keeping
// the resolution code in one place means both controllers agree about what
// "effective" means; in particular, both honor Optional in the same way.
type ResolutionResult struct {
	// Variables is the per-variable resolution outcome in the same order
	// the variables were declared. The caller decides how to handle entries
	// with non-empty Missing.
	Variables []ResolvedVariable

	// Unresolved is the subset of Variables whose Missing field is set and
	// whose source was not Optional. It is precomputed so callers do not
	// have to filter the full list themselves.
	Unresolved []v1alpha1.UnresolvedReference
}

// ResolvedVariable carries either a usable env-var source or a missing
// reference. Exactly one of (Inline, SecretRef, ConfigMapRef, Missing,
// OptionalMissing) is non-empty after resolution.
type ResolvedVariable struct {
	// Name is the Terraform variable name.
	Name string

	// Inline is the literal value when the variable used Spec.Value. Empty
	// for valueFrom-based variables, including secret refs (the secret
	// bytes are never read into the controller process).
	Inline *string

	// SecretRef is set when the variable resolved to a Secret key. The
	// referenced resourceVersion is captured so callers can fold it into a
	// fingerprint and detect rotations.
	SecretRef *ResolvedKeyRef

	// ConfigMapRef is set when the variable resolved to a ConfigMap key.
	ConfigMapRef *ResolvedKeyRef

	// Missing is set when the reference was required but could not be
	// resolved. The caller treats this as a hard failure.
	Missing *v1alpha1.UnresolvedReference

	// OptionalMissing is true when the reference was Optional and not
	// available. The variable is silently dropped from the pod env in that
	// case rather than surfaced as an error.
	OptionalMissing bool
}

// ResolvedKeyRef captures a successfully resolved Secret or ConfigMap key
// reference. ResourceVersion is the source object's resourceVersion at
// resolution time and feeds directly into the variables-hash fingerprint.
// Optional mirrors KeySelector.Optional so the workspace controller can
// stamp it onto the pod's SecretKeySelector/ConfigMapKeySelector and the
// kubelet will skip the env var instead of hard-failing the pod if the
// source disappears between resolve time and pod start.
type ResolvedKeyRef struct {
	Name            string
	Key             string
	ResourceVersion string
	Optional        bool
}

// Resolve walks a VariableSet's variables and resolves each one against the
// API server. It does not return an error when a single reference is missing;
// instead the missing entry is recorded in ResolvedVariable.Missing so the
// caller can either surface it on status (this controller) or fail the
// Workspace before creating a Job (the workspace controller).
//
// The only errors Resolve returns are unexpected API failures (transport
// errors, controller RBAC misconfiguration that is not a 403 on a single
// object). Those are returned as-is so the caller can decide whether to back
// off or requeue.
func Resolve(ctx context.Context, c client.Client, vs *v1alpha1.VariableSet) (ResolutionResult, error) {
	out := ResolutionResult{
		Variables: make([]ResolvedVariable, 0, len(vs.Spec.Variables)),
	}

	// Cache lookups of the same Secret/ConfigMap across multiple variables.
	// A single VariableSet often pulls several keys from one Secret (a
	// database admin password and a paired connection URL, for example)
	// and we should not pay an API round-trip per key.
	secretCache := map[string]*corev1.Secret{}
	configMapCache := map[string]*corev1.ConfigMap{}

	for i := range vs.Spec.Variables {
		v := &vs.Spec.Variables[i]
		resolved, err := resolveVariable(ctx, c, vs.Namespace, v, secretCache, configMapCache)
		if err != nil {
			return ResolutionResult{}, err
		}
		out.Variables = append(out.Variables, resolved)
		if resolved.Missing != nil {
			out.Unresolved = append(out.Unresolved, *resolved.Missing)
		}
	}

	return out, nil
}

// resolveVariable is the per-variable inner loop of Resolve. It is a separate
// function so the cache plumbing stays out of the main flow.
func resolveVariable(ctx context.Context, c client.Client, namespace string, v *v1alpha1.Variable, secrets map[string]*corev1.Secret, configMaps map[string]*corev1.ConfigMap) (ResolvedVariable, error) {
	r := ResolvedVariable{Name: v.Name}

	switch {
	case v.Value != nil:
		r.Inline = v.Value
		return r, nil

	case v.ValueFrom != nil && v.ValueFrom.SecretKeyRef != nil:
		sel := v.ValueFrom.SecretKeyRef
		secret, err := lookupSecret(ctx, c, namespace, sel.Name, secrets)
		if err != nil {
			if errors.IsNotFound(err) {
				return missingOrOptional(v.Name, v1alpha1.VariableSourceKindSecret, sel, v1alpha1.ReasonResourceNotFound), nil
			}
			if errors.IsForbidden(err) {
				return missingOrOptional(v.Name, v1alpha1.VariableSourceKindSecret, sel, v1alpha1.ReasonForbidden), nil
			}
			return ResolvedVariable{}, fmt.Errorf("read secret %s/%s: %w", namespace, sel.Name, err)
		}
		if _, ok := secret.Data[sel.Key]; !ok {
			return missingOrOptional(v.Name, v1alpha1.VariableSourceKindSecret, sel, v1alpha1.ReasonKeyNotFound), nil
		}
		r.SecretRef = &ResolvedKeyRef{
			Name:            secret.Name,
			Key:             sel.Key,
			ResourceVersion: secret.ResourceVersion,
			Optional:        sel.Optional,
		}
		return r, nil

	case v.ValueFrom != nil && v.ValueFrom.ConfigMapKeyRef != nil:
		sel := v.ValueFrom.ConfigMapKeyRef
		cm, err := lookupConfigMap(ctx, c, namespace, sel.Name, configMaps)
		if err != nil {
			if errors.IsNotFound(err) {
				return missingOrOptional(v.Name, v1alpha1.VariableSourceKindConfigMap, sel, v1alpha1.ReasonResourceNotFound), nil
			}
			if errors.IsForbidden(err) {
				return missingOrOptional(v.Name, v1alpha1.VariableSourceKindConfigMap, sel, v1alpha1.ReasonForbidden), nil
			}
			return ResolvedVariable{}, fmt.Errorf("read configmap %s/%s: %w", namespace, sel.Name, err)
		}
		// ConfigMaps may carry binary data under BinaryData. Either is
		// fine as the source for a TF_VAR_*; the kubelet handles the
		// decoding when it materializes the env var.
		_, inData := cm.Data[sel.Key]
		_, inBinary := cm.BinaryData[sel.Key]
		if !inData && !inBinary {
			return missingOrOptional(v.Name, v1alpha1.VariableSourceKindConfigMap, sel, v1alpha1.ReasonKeyNotFound), nil
		}
		r.ConfigMapRef = &ResolvedKeyRef{
			Name:            cm.Name,
			Key:             sel.Key,
			ResourceVersion: cm.ResourceVersion,
			Optional:        sel.Optional,
		}
		return r, nil
	}

	// Should be unreachable because CEL on Variable enforces that exactly
	// one of Value/ValueFrom is set. Fall through to a clearly labeled
	// missing entry so that an out-of-band CR (created before CEL existed,
	// or by a bypassing admission controller) does not silently disappear.
	r.Missing = &v1alpha1.UnresolvedReference{
		Variable: v.Name,
		Reason:   "InvalidSpec",
	}
	return r, nil
}

// missingOrOptional turns a failed lookup into either a hard miss
// (reported on status) or a silent optional miss (dropped from the env). The
// branching lives in one place so both Secret and ConfigMap paths handle
// Optional identically.
func missingOrOptional(varName, kind string, sel *v1alpha1.KeySelector, reason string) ResolvedVariable {
	if sel.Optional {
		return ResolvedVariable{Name: varName, OptionalMissing: true}
	}
	return ResolvedVariable{
		Name: varName,
		Missing: &v1alpha1.UnresolvedReference{
			Variable: varName,
			Kind:     kind,
			Name:     sel.Name,
			Key:      sel.Key,
			Reason:   reason,
		},
	}
}

func lookupSecret(ctx context.Context, c client.Client, namespace, name string, cache map[string]*corev1.Secret) (*corev1.Secret, error) {
	if existing, ok := cache[name]; ok {
		if existing == nil {
			// Negative cache: a prior lookup already returned NotFound.
			// Re-issue the same NotFound so the caller's switch handles
			// it uniformly without us having to thread a sentinel.
			return nil, errors.NewNotFound(corev1.Resource("secrets"), name)
		}
		return existing, nil
	}
	secret := &corev1.Secret{}
	err := c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, secret)
	if err != nil {
		if errors.IsNotFound(err) {
			cache[name] = nil
		}
		return nil, err
	}
	cache[name] = secret
	return secret, nil
}

func lookupConfigMap(ctx context.Context, c client.Client, namespace, name string, cache map[string]*corev1.ConfigMap) (*corev1.ConfigMap, error) {
	if existing, ok := cache[name]; ok {
		if existing == nil {
			return nil, errors.NewNotFound(corev1.Resource("configmaps"), name)
		}
		return existing, nil
	}
	cm := &corev1.ConfigMap{}
	err := c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, cm)
	if err != nil {
		if errors.IsNotFound(err) {
			cache[name] = nil
		}
		return nil, err
	}
	cache[name] = cm
	return cm, nil
}

// ComposeEffective resolves the ordered concatenation of multiple
// VariableSets into a single flat list, applying the layered-override rule:
// for a given variable name, the last successful resolution wins. Earlier
// entries that ended up shadowed are dropped from the result.
//
// This is the helper the workspace controller uses to build TF_VAR_* env
// vars. It deliberately returns the same ResolvedVariable shape that Resolve
// uses, so the workspace controller can treat individual variables uniformly
// regardless of which VariableSet they came from.
//
// Missing entries (non-optional) are preserved so the workspace controller
// can fail fast with a clear status before creating a Job. OptionalMissing
// entries are dropped because they will not produce env vars anyway.
func ComposeEffective(results []ResolutionResult) []ResolvedVariable {
	byName := map[string]ResolvedVariable{}
	order := []string{}

	for _, res := range results {
		for _, v := range res.Variables {
			if v.OptionalMissing {
				continue
			}
			if _, ok := byName[v.Name]; !ok {
				order = append(order, v.Name)
			}
			byName[v.Name] = v
		}
	}

	out := make([]ResolvedVariable, 0, len(order))
	for _, name := range order {
		out = append(out, byName[name])
	}
	return out
}

// Fingerprint produces a short, stable hash of an effective variable list.
// The hash incorporates each variable's source identity (kind, name, key,
// resourceVersion) so rotating a referenced Secret produces a different
// fingerprint without requiring the controller to read the secret bytes.
//
// For inline variables, the literal value is folded in directly; that is
// safe because inline values are already plaintext in the CR.
//
// The output is the first 16 hex characters of sha256, which is plenty of
// entropy for change detection and short enough to fit on a status field
// without bloating object size.
func Fingerprint(vars []ResolvedVariable) string {
	// An empty effective composition has no fingerprint. Returning ""
	// rather than the sha256 of the empty input keeps the "no VariableSet
	// attached" case from looking like a change relative to an unstamped
	// Workspace.Status.VariablesHash (also ""), which would otherwise
	// trigger a spurious VariablesChanged reset on every freshly created
	// Workspace.
	if len(vars) == 0 {
		return ""
	}

	// Sort by name before hashing so that the order in which VariableSets
	// were composed does not change the fingerprint as long as the same
	// (name -> source) mapping wins. Re-ordering refs without changing
	// effective values is therefore a no-op.
	sorted := make([]ResolvedVariable, len(vars))
	copy(sorted, vars)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })

	// sha256.Hash.Write never returns an error, so we can drop the byte
	// count and treat the writes as infallible. Building the input through
	// io.WriteString instead of fmt.Fprintf keeps the linter happy without
	// sprinkling errcheck ignores.
	h := sha256.New()
	write := func(parts ...string) {
		for _, p := range parts {
			h.Write([]byte(p))
			h.Write([]byte{0})
		}
	}
	for _, v := range sorted {
		write(v.Name)
		switch {
		case v.Inline != nil:
			write("inline", *v.Inline)
		case v.SecretRef != nil:
			write("secret", v.SecretRef.Name, v.SecretRef.Key, v.SecretRef.ResourceVersion, fmt.Sprintf("%t", v.SecretRef.Optional))
		case v.ConfigMapRef != nil:
			write("configmap", v.ConfigMapRef.Name, v.ConfigMapRef.Key, v.ConfigMapRef.ResourceVersion, fmt.Sprintf("%t", v.ConfigMapRef.Optional))
		case v.Missing != nil:
			// A missing reference still contributes to the hash so that
			// re-creating the source Secret with the same resourceVersion
			// flips the fingerprint exactly once: from "missing" to
			// "resolved".
			write("missing", v.Missing.Kind, v.Missing.Name, v.Missing.Key)
		}
	}
	sum := h.Sum(nil)
	return hex.EncodeToString(sum)[:16]
}
