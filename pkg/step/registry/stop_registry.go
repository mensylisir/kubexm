package registry

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"strings"
	"time"
)

// StopRegistryServiceStep 是一个无状态的节点执行步骤。
type StopRegistryServiceStep struct {
	step.Base
}

type StopRegistryServiceStepBuilder struct {
	step.Builder[StopRegistryServiceStepBuilder, *StopRegistryServiceStep]
}

func NewStopRegistryServiceStepBuilder(ctx runtime.Context, instanceName string) *StopRegistryServiceStepBuilder {
	s := &StopRegistryServiceStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Stop registry systemd service", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.Timeout = 1 * time.Minute

	b := new(StopRegistryServiceStepBuilder).Init(s)
	return b
}

func (s *StopRegistryServiceStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *StopRegistryServiceStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}
	// is-active 返回 inactive 或 failed 都算作已停止
	output, err := runner.Run(ctx.GoContext(), conn, "systemctl is-active registry.service", s.Sudo)
	return err != nil || strings.TrimSpace(output) != "active", nil
}

func (s *StopRegistryServiceStep) Run(ctx runtime.ExecutionContext) error {
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}
	if _, err := runner.Run(ctx.GoContext(), conn, "systemctl stop registry.service", s.Sudo); err != nil {
		// 如果服务不存在，stop 会失败，但我们的目标已经达成，所以可以忽略某些错误
	}
	return nil
}

func (s *StopRegistryServiceStep) Rollback(ctx runtime.ExecutionContext) error {
	// 回滚一个 stop 操作就是 start
	runner := ctx.GetRunner()
	conn, _ := ctx.GetCurrentHostConnector()
	_, _ = runner.Run(ctx.GoContext(), conn, "systemctl start registry.service", s.Sudo)
	return nil
}

var _ step.Step = (*StopRegistryServiceStep)(nil)
