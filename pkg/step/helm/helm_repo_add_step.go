package helm

import (
	"fmt"
	"regexp" // Added for regexp.QuoteMeta
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// HelmRepoAddStep executes 'helm repo add' to add a chart repository.
type HelmRepoAddStep struct {
	meta        spec.StepMeta
	RepoName    string
	RepoURL     string
	Username    string // Optional
	Password    string // Optional
	Sudo        bool   // If helm command itself needs sudo (rare)
	ForceUpdate bool   // Corresponds to --force-update for `helm repo add`
}

// NewHelmRepoAddStep creates a new HelmRepoAddStep.
func NewHelmRepoAddStep(instanceName, repoName, repoURL, username, password string, sudo, forceUpdate bool) step.Step {
	name := instanceName
	if name == "" {
		name = fmt.Sprintf("HelmRepoAdd-%s", repoName)
	}
	return &HelmRepoAddStep{
		meta: spec.StepMeta{
			Name:        name,
			Description: fmt.Sprintf("Adds Helm repository '%s' from URL '%s'", repoName, repoURL),
		},
		RepoName:    repoName,
		RepoURL:     repoURL,
		Username:    username,
		Password:    password,
		Sudo:        sudo,
		ForceUpdate: forceUpdate,
	}
}

func (s *HelmRepoAddStep) Meta() *spec.StepMeta {
	return &s.meta
}

func (s *HelmRepoAddStep) Precheck(ctx step.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName())
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector: %w", err)
	}

	// Check if helm is installed
	if _, err := runnerSvc.LookPath(ctx.GoContext(), conn, "helm"); err != nil {
		logger.Info("Helm command not found, assuming repo does not exist and step needs to run (helm install will likely fail later).")
		return false, nil // Let Run proceed, subsequent helm steps will fail if helm not present
	}

	// helm repo list -o json (or yaml) would be more robust to parse.
	// For simplicity, using grep.
	// This checks if a line exists with the exact name and URL.
	// Format: NAME                                    URL
	//         prometheus-community                    https://prometheus-community.github.io/helm-charts
	checkCmd := fmt.Sprintf("helm repo list | grep -E '^%s[[:space:]]+%s[[:space:]]*$'", regexp.QuoteMeta(s.RepoName), regexp.QuoteMeta(s.RepoURL))

	// RunWithOptions with Check:true so non-zero exit (grep not found) doesn't cause Go error
	_, _, errCmd := runnerSvc.RunWithOptions(ctx.GoContext(), conn, checkCmd, &connector.ExecOptions{Sudo: s.Sudo, Check: true})

	if errCmd == nil { // Exit code 0 means grep found the match
		logger.Info("Helm repository already exists with the same name and URL.", "repo_name", s.RepoName, "repo_url", s.RepoURL)
		if s.ForceUpdate {
			logger.Info("--force-update is set, repo add will run to update.")
			return false, nil // Run to force update
		}
		return true, nil // Repo exists and no force update
	}

	// If errCmd is not nil, it means grep didn't find it (exit 1) or another error occurred.
	// For robustness, check if it's specifically a "not found" type of error from grep.
	if cmdErr, ok := errCmd.(*connector.CommandError); ok && cmdErr.ExitCode == 1 {
		logger.Info("Helm repository not found or URL mismatch. Repo add will proceed.", "repo_name", s.RepoName, "repo_url", s.RepoURL)
		return false, nil
	}

	// Some other error occurred during the check command
	logger.Warn("Could not reliably determine if helm repository exists. Repo add will proceed.", "error", errCmd)
	return false, nil
}

func (s *HelmRepoAddStep) Run(ctx step.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName())
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector: %w", err)
	}

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "helm", "repo", "add", s.RepoName, s.RepoURL)

	if s.Username != "" {
		cmdArgs = append(cmdArgs, "--username", s.Username)
	}
	if s.Password != "" {
		cmdArgs = append(cmdArgs, "--password", s.Password, "--pass-credentials")
	}
	if s.ForceUpdate {
		cmdArgs = append(cmdArgs, "--force-update")
	}

	cmd := strings.Join(cmdArgs, " ")
	logger.Info("Running helm repo add command", "command", cmd)
	stdout, stderr, err := runnerSvc.Run(ctx.GoContext(), conn, cmd, s.Sudo) // Sudo here applies to helm command itself
	if err != nil {
		logger.Error("helm repo add failed", "error", err, "stdout", string(stdout), "stderr", string(stderr))
		return fmt.Errorf("helm repo add for %s (%s) failed: %w. Stdout: %s, Stderr: %s", s.RepoName, s.RepoURL, err, string(stdout), string(stderr))
	}

	logger.Info("Helm repo add completed successfully.", "repo_name", s.RepoName, "stdout", string(stdout))
	return nil
}

func (s *HelmRepoAddStep) Rollback(ctx step.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName())
	// Rollback could be 'helm repo remove s.RepoName'
	// This is only safe if we are sure this step was the one that added it AND no other charts depend on it.
	// For now, a best-effort removal.

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		logger.Warn("Failed to get connector for host during rollback, cannot remove helm repo.", "error", err)
		return nil
	}

	cmd := fmt.Sprintf("helm repo remove %s", s.RepoName)
	logger.Info("Attempting helm repo remove for rollback", "command", cmd)
	_, stderr, err := runnerSvc.Run(ctx.GoContext(), conn, cmd, s.Sudo)
	if err != nil {
		logger.Warn("helm repo remove command failed during rollback (best effort).", "error", err, "stderr", string(stderr))
	} else {
		logger.Info("helm repo remove executed successfully for rollback.")
	}
	return nil
}

var _ step.Step = (*HelmRepoAddStep)(nil)
```
