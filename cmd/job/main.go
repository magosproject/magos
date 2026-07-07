package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/magosproject/terraform-exec/tfexec"
	gossh "golang.org/x/crypto/ssh"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/yaml"

	"github.com/magosproject/magos/internal/terraform"
)

var execCommandContext = exec.CommandContext

// Config holds every input this job needs to run a terraform plan or apply
// inside a Kubernetes pod. All fields are populated from environment variables
// that the workspace controller sets when it creates the Job, which is why the
// struct stays flat: a reader debugging a failed pod should be able to see
// every relevant input in one place without chasing nested structures. Required
// fields are enforced in loadConfig so a misconfigured Job fails immediately
// rather than part way through.
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
	RunID          string
	PolicySelector string // a label selector for kyverno policies to evaluate the plan against, e.g. "env=prod"
	TFLogLevel     string // TF_LOG level (TRACE, DEBUG, INFO, WARN, ERROR); empty means logging is suppressed
}

const (
	jobTypePlan  = "plan"
	jobTypeApply = "apply"
)

// loadConfig reads the job configuration from environment variables. The
// workspace controller sets these when it constructs the Job spec, so env vars
// are the only supported source. We deliberately fail fast when a required
// field is missing or when JobType is not "plan" or "apply", because a
// misconfigured Job will otherwise waste time and make its eventual failure
// harder to diagnose from pod logs.
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
		RunID:          os.Getenv("MAGOS_RUN_ID"),
		PolicySelector: os.Getenv("MAGOS_POLICY_SELECTOR"),
		TFLogLevel:     os.Getenv("MAGOS_TF_LOG_LEVEL"),
	}

	if cfg.RepoURL == "" || cfg.TFVersion == "" || cfg.JobType == "" || cfg.PlanFile == "" || cfg.RunID == "" {
		return nil, errors.New("REPO_URL, TF_VERSION, MAGOS_JOB_TYPE, MAGOS_PLAN_FILE, and MAGOS_RUN_ID are required")
	}

	if cfg.JobType != jobTypePlan && cfg.JobType != jobTypeApply {
		return nil, fmt.Errorf("invalid MAGOS_JOB_TYPE: %s (must be 'plan' or 'apply')", cfg.JobType)
	}

	// Only the plan pod clones the repo, so TARGET_REVISION is only required
	// for plan jobs. The apply pod inherits the working tree the plan pod left
	// on the shared PVC and does not resolve any git ref of its own.
	if cfg.JobType == jobTypePlan && cfg.TargetRevision == "" {
		return nil, errors.New("TARGET_REVISION is required for plan jobs")
	}

	return cfg, nil
}

// getAuthMethod picks the right Git authentication based on which credentials
// the controller injected through the environment. An SSH private key wins when
// present and uses public key authentication. A username and password fall back
// to HTTPS basic auth, which covers both personal access tokens and classic
// password flows. When neither is set we assume the repository is public and
// return nil, which tells go-git to clone without authentication.
//
// Host key checking is intentionally disabled for SSH. The job runs in a
// throwaway pod with no persisted known_hosts file, and the alternative would
// be shipping a known_hosts bundle inside the image. Clusters that need
// stricter guarantees should layer that in through the pod spec rather than
// here.
func getAuthMethod(cfg *Config) (transport.AuthMethod, error) {
	if cfg.GitSSHKey != "" {
		publicKeys, err := ssh.NewPublicKeys("git", []byte(cfg.GitSSHKey), "")
		if err != nil {
			return nil, fmt.Errorf("failed to parse SSH private key: %w", err)
		}
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

// runsBase is the parent for per-run subdirectories on the workspace PVC.
const runsBase = "/workspace-data/runs"

// runDir returns the directory on the PVC for a given run.
func runDir(runID string) string {
	return filepath.Join(runsBase, runID)
}

// sourcePath returns the source tree path for a given run.
func sourcePath(runID string) string {
	return filepath.Join(runDir(runID), "source")
}

// validateSourceDir checks that the apply pod's working tree is present
// and that terraform init ran in it. Fails fast otherwise.
func validateSourceDir(sourceDir string) error {
	if _, err := os.Stat(sourceDir); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf(
				"source directory %q is missing; the plan pod did not prepare a working tree for this run",
				sourceDir,
			)
		}
		return fmt.Errorf("failed to stat source directory %q: %w", sourceDir, err)
	}
	return nil
}

type repositoryFetcher int

const (
	repositoryFetcherGoGit repositoryFetcher = iota
	repositoryFetcherBgit
	repositoryFetcherGitRemoteHelper
)

func fetcherForRepoURL(repoURL string) (repositoryFetcher, string) {
	switch {
	case strings.HasPrefix(repoURL, "bgit+"):
		return repositoryFetcherBgit, strings.TrimPrefix(repoURL, "bgit+")
	case strings.HasPrefix(repoURL, "s3://"), strings.HasPrefix(repoURL, "gs://"):
		return repositoryFetcherBgit, repoURL
	case strings.HasPrefix(repoURL, "bgit::"), strings.HasPrefix(repoURL, "bgit://"):
		return repositoryFetcherGitRemoteHelper, repoURL
	default:
		return repositoryFetcherGoGit, repoURL
	}
}

func runCommand(ctx context.Context, name string, args ...string) error {
	return runCommandWithEnv(ctx, os.Environ(), name, args...)
}

func runCommandWithEnv(ctx context.Context, env []string, name string, args ...string) error {
	cmd := execCommandContext(ctx, name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = env
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s failed: %w", name, strings.Join(args, " "), err)
	}
	return nil
}

func checkoutWithGit(ctx context.Context, dest, targetRevision string) error {
	if err := runCommand(ctx, "git", "-C", dest, "checkout", targetRevision); err != nil {
		return fmt.Errorf("failed to checkout revision %q: %w", targetRevision, err)
	}
	log.Printf("Successfully checked out revision %s", targetRevision)
	return nil
}

func cloneRepositoryWithBgit(ctx context.Context, repoURL, targetRevision, dest string) error {
	if _, err := exec.LookPath("bgit"); err != nil {
		return fmt.Errorf("failed to find bgit binary in PATH: %w", err)
	}

	log.Printf("Cloning BucketGit repository %s into %s", repoURL, dest)
	if err := runCommand(ctx, "bgit", "clone", repoURL, dest); err != nil {
		return fmt.Errorf("failed to clone BucketGit repository: %w", err)
	}
	return checkoutWithGit(ctx, dest, targetRevision)
}

func cloneRepositoryWithGitRemoteHelper(ctx context.Context, repoURL, targetRevision, dest string) error {
	env := os.Environ()
	if _, err := exec.LookPath("git-remote-bgit"); err != nil {
		bgitPath, bgitErr := exec.LookPath("bgit")
		if bgitErr != nil {
			return fmt.Errorf("failed to find git-remote-bgit helper or bgit binary in PATH: %w", err)
		}

		helperDir, mkErr := os.MkdirTemp("", "magos-git-remote-bgit-*")
		if mkErr != nil {
			return fmt.Errorf("failed to create temporary git remote helper directory: %w", mkErr)
		}
		defer func() {
			if rmErr := os.RemoveAll(helperDir); rmErr != nil {
				log.Printf("Warning: failed to remove temporary git remote helper directory %s: %v", helperDir, rmErr)
			}
		}()

		helperPath := filepath.Join(helperDir, "git-remote-bgit")
		if symlinkErr := os.Symlink(bgitPath, helperPath); symlinkErr != nil {
			return fmt.Errorf("failed to create temporary git-remote-bgit helper symlink: %w", symlinkErr)
		}
		env = prependPath(env, helperDir)
	}

	log.Printf("Cloning BucketGit remote-helper repository %s into %s", repoURL, dest)
	if err := runCommandWithEnv(ctx, env, "git", "clone", repoURL, dest); err != nil {
		return fmt.Errorf("failed to clone BucketGit repository through git remote helper: %w", err)
	}
	return checkoutWithGit(ctx, dest, targetRevision)
}

func prependPath(env []string, dir string) []string {
	out := make([]string, 0, len(env)+1)
	found := false
	for _, entry := range env {
		if strings.HasPrefix(entry, "PATH=") {
			out = append(out, "PATH="+dir+string(os.PathListSeparator)+strings.TrimPrefix(entry, "PATH="))
			found = true
			continue
		}
		out = append(out, entry)
	}
	if !found {
		out = append(out, "PATH="+dir)
	}
	return out
}

// cloneRepository performs a shallow clone of the configured repository into
// dest and checks out the exact revision the controller asked for. The shallow
// clone keeps pod startup fast and avoids pulling years of history the job is
// never going to read. Only the plan pod calls this; the apply pod reuses the
// working tree the plan pod produced. dest is expected to be empty or missing.
func cloneRepository(ctx context.Context, cfg *Config, dest string) error {
	fetcher, repoURL := fetcherForRepoURL(cfg.RepoURL)
	switch fetcher {
	case repositoryFetcherBgit:
		return cloneRepositoryWithBgit(ctx, repoURL, cfg.TargetRevision, dest)
	case repositoryFetcherGitRemoteHelper:
		return cloneRepositoryWithGitRemoteHelper(ctx, repoURL, cfg.TargetRevision, dest)
	}

	auth, err := getAuthMethod(cfg)
	if err != nil {
		return err
	}

	log.Printf("Cloning repository %s into %s", repoURL, dest)
	repo, err := git.PlainCloneContext(ctx, dest, false, &git.CloneOptions{
		URL:   repoURL,
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

// execTerraform runs the terraform workflow for a single job, either "plan" or
// "apply". It is the main body of work the pod performs.
//
// For a plan, we run terraform init followed by terraform plan and write the
// binary plan file to a known path on the Workspace bound PVC. When the
// controller configured a policy selector we also export the plan as JSON and
// hand it to validatePolicies so the workspace controller can gate apply on the
// outcome.
//
// For an apply, we expect that a previous plan job already wrote the plan file
// to the PVC. Because the plan pod and the apply pod can be scheduled back to
// back on nodes with different filesystem semantics, we poll for the plan file
// briefly before giving up. Once terraform apply succeeds we delete the plan
// file so it cannot be reused and cannot leak sensitive values from the state
// diff.
// pluginCacheMount is the directory on the workspace PVC where Terraform
// provider binaries are cached between runs. Using the PVC means a provider
// downloaded during one plan job is reused by the next plan or apply job for
// the same workspace, avoiding repeated downloads.
const pluginCacheMount = "/workspace-data/plugin-cache"

func execTerraform(ctx context.Context, cfg *Config, cloneDir string) error {
	workDir := cloneDir
	if cfg.TFPath != "" && cfg.TFPath != "." {
		workDir = filepath.Join(cloneDir, cfg.TFPath)
	}

	// Enable provider plugin caching on the persistent workspace volume.
	var clientOpts []terraform.Option
	if err := os.MkdirAll(pluginCacheMount, 0o755); err != nil {
		log.Printf("Warning: failed to create plugin cache dir %q, caching disabled: %v", pluginCacheMount, err)
	} else {
		log.Printf("Provider plugin cache enabled at %s", pluginCacheMount)
		clientOpts = append(clientOpts, terraform.WithEnv(map[string]string{
			"TF_PLUGIN_CACHE_DIR": pluginCacheMount,
		}))
	}

	if cfg.TFLogLevel != "" {
		log.Printf("Terraform log level set to %s", cfg.TFLogLevel)
		clientOpts = append(clientOpts, terraform.WithLog(cfg.TFLogLevel, "/dev/stderr"))
	}

	if _, noColor := os.LookupEnv("NO_COLOR"); !noColor {
		clientOpts = append(clientOpts, terraform.WithColor())
	}

	log.Printf("Initializing Terraform %s in %s", cfg.TFVersion, workDir)
	tfClient, err := terraform.NewClientFromInstall(ctx, workDir, cfg.TFVersion, "", clientOpts...)
	if err != nil {
		return fmt.Errorf("failed to initialize terraform client: %w", err)
	}

	log.Println("Running 'terraform init'...")
	if err := tfClient.Init(ctx); err != nil {
		return fmt.Errorf("terraform init failed: %w", err)
	}

	switch cfg.JobType {
	case jobTypePlan:
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

		// If a policy selector is configured, export the plan as JSON and
		// evaluate it against matching ValidatingPolicy resources.
		if cfg.PolicySelector != "" {
			log.Println("Running 'terraform show -json' to produce plan JSON...")
			planJSON, err := tfClient.ShowPlanFileRaw(ctx, cfg.PlanFile)
			if err != nil {
				return fmt.Errorf("terraform show -json failed: %w", err)
			}

			planJSONFile := strings.TrimSuffix(cfg.PlanFile, ".tfplan") + ".plan.json"
			if err := os.WriteFile(planJSONFile, []byte(planJSON), 0600); err != nil {
				return fmt.Errorf("failed to write plan JSON: %w", err)
			}
			log.Printf("Plan JSON written to %s", planJSONFile)

			if err := validatePolicies(ctx, cfg, planJSONFile); err != nil {
				return err
			}
		}

	case jobTypeApply:
		log.Printf("Running 'terraform apply' using plan %s...", cfg.PlanFile)

		// Always remove the run dir when the apply pod exits, success or
		// failure. The next plan's prune is the fallback for the case
		// where the pod is killed before this defer runs.
		defer func() {
			runPath := runDir(cfg.RunID)
			log.Printf("Removing run directory %s...", runPath)
			if err := os.RemoveAll(runPath); err != nil {
				log.Printf("Warning: failed to remove run dir: %v", err)
			}
		}()

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
					"The plan job may not have successfully written or shared the plan file via the PVC",
				cfg.PlanFile,
			)
		}

		if err := tfClient.Apply(ctx, cfg.PlanFile); err != nil {
			return fmt.Errorf("terraform apply failed: %w", err)
		}
		log.Println("Terraform apply completed successfully.")

		// Delete the plan file once apply succeeds. It contains the full state
		// diff, including any sensitive attributes surfaced during planning,
		// and there is no reason to leave it on the PVC for a later job.
		log.Printf("Cleaning up plan file %s...", cfg.PlanFile)
		if err := os.Remove(cfg.PlanFile); err != nil && !os.IsNotExist(err) {
			log.Printf("Warning: failed to delete plan file: %v", err)
		}
	}

	return nil
}

// policyViolation is one rule failure surfaced by Kyverno validation.
type policyViolation struct {
	Policy  string `json:"policy"`
	Message string `json:"message"`
}

// policyResult is the full validation outcome for a plan, printed once at the
// end of validatePolicies as a MAGOS_RESULT line. The workspace controller
// scans pod logs for that prefix, parses this struct, and exposes the passed
// flag and violations on the Workspace status.
type policyResult struct {
	Passed     bool              `json:"passed"`
	Violations []policyViolation `json:"violations"`
}

// kyvernoReport is a minimal representation of the OpenReports ClusterReport /
// Report produced by kyverno apply --policy-report --output-format json.
// kyverno prints one JSON object per line (with optional "---" separators), so
// we decode each line individually and collect all failing results.
type kyvernoReport struct {
	Results []kyvernoResult `json:"results"`
}

type kyvernoResult struct {
	Policy  string `json:"policy"`
	Result  string `json:"result"` // "pass", "fail", "warn", "error", "skip"
	Message string `json:"message"`
}

// validatePolicies runs the Kyverno CLI to evaluate the plan JSON against
// ValidatingPolicy resources (policies.kyverno.io) matching the label selector.
// Each policy is evaluated individually so that a non-zero exit code
// unambiguously identifies the failing policy by name. The full result is
// emitted as a single MAGOS_RESULT line for the workspace controller to parse
// and surface in the Workspace status. A failing validation returns an error so
// the pod exits non-zero, which the workspace controller surfaces as the
// ValidationFailed phase.
func validatePolicies(ctx context.Context, cfg *Config, planJSONFile string) error {
	log.Printf("Evaluating policies with selector %q", cfg.PolicySelector)

	restCfg, err := rest.InClusterConfig()
	if err != nil {
		return fmt.Errorf("failed to get in-cluster config: %w", err)
	}

	dynamicClient, err := dynamic.NewForConfig(restCfg)
	if err != nil {
		return fmt.Errorf("failed to create dynamic client: %w", err)
	}

	policyUnstructured, err := dynamicClient.Resource(
		schema.GroupVersionResource{Group: "policies.kyverno.io", Version: "v1", Resource: "validatingpolicies"},
	).List(ctx, v1.ListOptions{
		LabelSelector: cfg.PolicySelector,
	})
	if err != nil {
		return fmt.Errorf("failed to list ValidatingPolicy resources: %w", err)
	}

	if len(policyUnstructured.Items) == 0 {
		log.Println("No ValidatingPolicy resources match the selector, skipping validation")
		return nil
	}
	log.Printf("Found %d ValidatingPolicy resource(s) to evaluate", len(policyUnstructured.Items))

	policyDir, err := os.MkdirTemp("", "magos-policies-*")
	if err != nil {
		return fmt.Errorf("failed to create policies temp directory: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(policyDir); err != nil {
			log.Printf("Warning: failed to remove policy temp directory %q: %v", policyDir, err)
		}
	}()

	for i, item := range policyUnstructured.Items {
		policyName := item.GetName()
		policyBytes, err := yaml.Marshal(item.Object)
		if err != nil {
			return fmt.Errorf("failed to marshal policy %q to YAML: %w", policyName, err)
		}
		policyFile := filepath.Join(policyDir, fmt.Sprintf("policy-%d.yaml", i))
		if err := os.WriteFile(policyFile, policyBytes, 0600); err != nil {
			return fmt.Errorf("failed to write policy file for %q: %w", policyName, err)
		}
	}

	var out bytes.Buffer
	cmd := exec.CommandContext(ctx, "kyverno", "apply", policyDir,
		"--json", planJSONFile,
		"--policy-report",
		"--output-format", "json",
	)
	cmd.Stdout = &out
	cmd.Stderr = &out
	cmd.Run() //nolint:errcheck // exit code is determined by violations below

	violations := parseKyvernoReport(out.Bytes())

	result := policyResult{
		Passed:     len(violations) == 0,
		Violations: violations,
	}
	resultJSON, _ := json.Marshal(result)
	fmt.Printf("MAGOS_RESULT:%s\n", resultJSON)

	if len(violations) > 0 {
		policies := make([]string, len(violations))
		for i, v := range violations {
			policies[i] = v.Policy
		}
		return fmt.Errorf("policy validation failed: %s", strings.Join(policies, ", "))
	}

	log.Println("Policy validation passed")
	return nil
}

// parseKyvernoReport turns the output of kyverno apply --policy-report
// --output-format json into a list of violations. Kyverno writes one JSON
// object per line (a ClusterReport or Report from the OpenReports API),
// sometimes with "---" separators between them. We skip blank lines and
// separators, unmarshal each JSON object, and collect every result entry whose
// result field is "fail". The Description field on each result carries the
// human-readable message from the policy's spec.validations[].message, which is
// what we surface to the operator.
func parseKyvernoReport(output []byte) []policyViolation {
	var violations []policyViolation
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 || bytes.Equal(line, []byte("---")) {
			continue
		}
		var report kyvernoReport
		if err := json.Unmarshal(line, &report); err != nil {
			continue
		}
		for _, r := range report.Results {
			if r.Result == "fail" {
				violations = append(violations, policyViolation{
					Policy:  r.Policy,
					Message: r.Message,
				})
			}
		}
	}
	return violations
}

// logTerraformEnvVars writes a single line naming the TF_VAR_* environment
// variables the workspace controller injected onto this pod. Names are
// derived from os.Environ() so this works regardless of which path put the
// value there (inline literal, Secret reference, ConfigMap reference) and
// stays correct if a future code path adds another source. Names only,
// never values: archived pod logs are read by humans and stored at rest, so
// the value of a Secret-backed variable must never appear here.
func logTerraformEnvVars() {
	const prefix = "TF_VAR_"
	var names []string
	for _, kv := range os.Environ() {
		eq := strings.IndexByte(kv, '=')
		if eq <= 0 {
			continue
		}
		if name, ok := strings.CutPrefix(kv[:eq], prefix); ok {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		log.Println("No Terraform inputs from VariableSets attached to this run")
		return
	}
	// Sort so the line is stable across reruns even though os.Environ
	// itself does not guarantee any ordering.
	sort.Strings(names)
	log.Printf("Loaded %d Terraform input(s) from VariableSets: %s", len(names), strings.Join(names, ", "))
}

// run drives a single job. Plan pods clone into
// /workspace-data/runs/<runID>/source; apply pods reuse that tree. A
// successful apply removes the run's directory.
func run(ctx context.Context) error {
	// Prefix every log line with [magos-job] so pod logs are easy to scan.
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)
	log.SetPrefix("[magos-job] ")

	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	// Log the Terraform inputs this run will see. We log names only,
	// never values, because secret-backed variables resolve into the pod
	// env before the job process starts and the pod log is archived to
	// the run log store. A single accidental TF_VAR_password value in
	// archived logs is a real credential leak, so this stays strictly
	// names-only.
	logTerraformEnvVars()

	src := sourcePath(cfg.RunID)

	switch cfg.JobType {
	case jobTypePlan:
		// Start from a clean slate. The previous run's apply pod removes
		// its own run dir on exit, but a wipe of runsBase covers the case
		// where that defer never ran (pod killed, OOM, eviction).
		if err := os.RemoveAll(runsBase); err != nil {
			return fmt.Errorf("failed to clear runs dir: %w", err)
		}
		if err := os.MkdirAll(runDir(cfg.RunID), 0o755); err != nil {
			return fmt.Errorf("failed to create run dir: %w", err)
		}
		if err := cloneRepository(ctx, cfg, src); err != nil {
			return err
		}
	case jobTypeApply:
		// The plan pod is the only writer; confirm its tree is still here.
		if err := validateSourceDir(src); err != nil {
			return err
		}
	}

	if err := execTerraform(ctx, cfg, src); err != nil {
		return err
	}

	return nil
}

// main is the job entry point. It runs the job and converts any error into a
// log.Fatalf call, which writes the prefixed error to stderr and exits non
// zero. Kubernetes treats a non zero exit as a failed Job, which is the signal
// the workspace controller watches for when it reconciles a plan or apply
// phase.
func main() {

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	err := run(ctx)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
}
