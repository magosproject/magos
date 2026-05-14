package terraform

import (
	"io"
	// "github.com/mobius/mobius/internal/view"
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

// WithLog enables Terraform debug logging at the given level (TRACE, DEBUG, INFO, WARN, ERROR).
// Output is written to logPath, which should be /dev/stderr to appear in pod logs.
// SetLog must be called before SetLogPath in the tfexec API, which this option handles correctly.
func WithLog(level, path string) Option {
	return func(c *TerraformClient) {
		c.logLevel = level
		c.logPath = path
	}
}

// WithColor enables ANSI colour sequences in Terraform output. By default
// the client runs Terraform with -no-color to ensure plain-text output.
func WithColor() Option {
	return func(c *TerraformClient) {
		c.color = true
	}
}

// // WithLogger sets a logger for the terraform client.
// func WithLogger(logger view.Logger) Option {
// 	return func(c *TerraformClient) {
// 		c.logger = logger
// 	}
// }
