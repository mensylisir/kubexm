package longhorn

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/helm"
)

// InstallLonghornHelmChartStep is a step to install the Longhorn Helm chart.
type InstallLonghornHelmChartStep struct {
	step.Base
	Chart               *helm.HelmChart
	ReleaseName         string
	Namespace           string
	RemoteChartPath     string
	RemoteValuesPath    string
	AdminKubeconfigPath string
}

// InstallLonghornHelmChartStepBuilder is used to build instances.
type InstallLonghornHelmChartStepBuilder struct {
	step.Builder[InstallLonghornHelmChartStepBuilder, *InstallLonghornHelmChartStep]
}

// NewInstallLonghornHelmChartStepBuilder is the constructor.
func NewInstallLonghornHelmChartStepBuilder(ctx runtime.Context, instanceName string) *InstallLonghornHelmChartStepBuilder {
	helmProvider := helm.NewHelmProvider(&ctx)
	chart := helmProvider.GetChart("longhorn")

	if chart == nil {
		// TODO: Add a check for whether longhorn is enabled
		return nil
	}

	s := &InstallLonghornHelmChartStep{
		Chart: chart,
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install or upgrade Longhorn via Helm chart", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 20 * time.Minute // Longhorn can take a while to install

	s.ReleaseName = "longhorn"
	s.Namespace = "longhorn-system"

	s.AdminKubeconfigPath = filepath.Join(common.KubernetesConfigDir, common.AdminKubeconfigFileName)

	remoteDir := filepath.Join(common.DefaultUploadTmpDir, chart.RepoName(), chart.ChartName()+"-"+chart.Version)
	s.RemoteValuesPath = filepath.Join(remoteDir, "longhorn-values.yaml")
	chartFileName := fmt.Sprintf("%s-%s.tgz", chart.ChartName(), chart.Version)
	s.RemoteChartPath = filepath.Join(remoteDir, chartFileName)

	b := new(InstallLonghornHelmChartStepBuilder).Init(s)
	return b
}

func (s *InstallLonghornHelmChartStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *InstallLonghornHelmChartStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
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
		return false, fmt.Errorf("required Longhorn values file not found at path %s", s.RemoteValuesPath)
	}

	chartExists, err := runner.Exists(ctx.GoContext(), conn, s.RemoteChartPath)
	if err != nil {
		return false, fmt.Errorf("failed to check for chart file: %w", err)
	}
	if !chartExists {
		return false, fmt.Errorf("Longhorn Helm chart .tgz file not found at path %s", s.RemoteChartPath)
	}

	kubeconfigExists, err := runner.Exists(ctx.GoContext(), conn, s.AdminKubeconfigPath)
	if err != nil {
		return false, fmt.Errorf("failed to check for kubeconfig file: %w", err)
	}
	if !kubeconfigExists {
		return false, fmt.Errorf("admin kubeconfig not found at %s", s.AdminKubeconfigPath)
	}

	logger.Info("All required Longhorn artifacts found on remote host.")
	return false, nil
}

func (s *InstallLonghornHelmChartStep) Run(ctx runtime.ExecutionContext) error {
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
		return fmt.Errorf("failed to install/upgrade Longhorn Helm chart: %w\nOutput: %s", err, output)
	}

	logger.Info("Longhorn Helm chart installed/upgraded successfully.")
	logger.Debugf("Helm command output:\n%s", output)
	return nil
}

func (s *InstallLonghornHelmChartStep) Rollback(ctx runtime.ExecutionContext) error {
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
		logger.Errorf("Failed to uninstall Longhorn Helm release: %v", err)
	} else {
		logger.Info("Successfully executed Helm uninstall command for Longhorn.")
	}

	return nil
}

var _ step.Step = (*InstallLonghornHelmChartStep)(nil)
