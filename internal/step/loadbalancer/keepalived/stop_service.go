package keepalived

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

// StopKeepalivedService stops Keepalived service
type StopKeepalivedService struct {
	step.Base
}

type StopKeepalivedStepBuilder struct {
	step.Builder[StopKeepalivedStepBuilder, *StopKeepalivedService]
}

func NewStopKeepalivedStepBuilder(ctx runtime.ExecutionContext, instanceName string) *StopKeepalivedStepBuilder {
	s := &StopKeepalivedService{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Stop Keepalived service", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.Timeout = 1 * time.Minute

	b := new(StopKeepalivedStepBuilder).Init(s)
	return b
}

func (s *StopKeepalivedService) Meta() *spec.StepMeta { return &s.Base.Meta }
func (s *StopKeepalivedService) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil
}

func (s *StopKeepalivedService) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	facts, _ := ctx.GetHostFacts(ctx.GetHost())
	conn, _ := ctx.GetCurrentHostConnector()
	cmd := fmt.Sprintf(facts.InitSystem.StopCmd, "keepalived")
	_, _, err := conn.Exec(ctx.GoContext(), cmd, nil)
	if err != nil {
		result.MarkFailed(err, "failed to stop keepalived service")
		return result, err
	}
	result.MarkCompleted("Keepalived service stopped successfully")
	return result, nil
}

func (s *StopKeepalivedService) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

var _ step.Step = (*StopKeepalivedService)(nil)