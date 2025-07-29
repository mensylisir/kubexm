package registry

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"strings"
	"time"
)

type EnableRegistryServiceStep struct {
	step.Base
}

type EnableRegistryServiceStepBuilder struct {
	step.Builder[EnableRegistryServiceStepBuilder, *EnableRegistryServiceStep]
}

func NewEnableRegistryServiceStepBuilder(ctx runtime.Context, instanceName string) *EnableRegistryServiceStepBuilder {
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
	output, err := runner.Run(ctx.GoContext(), conn, "systemctl is-enabled registry.service", s.Sudo)
	return err == nil && strings.TrimSpace(output) == "enabled", nil
}

func (s *EnableRegistryServiceStep) Run(ctx runtime.ExecutionContext) error {
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}
	if _, err := runner.Run(ctx.GoContext(), conn, "systemctl daemon-reload", s.Sudo); err != nil {
		return err
	}
	if _, err := runner.Run(ctx.GoContext(), conn, "systemctl enable registry.service", s.Sudo); err != nil {
		return err
	}
	return nil
}

func (s *EnableRegistryServiceStep) Rollback(ctx runtime.ExecutionContext) error {
	runner := ctx.GetRunner()
	conn, _ := ctx.GetCurrentHostConnector()
	_, _ = runner.Run(ctx.GoContext(), conn, "systemctl disable registry.service", s.Sudo)
	return nil
}

var _ step.Step = (*EnableRegistryServiceStep)(nil)
