package keepalived

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// StopKeepalivedService stops Keepalived service
type StopKeepalivedService struct {
	step.Base
}

func NewStopKeepalivedService(ctx runtime.Context, name string) *StopKeepalivedService {
	s := &StopKeepalivedService{}
	s.Base.Meta.Name = name
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Stop Keepalived service", name)
	s.Base.Sudo = true
	s.Base.Timeout = 1 * time.Minute
	return s
}

func (s *StopKeepalivedService) Meta() *spec.StepMeta { return &s.Base.Meta }
func (s *StopKeepalivedService) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil
}

func (s *StopKeepalivedService) Run(ctx runtime.ExecutionContext) error {
	facts, _ := ctx.GetHostFacts(ctx.GetHost())
	conn, _ := ctx.GetCurrentHostConnector()
	cmd := fmt.Sprintf(facts.InitSystem.StopCmd, "keepalived")
	_, _, err := conn.Exec(ctx.GoContext(), cmd, nil)
	return err
}

func (s *StopKeepalivedService) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}
