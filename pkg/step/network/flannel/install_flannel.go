package flannel

import (
	"fmt"
	"path/filepath"
	"time"

	// 引入必要的包
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/helm"
)

type InstallFlannelHelmChartStep struct {
	step.Base
	Chart               *helm.HelmChart
	ReleaseName         string
	Namespace           string
	RemoteChartPath     string
	RemoteValuesPath    string
	AdminKubeconfigPath string
}

type InstallFlannelHelmChartStepBuilder struct {
	step.Builder[InstallFlannelHelmChartStepBuilder, *InstallFlannelHelmChartStep]
}

func NewInstallFlannelHelmChartStepBuilder(ctx runtime.Context, instanceName string) *InstallFlannelHelmChartStepBuilder {
	helmProvider := helm.NewHelmProvider(&ctx)
	flannelChart := helmProvider.GetChart(string(common.CNITypeFlannel))

	if flannelChart == nil {
		return nil
	}

	s := &InstallFlannelHelmChartStep{
		Chart: flannelChart,
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install or upgrade Flannel via Helm chart", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 15 * time.Minute

	s.ReleaseName = flannelChart.ChartName()
	s.Namespace = "kube-system"

	s.AdminKubeconfigPath = filepath.Join(common.KubernetesConfigDir, common.AdminKubeconfigFileName)

	remoteDir := filepath.Join(ctx.GetUploadDir(), flannelChart.RepoName(), flannelChart.ChartName()+"-"+flannelChart.Version)
	s.RemoteValuesPath = filepath.Join(remoteDir, "flannel-values.yaml")
	chartFileName := fmt.Sprintf("%s-%s.tgz", flannelChart.ChartName(), flannelChart.Version)
	s.RemoteChartPath = filepath.Join(remoteDir, chartFileName)

	b := new(InstallFlannelHelmChartStepBuilder).Init(s)
	return b
}

func (s *InstallFlannelHelmChartStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *InstallFlannelHelmChartStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	valuesExists, err := runner.Exists(ctx.GoContext(), conn, s.RemoteValuesPath)
	if err != nil {
		return false, fmt.Errorf("failed to check for values file: %w", err)
	}
	if !valuesExists {
		return false, fmt.Errorf("required Flannel values file not found at precise path %s, cannot proceed", s.RemoteValuesPath)
	}

	chartExists, err := runner.Exists(ctx.GoContext(), conn, s.RemoteChartPath)
	if err != nil {
		return false, fmt.Errorf("failed to check for chart file: %w", err)
	}
	if !chartExists {
		return false, fmt.Errorf("Flannel Helm chart .tgz file not found at precise path %s", s.RemoteChartPath)
	}

	kubeconfigExists, err := runner.Exists(ctx.GoContext(), conn, s.AdminKubeconfigPath)
	if err != nil {
		return false, fmt.Errorf("failed to check for kubeconfig file: %w", err)
	}
	if !kubeconfigExists {
		return false, fmt.Errorf("admin kubeconfig not found at its permanent location %s; ensure DistributeKubeconfigsStep ran successfully", s.AdminKubeconfigPath)
	}

	logger.Info("All required Flannel artifacts (chart, values, kubeconfig) found on remote host. Ready to install.")
	return false, nil
}

func (s *InstallFlannelHelmChartStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	cmd := fmt.Sprintf(
		"helm upgrade --install %s %s "+
			"--namespace %s "+
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
		return fmt.Errorf("failed to install/upgrade Flannel Helm chart: %w\nOutput: %s", err, output)
	}

	logger.Info("Flannel Helm chart installed/upgraded successfully.")
	logger.Debugf("Helm command output:\n%s", output)
	return nil
}

func (s *InstallFlannelHelmChartStep) Rollback(ctx runtime.ExecutionContext) error {
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
		logger.Errorf("Failed to uninstall Flannel Helm release (this may be expected if installation failed): %v", err)
	} else {
		logger.Info("Successfully executed Helm uninstall command for Flannel.")
	}

	return nil
}

var _ step.Step = (*InstallFlannelHelmChartStep)(nil)
