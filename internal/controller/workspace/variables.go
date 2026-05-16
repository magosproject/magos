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
	"fmt"
	"strings"

	"github.com/magosproject/magos/internal/controller/variableset"
	"github.com/magosproject/magos/types/magosproject/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// resolveWorkspaceVariables returns the effective list of resolved variables
// for a Workspace and a list of unresolved (hard miss) references. The
// effective list is the ordered concatenation of the parent Project's
// VariableSetRef followed by the Workspace's own VariableSetRef, with
// later-set values shadowing earlier ones when names collide. The composition
// happens entirely inside the variableset package so this controller and the
// VariableSet controller stay in lockstep about precedence semantics.
//
// A returned error indicates an unexpected API failure such as a transport
// problem reading the parent Project or one of the VariableSets. A missing
// VariableSet or a missing Secret/ConfigMap reference does NOT return an
// error; those land in the unresolved list so the caller can surface a
// targeted PhaseFailed status without forcing the controller into an
// exponential-backoff loop against a known-bad reference.
func (r *WorkspaceReconciler) resolveWorkspaceVariables(ctx context.Context, ws *v1alpha1.Workspace) ([]variableset.ResolvedVariable, []v1alpha1.UnresolvedReference, error) {
	logger := log.FromContext(ctx)

	// Step 1: Pull the parent Project so we can read its VariableSetRef
	// list. A missing Project is not a fatal error here, because the
	// Workspace can still attach VariableSets directly through its own
	// spec. The Project controller will surface its own status if the
	// parent is genuinely misconfigured.
	var projectRefs []v1alpha1.VariableSetReference
	project := &v1alpha1.Project{}
	if err := r.Get(ctx, types.NamespacedName{Name: ws.Spec.ProjectRef.Name, Namespace: ws.Namespace}, project); err != nil {
		if !errors.IsNotFound(err) {
			return nil, nil, fmt.Errorf("get parent project: %w", err)
		}
		logger.V(1).Info("Parent Project not found while resolving variables, using workspace refs only")
	} else {
		projectRefs = project.Spec.VariableSetRef
	}

	// Step 2: Resolve each VariableSet referenced by the Project first,
	// then each one referenced by the Workspace. The order is what gives
	// us the layered-override semantic: a workspace-scoped variable wins
	// over a project-scoped variable of the same name.
	ordered := make([]v1alpha1.VariableSetReference, 0, len(projectRefs)+len(ws.Spec.VariableSetRef))
	ordered = append(ordered, projectRefs...)
	ordered = append(ordered, ws.Spec.VariableSetRef...)

	results := make([]variableset.ResolutionResult, 0, len(ordered))
	var unresolved []v1alpha1.UnresolvedReference

	for _, ref := range ordered {
		vs := &v1alpha1.VariableSet{}
		err := r.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: ws.Namespace}, vs)
		if err != nil {
			if errors.IsNotFound(err) {
				// Surface the missing VariableSet on the workspace
				// status as a single synthetic unresolved entry. We
				// reuse the UnresolvedReference shape so the message
				// generator at the call site does not need to know
				// about a second kind of failure.
				unresolved = append(unresolved, v1alpha1.UnresolvedReference{
					Variable: "",
					Kind:     "VariableSet",
					Name:     ref.Name,
					Key:      "",
					Reason:   v1alpha1.ReasonResourceNotFound,
				})
				continue
			}
			return nil, nil, fmt.Errorf("get variableset %s: %w", ref.Name, err)
		}

		res, err := variableset.Resolve(ctx, r.Client, vs)
		if err != nil {
			return nil, nil, fmt.Errorf("resolve variableset %s: %w", ref.Name, err)
		}
		results = append(results, res)
		unresolved = append(unresolved, res.Unresolved...)
	}

	return variableset.ComposeEffective(results), unresolved, nil
}

// variableEnvVars converts a composed list of resolved variables into the
// container env entries that the plan and apply pods consume. Inline values
// land verbatim. Secret and ConfigMap references are forwarded as valueFrom
// entries so the kubelet performs the actual read at pod startup; this means
// the controller process never holds the secret bytes and a rotation in the
// source resource takes effect on the next pod scheduling without any code
// path in the controller seeing the new value.
//
// Variables flagged as Missing are skipped silently here. The caller is
// expected to surface them on status before invoking this helper, and
// dropping them keeps the pod env clean of placeholder TF_VAR_* entries that
// would otherwise confuse Terraform.
func variableEnvVars(vars []variableset.ResolvedVariable) []corev1.EnvVar {
	if len(vars) == 0 {
		return nil
	}
	out := make([]corev1.EnvVar, 0, len(vars))
	for i := range vars {
		v := &vars[i]
		envName := "TF_VAR_" + v.Name
		switch {
		case v.Inline != nil:
			out = append(out, corev1.EnvVar{Name: envName, Value: *v.Inline})
		case v.SecretRef != nil:
			out = append(out, corev1.EnvVar{
				Name: envName,
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: v.SecretRef.Name},
						Key:                  v.SecretRef.Key,
					},
				},
			})
		case v.ConfigMapRef != nil:
			out = append(out, corev1.EnvVar{
				Name: envName,
				ValueFrom: &corev1.EnvVarSource{
					ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: v.ConfigMapRef.Name},
						Key:                  v.ConfigMapRef.Key,
					},
				},
			})
		}
	}
	return out
}

// formatUnresolved produces a short, human-readable summary of unresolved
// references for use in the Workspace status message. It is intentionally
// truncated: a workspace that depends on a missing Secret typically has at
// most a handful of failing variables, and the full list is already
// available on the underlying VariableSet's status for anyone who wants
// detail.
func formatUnresolved(refs []v1alpha1.UnresolvedReference) string {
	const maxItems = 3
	parts := make([]string, 0, len(refs))
	for i, r := range refs {
		if i >= maxItems {
			parts = append(parts, fmt.Sprintf("and %d more", len(refs)-maxItems))
			break
		}
		switch r.Kind {
		case "VariableSet":
			parts = append(parts, fmt.Sprintf("VariableSet %q: %s", r.Name, r.Reason))
		default:
			parts = append(parts, fmt.Sprintf("%s %q key %q: %s", r.Kind, r.Name, r.Key, r.Reason))
		}
	}
	return strings.Join(parts, "; ")
}

// findWorkspacesForVariableSet maps VariableSet watch events to Workspace
// reconcile requests.
//
// When a VariableSet changes its spec, or when the VariableSet controller
// updates its status after observing a rotated Secret, every Workspace that
// references this set (directly or via its parent Project) should re-resolve
// its inputs and recompute the variables fingerprint. This mapper performs
// that fanout so the workspace controller does not have to watch every
// Secret and ConfigMap in the namespace itself.
func (r *WorkspaceReconciler) findWorkspacesForVariableSet(ctx context.Context, o client.Object) []reconcile.Request {
	vs, ok := o.(*v1alpha1.VariableSet)
	if !ok {
		return nil
	}

	var workspaces v1alpha1.WorkspaceList
	if err := r.List(ctx, &workspaces, client.InNamespace(vs.Namespace)); err != nil {
		log.FromContext(ctx).Error(err, "Failed to list workspaces for variableset change", "variableset", vs.Name)
		return nil
	}

	// Cache project lookups so a namespace with many workspaces sharing
	// the same project only pays for a single API round trip per project.
	projectCache := map[string]*v1alpha1.Project{}

	var requests []reconcile.Request
	for i := range workspaces.Items {
		ws := &workspaces.Items[i]
		if workspaceReferencesVariableSet(ctx, r.Client, ws, vs.Name, projectCache) {
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: ws.Name, Namespace: ws.Namespace},
			})
		}
	}
	return requests
}

// workspaceReferencesVariableSet reports whether a Workspace's effective
// VariableSet composition includes the named set, either directly via
// spec.variableSetRef or transitively via its parent Project. The project
// lookup is cached so the caller can batch many workspace checks together
// cheaply.
func workspaceReferencesVariableSet(ctx context.Context, c client.Client, ws *v1alpha1.Workspace, name string, projectCache map[string]*v1alpha1.Project) bool {
	for _, ref := range ws.Spec.VariableSetRef {
		if ref.Name == name {
			return true
		}
	}

	projectKey := ws.Namespace + "/" + ws.Spec.ProjectRef.Name
	project, cached := projectCache[projectKey]
	if !cached {
		project = &v1alpha1.Project{}
		if err := c.Get(ctx, types.NamespacedName{Name: ws.Spec.ProjectRef.Name, Namespace: ws.Namespace}, project); err != nil {
			projectCache[projectKey] = nil
			return false
		}
		projectCache[projectKey] = project
	}
	if project == nil {
		return false
	}
	for _, ref := range project.Spec.VariableSetRef {
		if ref.Name == name {
			return true
		}
	}
	return false
}
