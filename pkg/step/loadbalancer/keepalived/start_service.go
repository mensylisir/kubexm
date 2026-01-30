package keepalived

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// StartKeepalivedService starts Keepalived service
type StartKeepalivedService struct {
	step.Base
}

func NewStartKeepalivedService(ctx runtime.Context, name string) *StartKeepalivedService {
	s := &StartKeepalivedService{}
	s.Base.Meta.Name = name
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Start Keepalived service", name)
	s.Base.Sudo = true
	s.Base.Timeout = 1 * time.Minute
	return s
}

func (s *StartKeepalivedService) Meta() *spec.StepMeta { return &s.Base.Meta }
func (s *StartKeepalivedService) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil
}

func (s *StartKeepalivedService) Run(ctx runtime.ExecutionContext) error {
	facts, _ := ctx.GetHostFacts(ctx.GetHost())
	conn, _ := ctx.GetCurrentHostConnector()
	cmd := fmt.Sprintf(facts.InitSystem.StartCmd, "keepalived")
	_, _, err := conn.Exec(ctx.GoContext(), cmd, nil)
	return err
}

func (s *StartKeepalivedService) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}
