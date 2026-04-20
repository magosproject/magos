package terraform

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"

	version "github.com/magosproject/go-version"
	tfexec "github.com/magosproject/terraform-exec/tfexec"
)

type TerraformClient struct {
	workDir           string
	execPath          string
	tf                *tfexec.Terraform
	stdout            io.Writer
	stderr            io.Writer
	env               map[string]string
	versionConstraint string
	// logger            view.Logger
}

func NewClient(workDir string, opts ...Option) (*TerraformClient, error) {
	c := &TerraformClient{workDir: workDir}
	for _, o := range opts {
		o(c)
	}

	// // Ensure we always have a valid logger
	// if c.logger == nil {
	// 	c.logger = view.NewNopLogger()
	// }

	// Default to os.Stdout and os.Stderr if not set
	if c.stdout == nil {
		c.stdout = os.Stdout
	}
	if c.stderr == nil {
		c.stderr = os.Stderr
	}

	if c.execPath == "" {
		// discover via PATH
		p, err := exec.LookPath("terraform")
		if err != nil {
			return nil, fmt.Errorf("terraform binary not found in PATH: %w", err)
		}
		c.execPath = p
	}
	abs, err := filepath.Abs(workDir)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve work dir: %w", err)
	}
	c.workDir = abs
	return c, nil
}

// Ensure initializes the underlying tfexec instance and validates version constraint if provided.
func (c *TerraformClient) Ensure(ctx context.Context) error {
	if c.tf != nil {
		return nil
	}

	// verify workdir exists
	if fi, err := os.Stat(c.workDir); err != nil || !fi.IsDir() {
		return fmt.Errorf("workdir invalid: %s", c.workDir)
	}

	// c.logger.Debug("Initializing terraform client", "workDir", c.workDir, "execPath", c.execPath)

	terraform, err := tfexec.NewTerraform(c.workDir, c.execPath)
	if err != nil {
		// c.logger.Error("Failed to create terraform instance", "error", err, "workDir", c.workDir)
		return fmt.Errorf("failed to create terraform instance: %w", err)
	}
	c.tf = terraform

	terraform.SetStdout(c.stdout)
	terraform.SetStderr(c.stderr)
	if c.env != nil {
		if err := terraform.SetEnv(c.env); err != nil {
			// c.logger.Error("Failed to set environment variables", "error", err)
			return fmt.Errorf("failed to set environment variables: %w", err)
		}
	}

	// version check
	if c.versionConstraint != "" {
		// c.logger.Debug("Checking terraform version constraint", "constraint", c.versionConstraint)
		v, _, err := terraform.Version(ctx, true)
		if err != nil {
			// c.logger.Error("Failed to get terraform version", "error", err)
			return fmt.Errorf("failed to get terraform version: %w", err)
		}
		cons, err := version.NewConstraint(c.versionConstraint)
		if err != nil {
			// c.logger.Error("Invalid version constraint", "constraint", c.versionConstraint, "error", err)
			return fmt.Errorf("invalid version constraint %q: %w", c.versionConstraint, err)
		}
		if !cons.Check(v) {
			// c.logger.Error("Terraform version does not meet constraint", "version", v.String(), "constraint", c.versionConstraint)
			return fmt.Errorf("terraform version %s does not satisfy constraint %s", v.String(), c.versionConstraint)
		}
		// c.logger.Debug("Terraform version check passed", "version", v.String(), "constraint", c.versionConstraint)
	}

	// c.logger.Debug("Terraform client initialized successfully")
	return nil
}

// Init runs `terraform init` with optional extra options.
func (c *TerraformClient) Init(ctx context.Context, extra ...tfexec.InitOption) error {
	// c.logger.Info("Running terraform init", "workDir", c.workDir)

	if err := c.Ensure(ctx); err != nil {
		return err
	}

	if err := c.tf.Init(ctx, extra...); err != nil {
		// c.logger.Error("Terraform init failed", "error", err)
		return fmt.Errorf("failed to run terraform init: %w", err)
	}

	// c.logger.Info("Terraform init completed successfully")
	return nil
}

// Plan executes terraform plan. If planOutPath not empty, a plan file is created.
func (c *TerraformClient) Plan(ctx context.Context, planOutPath string, extra ...tfexec.PlanOption) (bool, error) {
	// c.logger.Info("Running terraform plan", "planFile", planOutPath)

	if err := c.Ensure(ctx); err != nil {
		return false, err
	}
	opts := append([]tfexec.PlanOption{}, extra...)
	if planOutPath != "" {
		opts = append(opts, tfexec.Out(planOutPath))
	}
	changed, err := c.tf.Plan(ctx, opts...)
	if err != nil {
		// c.logger.Error("Terraform plan failed", "error", err)
		return false, fmt.Errorf("failed to run terraform plan: %w", err)
	}

	// c.logger.Info("Terraform plan completed", "hasChanges", changed)
	return changed, nil
}

// Apply applies changes. If planPath provided, uses that plan file; else applies the directory (may prompt interactively).
func (c *TerraformClient) Apply(ctx context.Context, planPath string, extra ...tfexec.ApplyOption) error {
	// c.logger.Info("Running terraform apply", "planFile", planPath)

	if err := c.Ensure(ctx); err != nil {
		return err
	}
	opts := append([]tfexec.ApplyOption{}, extra...)
	if planPath != "" {
		// DirOrPlan option instructs terraform to apply a saved plan file
		opts = append(opts, tfexec.DirOrPlan(planPath))
	}
	if err := c.tf.Apply(ctx, opts...); err != nil {
		// c.logger.Error("Terraform apply failed", "error", err)
		return fmt.Errorf("failed to run terraform apply: %w", err)
	}

	// c.logger.Info("Terraform apply completed successfully")
	return nil
}

// Destroy destroys infrastructure. Optional extra destroy options.
func (c *TerraformClient) Destroy(ctx context.Context, extra ...tfexec.DestroyOption) error {
	// c.logger.Info("Running terraform destroy")

	if err := c.Ensure(ctx); err != nil {
		return err
	}

	if err := c.tf.Destroy(ctx, extra...); err != nil {
		// c.logger.Error("Terraform destroy failed", "error", err)
		return fmt.Errorf("failed to run terraform destroy: %w", err)
	}

	// c.logger.Info("Terraform destroy completed successfully")
	return nil
}

// Output retrieves terraform outputs.
func (c *TerraformClient) Output(ctx context.Context, extra ...tfexec.OutputOption) (map[string]tfexec.OutputMeta, error) {
	if err := c.Ensure(ctx); err != nil {
		return nil, err
	}
	out, err := c.tf.Output(ctx, extra...)
	if err != nil {
		return nil, fmt.Errorf("failed to get terraform output: %w", err)
	}
	return out, nil
}

// Version returns terraform CLI version string.
func (c *TerraformClient) Version(ctx context.Context) (string, error) {
	if err := c.Ensure(ctx); err != nil {
		return "", err
	}
	v, _, err := c.tf.Version(ctx, true)
	if err != nil {
		return "", fmt.Errorf("failed to get terraform version: %w", err)
	}
	return v.String(), nil
}

// WorkspaceSelectOrNew selects the workspace, creating it if missing.
func (c *TerraformClient) WorkspaceSelectOrNew(ctx context.Context, name string) error {
	if err := c.Ensure(ctx); err != nil {
		return err
	}
	if name == "" || name == "default" {
		return nil
	}
	list, _, err := c.tf.WorkspaceList(ctx)
	if err != nil {
		return fmt.Errorf("failed to list workspaces: %w", err)
	}
	if slices.Contains(list, name) {
		return c.tf.WorkspaceSelect(ctx, name)
	}
	if err := c.tf.WorkspaceNew(ctx, name); err != nil {
		return fmt.Errorf("failed to create workspace %s: %w", name, err)
	}
	return nil
}

// ShowPlanFileRaw returns the JSON representation of a saved plan file,
// equivalent to running `terraform show -json <planFile>`.
//
// Note: tfexec.ShowPlanFileRaw despite its name runs `terraform show` without
// `-json`, returning human-readable text. We call tfexec.ShowPlanFile (which
// does produce structured JSON) and marshal it ourselves so callers get real
// JSON to feed into downstream tools like the kyverno-json engine.
func (c *TerraformClient) ShowPlanFileRaw(ctx context.Context, planFile string) (string, error) {
	if err := c.Ensure(ctx); err != nil {
		return "", err
	}
	// tfexec tees the child process stdout to the writer we set via SetStdout,
	// which is os.Stdout by default. For show -json that means the full plan
	// JSON is dumped into pod logs on every run. We already capture the parsed
	// plan below, so silence stdout for this call and restore it after.
	c.tf.SetStdout(io.Discard)
	defer c.tf.SetStdout(c.stdout)

	plan, err := c.tf.ShowPlanFile(ctx, planFile)
	if err != nil {
		return "", fmt.Errorf("failed to run terraform show -json: %w", err)
	}
	b, err := json.Marshal(plan)
	if err != nil {
		return "", fmt.Errorf("failed to marshal plan JSON: %w", err)
	}
	return string(b), nil
}

// SetEnv replaces environment variables for future terraform commands.
func (c *TerraformClient) SetEnv(vars map[string]string) error {
	if c.tf == nil {
		c.env = vars
		return nil
	}
	return c.tf.SetEnv(vars)
}
