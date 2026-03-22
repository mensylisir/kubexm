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

type StopRegistryServiceStep struct {
	step.Base
}

type StopRegistryServiceStepBuilder struct {
	step.Builder[StopRegistryServiceStepBuilder, *StopRegistryServiceStep]
}

func NewStopRegistryServiceStepBuilder(ctx runtime.ExecutionContext, instanceName string) *StopRegistryServiceStepBuilder {
	s := &StopRegistryServiceStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Stop registry systemd service", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.Timeout = 1 * time.Minute

	b := new(StopRegistryServiceStepBuilder).Init(s)
	return b
}

func (s *StopRegistryServiceStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *StopRegistryServiceStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}
	output, err := runner.Run(ctx.GoContext(), conn, "systemctl is-active registry.service", s.Sudo)
	return err != nil || strings.TrimSpace(output) != "active", nil
}

func (s *StopRegistryServiceStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector")
		return result, err
	}
	if _, err := runner.Run(ctx.GoContext(), conn, "systemctl stop registry.service", s.Sudo); err != nil {
		result.MarkFailed(err, "failed to stop registry service")
		return result, err
	}
	result.MarkCompleted("registry service stopped successfully")
	return result, nil
}

func (s *StopRegistryServiceStep) Rollback(ctx runtime.ExecutionContext) error {
	runner := ctx.GetRunner()
	conn, _ := ctx.GetCurrentHostConnector()
	_, _ = runner.Run(ctx.GoContext(), conn, "systemctl start registry.service", s.Sudo)
	return nil
}

var _ step.Step = (*StopRegistryServiceStep)(nil)
