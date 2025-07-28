package hybridnet

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type InstallHybridnetHelmChartStep struct {
	step.Base
	ReleaseName         string
	Namespace           string
	RemoteChartPath     string
	RemoteValuesPath    string
	AdminKubeconfigPath string
}

type InstallHybridnetHelmChartStepBuilder struct {
	step.Builder[InstallHybridnetHelmChartStepBuilder, *InstallHybridnetHelmChartStep]
}

func NewInstallHybridnetHelmChartStepBuilder(ctx runtime.Context, instanceName string) *InstallHybridnetHelmChartStepBuilder {
	s := &InstallHybridnetHelmChartStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install or upgrade Hybridnet via Helm chart", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 20 * time.Minute

	s.ReleaseName = "hybridnet"
	s.Namespace = "kube-system"

	s.AdminKubeconfigPath = filepath.Join(ctx.GetGlobalWorkDir(), "kubeconfigs", common.AdminKubeconfigFileName)

	remoteDir := filepath.Join(common.DefaultUploadTmpDir, "hybridnet")
	s.RemoteValuesPath = filepath.Join(remoteDir, "hybridnet-values.yaml")

	b := new(InstallHybridnetHelmChartStepBuilder).Init(s)
	return b
}

func (s *InstallHybridnetHelmChartStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *InstallHybridnetHelmChartStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	valuesExists, err := runner.Exists(ctx.GoContext(), conn, s.RemoteValuesPath)
	if err != nil {
		return false, err
	}
	if !valuesExists {
		return false, fmt.Errorf("required Hybridnet values file not found at %s, cannot proceed", s.RemoteValuesPath)
	}

	remoteDir := filepath.Dir(s.RemoteValuesPath)
	findCmd := fmt.Sprintf("find %s -name '*.tgz' -print -quit", remoteDir)
	chartPath, err := runner.Run(ctx.GoContext(), conn, findCmd, s.Sudo)
	if err != nil || strings.TrimSpace(chartPath) == "" {
		return false, fmt.Errorf("Hybridnet Helm chart .tgz file not found in remote directory %s", remoteDir)
	}

	logger.Info("Helm artifacts (chart and values) for Hybridnet found on remote host. Ready to install.")
	return false, nil
}

func (s *InstallHybridnetHelmChartStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	remoteDir := filepath.Dir(s.RemoteValuesPath)
	findCmd := fmt.Sprintf("find %s -name '*.tgz' -print -quit", remoteDir)
	remoteChartPath, err := runner.Run(ctx.GoContext(), conn, findCmd, s.Sudo)
	if err != nil || strings.TrimSpace(remoteChartPath) == "" {
		return fmt.Errorf("failed to find Hybridnet Helm chart .tgz file in remote directory %s", remoteDir)
	}
	s.RemoteChartPath = strings.TrimSpace(remoteChartPath)

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

	logger.Infof("Executing remote Helm command for Hybridnet: %s", cmd)
	output, err := runner.Run(ctx.GoContext(), conn, cmd, s.Sudo)
	if err != nil {
		return fmt.Errorf("failed to install/upgrade Hybridnet Helm chart: %w\nOutput: %s", err, output)
	}

	logger.Info("Hybridnet Helm chart installed/upgraded successfully.")
	logger.Debugf("Helm command output:\n%s", output)
	return nil
}

func (s *InstallHybridnetHelmChartStep) Rollback(ctx runtime.ExecutionContext) error {
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
		logger.Errorf("Failed to uninstall Hybridnet Helm release (this may be expected if installation failed): %v", err)
	} else {
		logger.Info("Successfully executed Helm uninstall command for Hybridnet.")
	}

	return nil
}

var _ step.Step = (*InstallHybridnetHelmChartStep)(nil)
