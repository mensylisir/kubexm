package keepalived

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// DisableKeepalivedService disables Keepalived service
type DisableKeepalivedService struct {
	step.Base
}

func NewDisableKeepalivedService(ctx runtime.Context, name string) *DisableKeepalivedService {
	s := &DisableKeepalivedService{}
	s.Base.Meta.Name = name
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Disable Keepalived service", name)
	s.Base.Sudo = true
	s.Base.Timeout = 1 * time.Minute
	return s
}

func (s *DisableKeepalivedService) Meta() *spec.StepMeta { return &s.Base.Meta }
func (s *DisableKeepalivedService) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil
}

func (s *DisableKeepalivedService) Run(ctx runtime.ExecutionContext) error {
	facts, _ := ctx.GetHostFacts(ctx.GetHost())
	conn, _ := ctx.GetCurrentHostConnector()
	cmd := fmt.Sprintf(facts.InitSystem.DisableCmd, "keepalived")
	_, _, err := conn.Exec(ctx.GoContext(), cmd, nil)
	return err
}

func (s *DisableKeepalivedService) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}
