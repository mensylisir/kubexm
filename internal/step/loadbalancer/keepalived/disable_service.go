package keepalived

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

// DisableKeepalivedService disables Keepalived service
type DisableKeepalivedService struct {
	step.Base
}

type DisableKeepalivedStepBuilder struct {
	step.Builder[DisableKeepalivedStepBuilder, *DisableKeepalivedService]
}

func NewDisableKeepalivedStepBuilder(ctx runtime.ExecutionContext, instanceName string) *DisableKeepalivedStepBuilder {
	s := &DisableKeepalivedService{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Disable Keepalived service", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.Timeout = 1 * time.Minute

	b := new(DisableKeepalivedStepBuilder).Init(s)
	return b
}

func (s *DisableKeepalivedService) Meta() *spec.StepMeta { return &s.Base.Meta }
func (s *DisableKeepalivedService) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil
}

func (s *DisableKeepalivedService) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	facts, _ := ctx.GetHostFacts(ctx.GetHost())
	conn, _ := ctx.GetCurrentHostConnector()
	cmd := fmt.Sprintf(facts.InitSystem.DisableCmd, "keepalived")
	_, _, err := conn.Exec(ctx.GoContext(), cmd, nil)
	if err != nil {
		result.MarkFailed(err, "failed to disable keepalived service")
		return result, err
	}
	result.MarkCompleted("Keepalived service disabled successfully")
	return result, nil
}

func (s *DisableKeepalivedService) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

var _ step.Step = (*DisableKeepalivedService)(nil)