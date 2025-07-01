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
// Corresponds to `helm install [NAME] [CHART] [flags]`.
func (r *defaultRunner) HelmInstall(ctx context.Context, conn connector.Connector, releaseName, chartPath string, opts HelmInstallOptions) error {
	if conn == nil {
		return errors.New("connector cannot be nil")
	}
	if releaseName == "" || chartPath == "" {
		return errors.New("releaseName and chartPath are required for HelmInstall")
	}

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "helm", "install", shellEscape(releaseName), shellEscape(chartPath))

	if opts.Namespace != "" {
		cmdArgs = append(cmdArgs, "--namespace", shellEscape(opts.Namespace))
	}
	if opts.KubeconfigPath != "" {
		cmdArgs = append(cmdArgs, "--kubeconfig", shellEscape(opts.KubeconfigPath))
	}
	if opts.CreateNamespace {
		cmdArgs = append(cmdArgs, "--create-namespace")
	}
	if opts.Version != "" {
		cmdArgs = append(cmdArgs, "--version", shellEscape(opts.Version))
	}
	for _, vf := range opts.ValuesFiles {
		cmdArgs = append(cmdArgs, "--values", shellEscape(vf))
	}
	for _, sv := range opts.SetValues {
		// Helm set values can be complex, e.g., name=value, array[0]=value, object.key=value
		// Ensure shellEscape handles this appropriately or consider more specific escaping if needed.
		cmdArgs = append(cmdArgs, "--set", shellEscape(sv))
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
		cmdArgs = append(cmdArgs, "--description", shellEscape(opts.Description))
	}

	cmd := strings.Join(cmdArgs, " ")
	execTimeout := DefaultHelmTimeout
	if opts.Timeout > 0 { // If helm's internal timeout is set, make sure exec timeout is larger
		execTimeout = opts.Timeout + (1 * time.Minute) // Add a buffer
	}

	execOptions := &connector.ExecOptions{
		Sudo:       opts.Sudo,
		Timeout:    execTimeout,
		Retries:    opts.Retries,
		RetryDelay: opts.RetryDelay,
	}

	stdout, stderr, err := conn.Exec(ctx, cmd, execOptions)
	if err != nil {
		// Include stdout and stderr for better debugging, as Helm often prints useful info there.
		return errors.Wrapf(err, "helm install for release '%s' (chart '%s') failed. Stdout: %s, Stderr: %s", releaseName, chartPath, string(stdout), string(stderr))
	}
	// Helm install usually prints NOTES to stdout on success.
	// We don't capture/return these notes here, but they are in stdout if needed.
	return nil
}

// HelmUninstall uninstalls a Helm release.
// Corresponds to `helm uninstall [NAME] [flags]`.
func (r *defaultRunner) HelmUninstall(ctx context.Context, conn connector.Connector, releaseName string, opts HelmUninstallOptions) error {
	if conn == nil {
		return errors.New("connector cannot be nil")
	}
	if releaseName == "" {
		return errors.New("releaseName is required for HelmUninstall")
	}

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "helm", "uninstall", shellEscape(releaseName))

	if opts.Namespace != "" {
		cmdArgs = append(cmdArgs, "--namespace", shellEscape(opts.Namespace))
	}
	if opts.KubeconfigPath != "" {
		cmdArgs = append(cmdArgs, "--kubeconfig", shellEscape(opts.KubeconfigPath))
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
	execOptions := &connector.ExecOptions{Sudo: opts.Sudo, Timeout: execTimeout}

	_, stderr, err := conn.Exec(ctx, cmd, execOptions)
	if err != nil {
		// Idempotency: If release not found, helm uninstall errors.
		// "Error: uninstall: Release not loaded: <releaseName>: release: not found"
		if strings.Contains(string(stderr), "release: not found") {
			return nil // Consider "not found" as success for uninstall idempotency
		}
		return errors.Wrapf(err, "helm uninstall for release '%s' failed. Stderr: %s", releaseName, string(stderr))
	}
	return nil
}

// HelmList lists Helm releases.
// Corresponds to `helm list [flags]`.
func (r *defaultRunner) HelmList(ctx context.Context, conn connector.Connector, opts HelmListOptions) ([]HelmReleaseInfo, error) {
	if conn == nil {
		return nil, errors.New("connector cannot be nil")
	}

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "helm", "list")

	if opts.Namespace != "" {
		cmdArgs = append(cmdArgs, "--namespace", shellEscape(opts.Namespace))
	}
	if opts.KubeconfigPath != "" {
		cmdArgs = append(cmdArgs, "--kubeconfig", shellEscape(opts.KubeconfigPath))
	}
	if opts.AllNamespaces {
		cmdArgs = append(cmdArgs, "--all-namespaces")
	}
	if opts.Filter != "" {
		cmdArgs = append(cmdArgs, "--filter", shellEscape(opts.Filter))
	}
	if opts.Selector != "" {
		cmdArgs = append(cmdArgs, "--selector", shellEscape(opts.Selector))
	}
	if opts.Max > 0 {
		cmdArgs = append(cmdArgs, "--max", fmt.Sprintf("%d", opts.Max))
	}
	if opts.Offset > 0 {
		cmdArgs = append(cmdArgs, "--offset", fmt.Sprintf("%d", opts.Offset))
	}
	if opts.ByDate { // --date is the flag for Helm 3
		cmdArgs = append(cmdArgs, "--date")
	}
	if opts.SortReverse { // --reverse is the flag
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
	if opts.Uninstalled { // Shows deleted releases that are preserved via --keep-history
		cmdArgs = append(cmdArgs, "--uninstalled")
	}
	if opts.Uninstalling { // Shows releases that are currently uninstalling
		cmdArgs = append(cmdArgs, "--uninstalling")
	}

	// Always request JSON output for easier parsing
	cmdArgs = append(cmdArgs, "-o", "json")

	cmd := strings.Join(cmdArgs, " ")
	execOptions := &connector.ExecOptions{Sudo: opts.Sudo, Timeout: DefaultHelmTimeout}

	stdout, stderr, err := conn.Exec(ctx, cmd, execOptions)
	if err != nil {
		return nil, errors.Wrapf(err, "helm list failed. Stderr: %s", string(stderr))
	}

	var releases []HelmReleaseInfo
	if err := json.Unmarshal(stdout, &releases); err != nil {
		// Helm list -o json might return empty string if no releases, not "[]"
		if strings.TrimSpace(string(stdout)) == "" || strings.TrimSpace(string(stdout)) == "null" {
			return []HelmReleaseInfo{}, nil // No releases found
		}
		return nil, errors.Wrapf(err, "failed to parse helm list JSON output. Output: %s", string(stdout))
	}
	return releases, nil
}


// HelmStatus gets the status of a Helm release.
// Corresponds to `helm status [NAME] [flags]`.
func (r *defaultRunner) HelmStatus(ctx context.Context, conn connector.Connector, releaseName string, opts HelmStatusOptions) (*HelmReleaseInfo, error) {
	if conn == nil {
		return nil, errors.New("connector cannot be nil")
	}
	if releaseName == "" {
		return nil, errors.New("releaseName is required for HelmStatus")
	}

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "helm", "status", shellEscape(releaseName))

	if opts.Namespace != "" {
		cmdArgs = append(cmdArgs, "--namespace", shellEscape(opts.Namespace))
	}
	if opts.KubeconfigPath != "" {
		cmdArgs = append(cmdArgs, "--kubeconfig", shellEscape(opts.KubeconfigPath))
	}
	if opts.Revision > 0 {
		cmdArgs = append(cmdArgs, "--revision", fmt.Sprintf("%d", opts.Revision))
	}
	// Helm status does not have --show-desc. Description is part of the default output or JSON.
	// The `Notes` field in `HelmReleaseInfo` will capture this if available in JSON.

	cmdArgs = append(cmdArgs, "-o", "json") // Request JSON output

	cmd := strings.Join(cmdArgs, " ")
	execOptions := &connector.ExecOptions{Sudo: opts.Sudo, Timeout: DefaultHelmTimeout}

	stdout, stderr, err := conn.Exec(ctx, cmd, execOptions)
	if err != nil {
		// If release not found "Error: release: not found"
		if strings.Contains(string(stderr), "release: not found") {
			return nil, nil // Not found, return nil, nil similar to other inspect/get methods
		}
		return nil, errors.Wrapf(err, "helm status for release '%s' failed. Stderr: %s", releaseName, string(stderr))
	}

	var status HelmReleaseInfo
	if err := json.Unmarshal(stdout, &status); err != nil {
		return nil, errors.Wrapf(err, "failed to parse helm status JSON output for release '%s'. Output: %s", releaseName, string(stdout))
	}
	return &status, nil
}


// HelmRepoAdd adds a chart repository.
// Corresponds to `helm repo add [NAME] [URL] [flags]`.
func (r *defaultRunner) HelmRepoAdd(ctx context.Context, conn connector.Connector, name, url string, opts HelmRepoAddOptions) error {
	if conn == nil {
		return errors.New("connector cannot be nil")
	}
	if name == "" || url == "" {
		return errors.New("name and url are required for HelmRepoAdd")
	}

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "helm", "repo", "add", shellEscape(name), shellEscape(url))

	if opts.Username != "" {
		cmdArgs = append(cmdArgs, "--username", shellEscape(opts.Username))
	}
	if opts.Password != "" {
		cmdArgs = append(cmdArgs, "--password", shellEscape(opts.Password)) // Consider using --password-stdin for security
	}
	if opts.CAFile != "" {
		cmdArgs = append(cmdArgs, "--ca-file", shellEscape(opts.CAFile))
	}
	if opts.CertFile != "" {
		cmdArgs = append(cmdArgs, "--cert-file", shellEscape(opts.CertFile))
	}
	if opts.KeyFile != "" {
		cmdArgs = append(cmdArgs, "--key-file", shellEscape(opts.KeyFile))
	}
	if opts.Insecure {
		cmdArgs = append(cmdArgs, "--insecure-skip-tls-verify")
	}
	if opts.ForceUpdate { // Helm v3 uses --force-update
		cmdArgs = append(cmdArgs, "--force-update")
	}
	if opts.PassCredentials {
		cmdArgs = append(cmdArgs, "--pass-credentials")
	}

	cmd := strings.Join(cmdArgs, " ")
	execOptions := &connector.ExecOptions{Sudo: opts.Sudo, Timeout: DefaultHelmTimeout}

	_, stderr, err := conn.Exec(ctx, cmd, execOptions)
	if err != nil {
		// Idempotency: If repo already exists, `helm repo add` errors unless --force-update is used.
		// Error: repository name (NAME) already exists, use --force-update to overwrite
		if strings.Contains(string(stderr), "already exists") && !opts.ForceUpdate {
			// If it already exists and we are not forcing, this is an error by helm's definition.
			// If we want this to be idempotent success, we could return nil here.
			// For now, let helm's behavior dictate: error if not --force-update and exists.
			return errors.Wrapf(err, "helm repo add '%s' ('%s') failed as it already exists and --force-update not used. Stderr: %s", name, url, string(stderr))
		} else if strings.Contains(string(stderr), "already exists") && opts.ForceUpdate {
			// This shouldn't happen as --force-update should prevent the error.
			// But if Helm still errors, pass it through.
		}
		return errors.Wrapf(err, "helm repo add '%s' ('%s') failed. Stderr: %s", name, url, string(stderr))
	}
	return nil
}


// HelmRepoUpdate updates information of available charts locally from chart repositories.
// Corresponds to `helm repo update [REPO1 [REPO2 ...]]`. If no repo names given, updates all.
func (r *defaultRunner) HelmRepoUpdate(ctx context.Context, conn connector.Connector, repoNames []string) error {
	if conn == nil {
		return errors.New("connector cannot be nil")
	}
	var cmdArgs []string
	cmdArgs = append(cmdArgs, "helm", "repo", "update")
	for _, name := range repoNames {
		cmdArgs = append(cmdArgs, shellEscape(name))
	}
	cmd := strings.Join(cmdArgs, " ")
	// Repo update can take time depending on network and number of repos.
	execOptions := &connector.ExecOptions{Sudo: false, Timeout: 5 * time.Minute} // Sudo typically not needed for repo update

	_, stderr, err := conn.Exec(ctx, cmd, execOptions)
	if err != nil {
		return errors.Wrapf(err, "helm repo update failed. Stderr: %s", string(stderr))
	}
	return nil
}

// HelmVersion gets the Helm client version information.
// Corresponds to `helm version --template {{json .}}` or `helm version -o json`.
func (r *defaultRunner) HelmVersion(ctx context.Context, conn connector.Connector) (*HelmVersionInfo, error) {
	if conn == nil {
		return nil, errors.New("connector cannot be nil")
	}
	cmd := "helm version -o json" // Helm 3+
	execOptions := &connector.ExecOptions{Sudo: false, Timeout: 30 * time.Second}

	stdout, stderr, err := conn.Exec(ctx, cmd, execOptions)
	if err != nil {
		return nil, errors.Wrapf(err, "helm version failed. Stderr: %s", string(stderr))
	}

	var versionInfo HelmVersionInfo
	if err := json.Unmarshal(stdout, &versionInfo); err != nil {
		// Fallback for older helm versions or if -o json isn't supported as expected
		// Try parsing plain text output: Version:vX.Y.Z GitCommit:...
		// This is more fragile.
		// For now, rely on JSON output.
		return nil, errors.Wrapf(err, "failed to parse helm version JSON output. Output: %s", string(stdout))
	}
	return &versionInfo, nil
}


// HelmSearchRepo searches repositories for a keyword.
// Corresponds to `helm search repo [KEYWORD] [flags]`.
func (r *defaultRunner) HelmSearchRepo(ctx context.Context, conn connector.Connector, keyword string, opts HelmSearchOptions) ([]HelmChartInfo, error) {
	if conn == nil {
		return nil, errors.New("connector cannot be nil")
	}
	if keyword == "" {
		return nil, errors.New("keyword is required for HelmSearchRepo")
	}

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "helm", "search", "repo", shellEscape(keyword))

	if opts.Regexp {
		cmdArgs = append(cmdArgs, "--regexp")
	}
	if opts.Devel {
		cmdArgs = append(cmdArgs, "--devel")
	}
	if opts.Version != "" { // Specific version constraint
		cmdArgs = append(cmdArgs, "--version", shellEscape(opts.Version))
	}
	if opts.Versions { // Show all versions
		cmdArgs = append(cmdArgs, "--versions")
	}

	// Always request JSON output for easier parsing
	cmdArgs = append(cmdArgs, "-o", "json")
	// Note: opts.OutputFormat is defined in interface but helm search repo -o json is fairly standard.
	// If table/yaml is needed, the parsing logic here would need to change or return raw string.

	cmd := strings.Join(cmdArgs, " ")
	execOptions := &connector.ExecOptions{Sudo: opts.Sudo, Timeout: DefaultHelmTimeout}

	stdout, stderr, err := conn.Exec(ctx, cmd, execOptions)
	if err != nil {
		return nil, errors.Wrapf(err, "helm search repo for keyword '%s' failed. Stderr: %s", keyword, string(stderr))
	}

	var charts []HelmChartInfo
	if err := json.Unmarshal(stdout, &charts); err != nil {
		// Helm search repo -o json might return empty string or "null" if no results.
		if strings.TrimSpace(string(stdout)) == "" || strings.TrimSpace(string(stdout)) == "null" || strings.TrimSpace(string(stdout)) == "[]" {
			return []HelmChartInfo{}, nil // No charts found
		}
		return nil, errors.Wrapf(err, "failed to parse helm search repo JSON output. Output: %s", string(stdout))
	}
	return charts, nil
}

// HelmPull downloads a chart from a repository and (optionally) unpacks it in local directory.
// Corresponds to `helm pull [CHART] [flags]`.
// Returns the path to the downloaded chart (or directory if untarred). This is usually printed to stdout by Helm.
func (r *defaultRunner) HelmPull(ctx context.Context, conn connector.Connector, chartPath string, opts HelmPullOptions) (string, error) {
	if conn == nil {
		return "", errors.New("connector cannot be nil")
	}
	if chartPath == "" {
		return "", errors.New("chartPath is required for HelmPull (can be repo/chart_name or URL)")
	}

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "helm", "pull", shellEscape(chartPath))

	if opts.Destination != "" {
		cmdArgs = append(cmdArgs, "--destination", shellEscape(opts.Destination))
	}
	if opts.Prov {
		cmdArgs = append(cmdArgs, "--prov")
	}
	if opts.Untar {
		cmdArgs = append(cmdArgs, "--untar")
		if opts.UntarDir != "" { // Only makes sense if --untar is true
			cmdArgs = append(cmdArgs, "--untardir", shellEscape(opts.UntarDir))
		}
	}
	if opts.Verify {
		cmdArgs = append(cmdArgs, "--verify")
		if opts.Keyring != "" { // Only makes sense if --verify is true
			cmdArgs = append(cmdArgs, "--keyring", shellEscape(opts.Keyring))
		}
	}
	if opts.Version != "" {
		cmdArgs = append(cmdArgs, "--version", shellEscape(opts.Version))
	}
	if opts.CAFile != "" {
		cmdArgs = append(cmdArgs, "--ca-file", shellEscape(opts.CAFile))
	}
	if opts.CertFile != "" {
		cmdArgs = append(cmdArgs, "--cert-file", shellEscape(opts.CertFile))
	}
	if opts.KeyFile != "" {
		cmdArgs = append(cmdArgs, "--key-file", shellEscape(opts.KeyFile))
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
		cmdArgs = append(cmdArgs, "--username", shellEscape(opts.Username))
	}
	if opts.Password != "" {
		cmdArgs = append(cmdArgs, "--password", shellEscape(opts.Password))
	}


	cmd := strings.Join(cmdArgs, " ")
	// Pulling can take time, especially for large charts or slow networks.
	execOptions := &connector.ExecOptions{Sudo: opts.Sudo, Timeout: 5 * time.Minute}

	stdout, stderr, err := conn.Exec(ctx, cmd, execOptions)
	if err != nil {
		return "", errors.Wrapf(err, "helm pull for chart '%s' failed. Stderr: %s", chartPath, string(stderr))
	}

	// Helm pull usually prints the location of the pulled chart to stdout on success,
	// e.g., "Successfully downloaded chart to /path/to/chart-0.1.0.tgz" or similar if untarred.
	// We return this output.
	return strings.TrimSpace(string(stdout)), nil
}

// HelmPackage packages a chart directory into a chart archive.
// Corresponds to `helm package [CHART_PATH] [flags]`.
// Returns the path to the packaged chart, usually printed to stdout by Helm.
func (r *defaultRunner) HelmPackage(ctx context.Context, conn connector.Connector, chartPath string, opts HelmPackageOptions) (string, error) {
	if conn == nil {
		return "", errors.New("connector cannot be nil")
	}
	if chartPath == "" {
		return "", errors.New("chartPath (directory of the chart) is required for HelmPackage")
	}

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "helm", "package", shellEscape(chartPath))

	if opts.Destination != "" {
		cmdArgs = append(cmdArgs, "--destination", shellEscape(opts.Destination))
	}
	if opts.Sign {
		cmdArgs = append(cmdArgs, "--sign")
		if opts.Key != "" {
			cmdArgs = append(cmdArgs, "--key", shellEscape(opts.Key))
		}
		if opts.Keyring != "" {
			cmdArgs = append(cmdArgs, "--keyring", shellEscape(opts.Keyring))
		}
		if opts.PassphraseFile != "" {
			cmdArgs = append(cmdArgs, "--passphrase-file", shellEscape(opts.PassphraseFile))
		}
	}
	if opts.Version != "" {
		cmdArgs = append(cmdArgs, "--version", shellEscape(opts.Version))
	}
	if opts.AppVersion != "" {
		cmdArgs = append(cmdArgs, "--app-version", shellEscape(opts.AppVersion))
	}
	if opts.DependencyUpdate {
		cmdArgs = append(cmdArgs, "--dependency-update")
	}

	cmd := strings.Join(cmdArgs, " ")
	execOptions := &connector.ExecOptions{Sudo: opts.Sudo, Timeout: DefaultHelmTimeout}

	stdout, stderr, err := conn.Exec(ctx, cmd, execOptions)
	if err != nil {
		return "", errors.Wrapf(err, "helm package for chart at '%s' failed. Stderr: %s", chartPath, string(stderr))
	}

	// Helm package prints the path of the created package, e.g., "Successfully packaged chart and saved it to: /path/chart-0.1.0.tgz"
	// We need to parse this output to get the actual path.
	// Example line: Successfully packaged chart and saved it to: /path/to/your/chart-0.1.0.tgz
	outputStr := strings.TrimSpace(string(stdout))
	prefix := "Successfully packaged chart and saved it to: "
	if strings.HasPrefix(outputStr, prefix) {
		return strings.TrimSpace(strings.TrimPrefix(outputStr, prefix)), nil
	}
	// If output format is unexpected, return the raw stdout or an error.
	// For now, returning raw stdout if prefix not found, caller can parse.
	// A more robust solution would be a regex.
	return outputStr, nil
}
