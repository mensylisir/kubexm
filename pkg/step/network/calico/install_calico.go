package calico

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

type InstallCalicoHelmChartStep struct {
	step.Base
	Chart               *helm.HelmChart
	ReleaseName         string
	Namespace           string
	RemoteChartPath     string
	RemoteValuesPath    string
	AdminKubeconfigPath string
}

type InstallCalicoHelmChartStepBuilder struct {
	step.Builder[InstallCalicoHelmChartStepBuilder, *InstallCalicoHelmChartStep]
}

func NewInstallCalicoHelmChartStepBuilder(ctx runtime.Context, instanceName string) *InstallCalicoHelmChartStepBuilder {
	if ctx.GetClusterConfig().Spec.Network.Plugin != string(common.CNITypeCalico) {
		return nil
	}

	helmProvider := helm.NewHelmProvider(&ctx)
	calicoChart := helmProvider.GetChart(string(common.CNITypeCalico))
	if calicoChart == nil {
		return nil
	}
	s := &InstallCalicoHelmChartStep{
		Chart: calicoChart,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install or upgrade Calico via Helm chart", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 20 * time.Minute

	s.ReleaseName = calicoChart.ChartName()
	s.Namespace = "tigera-operator"
	s.AdminKubeconfigPath = filepath.Join(common.KubernetesConfigDir, common.AdminKubeconfigFileName)

	remoteDir := filepath.Join(ctx.GetUploadDir(), calicoChart.RepoName(), calicoChart.Version)

	valuesFileName := fmt.Sprintf("%s-values.yaml", calicoChart.RepoName())
	s.RemoteValuesPath = filepath.Join(remoteDir, valuesFileName)
	chartFileName := fmt.Sprintf("%s-%s.tgz", calicoChart.ChartName(), calicoChart.Version)
	s.RemoteChartPath = filepath.Join(remoteDir, chartFileName)

	b := new(InstallCalicoHelmChartStepBuilder).Init(s)
	return b
}

func (s *InstallCalicoHelmChartStep) Meta() *spec.StepMeta {
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

func (s *InstallCalicoHelmChartStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
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
	logger.Debug("All required artifacts found on remote host.")

	statusCmd := fmt.Sprintf(
		"helm status %s --namespace %s --kubeconfig %s -o json",
		s.ReleaseName,
		s.Namespace,
		s.AdminKubeconfigPath,
	)

	output, err := runner.Run(ctx.GoContext(), conn, statusCmd, s.Sudo)
	if err != nil {
		logger.Infof("Helm release '%s' not found. Installation is required.", s.ReleaseName)
		return false, nil
	}

	var status helmStatusOutput
	if err := json.Unmarshal([]byte(output), &status); err != nil {
		return false, errors.Wrap(err, "failed to parse helm status JSON output")
	}

	if status.Info.Status == "deployed" && status.Chart.Metadata.Version == s.Chart.Version {
		logger.Infof("Helm release '%s' version %s is already deployed in namespace '%s'. Skipping.", s.ReleaseName, s.Chart.Version, s.Namespace)
		return true, nil
	}

	logger.Infof("Helm release '%s' found, but its status ('%s') or version ('%s') is not as expected ('deployed', '%s'). Upgrade is required.", s.ReleaseName, status.Info.Status, status.Chart.Metadata.Version, s.Chart.Version)
	return false, nil
}

func (s *InstallCalicoHelmChartStep) Run(ctx runtime.ExecutionContext) error {
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

	logger.Infof("Executing remote Helm command: %s", cmd)
	output, err := runner.Run(ctx.GoContext(), conn, cmd, s.Sudo)
	if err != nil {
		return fmt.Errorf("failed to install/upgrade Calico Helm chart: %w\nOutput: %s", err, output)
	}

	logger.Info("Calico Helm chart installed/upgraded successfully.")
	logger.Debugf("Helm command output:\n%s", output)
	return nil
}

func (s *InstallCalicoHelmChartStep) Rollback(ctx runtime.ExecutionContext) error {
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

	logger.Warnf("Rolling back by uninstalling Helm release '%s' from namespace '%s'...", s.ReleaseName, s.Namespace)
	if _, err := runner.Run(ctx.GoContext(), conn, cmd, s.Sudo); err != nil {
		logger.Errorf("Failed to uninstall Calico Helm release (this is expected if installation failed): %v", err)
	} else {
		logger.Info("Successfully executed Helm uninstall command for Calico.")
	}

	return nil
}

var _ step.Step = (*InstallCalicoHelmChartStep)(nil)
