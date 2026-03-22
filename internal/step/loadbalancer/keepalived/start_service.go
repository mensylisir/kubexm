package keepalived

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

// StartKeepalivedService starts Keepalived service
type StartKeepalivedService struct {
	step.Base
}

type StartKeepalivedStepBuilder struct {
	step.Builder[StartKeepalivedStepBuilder, *StartKeepalivedService]
}

func NewStartKeepalivedStepBuilder(ctx runtime.ExecutionContext, instanceName string) *StartKeepalivedStepBuilder {
	s := &StartKeepalivedService{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Start Keepalived service", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.Timeout = 1 * time.Minute

	b := new(StartKeepalivedStepBuilder).Init(s)
	return b
}

func (s *StartKeepalivedService) Meta() *spec.StepMeta { return &s.Base.Meta }
func (s *StartKeepalivedService) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil
}

func (s *StartKeepalivedService) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	facts, _ := ctx.GetHostFacts(ctx.GetHost())
	conn, _ := ctx.GetCurrentHostConnector()
	cmd := fmt.Sprintf(facts.InitSystem.StartCmd, "keepalived")
	_, _, err := conn.Exec(ctx.GoContext(), cmd, nil)
	if err != nil {
		result.MarkFailed(err, "failed to start keepalived service")
		return result, err
	}
	result.MarkCompleted("Keepalived service started successfully")
	return result, nil
}

func (s *StartKeepalivedService) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

var _ step.Step = (*StartKeepalivedService)(nil)

// RestartKeepalivedService restarts Keepalived service
type RestartKeepalivedService struct {
	step.Base
}

type RestartKeepalivedStepBuilder struct {
	step.Builder[RestartKeepalivedStepBuilder, *RestartKeepalivedService]
}

func NewRestartKeepalivedStepBuilder(ctx runtime.ExecutionContext, instanceName string) *RestartKeepalivedStepBuilder {
	s := &RestartKeepalivedService{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Restart Keepalived service", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.Timeout = 1 * time.Minute

	b := new(RestartKeepalivedStepBuilder).Init(s)
	return b
}

func (s *RestartKeepalivedService) Meta() *spec.StepMeta { return &s.Base.Meta }
func (s *RestartKeepalivedService) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil
}

func (s *RestartKeepalivedService) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	facts, _ := ctx.GetHostFacts(ctx.GetHost())
	conn, _ := ctx.GetCurrentHostConnector()
	cmd := fmt.Sprintf(facts.InitSystem.RestartCmd, "keepalived")
	_, _, err := conn.Exec(ctx.GoContext(), cmd, nil)
	if err != nil {
		result.MarkFailed(err, "failed to restart keepalived service")
		return result, err
	}
	result.MarkCompleted("Keepalived service restarted successfully")
	return result, nil
}

func (s *RestartKeepalivedService) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

var _ step.Step = (*RestartKeepalivedService)(nil)