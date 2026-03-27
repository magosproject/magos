package terraform

import (
	"io"
	// "github.je-labs.com/mobius/mobius/internal/view"
)

// Option configures a TerraformClient.
type Option func(*TerraformClient)

// WithExecPath sets a custom path to the terraform binary.
func WithExecPath(p string) Option {
	return func(c *TerraformClient) {
		c.execPath = p
	}
}

// WithOutput sets custom stdout and stderr writers.
func WithOutput(stdout, stderr io.Writer) Option {
	return func(c *TerraformClient) {
		c.stdout = stdout
		c.stderr = stderr
	}
}

// WithStdout sets the stdout writer.
func WithStdout(w io.Writer) Option {
	return func(c *TerraformClient) {
		c.stdout = w
	}
}

// WithStderr sets the stderr writer.
func WithStderr(w io.Writer) Option {
	return func(c *TerraformClient) {
		c.stderr = w
	}
}

// WithEnv sets environment variables for Terraform execution.
func WithEnv(env map[string]string) Option {
	return func(c *TerraformClient) {
		c.env = env
	}
}

// WithVersionConstraint sets a required version constraint for terraform binary (e.g. ">= 1.5, < 2.0").
func WithVersionConstraint(constraint string) Option {
	return func(c *TerraformClient) {
		c.versionConstraint = constraint
	}
}

// // WithLogger sets a logger for the terraform client.
// func WithLogger(logger view.Logger) Option {
// 	return func(c *TerraformClient) {
// 		c.logger = logger
// 	}
// }
