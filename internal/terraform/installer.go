package terraform

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	version "github.com/hashicorp/go-version"
	install "github.com/hashicorp/hc-install"
	"github.com/hashicorp/hc-install/fs"
	"github.com/hashicorp/hc-install/product"
	"github.com/hashicorp/hc-install/releases"
	"github.com/hashicorp/hc-install/src"
)

// BinaryInstaller handles acquiring a terraform binary that satisfies a version constraint.
type BinaryInstaller struct {
	constraint string
	exact      string
	logger     *log.Logger
}

// NewBinaryInstaller creates an installer; pass either exactVersion ("1.8.6") or constraint (">= 1.6, < 2.0").
func NewBinaryInstaller(exactVersion, constraint string) *BinaryInstaller {
	return &BinaryInstaller{constraint: constraint, exact: exactVersion}
}

// WithLogger sets a logger used by hc-install.
func (b *BinaryInstaller) WithLogger(l *log.Logger) *BinaryInstaller { b.logger = l; return b }

// Install ensures a terraform binary and returns its absolute execPath.
func (b *BinaryInstaller) Install(ctx context.Context) (string, error) {
	if b.exact == "" && b.constraint == "" {
		return "", fmt.Errorf("either exact version or constraint must be specified")
	}
	inst := install.NewInstaller()
	if b.logger != nil {
		inst.SetLogger(b.logger)
	}

	var sources []src.Source
	if b.exact != "" {
		ver := version.Must(version.NewVersion(b.exact))
		sources = append(sources, &releases.ExactVersion{Product: product.Terraform, Version: ver})
	} else if b.constraint != "" {
		cons := version.MustConstraints(version.NewConstraint(b.constraint))
		sources = append(sources,
			&fs.Version{Product: product.Terraform, Constraints: cons},             // prefer existing
			&releases.LatestVersion{Product: product.Terraform, Constraints: cons}, // else download
		)
	} else {
		// any version in PATH else latest
		sources = append(sources, &fs.AnyVersion{Product: &product.Terraform}, &releases.LatestVersion{Product: product.Terraform})
	}

	execPath, err := inst.Ensure(ctx, sources)
	if err != nil {
		return "", fmt.Errorf("ensure terraform binary: %w", err)
	}
	return execPath, nil
}

// NewClientFromInstall obtains a terraform binary and returns a ready TerraformClient (not initialized) with version constraint enforcement.
func NewClientFromInstall(ctx context.Context, workDir string, exactVersion, constraint string, opts ...Option) (*TerraformClient, error) {
	execPath, err := NewBinaryInstaller(exactVersion, constraint).Install(ctx)
	if err != nil {
		return nil, err
	}
	// include version constraint option so Ensure validates
	if constraint != "" {
		opts = append(opts, WithVersionConstraint(constraint))
	}
	opts = append([]Option{WithExecPath(execPath)}, opts...)
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return nil, fmt.Errorf("create work dir: %w", err)
	}
	abs, _ := filepath.Abs(workDir)
	c, err := NewClient(abs, opts...)
	if err != nil {
		return nil, err
	}
	if err := c.Ensure(ctx); err != nil {
		return nil, err
	}
	return c, nil
}

// ParseConstraint normalizes a constraint string to what go-version expects (returns empty if input empty).
func ParseConstraint(raw string) (string, error) {
	if raw == "" {
		return "", nil
	}
	// Validate by constructing constraints
	if _, err := version.NewConstraint(raw); err != nil {
		return "", fmt.Errorf("invalid constraint: %w", err)
	}
	return raw, nil
}
