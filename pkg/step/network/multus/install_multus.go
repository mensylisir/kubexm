package multus

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

type InstallMultusHelmChartStep struct {
	step.Base
	ReleaseName         string
	Namespace           string
	RemoteChartPath     string
	RemoteValuesPath    string
	AdminKubeconfigPath string
}

type InstallMultusHelmChartStepBuilder struct {
	step.Builder[InstallMultusHelmChartStepBuilder, *InstallMultusHelmChartStep]
}

func NewInstallMultusHelmChartStepBuilder(ctx runtime.Context, instanceName string) *InstallMultusHelmChartStepBuilder {
	s := &InstallMultusHelmChartStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install or upgrade Multus CNI via Helm chart", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 15 * time.Minute

	s.ReleaseName = "multus"
	s.Namespace = "kube-system"

	clusterCfg := ctx.GetClusterConfig()
	if clusterCfg.Spec.Network.Multus != nil &&
		clusterCfg.Spec.Network.Multus.Installation != nil &&
		clusterCfg.Spec.Network.Multus.Installation.Namespace != "" {
		s.Namespace = clusterCfg.Spec.Network.Multus.Installation.Namespace
	}

	s.AdminKubeconfigPath = filepath.Join(ctx.GetGlobalWorkDir(), "kubeconfigs", common.AdminKubeconfigFileName)

	remoteDir := filepath.Join(common.DefaultUploadTmpDir, "multus")
	s.RemoteValuesPath = filepath.Join(remoteDir, "multus-values.yaml")

	b := new(InstallMultusHelmChartStepBuilder).Init(s)
	return b
}

func (s *InstallMultusHelmChartStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *InstallMultusHelmChartStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
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
		return false, fmt.Errorf("required Multus CNI values file not found at %s, cannot proceed", s.RemoteValuesPath)
	}

	remoteDir := filepath.Dir(s.RemoteValuesPath)
	findCmd := fmt.Sprintf("find %s -name '*.tgz' -print -quit", remoteDir)
	chartPath, err := runner.Run(ctx.GoContext(), conn, findCmd, s.Sudo)
	if err != nil || strings.TrimSpace(chartPath) == "" {
		return false, fmt.Errorf("Multus CNI Helm chart .tgz file not found in remote directory %s", remoteDir)
	}

	logger.Info("Helm artifacts (chart and values) for Multus CNI found on remote host. Ready to install.")
	return false, nil
}

func (s *InstallMultusHelmChartStep) Run(ctx runtime.ExecutionContext) error {
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
		return fmt.Errorf("failed to find Multus CNI Helm chart .tgz file in remote directory %s", remoteDir)
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

	logger.Infof("Executing remote Helm command for Multus CNI: %s", cmd)
	output, err := runner.Run(ctx.GoContext(), conn, cmd, s.Sudo)
	if err != nil {
		return fmt.Errorf("failed to install/upgrade Multus CNI Helm chart: %w\nOutput: %s", err, output)
	}

	logger.Info("Multus CNI Helm chart installed/upgraded successfully.")
	logger.Debugf("Helm command output:\n%s", output)
	return nil
}

func (s *InstallMultusHelmChartStep) Rollback(ctx runtime.ExecutionContext) error {
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
		logger.Errorf("Failed to uninstall Multus CNI Helm release (this may be expected if installation failed): %v", err)
	} else {
		logger.Info("Successfully executed Helm uninstall command for Multus CNI.")
	}

	return nil
}

var _ step.Step = (*InstallMultusHelmChartStep)(nil)
