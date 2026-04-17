package common

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

// HelmStatusOutput represents helm status JSON result.
type HelmStatusOutput struct {
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

// InstallHelmChartStep installs or upgrades a helm chart.
type InstallHelmChartStep struct {
	step.Base
	ChartPath      string
	ReleaseName    string
	Namespace      string
	ValuesPath     string
	KubeconfigPath string
	Wait           bool
	Atomic         bool
}

type InstallHelmChartStepBuilder struct {
	step.Builder[InstallHelmChartStepBuilder, *InstallHelmChartStep]
}

func NewInstallHelmChartStepBuilder(ctx runtime.ExecutionContext, instanceName, chartPath, releaseName, namespace, valuesPath, kubeconfigPath string) *InstallHelmChartStepBuilder {
	s := &InstallHelmChartStep{
		ChartPath:      chartPath,
		ReleaseName:    releaseName,
		Namespace:      namespace,
		ValuesPath:     valuesPath,
		KubeconfigPath: kubeconfigPath,
		Wait:           true,
		Atomic:         true,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install helm chart %s", instanceName, releaseName)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 15 * time.Minute
	return new(InstallHelmChartStepBuilder).Init(s)
}

func (s *InstallHelmChartStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *InstallHelmChartStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil
}

func (s *InstallHelmChartStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector")
		return result, err
	}

	cmd := fmt.Sprintf("helm upgrade --install %s %s --namespace %s --create-namespace --values %s --kubeconfig %s",
		s.ReleaseName, s.ChartPath, s.Namespace, s.ValuesPath, s.KubeconfigPath)
	if s.Wait {
		cmd += " --wait"
	}
	if s.Atomic {
		cmd += " --atomic"
	}

	logger.Infof("Running: %s", cmd)
	if _, err := runner.Run(ctx.GoContext(), conn, cmd, s.Base.Sudo); err != nil {
		result.MarkFailed(err, "helm install/upgrade failed")
		return result, err
	}

	logger.Infof("Helm chart %s installed successfully", s.ReleaseName)
	result.MarkCompleted("Helm chart installed")
	return result, nil
}

func (s *InstallHelmChartStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	cmd := fmt.Sprintf("helm uninstall %s --namespace %s --kubeconfig %s", s.ReleaseName, s.Namespace, s.KubeconfigPath)
	logger.Warnf("Rolling back: %s", cmd)
	_, _ = runner.Run(ctx.GoContext(), conn, cmd, s.Base.Sudo)
	return nil
}

var _ step.Step = (*InstallHelmChartStep)(nil)

// CheckHelmReleaseStep checks if a helm release is deployed.
type CheckHelmReleaseStep struct {
	step.Base
	ReleaseName    string
	Namespace      string
	KubeconfigPath string
}

type CheckHelmReleaseStepBuilder struct {
	step.Builder[CheckHelmReleaseStepBuilder, *CheckHelmReleaseStep]
}

func NewCheckHelmReleaseStepBuilder(ctx runtime.ExecutionContext, instanceName, releaseName, namespace, kubeconfigPath string) *CheckHelmReleaseStepBuilder {
	s := &CheckHelmReleaseStep{
		ReleaseName:    releaseName,
		Namespace:      namespace,
		KubeconfigPath: kubeconfigPath,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Check helm release %s", instanceName, releaseName)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute
	return new(CheckHelmReleaseStepBuilder).Init(s)
}

func (s *CheckHelmReleaseStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *CheckHelmReleaseStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil
}

func (s *CheckHelmReleaseStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector")
		return result, err
	}

	cmd := fmt.Sprintf("helm status %s --namespace %s --kubeconfig %s -o json", s.ReleaseName, s.Namespace, s.KubeconfigPath)
	logger.Infof("Running: %s", cmd)

	runResult, err := runner.Run(ctx.GoContext(), conn, cmd, s.Base.Sudo)
	if err != nil {
		result.MarkFailed(err, "helm release not found")
		return result, err
	}

	var status HelmStatusOutput
	if err := json.Unmarshal([]byte(runResult.Stdout), &status); err != nil {
		result.MarkFailed(err, "failed to parse helm status")
		return result, err
	}

	if status.Info.Status != "deployed" {
		result.MarkFailed(fmt.Errorf("helm release status is %s", status.Info.Status), "not deployed")
		return result, fmt.Errorf("helm release not deployed")
	}

	logger.Infof("Helm release %s is deployed", s.ReleaseName)
	result.MarkCompleted("Helm release is deployed")
	return result, nil
}

func (s *CheckHelmReleaseStep) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

var _ step.Step = (*CheckHelmReleaseStep)(nil)
