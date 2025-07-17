package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/mensylisir/kubexm/pkg/connector"
)

const (
	DefaultHelmTimeout = 5 * time.Minute
)

// HelmInstall installs a Helm chart.
func (r *defaultRunner) HelmInstall(ctx context.Context, conn connector.Connector, releaseName, chartPath string, opts HelmInstallOptions) error {
	if conn == nil {
		return errors.New("connector cannot be nil")
	}
	if releaseName == "" || chartPath == "" {
		return errors.New("releaseName and chartPath are required")
	}

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "helm", "install", releaseName, chartPath)
	if opts.Namespace != "" {
		cmdArgs = append(cmdArgs, "--namespace", opts.Namespace)
	}
	if opts.KubeconfigPath != "" {
		cmdArgs = append(cmdArgs, "--kubeconfig", opts.KubeconfigPath)
	}
	if opts.CreateNamespace {
		cmdArgs = append(cmdArgs, "--create-namespace")
	}
	if opts.Version != "" {
		cmdArgs = append(cmdArgs, "--version", opts.Version)
	}
	for _, vf := range opts.ValuesFiles {
		cmdArgs = append(cmdArgs, "--values", vf)
	}
	for _, sv := range opts.SetValues {
		cmdArgs = append(cmdArgs, "--set", sv)
	}
	if opts.Wait {
		cmdArgs = append(cmdArgs, "--wait")
	}
	if opts.Timeout > 0 {
		cmdArgs = append(cmdArgs, "--timeout", opts.Timeout.String())
	}
	if opts.Atomic {
		cmdArgs = append(cmdArgs, "--atomic")
	}
	if opts.DryRun {
		cmdArgs = append(cmdArgs, "--dry-run")
	}
	if opts.Devel {
		cmdArgs = append(cmdArgs, "--devel")
	}
	if opts.Description != "" {
		cmdArgs = append(cmdArgs, "--description", opts.Description)
	}

	cmd := strings.Join(cmdArgs, " ")
	execTimeout := DefaultHelmTimeout
	if opts.Timeout > 0 {
		execTimeout = opts.Timeout + (1 * time.Minute)
	}
	execOpts := &connector.ExecOptions{Sudo: opts.Sudo, Timeout: execTimeout, Retries: opts.Retries, RetryDelay: opts.RetryDelay}
	stdout, stderr, err := conn.Exec(ctx, cmd, execOpts)
	if err != nil {
		return errors.Wrapf(err, "helm install for '%s' ('%s') failed. Stdout: %s, Stderr: %s", releaseName, chartPath, string(stdout), string(stderr))
	}
	return nil
}

// HelmUninstall uninstalls a Helm release.
func (r *defaultRunner) HelmUninstall(ctx context.Context, conn connector.Connector, releaseName string, opts HelmUninstallOptions) error {
	if conn == nil {
		return errors.New("connector cannot be nil")
	}
	if releaseName == "" {
		return errors.New("releaseName is required")
	}

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "helm", "uninstall", releaseName)
	if opts.Namespace != "" {
		cmdArgs = append(cmdArgs, "--namespace", opts.Namespace)
	}
	if opts.KubeconfigPath != "" {
		cmdArgs = append(cmdArgs, "--kubeconfig", opts.KubeconfigPath)
	}
	if opts.KeepHistory {
		cmdArgs = append(cmdArgs, "--keep-history")
	}
	if opts.Timeout > 0 {
		cmdArgs = append(cmdArgs, "--timeout", opts.Timeout.String())
	}
	if opts.DryRun {
		cmdArgs = append(cmdArgs, "--dry-run")
	}

	cmd := strings.Join(cmdArgs, " ")
	execTimeout := DefaultHelmTimeout
	if opts.Timeout > 0 {
		execTimeout = opts.Timeout + (1 * time.Minute)
	}
	_, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: opts.Sudo, Timeout: execTimeout})
	if err != nil {
		if strings.Contains(string(stderr), "release: not found") {
			return nil
		}
		return errors.Wrapf(err, "helm uninstall for '%s' failed. Stderr: %s", releaseName, string(stderr))
	}
	return nil
}

// HelmList lists Helm releases.
func (r *defaultRunner) HelmList(ctx context.Context, conn connector.Connector, opts HelmListOptions) ([]HelmReleaseInfo, error) {
	if conn == nil {
		return nil, errors.New("connector cannot be nil")
	}
	var cmdArgs []string
	cmdArgs = append(cmdArgs, "helm", "list", "-o", "json")
	if opts.Namespace != "" {
		cmdArgs = append(cmdArgs, "--namespace", opts.Namespace)
	}
	if opts.KubeconfigPath != "" {
		cmdArgs = append(cmdArgs, "--kubeconfig", opts.KubeconfigPath)
	}
	if opts.AllNamespaces {
		cmdArgs = append(cmdArgs, "--all-namespaces")
	}
	if opts.Filter != "" {
		cmdArgs = append(cmdArgs, "--filter", opts.Filter)
	}
	if opts.Selector != "" {
		cmdArgs = append(cmdArgs, "--selector", opts.Selector)
	}
	if opts.Max > 0 {
		cmdArgs = append(cmdArgs, "--max", fmt.Sprintf("%d", opts.Max))
	}
	if opts.Offset > 0 {
		cmdArgs = append(cmdArgs, "--offset", fmt.Sprintf("%d", opts.Offset))
	}
	if opts.ByDate {
		cmdArgs = append(cmdArgs, "--date")
	}
	if opts.SortReverse {
		cmdArgs = append(cmdArgs, "--reverse")
	}
	if opts.Deployed {
		cmdArgs = append(cmdArgs, "--deployed")
	}
	if opts.Failed {
		cmdArgs = append(cmdArgs, "--failed")
	}
	if opts.Pending {
		cmdArgs = append(cmdArgs, "--pending")
	}
	if opts.Uninstalled {
		cmdArgs = append(cmdArgs, "--uninstalled")
	}
	if opts.Uninstalling {
		cmdArgs = append(cmdArgs, "--uninstalling")
	}

	stdout, stderr, err := conn.Exec(ctx, strings.Join(cmdArgs, " "), &connector.ExecOptions{Sudo: opts.Sudo, Timeout: DefaultHelmTimeout})
	if err != nil {
		return nil, errors.Wrapf(err, "helm list failed. Stderr: %s", string(stderr))
	}
	var releases []HelmReleaseInfo
	if err := json.Unmarshal(stdout, &releases); err != nil {
		if strings.TrimSpace(string(stdout)) == "" || strings.TrimSpace(string(stdout)) == "null" {
			return []HelmReleaseInfo{}, nil
		}
		return nil, errors.Wrapf(err, "failed to parse helm list JSON. Output: %s", string(stdout))
	}
	return releases, nil
}

// HelmStatus gets the status of a Helm release.
func (r *defaultRunner) HelmStatus(ctx context.Context, conn connector.Connector, releaseName string, opts HelmStatusOptions) (*HelmReleaseInfo, error) {
	if conn == nil {
		return nil, errors.New("connector cannot be nil")
	}
	if releaseName == "" {
		return nil, errors.New("releaseName is required")
	}
	var cmdArgs []string
	cmdArgs = append(cmdArgs, "helm", "status", releaseName, "-o", "json")
	if opts.Namespace != "" {
		cmdArgs = append(cmdArgs, "--namespace", opts.Namespace)
	}
	if opts.KubeconfigPath != "" {
		cmdArgs = append(cmdArgs, "--kubeconfig", opts.KubeconfigPath)
	}
	if opts.Revision > 0 {
		cmdArgs = append(cmdArgs, "--revision", fmt.Sprintf("%d", opts.Revision))
	}

	stdout, stderr, err := conn.Exec(ctx, strings.Join(cmdArgs, " "), &connector.ExecOptions{Sudo: opts.Sudo, Timeout: DefaultHelmTimeout})
	if err != nil {
		if strings.Contains(string(stderr), "release: not found") {
			return nil, nil
		}
		return nil, errors.Wrapf(err, "helm status for '%s' failed. Stderr: %s", releaseName, string(stderr))
	}
	var status HelmReleaseInfo
	if err := json.Unmarshal(stdout, &status); err != nil {
		return nil, errors.Wrapf(err, "failed to parse helm status JSON for '%s'. Output: %s", releaseName, string(stdout))
	}
	return &status, nil
}

// HelmRepoAdd adds a chart repository.
func (r *defaultRunner) HelmRepoAdd(ctx context.Context, conn connector.Connector, name, url string, opts HelmRepoAddOptions) error {
	if conn == nil {
		return errors.New("connector cannot be nil")
	}
	if name == "" || url == "" {
		return errors.New("name and url are required")
	}
	var cmdArgs []string
	cmdArgs = append(cmdArgs, "helm", "repo", "add", name, url)
	if opts.Username != "" {
		cmdArgs = append(cmdArgs, "--username", opts.Username)
	}
	if opts.Password != "" {
		cmdArgs = append(cmdArgs, "--password", opts.Password)
	}
	if opts.CAFile != "" {
		cmdArgs = append(cmdArgs, "--ca-file", opts.CAFile)
	}
	if opts.CertFile != "" {
		cmdArgs = append(cmdArgs, "--cert-file", opts.CertFile)
	}
	if opts.KeyFile != "" {
		cmdArgs = append(cmdArgs, "--key-file", opts.KeyFile)
	}
	if opts.Insecure {
		cmdArgs = append(cmdArgs, "--insecure-skip-tls-verify")
	}
	if opts.ForceUpdate {
		cmdArgs = append(cmdArgs, "--force-update")
	}
	if opts.PassCredentials {
		cmdArgs = append(cmdArgs, "--pass-credentials")
	}

	_, stderr, err := conn.Exec(ctx, strings.Join(cmdArgs, " "), &connector.ExecOptions{Sudo: opts.Sudo, Timeout: DefaultHelmTimeout})
	if err != nil {
		if strings.Contains(string(stderr), "already exists") && !opts.ForceUpdate {
			return errors.Wrapf(err, "helm repo add '%s' failed: already exists and --force-update not used. Stderr: %s", name, string(stderr))
		}
		return errors.Wrapf(err, "helm repo add '%s' failed. Stderr: %s", name, string(stderr))
	}
	return nil
}

func (r *defaultRunner) HelmRepoUpdate(ctx context.Context, conn connector.Connector, repoNames []string) error {
	if conn == nil {
		return errors.New("connector cannot be nil")
	}
	var cmdArgs []string
	cmdArgs = append(cmdArgs, "helm", "repo", "update")
	for _, name := range repoNames {
		cmdArgs = append(cmdArgs, name)
	}
	_, stderr, err := conn.Exec(ctx, strings.Join(cmdArgs, " "), &connector.ExecOptions{Sudo: false, Timeout: 5 * time.Minute})
	if err != nil {
		return errors.Wrapf(err, "helm repo update failed. Stderr: %s", string(stderr))
	}
	return nil
}

// HelmVersion gets the Helm client version information.
func (r *defaultRunner) HelmVersion(ctx context.Context, conn connector.Connector) (*HelmVersionInfo, error) {
	if conn == nil {
		return nil, errors.New("connector cannot be nil")
	}
	stdout, stderr, err := conn.Exec(ctx, "helm version -o json", &connector.ExecOptions{Sudo: false, Timeout: 30 * time.Second})
	if err != nil {
		return nil, errors.Wrapf(err, "helm version failed. Stderr: %s", string(stderr))
	}
	var versionInfo HelmVersionInfo
	if err := json.Unmarshal(stdout, &versionInfo); err != nil {
		return nil, errors.Wrapf(err, "failed to parse helm version JSON. Output: %s", string(stdout))
	}
	return &versionInfo, nil
}

// HelmSearchRepo searches repositories for a keyword.
func (r *defaultRunner) HelmSearchRepo(ctx context.Context, conn connector.Connector, keyword string, opts HelmSearchOptions) ([]HelmChartInfo, error) {
	if conn == nil {
		return nil, errors.New("connector cannot be nil")
	}
	if keyword == "" {
		return nil, errors.New("keyword is required")
	}
	var cmdArgs []string
	cmdArgs = append(cmdArgs, "helm", "search", "repo", keyword, "-o", "json")
	if opts.Regexp {
		cmdArgs = append(cmdArgs, "--regexp")
	}
	if opts.Devel {
		cmdArgs = append(cmdArgs, "--devel")
	}
	if opts.Version != "" {
		cmdArgs = append(cmdArgs, "--version", opts.Version)
	}
	if opts.Versions {
		cmdArgs = append(cmdArgs, "--versions")
	}

	stdout, stderr, err := conn.Exec(ctx, strings.Join(cmdArgs, " "), &connector.ExecOptions{Sudo: opts.Sudo, Timeout: DefaultHelmTimeout})
	if err != nil {
		return nil, errors.Wrapf(err, "helm search repo for '%s' failed. Stderr: %s", keyword, string(stderr))
	}
	var charts []HelmChartInfo
	if err := json.Unmarshal(stdout, &charts); err != nil {
		if strings.TrimSpace(string(stdout)) == "" || strings.TrimSpace(string(stdout)) == "null" || strings.TrimSpace(string(stdout)) == "[]" {
			return []HelmChartInfo{}, nil
		}
		return nil, errors.Wrapf(err, "failed to parse helm search repo JSON. Output: %s", string(stdout))
	}
	return charts, nil
}

// HelmPull downloads a chart from a repository.
func (r *defaultRunner) HelmPull(ctx context.Context, conn connector.Connector, chartPath string, opts HelmPullOptions) (string, error) {
	if conn == nil {
		return "", errors.New("connector cannot be nil")
	}
	if chartPath == "" {
		return "", errors.New("chartPath is required")
	}
	var cmdArgs []string
	cmdArgs = append(cmdArgs, "helm", "pull", chartPath)
	if opts.Destination != "" {
		cmdArgs = append(cmdArgs, "--destination", opts.Destination)
	}
	if opts.Prov {
		cmdArgs = append(cmdArgs, "--prov")
	}
	if opts.Untar {
		cmdArgs = append(cmdArgs, "--untar")
		if opts.UntarDir != "" {
			cmdArgs = append(cmdArgs, "--untardir", opts.UntarDir)
		}
	}
	if opts.Verify {
		cmdArgs = append(cmdArgs, "--verify")
		if opts.Keyring != "" {
			cmdArgs = append(cmdArgs, "--keyring", opts.Keyring)
		}
	}
	if opts.Version != "" {
		cmdArgs = append(cmdArgs, "--version", opts.Version)
	}
	if opts.CAFile != "" {
		cmdArgs = append(cmdArgs, "--ca-file", opts.CAFile)
	}
	if opts.CertFile != "" {
		cmdArgs = append(cmdArgs, "--cert-file", opts.CertFile)
	}
	if opts.KeyFile != "" {
		cmdArgs = append(cmdArgs, "--key-file", opts.KeyFile)
	}
	if opts.Insecure {
		cmdArgs = append(cmdArgs, "--insecure-skip-tls-verify")
	}
	if opts.Devel {
		cmdArgs = append(cmdArgs, "--devel")
	}
	if opts.PassCredentials {
		cmdArgs = append(cmdArgs, "--pass-credentials")
	}
	if opts.Username != "" {
		cmdArgs = append(cmdArgs, "--username", opts.Username)
	}
	if opts.Password != "" {
		cmdArgs = append(cmdArgs, "--password", opts.Password)
	}

	stdout, stderr, err := conn.Exec(ctx, strings.Join(cmdArgs, " "), &connector.ExecOptions{Sudo: opts.Sudo, Timeout: 5 * time.Minute})
	if err != nil {
		return "", errors.Wrapf(err, "helm pull for '%s' failed. Stderr: %s", chartPath, string(stderr))
	}
	return strings.TrimSpace(string(stdout)), nil
}

// HelmPackage packages a chart directory into a chart archive.
func (r *defaultRunner) HelmPackage(ctx context.Context, conn connector.Connector, chartPath string, opts HelmPackageOptions) (string, error) {
	if conn == nil {
		return "", errors.New("connector cannot be nil")
	}
	if chartPath == "" {
		return "", errors.New("chartPath is required")
	}
	var cmdArgs []string
	cmdArgs = append(cmdArgs, "helm", "package", chartPath)
	if opts.Destination != "" {
		cmdArgs = append(cmdArgs, "--destination", opts.Destination)
	}
	if opts.Sign {
		cmdArgs = append(cmdArgs, "--sign")
		if opts.Key != "" {
			cmdArgs = append(cmdArgs, "--key", opts.Key)
		}
		if opts.Keyring != "" {
			cmdArgs = append(cmdArgs, "--keyring", opts.Keyring)
		}
		if opts.PassphraseFile != "" {
			cmdArgs = append(cmdArgs, "--passphrase-file", opts.PassphraseFile)
		}
	}
	if opts.Version != "" {
		cmdArgs = append(cmdArgs, "--version", opts.Version)
	}
	if opts.AppVersion != "" {
		cmdArgs = append(cmdArgs, "--app-version", opts.AppVersion)
	}
	if opts.DependencyUpdate {
		cmdArgs = append(cmdArgs, "--dependency-update")
	}

	stdout, stderr, err := conn.Exec(ctx, strings.Join(cmdArgs, " "), &connector.ExecOptions{Sudo: opts.Sudo, Timeout: DefaultHelmTimeout})
	if err != nil {
		return "", errors.Wrapf(err, "helm package for '%s' failed. Stderr: %s", chartPath, string(stderr))
	}
	outputStr := strings.TrimSpace(string(stdout))
	prefix := "Successfully packaged chart and saved it to: "
	if strings.HasPrefix(outputStr, prefix) {
		return strings.TrimSpace(strings.TrimPrefix(outputStr, prefix)), nil
	}
	return outputStr, nil // Return raw output if parsing fails
}

// HelmUpgrade upgrades a release.
func (r *defaultRunner) HelmUpgrade(ctx context.Context, conn connector.Connector, releaseName, chartPath string, opts HelmUpgradeOptions) error {
	if conn == nil {
		return errors.New("connector cannot be nil")
	}
	if releaseName == "" || chartPath == "" {
		return errors.New("releaseName and chartPath are required")
	}
	var cmdArgs []string
	cmdArgs = append(cmdArgs, "helm", "upgrade", releaseName, chartPath)
	if opts.Install {
		cmdArgs = append(cmdArgs, "--install")
	}
	if opts.Namespace != "" {
		cmdArgs = append(cmdArgs, "--namespace", opts.Namespace)
	}
	if opts.KubeconfigPath != "" {
		cmdArgs = append(cmdArgs, "--kubeconfig", opts.KubeconfigPath)
	}
	if opts.Version != "" {
		cmdArgs = append(cmdArgs, "--version", opts.Version)
	}
	for _, vf := range opts.ValuesFiles {
		cmdArgs = append(cmdArgs, "--values", vf)
	}
	for _, sv := range opts.SetValues {
		cmdArgs = append(cmdArgs, "--set", sv)
	}
	if opts.Wait {
		cmdArgs = append(cmdArgs, "--wait")
	}
	if opts.Timeout > 0 {
		cmdArgs = append(cmdArgs, "--timeout", opts.Timeout.String())
	}
	if opts.Atomic {
		cmdArgs = append(cmdArgs, "--atomic")
	}
	if opts.Force {
		cmdArgs = append(cmdArgs, "--force")
	}
	if opts.ResetValues {
		cmdArgs = append(cmdArgs, "--reset-values")
	}
	if opts.ReuseValues {
		cmdArgs = append(cmdArgs, "--reuse-values")
	}
	if opts.CleanupOnFail {
		cmdArgs = append(cmdArgs, "--cleanup-on-fail")
	}
	if opts.MaxHistory > 0 {
		cmdArgs = append(cmdArgs, "--history-max", fmt.Sprintf("%d", opts.MaxHistory))
	}

	cmd := strings.Join(cmdArgs, " ")
	execTimeout := DefaultHelmTimeout
	if opts.Timeout > 0 {
		execTimeout = opts.Timeout + (1 * time.Minute)
	}
	_, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: opts.Sudo, Timeout: execTimeout})
	if err != nil {
		return errors.Wrapf(err, "helm upgrade for '%s' failed. Stderr: %s", releaseName, string(stderr))
	}
	return nil
}

// HelmRollback rolls back a release to a previous version.
func (r *defaultRunner) HelmRollback(ctx context.Context, conn connector.Connector, releaseName string, revision int, opts HelmRollbackOptions) error {
	if conn == nil {
		return errors.New("connector cannot be nil")
	}
	if releaseName == "" {
		return errors.New("releaseName is required")
	}
	// revision 0 means rollback to previous version
	var cmdArgs []string
	cmdArgs = append(cmdArgs, "helm", "rollback", releaseName, fmt.Sprintf("%d", revision))
	if opts.Namespace != "" {
		cmdArgs = append(cmdArgs, "--namespace", opts.Namespace)
	}
	if opts.KubeconfigPath != "" {
		cmdArgs = append(cmdArgs, "--kubeconfig", opts.KubeconfigPath)
	}
	if opts.Timeout > 0 {
		cmdArgs = append(cmdArgs, "--timeout", opts.Timeout.String())
	}
	if opts.Wait {
		cmdArgs = append(cmdArgs, "--wait")
	}
	if opts.CleanupOnFail {
		cmdArgs = append(cmdArgs, "--cleanup-on-fail")
	}
	if opts.DryRun {
		cmdArgs = append(cmdArgs, "--dry-run")
	}
	if opts.Force {
		cmdArgs = append(cmdArgs, "--force")
	}
	if opts.NoHooks {
		cmdArgs = append(cmdArgs, "--no-hooks")
	}
	if opts.RecreatePods {
		cmdArgs = append(cmdArgs, "--recreate-pods")
	}

	_, stderr, err := conn.Exec(ctx, strings.Join(cmdArgs, " "), &connector.ExecOptions{Sudo: opts.Sudo, Timeout: DefaultHelmTimeout})
	if err != nil {
		return errors.Wrapf(err, "helm rollback for '%s' to rev %d failed. Stderr: %s", releaseName, revision, string(stderr))
	}
	return nil
}

// HelmHistory displays the revision history of a release.
func (r *defaultRunner) HelmHistory(ctx context.Context, conn connector.Connector, releaseName string, opts HelmHistoryOptions) ([]HelmReleaseRevisionInfo, error) {
	if conn == nil {
		return nil, errors.New("connector cannot be nil")
	}
	if releaseName == "" {
		return nil, errors.New("releaseName is required")
	}
	var cmdArgs []string
	cmdArgs = append(cmdArgs, "helm", "history", releaseName, "-o", "json")
	if opts.Namespace != "" {
		cmdArgs = append(cmdArgs, "--namespace", opts.Namespace)
	}
	if opts.KubeconfigPath != "" {
		cmdArgs = append(cmdArgs, "--kubeconfig", opts.KubeconfigPath)
	}
	if opts.Max > 0 {
		cmdArgs = append(cmdArgs, "--max", fmt.Sprintf("%d", opts.Max))
	}

	stdout, stderr, err := conn.Exec(ctx, strings.Join(cmdArgs, " "), &connector.ExecOptions{Sudo: opts.Sudo, Timeout: DefaultHelmTimeout})
	if err != nil {
		return nil, errors.Wrapf(err, "helm history for '%s' failed. Stderr: %s", releaseName, string(stderr))
	}
	var history []HelmReleaseRevisionInfo
	if err := json.Unmarshal(stdout, &history); err != nil {
		if strings.TrimSpace(string(stdout)) == "" || strings.TrimSpace(string(stdout)) == "null" {
			return []HelmReleaseRevisionInfo{}, nil
		}
		return nil, errors.Wrapf(err, "failed to parse helm history JSON for '%s'. Output: %s", releaseName, string(stdout))
	}
	return history, nil
}

// HelmGetValues gets the values for a release.
func (r *defaultRunner) HelmGetValues(ctx context.Context, conn connector.Connector, releaseName string, opts HelmGetOptions) (string, error) {
	if conn == nil {
		return "", errors.New("connector cannot be nil")
	}
	if releaseName == "" {
		return "", errors.New("releaseName is required")
	}
	var cmdArgs []string
	cmdArgs = append(cmdArgs, "helm", "get", "values", releaseName)
	if opts.Namespace != "" {
		cmdArgs = append(cmdArgs, "--namespace", opts.Namespace)
	}
	if opts.KubeconfigPath != "" {
		cmdArgs = append(cmdArgs, "--kubeconfig", opts.KubeconfigPath)
	}
	if opts.Revision > 0 {
		cmdArgs = append(cmdArgs, "--revision", fmt.Sprintf("%d", opts.Revision))
	}
	if opts.AllValues {
		cmdArgs = append(cmdArgs, "--all")
	} // -a or --all
	cmdArgs = append(cmdArgs, "-o", "yaml") // Typically users want YAML for values

	stdout, stderr, err := conn.Exec(ctx, strings.Join(cmdArgs, " "), &connector.ExecOptions{Sudo: opts.Sudo, Timeout: DefaultHelmTimeout})
	if err != nil {
		return "", errors.Wrapf(err, "helm get values for '%s' failed. Stderr: %s", releaseName, string(stderr))
	}
	return string(stdout), nil
}

// HelmGetManifest gets the manifest for a release.
func (r *defaultRunner) HelmGetManifest(ctx context.Context, conn connector.Connector, releaseName string, opts HelmGetOptions) (string, error) {
	if conn == nil {
		return "", errors.New("connector cannot be nil")
	}
	if releaseName == "" {
		return "", errors.New("releaseName is required")
	}
	var cmdArgs []string
	cmdArgs = append(cmdArgs, "helm", "get", "manifest", releaseName)
	if opts.Namespace != "" {
		cmdArgs = append(cmdArgs, "--namespace", opts.Namespace)
	}
	if opts.KubeconfigPath != "" {
		cmdArgs = append(cmdArgs, "--kubeconfig", opts.KubeconfigPath)
	}
	if opts.Revision > 0 {
		cmdArgs = append(cmdArgs, "--revision", fmt.Sprintf("%d", opts.Revision))
	}

	stdout, stderr, err := conn.Exec(ctx, strings.Join(cmdArgs, " "), &connector.ExecOptions{Sudo: opts.Sudo, Timeout: DefaultHelmTimeout})
	if err != nil {
		return "", errors.Wrapf(err, "helm get manifest for '%s' failed. Stderr: %s", releaseName, string(stderr))
	}
	return string(stdout), nil
}

// HelmGetHooks gets the hooks for a release.
func (r *defaultRunner) HelmGetHooks(ctx context.Context, conn connector.Connector, releaseName string, opts HelmGetOptions) (string, error) {
	if conn == nil {
		return "", errors.New("connector cannot be nil")
	}
	if releaseName == "" {
		return "", errors.New("releaseName is required")
	}
	var cmdArgs []string
	cmdArgs = append(cmdArgs, "helm", "get", "hooks", releaseName)
	if opts.Namespace != "" {
		cmdArgs = append(cmdArgs, "--namespace", opts.Namespace)
	}
	if opts.KubeconfigPath != "" {
		cmdArgs = append(cmdArgs, "--kubeconfig", opts.KubeconfigPath)
	}
	if opts.Revision > 0 {
		cmdArgs = append(cmdArgs, "--revision", fmt.Sprintf("%d", opts.Revision))
	}

	stdout, stderr, err := conn.Exec(ctx, strings.Join(cmdArgs, " "), &connector.ExecOptions{Sudo: opts.Sudo, Timeout: DefaultHelmTimeout})
	if err != nil {
		return "", errors.Wrapf(err, "helm get hooks for '%s' failed. Stderr: %s", releaseName, string(stderr))
	}
	return string(stdout), nil // Hooks output is also YAML usually
}

// HelmTemplate renders chart templates locally.
func (r *defaultRunner) HelmTemplate(ctx context.Context, conn connector.Connector, releaseName, chartPath string, opts HelmTemplateOptions) (string, error) {
	if conn == nil {
		return "", errors.New("connector cannot be nil")
	}
	if chartPath == "" {
		return "", errors.New("chartPath is required")
	}
	var cmdArgs []string
	cmdArgs = append(cmdArgs, "helm", "template")
	if releaseName != "" {
		cmdArgs = append(cmdArgs, releaseName)
	}
	cmdArgs = append(cmdArgs, chartPath)

	if opts.Namespace != "" {
		cmdArgs = append(cmdArgs, "--namespace", opts.Namespace)
	}
	if opts.KubeconfigPath != "" {
		cmdArgs = append(cmdArgs, "--kubeconfig", opts.KubeconfigPath)
	}
	for _, vf := range opts.ValuesFiles {
		cmdArgs = append(cmdArgs, "--values", vf)
	}
	for _, sv := range opts.SetValues {
		cmdArgs = append(cmdArgs, "--set", sv)
	}
	if opts.CreateNamespace {
		cmdArgs = append(cmdArgs, "--create-namespace")
	} // May not be applicable to template, but included for consistency
	if opts.SkipCrds {
		cmdArgs = append(cmdArgs, "--skip-crds")
	}
	if opts.Validate {
		cmdArgs = append(cmdArgs, "--validate")
	}
	if opts.IncludeCrds {
		cmdArgs = append(cmdArgs, "--include-crds")
	}
	if opts.IsUpgrade {
		cmdArgs = append(cmdArgs, "--is-upgrade")
	}
	for _, so := range opts.ShowOnly {
		cmdArgs = append(cmdArgs, "--show-only", so)
	}

	stdout, stderr, err := conn.Exec(ctx, strings.Join(cmdArgs, " "), &connector.ExecOptions{Sudo: opts.Sudo, Timeout: DefaultHelmTimeout})
	if err != nil {
		return "", errors.Wrapf(err, "helm template for '%s' failed. Stderr: %s", chartPath, string(stderr))
	}
	return string(stdout), nil
}

// HelmDependencyUpdate updates chart dependencies.
func (r *defaultRunner) HelmDependencyUpdate(ctx context.Context, conn connector.Connector, chartPath string, opts HelmDependencyOptions) error {
	if conn == nil {
		return errors.New("connector cannot be nil")
	}
	if chartPath == "" {
		return errors.New("chartPath is required")
	}
	var cmdArgs []string
	cmdArgs = append(cmdArgs, "helm", "dependency", "update", chartPath)
	if opts.Keyring != "" {
		cmdArgs = append(cmdArgs, "--keyring", opts.Keyring)
	}
	if opts.SkipRefresh {
		cmdArgs = append(cmdArgs, "--skip-refresh")
	}
	if opts.Verify {
		cmdArgs = append(cmdArgs, "--verify")
	}

	_, stderr, err := conn.Exec(ctx, strings.Join(cmdArgs, " "), &connector.ExecOptions{Sudo: opts.Sudo, Timeout: DefaultHelmTimeout})
	if err != nil {
		return errors.Wrapf(err, "helm dependency update for '%s' failed. Stderr: %s", chartPath, string(stderr))
	}
	return nil
}

// HelmLint examines a chart for possible issues.
func (r *defaultRunner) HelmLint(ctx context.Context, conn connector.Connector, chartPath string, opts HelmLintOptions) (string, error) {
	if conn == nil {
		return "", errors.New("connector cannot be nil")
	}
	if chartPath == "" {
		return "", errors.New("chartPath is required")
	}
	var cmdArgs []string
	cmdArgs = append(cmdArgs, "helm", "lint", chartPath)
	if opts.Strict {
		cmdArgs = append(cmdArgs, "--strict")
	}
	for _, vf := range opts.ValuesFiles {
		cmdArgs = append(cmdArgs, "--values", vf)
	}
	for _, sv := range opts.SetValues {
		cmdArgs = append(cmdArgs, "--set", sv)
	}
	if opts.Quiet {
		cmdArgs = append(cmdArgs, "--quiet")
	}
	if opts.WithSubcharts {
		cmdArgs = append(cmdArgs, "--with-subcharts")
	}
	if opts.Namespace != "" {
		cmdArgs = append(cmdArgs, "--namespace", opts.Namespace)
	}
	// Helm lint -o json is not standard, output is usually text.
	// We will return the raw output.

	stdout, stderr, err := conn.Exec(ctx, strings.Join(cmdArgs, " "), &connector.ExecOptions{Sudo: opts.Sudo, Timeout: DefaultHelmTimeout})
	// Helm lint returns non-zero exit code if linting errors are found.
	// The output (stdout/stderr) contains the linting messages.
	// We should return the output even if there's an error, as it's informative.
	output := string(stdout) + string(stderr)
	if err != nil {
		// Don't wrap the error if it's just linting issues (non-zero exit from helm lint).
		// The output itself is the "result".
		// However, if it's an execution error of the helm command itself, then wrap.
		var cmdErr *connector.CommandError
		if errors.As(err, &cmdErr) {
			// It's a linting error, return output and the specific command error
			return output, err
		}
		return output, errors.Wrapf(err, "helm lint for '%s' failed execution. Output: %s", chartPath, output)
	}
	return output, nil
}
