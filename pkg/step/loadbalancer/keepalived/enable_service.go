package keepalived

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// EnableKeepalivedService enables Keepalived service
type EnableKeepalivedService struct {
	step.Base
}

func NewEnableKeepalivedService(ctx runtime.Context, name string) *EnableKeepalivedService {
	s := &EnableKeepalivedService{}
	s.Base.Meta.Name = name
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Enable Keepalived service", name)
	s.Base.Sudo = true
	s.Base.Timeout = 1 * time.Minute
	return s
}

func (s *EnableKeepalivedService) Meta() *spec.StepMeta { return &s.Base.Meta }
func (s *EnableKeepalivedService) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil
}

func (s *EnableKeepalivedService) Run(ctx runtime.ExecutionContext) error {
	facts, _ := ctx.GetHostFacts(ctx.GetHost())
	conn, _ := ctx.GetCurrentHostConnector()
	cmd := fmt.Sprintf(facts.InitSystem.EnableCmd, "keepalived")
	_, _, err := conn.Exec(ctx.GoContext(), cmd, nil)
	return err
}

func (s *EnableKeepalivedService) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}
