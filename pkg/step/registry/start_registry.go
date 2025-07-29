package registry

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"strings"
	"time"
)

type StartRegistryServiceStep struct {
	step.Base
}

type StartRegistryServiceStepBuilder struct {
	step.Builder[StartRegistryServiceStepBuilder, *StartRegistryServiceStep]
}

func NewStartRegistryServiceStepBuilder(ctx runtime.Context, instanceName string) *StartRegistryServiceStepBuilder {
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

func (s *StartRegistryServiceStep) Run(ctx runtime.ExecutionContext) error {
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}
	if _, err := runner.Run(ctx.GoContext(), conn, "systemctl restart registry.service", s.Sudo); err != nil {
		return err
	}
	return nil
}

func (s *StartRegistryServiceStep) Rollback(ctx runtime.ExecutionContext) error {
	runner := ctx.GetRunner()
	conn, _ := ctx.GetCurrentHostConnector()
	_, _ = runner.Run(ctx.GoContext(), conn, "systemctl stop registry.service", s.Sudo)
	return nil
}

var _ step.Step = (*StartRegistryServiceStep)(nil)
