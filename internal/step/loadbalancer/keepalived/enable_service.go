package keepalived

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runner"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

// EnableKeepalivedService enables Keepalived service
type EnableKeepalivedService struct {
	step.Base
}

type EnableKeepalivedStepBuilder struct {
	step.Builder[EnableKeepalivedStepBuilder, *EnableKeepalivedService]
}

func NewEnableKeepalivedStepBuilder(ctx runtime.ExecutionContext, instanceName string) *EnableKeepalivedStepBuilder {
	s := &EnableKeepalivedService{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Enable Keepalived service", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.Timeout = 1 * time.Minute

	b := new(EnableKeepalivedStepBuilder).Init(s)
	return b
}

func (s *EnableKeepalivedService) Meta() *spec.StepMeta { return &s.Base.Meta }
func (s *EnableKeepalivedService) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil
}

func (s *EnableKeepalivedService) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, fmt.Sprintf("failed to get connector for host %s", ctx.GetHost().GetName()))
		return result, err
	}
	facts, err := ctx.GetHostFacts(ctx.GetHost())
	if err != nil {
		result.MarkFailed(err, "failed to get host facts")
		return result, err
	}
	cmd := fmt.Sprintf(facts.InitSystem.EnableCmd, "keepalived")
	_, stderr, err := runnerSvc.RunWithOptions(ctx.GoContext(), conn, cmd, &runner.ExecOptions{Sudo: s.Base.Sudo})
	if err != nil {
		result.MarkFailed(err, fmt.Sprintf("failed to enable keepalived service: %s", string(stderr)))
		return result, err
	}
	result.MarkCompleted("Keepalived service enabled successfully")
	return result, nil
}

func (s *EnableKeepalivedService) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

var _ step.Step = (*EnableKeepalivedService)(nil)