package flannel

import (
	"fmt"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/helm"
)

type UninstallFlannelHelmChartStep struct {
	step.Base
	ReleaseName string
	Namespace   string
}

type UninstallFlannelHelmChartStepBuilder struct {
	step.Builder[UninstallFlannelHelmChartStepBuilder, *UninstallFlannelHelmChartStep]
}

func NewUninstallFlannelHelmChartStepBuilder(ctx runtime.Context, instanceName string) *UninstallFlannelHelmChartStepBuilder {
	helmProvider := helm.NewHelmProvider(&ctx)
	flannelChart := helmProvider.GetChart("flannel")
	if flannelChart == nil {
		b := new(UninstallFlannelHelmChartStepBuilder).Init(&UninstallFlannelHelmChartStep{})
		b.Error = fmt.Errorf("flannel chart information not found in BOM")
		return b
	}

	s := &UninstallFlannelHelmChartStep{
		ReleaseName: flannelChart.ChartName(),
		Namespace:   "kube-system",
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Uninstall Flannel Helm release", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 10 * time.Minute

	b := new(UninstallFlannelHelmChartStepBuilder).Init(s)
	return b
}

func (s *UninstallFlannelHelmChartStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *UninstallFlannelHelmChartStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	checkCmd := fmt.Sprintf("helm status %s -n %s", s.ReleaseName, s.Namespace)
	output, err := runner.Run(ctx.GoContext(), conn, checkCmd, s.Sudo)
	if err != nil {
		if strings.Contains(strings.ToLower(output), "release: not found") || strings.Contains(strings.ToLower(err.Error()), "release: not found") {
			logger.Info("Flannel Helm release not found. Step is done.")
			return true, nil
		}
		logger.Warnf("Failed to check Helm status, assuming cleanup is required. Error: %v", err)
		return false, nil
	}

	logger.Info("Flannel Helm release found. Cleanup is required.")
	return false, nil
}

func (s *UninstallFlannelHelmChartStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	uninstallCmd := fmt.Sprintf("helm uninstall %s -n %s --wait --timeout 5m", s.ReleaseName, s.Namespace)
	logger.Infof("Uninstalling Flannel Helm release with command: %s", uninstallCmd)
	if _, err := runner.Run(ctx.GoContext(), conn, uninstallCmd, s.Sudo); err != nil {
		if !strings.Contains(strings.ToLower(err.Error()), "release: not found") {
			logger.Warnf("Helm uninstall command failed (this may be ok): %v", err)
		}
	}

	logger.Info("Flannel Helm release uninstalled.")
	return nil
}

func (s *UninstallFlannelHelmChartStep) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

var _ step.Step = (*UninstallFlannelHelmChartStep)(nil)
