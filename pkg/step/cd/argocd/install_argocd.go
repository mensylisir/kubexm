package argocd

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/helm"
	"github.com/pkg/errors"
)

type InstallArgoCDHelmChartStep struct {
	step.Base
	Chart               *helm.HelmChart
	ReleaseName         string
	Namespace           string
	RemoteChartPath     string
	RemoteValuesPath    string
	AdminKubeconfigPath string
}

type InstallArgoCDHelmChartStepBuilder struct {
	step.Builder[InstallArgoCDHelmChartStepBuilder, *InstallArgoCDHelmChartStep]
}

func NewInstallArgoCDHelmChartStepBuilder(ctx runtime.Context, instanceName string) *InstallArgoCDHelmChartStepBuilder {
	helmProvider := helm.NewHelmProvider(&ctx)
	chart := helmProvider.GetChart("argocd")
	if chart == nil {
		return nil
	}

	s := &InstallArgoCDHelmChartStep{
		Chart: chart,
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install or upgrade Argo CD via Helm chart", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 15 * time.Minute

	s.ReleaseName = "argocd"
	s.Namespace = "argocd-system"

	s.AdminKubeconfigPath = filepath.Join(common.KubernetesConfigDir, common.AdminKubeconfigFileName)

	remoteDir := filepath.Join(ctx.GetUploadDir(), chart.RepoName(), chart.Version)

	valuesFileName := fmt.Sprintf("%s-values.yaml", chart.RepoName())
	s.RemoteValuesPath = filepath.Join(remoteDir, valuesFileName)
	chartFileName := fmt.Sprintf("%s-%s.tgz", chart.ChartName(), chart.Version)
	s.RemoteChartPath = filepath.Join(remoteDir, chartFileName)

	b := new(InstallArgoCDHelmChartStepBuilder).Init(s)
	return b
}

func (s *InstallArgoCDHelmChartStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

type helmStatusOutput struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Info      struct {
		Status string `json:"status"`
	} `json:"info"`
	Chart struct {
		Metadata struct {
			Version string `json:"version"`
		} `json:"metadata"`
	} `json:"chart"`
}

func (s *InstallArgoCDHelmChartStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	if _, err := runner.LookPath(ctx.GoContext(), conn, "helm"); err != nil {
		return false, errors.Wrap(err, "helm command not found in PATH on the target host")
	}

	for _, path := range []string{s.RemoteValuesPath, s.RemoteChartPath, s.AdminKubeconfigPath} {
		exists, err := runner.Exists(ctx.GoContext(), conn, path)
		if err != nil {
			return false, fmt.Errorf("failed to check for required file %s: %w", path, err)
		}
		if !exists {
			return false, fmt.Errorf("required file not found at remote path %s", path)
		}
	}
	logger.Debug("All required Argo CD artifacts found on remote host.")

	statusCmd := fmt.Sprintf(
		"helm status %s --namespace %s --kubeconfig %s -o json",
		s.ReleaseName,
		s.Namespace,
		s.AdminKubeconfigPath,
	)

	output, err := runner.Run(ctx.GoContext(), conn, statusCmd, s.Sudo)
	if err != nil {
		logger.Info("Helm release not found. Installation is required.", "release", s.ReleaseName)
		return false, nil
	}

	var status helmStatusOutput
	if err := json.Unmarshal([]byte(output), &status); err != nil {
		return false, errors.Wrap(err, "failed to parse helm status JSON output")
	}

	if status.Info.Status == "deployed" && status.Chart.Metadata.Version == s.Chart.Version {
		logger.Info("Helm release is already deployed and up-to-date. Skipping.", "release", s.ReleaseName, "version", s.Chart.Version, "namespace", s.Namespace)
		return true, nil
	}

	logger.Info("Helm release found, but needs upgrade.", "release", s.ReleaseName, "current_status", status.Info.Status, "current_version", status.Chart.Metadata.Version, "desired_version", s.Chart.Version)
	return false, nil
}

func (s *InstallArgoCDHelmChartStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	cmd := fmt.Sprintf(
		"helm upgrade --install %s %s "+
			"--namespace %s "+
			"--create-namespace "+
			"--values %s "+
			"--kubeconfig %s "+
			"--wait "+
			"--atomic",
		s.ReleaseName,
		s.RemoteChartPath,
		s.Namespace,
		s.RemoteValuesPath,
		s.AdminKubeconfigPath,
	)

	logger.Info("Executing remote Helm command.", "command", cmd)
	output, err := runner.Run(ctx.GoContext(), conn, cmd, s.Sudo)
	if err != nil {
		return fmt.Errorf("failed to install/upgrade Argo CD Helm chart: %w\nOutput: %s", err, output)
	}

	logger.Info("Argo CD Helm chart installed/upgraded successfully.")
	logger.Debug("Helm command output.", "output", output)
	return nil
}

func (s *InstallArgoCDHelmChartStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Errorf("Failed to get connector for rollback: %v", err)
		return nil
	}

	cmd := fmt.Sprintf(
		"helm uninstall %s --namespace %s --kubeconfig %s",
		s.ReleaseName,
		s.Namespace,
		s.AdminKubeconfigPath,
	)

	logger.Warn("Rolling back by uninstalling Helm release.", "release", s.ReleaseName, "namespace", s.Namespace)
	if _, err := runner.Run(ctx.GoContext(), conn, cmd, s.Sudo); err != nil {
		logger.Error(err, "Failed to uninstall Argo CD Helm release.")
	} else {
		logger.Info("Successfully executed Helm uninstall command for Argo CD.")
	}

	return nil
}

var _ step.Step = (*InstallArgoCDHelmChartStep)(nil)
