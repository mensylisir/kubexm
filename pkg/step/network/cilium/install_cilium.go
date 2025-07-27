package cilium

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"path/filepath"
	"strings"
	"time"
)

type InstallCiliumHelmChartStep struct {
	step.Base
	ReleaseName         string
	Namespace           string
	RemoteValuesPath    string
	AdminKubeconfigPath string
}

type InstallCiliumHelmChartStepBuilder struct {
	step.Builder[InstallCiliumHelmChartStepBuilder, *InstallCiliumHelmChartStep]
}

func NewInstallCiliumHelmChartStepBuilder(ctx runtime.Context, instanceName string) *InstallCiliumHelmChartStepBuilder {
	s := &InstallCiliumHelmChartStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install or upgrade Cilium via Helm chart", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 20 * time.Minute

	s.ReleaseName = "cilium"
	s.Namespace = "kube-system"

	s.AdminKubeconfigPath = filepath.Join(ctx.GetGlobalWorkDir(), "kubeconfigs", common.AdminKubeconfigFileName)
	remoteDir := filepath.Join(common.DefaultUploadTmpDir, "cilium-helm")
	s.RemoteValuesPath = filepath.Join(remoteDir, "cilium-values.yaml")

	b := new(InstallCiliumHelmChartStepBuilder).Init(s)
	return b
}

func (s *InstallCiliumHelmChartStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *InstallCiliumHelmChartStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
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
		return false, fmt.Errorf("required Cilium values file not found at %s, cannot proceed with installation", s.RemoteValuesPath)
	}

	remoteDir := filepath.Dir(s.RemoteValuesPath)
	findCmd := fmt.Sprintf("find %s -name '*.tgz' -print -quit", remoteDir)
	chartPath, err := runner.Run(ctx.GoContext(), conn, findCmd, s.Sudo)
	if err != nil || strings.TrimSpace(chartPath) == "" {
		return false, fmt.Errorf("Cilium Helm chart .tgz file not found in remote directory %s", remoteDir)
	}

	logger.Info("Cilium Helm artifacts (chart and values) found on remote host. Ready to install.")
	return false, nil
}

func (s *InstallCiliumHelmChartStep) Run(ctx runtime.ExecutionContext) error {
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
		return fmt.Errorf("failed to find Cilium Helm chart .tgz file in remote directory %s", remoteDir)
	}
	remoteChartPath = strings.TrimSpace(remoteChartPath)

	cmd := fmt.Sprintf(
		"helm upgrade --install %s %s "+
			"--namespace %s "+
			"--create-namespace "+
			"--values %s "+
			"--kubeconfig %s "+
			"--wait "+
			"--atomic",
		s.ReleaseName,
		remoteChartPath,
		s.Namespace,
		s.RemoteValuesPath,
		s.AdminKubeconfigPath,
	)

	logger.Infof("Executing remote Helm command: %s", cmd)
	output, err := runner.Run(ctx.GoContext(), conn, cmd, s.Sudo)
	if err != nil {
		return fmt.Errorf("failed to install/upgrade Cilium Helm chart: %w\nOutput: %s", err, output)
	}

	logger.Info("Cilium Helm chart installed/upgraded successfully.")
	logger.Debugf("Helm command output:\n%s", output)
	return nil
}

func (s *InstallCiliumHelmChartStep) Rollback(ctx runtime.ExecutionContext) error {
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
		logger.Errorf("Failed to uninstall Cilium Helm release (this may be expected if installation failed): %v", err)
	} else {
		logger.Info("Successfully executed Helm uninstall command for Cilium.")
	}

	return nil
}

var _ step.Step = (*InstallCiliumHelmChartStep)(nil)
