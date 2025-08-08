package addon

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1" // 严格使用 v1alpha1
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/pkg/errors"
)

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

type InstallAddonChartStep struct {
	step.Base
	AddonName           string
	SourceIndex         int
	ReleaseName         string
	Namespace           string
	ChartVersion        string
	SetValues           []string
	RemoteChartPath     string
	RemoteValuesPath    string
	AdminKubeconfigPath string
}

type InstallAddonChartStepBuilder struct {
	step.Builder[InstallAddonChartStepBuilder, *InstallAddonChartStep]
}

func NewInstallAddonChartStepBuilder(ctx runtime.Context, addonName string, sourceIndex int) *InstallAddonChartStepBuilder {
	var targetAddon *v1alpha1.Addon
	for i := range ctx.GetClusterConfig().Spec.Addons {
		if ctx.GetClusterConfig().Spec.Addons[i].Name == addonName {
			targetAddon = &ctx.GetClusterConfig().Spec.Addons[i]
			break
		}
	}
	if targetAddon == nil ||
		(targetAddon.Enabled != nil && !*targetAddon.Enabled) ||
		sourceIndex >= len(targetAddon.Sources) ||
		targetAddon.Sources[sourceIndex].Chart == nil {
		return nil
	}

	chartSource := targetAddon.Sources[sourceIndex].Chart
	sourceNamespace := targetAddon.Sources[sourceIndex].Namespace

	s := &InstallAddonChartStep{
		AddonName:    addonName,
		SourceIndex:  sourceIndex,
		ReleaseName:  chartSource.Name,
		Namespace:    sourceNamespace,
		ChartVersion: chartSource.Version,
		SetValues:    chartSource.Values,
	}

	if s.Namespace == "" {
		s.Namespace = targetAddon.Name
	}

	s.Base.Meta.Name = fmt.Sprintf("InstallAddonChart-%s-%d", addonName, sourceIndex)
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install or upgrade Helm chart '%s' for addon '%s'", s.Base.Meta.Name, s.ReleaseName, s.AddonName)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 15 * time.Minute
	if targetAddon.TimeoutSeconds != nil {
		s.Base.Timeout = time.Duration(*targetAddon.TimeoutSeconds) * time.Second
	}

	chartFileName := fmt.Sprintf("%s-%s.tgz", chartSource.Name, chartSource.Version)
	s.RemoteChartPath = filepath.Join(ctx.GetUploadDir(), s.AddonName, chartSource.Version, chartFileName)

	if chartSource.ValuesFile != "" {
		remoteValuesFileName := fmt.Sprintf("%s-values-%d.yaml", s.AddonName, s.SourceIndex)
		s.RemoteValuesPath = filepath.Join(ctx.GetUploadDir(), s.AddonName, chartSource.Version, remoteValuesFileName)
	}

	s.AdminKubeconfigPath = filepath.Join(common.KubernetesConfigDir, common.AdminKubeconfigFileName)

	b := new(InstallAddonChartStepBuilder).Init(s)
	return b
}

func (s *InstallAddonChartStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *InstallAddonChartStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	if _, err := runner.LookPath(ctx.GoContext(), conn, "helm"); err != nil {
		return false, errors.Wrap(err, "helm command not found on remote host")
	}
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
		logger.Warnf("Failed to parse helm status for release '%s', assuming upgrade is needed: %v", s.ReleaseName, err)
		return false, nil
	}

	if status.Info.Status == "deployed" && status.Chart.Metadata.Version == s.ChartVersion {
		logger.Infof("Helm release '%s' version %s is already deployed and up-to-date. Skipping.", s.ReleaseName, s.ChartVersion)
		return true, nil
	}

	logger.Infof("Helm release '%s' requires upgrade (current version: %s, status: %s).", s.ReleaseName, status.Chart.Metadata.Version, status.Info.Status)
	return false, nil
}

func (s *InstallAddonChartStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	var cmdBuilder strings.Builder
	cmdBuilder.WriteString(fmt.Sprintf("helm upgrade --install %s %s ", s.ReleaseName, s.RemoteChartPath))
	cmdBuilder.WriteString(fmt.Sprintf("--namespace %s --create-namespace ", s.Namespace))

	if s.RemoteValuesPath != "" {
		cmdBuilder.WriteString(fmt.Sprintf("--values %s ", s.RemoteValuesPath))
	}

	for _, value := range s.SetValues {
		cmdBuilder.WriteString(fmt.Sprintf("--set %s ", value))
	}

	cmdBuilder.WriteString(fmt.Sprintf("--kubeconfig %s --wait --atomic", s.AdminKubeconfigPath))

	cmd := cmdBuilder.String()

	logger.Infof("Executing remote Helm command: %s", cmd)
	output, err := runner.Run(ctx.GoContext(), conn, cmd, s.Sudo)
	if err != nil {
		return fmt.Errorf("failed to install/upgrade addon helm chart '%s': %w\nOutput: %s", s.ReleaseName, err, output)
	}

	logger.Info("Addon Helm chart installed/upgraded successfully.")
	logger.Debugf("Helm command output:\n%s", output)
	return nil
}

func (s *InstallAddonChartStep) Rollback(ctx runtime.ExecutionContext) error {
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

	logger.Warnf("Rolling back by uninstalling Helm release '%s'...", s.ReleaseName)
	if _, err := runner.Run(ctx.GoContext(), conn, cmd, s.Sudo); err != nil {
		logger.Errorf("Failed to uninstall Helm release: %v", err)
	} else {
		logger.Info("Successfully executed Helm uninstall command.")
	}

	return nil
}

var _ step.Step = (*InstallAddonChartStep)(nil)
