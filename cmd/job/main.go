package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/hashicorp/terraform-exec/tfexec"
	gossh "golang.org/x/crypto/ssh"

	"github.com/magosproject/magos/internal/terraform"
)

// Config holds the job configuration derived from environment variables.
type Config struct {
	RepoURL        string
	TargetRevision string
	TFVersion      string
	TFPath         string
	TFVarsPath     string
	GitUser        string
	GitPass        string
	GitSSHKey      string
	JobType        string
	PlanFile       string
}

// loadConfig reads and validates the required environment variables.
func loadConfig() (*Config, error) {
	cfg := &Config{
		RepoURL:        os.Getenv("REPO_URL"),
		TargetRevision: os.Getenv("TARGET_REVISION"),
		TFVersion:      os.Getenv("TF_VERSION"),
		TFPath:         os.Getenv("TF_PATH"),
		TFVarsPath:     os.Getenv("TF_VAR_FILE"),
		GitUser:        os.Getenv("GIT_USERNAME"),
		GitPass:        os.Getenv("GIT_PASSWORD"),
		GitSSHKey:      os.Getenv("GIT_SSH_PRIVATE_KEY"),
		JobType:        os.Getenv("MAGOS_JOB_TYPE"),
		PlanFile:       os.Getenv("MAGOS_PLAN_FILE"),
	}

	if cfg.RepoURL == "" || cfg.TargetRevision == "" || cfg.TFVersion == "" || cfg.JobType == "" || cfg.PlanFile == "" {
		return nil, errors.New("REPO_URL, TARGET_REVISION, TF_VERSION, MAGOS_JOB_TYPE, and MAGOS_PLAN_FILE are required")
	}

	if cfg.JobType != "plan" && cfg.JobType != "apply" {
		return nil, fmt.Errorf("invalid MAGOS_JOB_TYPE: %s (must be 'plan' or 'apply')", cfg.JobType)
	}

	return cfg, nil
}

// getAuthMethod resolves the git authentication strategy based on provided
// credentials.
func getAuthMethod(cfg *Config) (transport.AuthMethod, error) {
	if cfg.GitSSHKey != "" {
		publicKeys, err := ssh.NewPublicKeys("git", []byte(cfg.GitSSHKey), "")
		if err != nil {
			return nil, fmt.Errorf("failed to parse SSH private key: %w", err)
		}
		// In a highly sandboxed, ephemeral Kubernetes pod, we bypass strict
		// host key checking by default unless a known_hosts mechanism is
		// explicitly provided.
		publicKeys.HostKeyCallback = gossh.InsecureIgnoreHostKey()
		return publicKeys, nil
	}

	if cfg.GitUser != "" || cfg.GitPass != "" {
		return &http.BasicAuth{
			Username: cfg.GitUser,
			Password: cfg.GitPass,
		}, nil
	}

	return nil, nil // Public repository fallback
}

// cloneRepository clones the target repository and checks out the specific
// revision.
func cloneRepository(ctx context.Context, cfg *Config, dest string) error {
	auth, err := getAuthMethod(cfg)
	if err != nil {
		return err
	}

	log.Printf("Cloning repository %s into %s", cfg.RepoURL, dest)
	repo, err := git.PlainCloneContext(ctx, dest, false, &git.CloneOptions{
		URL:   cfg.RepoURL,
		Auth:  auth,
		Depth: 1, // Shallow clone to minimize memory and network usage
	})
	if err != nil {
		return fmt.Errorf("failed to clone repository: %w", err)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	hash, err := repo.ResolveRevision(plumbing.Revision(cfg.TargetRevision))
	if err != nil {
		return fmt.Errorf("failed to resolve target revision %q: %w", cfg.TargetRevision, err)
	}

	if err := worktree.Checkout(&git.CheckoutOptions{Hash: *hash}); err != nil {
		return fmt.Errorf("failed to checkout revision %s: %w", hash.String(), err)
	}

	log.Printf("Successfully checked out revision %s", hash.String())
	return nil
}

// execTerraform orchestrates the terraform init, plan, and apply lifecycle.
func execTerraform(ctx context.Context, cfg *Config, cloneDir string) error {
	workDir := cloneDir
	if cfg.TFPath != "" && cfg.TFPath != "." {
		workDir = filepath.Join(cloneDir, cfg.TFPath)
	}

	log.Printf("Initializing Terraform %s in %s", cfg.TFVersion, workDir)
	tfClient, err := terraform.NewClientFromInstall(ctx, workDir, cfg.TFVersion, "")
	if err != nil {
		return fmt.Errorf("failed to initialize terraform client: %w", err)
	}

	log.Println("Running 'terraform init'...")
	if err := tfClient.Init(ctx); err != nil {
		return fmt.Errorf("terraform init failed: %w", err)
	}

	switch cfg.JobType {
	case "plan":
		log.Println("Running 'terraform plan'...")
		var planOpts []tfexec.PlanOption
		if cfg.TFVarsPath != "" {
			tfvarsFile := filepath.Join(cloneDir, cfg.TFVarsPath)
			planOpts = append(planOpts, tfexec.VarFile(tfvarsFile))
		}
		planOpts = append(planOpts, tfexec.Out(cfg.PlanFile))

		hasChanges, err := tfClient.Plan(ctx, "", planOpts...)
		if err != nil {
			return fmt.Errorf("terraform plan failed: %w", err)
		}
		log.Printf("Terraform plan completed successfully. Changes present: %v", hasChanges)

	case "apply":
		log.Printf("Running 'terraform apply' using plan %s...", cfg.PlanFile)

		// Hack to handle local kind (Kubernetes-in-Docker) filesystem lag. My
		// (@bschaatsbergen) container runtime (using sshfs or virtiofs under
		// the hood) aggressively caches directory metadata. The 'plan' pod
		// writes the plan file and dies, but the 'apply' pod spins up so fast
		// it gets a cache miss and thinks the file is missing. We just poll for
		// a few seconds to let the VFS catch up. Real cloud block storage
		// usually doesn't have this problem.
		var planExists bool
		for range 10 {
			if _, err := os.Stat(cfg.PlanFile); err == nil {
				planExists = true
				break
			}
			log.Printf("Waiting for plan file %s to become available...", cfg.PlanFile)
			time.Sleep(1 * time.Second)
		}

		if !planExists {
			return fmt.Errorf(
				"terraform apply failed: plan file %s does not exist after waiting. "+
					"The plan job may not have successfully transferred the state via the PVC",
				cfg.PlanFile,
			)
		}

		if err := tfClient.Apply(ctx, cfg.PlanFile); err != nil {
			return fmt.Errorf("terraform apply failed: %w", err)
		}
		log.Println("Terraform apply completed successfully.")

		// Secure cleanup of the plan file post-apply to prevent state/secret
		// leakage
		log.Printf("Cleaning up plan file %s...", cfg.PlanFile)
		if err := os.Remove(cfg.PlanFile); err != nil && !os.IsNotExist(err) {
			log.Printf("Warning: failed to delete plan file: %v", err)
		}
	}

	return nil
}

func run() error {
	// Configure logging for better observability in Kubernetes pods
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)
	log.SetPrefix("[magos-job] ")

	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	tmpDir, err := os.MkdirTemp("", "magos-workspace-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary workspace directory: %w", err)
	}
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	ctx := context.Background()

	if err := cloneRepository(ctx, cfg, tmpDir); err != nil {
		return err
	}

	if err := execTerraform(ctx, cfg, tmpDir); err != nil {
		return err
	}

	return nil
}

func main() {
	if err := run(); err != nil {
		log.Fatalf("Error: %v", err)
	}
}
