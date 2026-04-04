package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/hashicorp/terraform-exec/tfexec"
	"github.com/magosproject/magos/internal/terraform"
)

func main() {
	// Read configuration from environment variables injected by the Operator
	repoURL := os.Getenv("REPO_URL")
	targetRevision := os.Getenv("TARGET_REVISION")
	tfVersion := os.Getenv("TF_VERSION")
	path := os.Getenv("TF_PATH")
	tfvarsPath := os.Getenv("TF_VAR_FILE")
	gitUser := os.Getenv("GIT_USERNAME")
	gitPass := os.Getenv("GIT_PASSWORD")

	if repoURL == "" || targetRevision == "" || tfVersion == "" {
		fmt.Println("Error: REPO_URL, TARGET_REVISION, and TF_VERSION are required environment variables")
		os.Exit(1)
	}

	ctx := context.Background()

	// 1. Setup Temporary Directory
	tmpDir, err := os.MkdirTemp("", "magos-job-*")
	if err != nil {
		fmt.Printf("Failed to create temporary directory: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			fmt.Printf("Failed to remove temporary directory %s: %v", tmpDir, err)
			os.Exit(1)
		}
	}()

	fmt.Printf("Cloning repository %s @ %s into %s\n", repoURL, targetRevision, tmpDir)

	cloneOpts := &git.CloneOptions{
		URL:   repoURL,
		Depth: 1, // Shallow clone for performance
	}

	if gitUser != "" || gitPass != "" {
		cloneOpts.Auth = &http.BasicAuth{
			Username: gitUser,
			Password: gitPass,
		}
	}

	// 2. Clone Repository
	repo, err := git.PlainClone(tmpDir, false, cloneOpts)
	if err != nil {
		fmt.Printf("Failed to clone repository: %v\n", err)
		os.Exit(1)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		fmt.Printf("Failed to get worktree: %v\n", err)
		os.Exit(1)
	}

	// 3. Checkout Target Revision
	hash, err := repo.ResolveRevision(plumbing.Revision(targetRevision))
	if err != nil {
		fmt.Printf("Failed to resolve target revision %s: %v\n", targetRevision, err)
		os.Exit(1)
	}

	err = worktree.Checkout(&git.CheckoutOptions{
		Hash: *hash,
	})
	if err != nil {
		fmt.Printf("Failed to checkout revision: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Successfully checked out repository at commit %s\n", hash.String())

	// 4. Resolve Terraform Working Directory
	workDir := tmpDir
	if path != "" && path != "." {
		workDir = filepath.Join(tmpDir, path)
	}

	// 5. Initialize Terraform Client (and download the binary)
	fmt.Printf("Initializing Terraform client (Version: %s)\n", tfVersion)
	tfClient, err := terraform.NewClientFromInstall(ctx, workDir, tfVersion, "")
	if err != nil {
		fmt.Printf("Failed to initialize Terraform client: %v\n", err)
		os.Exit(1)
	}

	// 6. Terraform Init
	fmt.Println("Running 'terraform init'...")
	if err := tfClient.Init(ctx); err != nil {
		fmt.Printf("Terraform Init failed: %v\n", err)
		os.Exit(1)
	}

	// 7. Terraform Plan
	fmt.Println("Running 'terraform plan'...")
	var planOpts []tfexec.PlanOption
	if tfvarsPath != "" {
		tfvarsFile := filepath.Join(tmpDir, tfvarsPath)
		planOpts = append(planOpts, tfexec.VarFile(tfvarsFile))
	}

	hasChanges, err := tfClient.Plan(ctx, "", planOpts...)
	if err != nil {
		fmt.Printf("Terraform Plan failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Terraform Plan completed successfully. Infrastructure has changes: %v\n", hasChanges)
}
