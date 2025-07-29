package registry

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"strings"
	"time"
)

// DisableRegistryServiceStep 是一个无状态的节点执行步骤。
type DisableRegistryServiceStep struct {
	step.Base
}

type DisableRegistryServiceStepBuilder struct {
	step.Builder[DisableRegistryServiceStepBuilder, *DisableRegistryServiceStep]
}

func NewDisableRegistryServiceStepBuilder(ctx runtime.Context, instanceName string) *DisableRegistryServiceStepBuilder {
	s := &DisableRegistryServiceStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Disable registry systemd service", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.Timeout = 1 * time.Minute

	b := new(DisableRegistryServiceStepBuilder).Init(s)
	return b
}

func (s *DisableRegistryServiceStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DisableRegistryServiceStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}
	output, err := runner.Run(ctx.GoContext(), conn, "systemctl is-enabled registry.service", s.Sudo)
	return err != nil || strings.TrimSpace(output) != "enabled", nil
}

func (s *DisableRegistryServiceStep) Run(ctx runtime.ExecutionContext) error {
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}
	if _, err := runner.Run(ctx.GoContext(), conn, "systemctl disable registry.service", s.Sudo); err != nil {
		// 忽略服务不存在的错误
	}
	return nil
}

func (s *DisableRegistryServiceStep) Rollback(ctx runtime.ExecutionContext) error {
	runner := ctx.GetRunner()
	conn, _ := ctx.GetCurrentHostConnector()
	_, _ = runner.Run(ctx.GoContext(), conn, "systemctl enable registry.service", s.Sudo)
	return nil
}

var _ step.Step = (*DisableRegistryServiceStep)(nil)
