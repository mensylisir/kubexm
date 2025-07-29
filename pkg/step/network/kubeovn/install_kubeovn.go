package kubeovn

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

type InstallKubeOvnHelmChartStep struct {
	step.Base
	Chart               *helm.HelmChart
	ReleaseName         string
	Namespace           string
	RemoteChartPath     string
	RemoteValuesPath    string
	AdminKubeconfigPath string
}

type InstallKubeOvnHelmChartStepBuilder struct {
	step.Builder[InstallKubeOvnHelmChartStepBuilder, *InstallKubeOvnHelmChartStep]
}

func NewInstallKubeOvnHelmChartStepBuilder(ctx runtime.Context, instanceName string) *InstallKubeOvnHelmChartStepBuilder {
	helmProvider := helm.NewHelmProvider(&ctx)
	kubeovnChart := helmProvider.GetChart(string(common.CNITypeKubeOvn))

	if kubeovnChart == nil {
		return nil
	}

	s := &InstallKubeOvnHelmChartStep{
		Chart: kubeovnChart,
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install or upgrade Kube-OVN via Helm chart", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 25 * time.Minute

	s.ReleaseName = kubeovnChart.ChartName()
	s.Namespace = "kube-system"

	s.AdminKubeconfigPath = filepath.Join(common.KubernetesConfigDir, common.AdminKubeconfigFileName)

	remoteDir := filepath.Join(common.DefaultUploadTmpDir, kubeovnChart.RepoName())
	s.RemoteValuesPath = filepath.Join(remoteDir, "kubeovn-values.yaml")
	chartFileName := fmt.Sprintf("%s-%s.tgz", kubeovnChart.ChartName(), kubeovnChart.Version)
	s.RemoteChartPath = filepath.Join(remoteDir, chartFileName)

	b := new(InstallKubeOvnHelmChartStepBuilder).Init(s)
	return b
}

func (s *InstallKubeOvnHelmChartStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *InstallKubeOvnHelmChartStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
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
		return false, fmt.Errorf("required Kube-OVN values file not found at precise path %s, cannot proceed", s.RemoteValuesPath)
	}

	chartExists, err := runner.Exists(ctx.GoContext(), conn, s.RemoteChartPath)
	if err != nil {
		return false, fmt.Errorf("failed to check for chart file: %w", err)
	}
	if !chartExists {
		return false, fmt.Errorf("Kube-OVN Helm chart .tgz file not found at precise path %s", s.RemoteChartPath)
	}

	kubeconfigExists, err := runner.Exists(ctx.GoContext(), conn, s.AdminKubeconfigPath)
	if err != nil {
		return false, fmt.Errorf("failed to check for kubeconfig file: %w", err)
	}
	if !kubeconfigExists {
		return false, fmt.Errorf("admin kubeconfig not found at its permanent location %s; ensure DistributeKubeconfigsStep ran successfully", s.AdminKubeconfigPath)
	}

	logger.Info("All required Kube-OVN artifacts (chart, values, kubeconfig) found on remote host. Ready to install.")
	return false, nil
}

func (s *InstallKubeOvnHelmChartStep) Run(ctx runtime.ExecutionContext) error {
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
		return fmt.Errorf("failed to install/upgrade Kube-OVN Helm chart: %w\nOutput: %s", err, output)
	}

	logger.Info("Kube-OVN Helm chart installed/upgraded successfully.")
	logger.Debugf("Helm command output:\n%s", output)
	return nil
}

func (s *InstallKubeOvnHelmChartStep) Rollback(ctx runtime.ExecutionContext) error {
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
		logger.Errorf("Failed to uninstall Kube-OVN Helm release (this may be expected if installation failed): %v", err)
	} else {
		logger.Info("Successfully executed Helm uninstall command for Kube-OVN.")
	}

	return nil
}

var _ step.Step = (*InstallKubeOvnHelmChartStep)(nil)
