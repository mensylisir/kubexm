package helm

import (
	"fmt"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// HelmInstallStep executes 'helm install' for a given chart.
type HelmInstallStep struct {
	meta           spec.StepMeta
	ReleaseName    string
	ChartPath      string   // Local path or URL to the chart, or <repo>/<chart>
	Namespace      string   // Namespace to install the release into
	ValuesFiles    []string // List of paths to values files on the target host
	SetValues      []string // List of set values (e.g., "key1=value1,key2.subkey=value2")
	Version        string   // Specify chart version
	CreateNamespace bool     // Whether to create the namespace if it doesn't exist
	KubeconfigPath string   // Optional: Path to kubeconfig file on the target host
	Sudo           bool     // If helm command itself needs sudo (very rare)
	Retries        int
	RetryDelay     int // seconds
}

// NewHelmInstallStep creates a new HelmInstallStep.
func NewHelmInstallStep(instanceName, releaseName, chartPath, namespace, version string, valuesFiles, setValues []string, createNamespace, sudo bool, retries, retryDelay int) step.Step {
	name := instanceName
	if name == "" {
		name = fmt.Sprintf("HelmInstall-%s", releaseName)
		if namespace != "" {
			name += fmt.Sprintf("-in-%s", namespace)
		}
	}
	return &HelmInstallStep{
		meta: spec.StepMeta{
			Name:        name,
			Description: fmt.Sprintf("Installs Helm chart %s as release %s in namespace %s", chartPath, releaseName, namespace),
		},
		ReleaseName:     releaseName,
		ChartPath:       chartPath,
		Namespace:       namespace,
		ValuesFiles:     valuesFiles,
		SetValues:       setValues,
		Version:         version,
		CreateNamespace: createNamespace,
		KubeconfigPath:  kubeconfigPath,
		Sudo:            sudo,
		Retries:         retries,
		RetryDelay:      retryDelay,
	}
}

func (s *HelmInstallStep) Meta() *spec.StepMeta {
	return &s.meta
}

func (s *HelmInstallStep) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	// Precheck could try 'helm status <releaseName> -n <namespace>'
	// If it succeeds, the release might already exist.
	// This is a simplified precheck; a more robust one would check version and values.
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName())
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector: %w", err)
	}

	statusCmd := "helm status " + s.ReleaseName
	if s.Namespace != "" {
		statusCmd += " -n " + s.Namespace
	}
	if s.KubeconfigPath != "" {
		statusCmd += " --kubeconfig " + s.KubeconfigPath
	}

	// Check if helm is installed first
	if _, err := runnerSvc.LookPath(ctx.GoContext(), conn, "helm"); err != nil {
		logger.Info("Helm command not found, assuming release does not exist.")
		return false, nil
	}

	logger.Debug("Checking Helm release status", "command", statusCmd)
	_, _, err = runnerSvc.RunWithOptions(ctx.GoContext(), conn, statusCmd, &connector.ExecOptions{Sudo: s.Sudo, Check: true})
	if err == nil {
		logger.Info("Helm release already exists.", "release", s.ReleaseName, "namespace", s.Namespace)
		return true, nil // Release exists
	}
	// If error, assume release does not exist or status check failed. Let Run proceed.
	logger.Info("Helm release does not exist or status check failed. Install will proceed.", "release", s.ReleaseName, "error_if_any", err)
	return false, nil
}

func (s *HelmInstallStep) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName())
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector: %w", err)
	}

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "helm", "install", s.ReleaseName, s.ChartPath)

	if s.Namespace != "" {
		cmdArgs = append(cmdArgs, "--namespace", s.Namespace)
	}
	if s.CreateNamespace {
		cmdArgs = append(cmdArgs, "--create-namespace")
	}
	if s.Version != "" {
		cmdArgs = append(cmdArgs, "--version", s.Version)
	}
	for _, vf := range s.ValuesFiles {
		cmdArgs = append(cmdArgs, "--values", vf)
	}
	for _, sv := range s.SetValues {
		cmdArgs = append(cmdArgs, "--set", sv)
	}
	if s.KubeconfigPath != "" {
		cmdArgs = append(cmdArgs, "--kubeconfig", s.KubeconfigPath)
	}
	// Add other common flags like --wait, --timeout if needed as struct fields

	cmd := strings.Join(cmdArgs, " ")
	execOptions := &connector.ExecOptions{
		Sudo:       s.Sudo,
		Retries:    s.Retries,
		RetryDelay: time.Duration(s.RetryDelay) * time.Second,
	}

	logger.Info("Running helm install command", "command", cmd)
	stdout, stderr, err := runnerSvc.RunWithOptions(ctx.GoContext(), conn, cmd, execOptions)
	if err != nil {
		logger.Error("helm install failed", "error", err, "stdout", string(stdout), "stderr", string(stderr))
		return fmt.Errorf("helm install for release %s (chart %s) failed: %w. Stdout: %s, Stderr: %s", s.ReleaseName, s.ChartPath, err, string(stdout), string(stderr))
	}

	logger.Info("Helm install completed successfully.", "release", s.ReleaseName, "stdout", string(stdout))
	return nil
}

func (s *HelmInstallStep) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName())
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		logger.Error("Failed to get connector for host during rollback", "error", err)
		return nil // Best effort
	}

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "helm", "uninstall", s.ReleaseName)
	if s.Namespace != "" {
		cmdArgs = append(cmdArgs, "--namespace", s.Namespace)
	}
	if s.KubeconfigPath != "" {
		cmdArgs = append(cmdArgs, "--kubeconfig", s.KubeconfigPath)
	}
	// Add --wait, --timeout if applicable for uninstall

	cmd := strings.Join(cmdArgs, " ")
	logger.Info("Attempting helm uninstall for rollback", "command", cmd)
	_, stderr, err := runnerSvc.RunWithOptions(ctx.GoContext(), conn, cmd, &connector.ExecOptions{Sudo: s.Sudo})
	if err != nil {
		logger.Warn("helm uninstall command failed during rollback (best effort).", "error", err, "stderr", string(stderr))
	} else {
		logger.Info("helm uninstall executed successfully for rollback.")
	}
	return nil
}

var _ step.Step = (*HelmInstallStep)(nil)

[end of pkg/step/helm/helm_install_step.go]
