package registry

import (
	"fmt"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// RestartRegistryServiceStep 是一个无状态的节点执行步骤。
type RestartRegistryServiceStep struct {
	step.Base
}

type RestartRegistryServiceStepBuilder struct {
	step.Builder[RestartRegistryServiceStepBuilder, *RestartRegistryServiceStep]
}

// NewRestartRegistryServiceStepBuilder 创建一个新的 Step 实例。
func NewRestartRegistryServiceStepBuilder(ctx runtime.Context, instanceName string) *RestartRegistryServiceStepBuilder {
	// 这个操作依赖于服务已经存在，所以我们可以在 Builder 中不做检查，
	// 让它依赖于前面的 SetupServiceStep。

	s := &RestartRegistryServiceStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Restart registry systemd service", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.Timeout = 2 * time.Minute

	b := new(RestartRegistryServiceStepBuilder).Init(s)
	return b
}

func (s *RestartRegistryServiceStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

// Precheck 检查服务是否已经是 active 状态。
// 注意：即使服务已经是 active，我们通常也希望执行 restart 来应用新配置。
// 因此，一个更实用的 Precheck 可能是总是返回 false。
// 但为了演示幂等性检查，这里我们先检查 active 状态。
// 如果您的流程是“配置 -> 重启”，那么总是返回 false 更合适。
func (s *RestartRegistryServiceStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	// 为了确保配置被应用，我们总是希望执行 restart。
	// Precheck 在这里的作用不大，总是返回 false 以强制执行 Run。
	return false, nil
}

// Run 执行 restart 操作。
func (s *RestartRegistryServiceStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	// 确保 daemon 被重载，以读取可能更新的 service 文件
	logger.Info("Executing 'systemctl daemon-reload' to ensure service definition is up-to-date.")
	if _, err := runner.Run(ctx.GoContext(), conn, "systemctl daemon-reload", s.Sudo); err != nil {
		return fmt.Errorf("failed to run daemon-reload: %w", err)
	}

	restartCmd := "systemctl restart registry.service"
	logger.Infof("Executing remote command: %s", restartCmd)
	if _, err := runner.Run(ctx.GoContext(), conn, restartCmd, s.Sudo); err != nil {
		// 尝试查看服务状态以获取更多错误信息
		statusCmd := "systemctl status registry.service"
		statusOutput, _ := runner.Run(ctx.GoContext(), conn, statusCmd, s.Sudo)
		journalCmd := "journalctl -u registry.service -n 50 --no-pager"
		journalOutput, _ := runner.Run(ctx.GoContext(), conn, journalCmd, s.Sudo)

		return fmt.Errorf("failed to restart registry service: %w\nStatus:\n%s\nJournal logs:\n%s", err, statusOutput, journalOutput)
	}

	// 增加一个短暂的等待和状态检查，确保服务真正启动成功
	time.Sleep(3 * time.Second)
	statusCmd := "systemctl is-active registry.service"
	output, err := runner.Run(ctx.GoContext(), conn, statusCmd, s.Sudo)
	if err != nil || strings.TrimSpace(output) != "active" {
		journalCmd := "journalctl -u registry.service -n 50 --no-pager"
		journalOutput, _ := runner.Run(ctx.GoContext(), conn, journalCmd, s.Sudo)
		return fmt.Errorf("registry service failed to become active after restart. Journal logs:\n%s", journalOutput)
	}

	logger.Info("Registry service has been restarted successfully and is active.")
	return nil
}

// Rollback 对于一个重启操作，回滚可以是停止服务。
func (s *RestartRegistryServiceStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	logger.Warn("Rolling back by stopping the registry service.")
	_, _ = runner.Run(ctx.GoContext(), conn, "systemctl stop registry.service", s.Sudo)
	return nil
}

var _ step.Step = (*RestartRegistryServiceStep)(nil)
