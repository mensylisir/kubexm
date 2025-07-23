package helm

import (
	"fmt"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type UninstallChartStep struct {
	step.Base
	ReleaseName string
	Namespace   string
}

type UninstallChartStepBuilder struct {
	step.Builder[UninstallChartStepBuilder, *UninstallChartStep]
}

func NewUninstallChartStepBuilder(ctx runtime.Context, instanceName string) *UninstallChartStepBuilder {
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

func (s *UninstallChartStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	cmd := "helm"
	args := []string{"uninstall", s.ReleaseName}
	if s.Namespace != "" {
		args = append(args, "--namespace", s.Namespace)
	}

	logger.Infof("Uninstalling helm chart with release name %s", s.ReleaseName)
	_, _, err = runner.OriginRun(ctx.GoContext(), conn, fmt.Sprintf("%s %s", cmd, strings.Join(args, " ")), s.Sudo)
	if err != nil {
		return fmt.Errorf("failed to uninstall helm chart: %w", err)
	}

	return nil
}

func (s *UninstallChartStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Rollback for UninstallChartStep is not applicable.")
	return nil
}
