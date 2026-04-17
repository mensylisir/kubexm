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

type EnableRegistryServiceStep struct {
	step.Base
}

type EnableRegistryServiceStepBuilder struct {
	step.Builder[EnableRegistryServiceStepBuilder, *EnableRegistryServiceStep]
}

func NewEnableRegistryServiceStepBuilder(ctx runtime.ExecutionContext, instanceName string) *EnableRegistryServiceStepBuilder {
	s := &EnableRegistryServiceStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Enable registry systemd service", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.Timeout = 1 * time.Minute
	b := new(EnableRegistryServiceStepBuilder).Init(s)
	return b
}

func (s *EnableRegistryServiceStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *EnableRegistryServiceStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}
	runResult, err := runner.Run(ctx.GoContext(), conn, "systemctl is-enabled registry.service", s.Sudo)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(runResult.Stdout) == "enabled", nil
}

func (s *EnableRegistryServiceStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector")
		return result, err
	}
	if _, err := runner.Run(ctx.GoContext(), conn, "systemctl daemon-reload", s.Sudo); err != nil {
		result.MarkFailed(err, "failed to run daemon-reload")
		return result, err
	}
	if _, err := runner.Run(ctx.GoContext(), conn, "systemctl enable registry.service", s.Sudo); err != nil {
		result.MarkFailed(err, "failed to enable registry service")
		return result, err
	}
	result.MarkCompleted("registry service enabled successfully")
	return result, nil
}

func (s *EnableRegistryServiceStep) Rollback(ctx runtime.ExecutionContext) error {
	runner := ctx.GetRunner()
	conn, _ := ctx.GetCurrentHostConnector()
	_, _ = runner.Run(ctx.GoContext(), conn, "systemctl disable registry.service", s.Sudo)
	return nil
}

var _ step.Step = (*EnableRegistryServiceStep)(nil)
