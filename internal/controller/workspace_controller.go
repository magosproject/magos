/*
Copyright 2026.

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

package controller

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/hashicorp/terraform-exec/tfexec"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	magosprojectiov1alpha1 "github.com/magosproject/magos/api/v1alpha1"
	"github.com/magosproject/magos/internal/terraform"
)

// WorkspaceReconciler reconciles a Workspace object
type WorkspaceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=magosproject.io,resources=workspaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=magosproject.io,resources=workspaces/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=magosproject.io,resources=workspaces/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *WorkspaceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Fetch the Workspace instance
	var workspace magosprojectiov1alpha1.Workspace
	if err := r.Get(ctx, req.NamespacedName, &workspace); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log.Info("Reconciling Workspace", "name", workspace.Name, "namespace", workspace.Namespace)

	// Create a temporary directory for the git clone
	tmpDir, err := os.MkdirTemp("", fmt.Sprintf("magos-workspace-%s-*", workspace.Name))
	if err != nil {
		log.Error(err, "Failed to create temporary directory")
		return ctrl.Result{}, err
	}
	defer os.RemoveAll(tmpDir) // Clean up the temp directory after reconciliation

	log.Info("Cloning repository", "url", workspace.Spec.Source.RepoURL, "revision", workspace.Spec.Source.TargetRevision, "dir", tmpDir)

	cloneOpts := &git.CloneOptions{
		URL:   workspace.Spec.Source.RepoURL,
		Depth: 1, // Shallow clone for performance and reduced disk/memory usage
	}

	// Configure Git authentication if a SecretRef is provided
	if workspace.Spec.Source.SecretRef != nil {
		var secret corev1.Secret
		secretKey := types.NamespacedName{
			Name:      workspace.Spec.Source.SecretRef.Name,
			Namespace: workspace.Namespace, // Assuming secret is in the same namespace
		}

		if err := r.Get(ctx, secretKey, &secret); err != nil {
			log.Error(err, "Failed to fetch Git authentication secret", "secret", secretKey.Name)
			return ctrl.Result{}, err
		}

		// Basic Auth (HTTPS)
		username := string(secret.Data["username"])
		password := string(secret.Data["password"])

		if username != "" || password != "" {
			cloneOpts.Auth = &http.BasicAuth{
				Username: username,
				Password: password,
			}
		}
		// Note: SSH auth (e.g. using ssh.PublicKeys) can be added here as an alternative
	}

	// Clone the repository
	repo, err := git.PlainClone(tmpDir, false, cloneOpts)
	if err != nil {
		log.Error(err, "Failed to clone repository")
		return ctrl.Result{}, err
	}

	// Checkout the target revision
	worktree, err := repo.Worktree()
	if err != nil {
		log.Error(err, "Failed to get worktree")
		return ctrl.Result{}, err
	}

	// Try to resolve the revision to a hash
	hash, err := repo.ResolveRevision(plumbing.Revision(workspace.Spec.Source.TargetRevision))
	if err != nil {
		log.Error(err, "Failed to resolve target revision")
		return ctrl.Result{}, err
	}

	err = worktree.Checkout(&git.CheckoutOptions{
		Hash: *hash,
	})
	if err != nil {
		log.Error(err, "Failed to checkout revision")
		return ctrl.Result{}, err
	}

	log.Info("Successfully checked out repository", "hash", hash.String())

	// Set up the Terraform working directory
	workDir := tmpDir
	if workspace.Spec.Source.Path != "" && workspace.Spec.Source.Path != "." {
		workDir = filepath.Join(tmpDir, workspace.Spec.Source.Path)
	}

	log.Info("Initializing Terraform client", "version", workspace.Spec.Terraform.Version, "workDir", workDir)

	// Create Terraform client and automatically install the specified binary version
	tfClient, err := terraform.NewClientFromInstall(ctx, workDir, workspace.Spec.Terraform.Version, "")
	if err != nil {
		log.Error(err, "Failed to initialize Terraform client")
		return ctrl.Result{}, err
	}

	// Run Terraform Init
	log.Info("Running Terraform Init")
	var initOpts []tfexec.InitOption
	if err := tfClient.Init(ctx, initOpts...); err != nil {
		log.Error(err, "Terraform Init failed")
		return ctrl.Result{}, err
	}

	// Run Terraform Plan
	log.Info("Running Terraform Plan")
	var planOpts []tfexec.PlanOption
	if workspace.Spec.Terraform.TfvarsPath != "" {
		// tfvars path should be evaluated relative to the repository root
		tfvarsFile := filepath.Join(tmpDir, workspace.Spec.Terraform.TfvarsPath)
		planOpts = append(planOpts, tfexec.VarFile(tfvarsFile))
	}

	hasChanges, err := tfClient.Plan(ctx, "", planOpts...)
	if err != nil {
		log.Error(err, "Terraform Plan failed")
		return ctrl.Result{}, err
	}

	log.Info("Terraform Plan completed", "hasChanges", hasChanges)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *WorkspaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&magosprojectiov1alpha1.Workspace{}).
		Named("workspace").
		Complete(r)
}
