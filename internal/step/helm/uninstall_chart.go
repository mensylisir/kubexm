package helm

import (
	"fmt"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

type UninstallChartStep struct {
	step.Base
	ReleaseName string
	Namespace   string
}

type UninstallChartStepBuilder struct {
	step.Builder[UninstallChartStepBuilder, *UninstallChartStep]
}

func NewUninstallChartStepBuilder(ctx runtime.ExecutionContext, instanceName string) *UninstallChartStepBuilder {
	s := &UninstallChartStep{}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Uninstall helm chart", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	b := new(UninstallChartStepBuilder).Init(s)
	return b
}

func (b *UninstallChartStepBuilder) WithReleaseName(releaseName string) *UninstallChartStepBuilder {
	b.Step.ReleaseName = releaseName
	return b
}

func (b *UninstallChartStepBuilder) WithNamespace(namespace string) *UninstallChartStepBuilder {
	b.Step.Namespace = namespace
	return b
}

func (s *UninstallChartStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *UninstallChartStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	return false, nil
}

func (s *UninstallChartStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector")
		return result, err
	}

	cmd := "helm"
	args := []string{"uninstall", s.ReleaseName}
	if s.Namespace != "" {
		args = append(args, "--namespace", s.Namespace)
	}

	logger.Infof("Uninstalling helm chart with release name %s", s.ReleaseName)
	_, _, err = runner.OriginRun(ctx.GoContext(), conn, fmt.Sprintf("%s %s", cmd, strings.Join(args, " ")), s.Sudo)
	if err != nil {
		err := fmt.Errorf("failed to uninstall helm chart: %w", err)
		result.MarkFailed(err, "failed to uninstall helm chart")
		return result, err
	}

	result.MarkCompleted("helm chart uninstalled successfully")
	return result, nil
}

func (s *UninstallChartStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Rollback for UninstallChartStep is not applicable.")
	return nil
}
