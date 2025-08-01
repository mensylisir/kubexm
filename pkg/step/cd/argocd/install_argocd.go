package argocd

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

// InstallArgoCDHelmChartStep is a step to install the Argo CD Helm chart.
type InstallArgoCDHelmChartStep struct {
	step.Base
	Chart               *helm.HelmChart
	ReleaseName         string
	Namespace           string
	RemoteChartPath     string
	RemoteValuesPath    string
	AdminKubeconfigPath string
}

// InstallArgoCDHelmChartStepBuilder is used to build instances.
type InstallArgoCDHelmChartStepBuilder struct {
	step.Builder[InstallArgoCDHelmChartStepBuilder, *InstallArgoCDHelmChartStep]
}

// NewInstallArgoCDHelmChartStepBuilder is the constructor.
func NewInstallArgoCDHelmChartStepBuilder(ctx runtime.Context, instanceName string) *InstallArgoCDHelmChartStepBuilder {
	helmProvider := helm.NewHelmProvider(&ctx)
	chart := helmProvider.GetChart("argocd")

	if chart == nil {
		// TODO: Add a check for whether argocd is enabled
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
	s.Namespace = "argocd"

	s.AdminKubeconfigPath = filepath.Join(common.KubernetesConfigDir, common.AdminKubeconfigFileName)

	remoteDir := filepath.Join(common.DefaultUploadTmpDir, chart.RepoName(), chart.ChartName()+"-"+chart.Version)
	s.RemoteValuesPath = filepath.Join(remoteDir, "argocd-values.yaml")
	chartFileName := fmt.Sprintf("%s-%s.tgz", chart.ChartName(), chart.Version)
	s.RemoteChartPath = filepath.Join(remoteDir, chartFileName)

	b := new(InstallArgoCDHelmChartStepBuilder).Init(s)
	return b
}

func (s *InstallArgoCDHelmChartStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *InstallArgoCDHelmChartStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
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
		return false, fmt.Errorf("required Argo CD values file not found at path %s", s.RemoteValuesPath)
	}

	chartExists, err := runner.Exists(ctx.GoContext(), conn, s.RemoteChartPath)
	if err != nil {
		return false, fmt.Errorf("failed to check for chart file: %w", err)
	}
	if !chartExists {
		return false, fmt.Errorf("Argo CD Helm chart .tgz file not found at path %s", s.RemoteChartPath)
	}

	kubeconfigExists, err := runner.Exists(ctx.GoContext(), conn, s.AdminKubeconfigPath)
	if err != nil {
		return false, fmt.Errorf("failed to check for kubeconfig file: %w", err)
	}
	if !kubeconfigExists {
		return false, fmt.Errorf("admin kubeconfig not found at %s", s.AdminKubeconfigPath)
	}

	logger.Info("All required Argo CD artifacts found on remote host.")
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

	logger.Infof("Executing remote Helm command: %s", cmd)
	output, err := runner.Run(ctx.GoContext(), conn, cmd, s.Sudo)
	if err != nil {
		return fmt.Errorf("failed to install/upgrade Argo CD Helm chart: %w\nOutput: %s", err, output)
	}

	logger.Info("Argo CD Helm chart installed/upgraded successfully.")
	logger.Debugf("Helm command output:\n%s", output)
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

	logger.Warnf("Rolling back by uninstalling Helm release '%s' from namespace '%s'...", s.ReleaseName, s.Namespace)
	if _, err := runner.Run(ctx.GoContext(), conn, cmd, s.Sudo); err != nil {
		logger.Errorf("Failed to uninstall Argo CD Helm release: %v", err)
	} else {
		logger.Info("Successfully executed Helm uninstall command for Argo CD.")
	}

	return nil
}

var _ step.Step = (*InstallArgoCDHelmChartStep)(nil)
