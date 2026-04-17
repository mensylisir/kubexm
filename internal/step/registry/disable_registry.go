package registry

import (
	"fmt"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

type DisableRegistryServiceStep struct {
	step.Base
}

type DisableRegistryServiceStepBuilder struct {
	step.Builder[DisableRegistryServiceStepBuilder, *DisableRegistryServiceStep]
}

func NewDisableRegistryServiceStepBuilder(ctx runtime.ExecutionContext, instanceName string) *DisableRegistryServiceStepBuilder {
	s := &DisableRegistryServiceStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Disable registry systemd service", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.Timeout = 1 * time.Minute

	b := new(DisableRegistryServiceStepBuilder).Init(s)
	return b
}

func (s *DisableRegistryServiceStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DisableRegistryServiceStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}
	runResult, err := runner.Run(ctx.GoContext(), conn, "systemctl is-enabled registry.service", s.Sudo)
	return err != nil || strings.TrimSpace(runResult.Stdout) != "enabled", nil
}

func (s *DisableRegistryServiceStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector")
		return result, err
	}
	if _, err := runner.Run(ctx.GoContext(), conn, "systemctl disable registry.service", s.Sudo); err != nil {
		result.MarkFailed(err, "failed to disable registry service")
		return result, err
	}
	result.MarkCompleted("registry service disabled successfully")
	return result, nil
}

func (s *DisableRegistryServiceStep) Rollback(ctx runtime.ExecutionContext) error {
	runner := ctx.GetRunner()
	conn, _ := ctx.GetCurrentHostConnector()
	_, _ = runner.Run(ctx.GoContext(), conn, "systemctl enable registry.service", s.Sudo)
	return nil
}

var _ step.Step = (*DisableRegistryServiceStep)(nil)
