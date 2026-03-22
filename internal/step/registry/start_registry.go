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

type StartRegistryServiceStep struct {
	step.Base
}

type StartRegistryServiceStepBuilder struct {
	step.Builder[StartRegistryServiceStepBuilder, *StartRegistryServiceStep]
}

func NewStartRegistryServiceStepBuilder(ctx runtime.ExecutionContext, instanceName string) *StartRegistryServiceStepBuilder {
	s := &StartRegistryServiceStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Start registry systemd service", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.Timeout = 1 * time.Minute
	b := new(StartRegistryServiceStepBuilder).Init(s)
	return b
}

func (s *StartRegistryServiceStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *StartRegistryServiceStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}
	output, err := runner.Run(ctx.GoContext(), conn, "systemctl is-active registry.service", s.Sudo)
	return err == nil && strings.TrimSpace(output) == "active", nil
}

func (s *StartRegistryServiceStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector")
		return result, err
	}
	if _, err := runner.Run(ctx.GoContext(), conn, "systemctl restart registry.service", s.Sudo); err != nil {
		result.MarkFailed(err, "failed to start registry service")
		return result, err
	}
	result.MarkCompleted("registry service started successfully")
	return result, nil
}

func (s *StartRegistryServiceStep) Rollback(ctx runtime.ExecutionContext) error {
	runner := ctx.GetRunner()
	conn, _ := ctx.GetCurrentHostConnector()
	_, _ = runner.Run(ctx.GoContext(), conn, "systemctl stop registry.service", s.Sudo)
	return nil
}

var _ step.Step = (*StartRegistryServiceStep)(nil)
