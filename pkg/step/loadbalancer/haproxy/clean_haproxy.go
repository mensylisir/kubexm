package haproxy

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/common"
	rn "github.com/mensylisir/kubexm/pkg/runner"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"time"
)

type CleanHAProxyStep struct {
	step.Base
}

type CleanHAProxyStepBuilder struct {
	step.Builder[CleanHAProxyStepBuilder, *CleanHAProxyStep]
}

func NewCleanHAProxyStepBuilder(ctx runtime.Context, instanceName string) *CleanHAProxyStepBuilder {
	s := &CleanHAProxyStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Clean HAProxy configuration and uninstall package", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute
	b := new(CleanHAProxyStepBuilder).Init(s)
	return b
}

func (s *CleanHAProxyStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *CleanHAProxyStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	return false, nil
}

func (s *CleanHAProxyStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	remoteConfigDir := common.HAProxyDefaultConfDirTarget
	logger.Infof("Removing HAProxy config directory: %s", remoteConfigDir)
	if err := runner.Remove(ctx.GoContext(), conn, remoteConfigDir, s.Sudo, true); err != nil {
		logger.Warnf("Failed to remove config directory %s (may not exist): %v", remoteConfigDir, err)
	}

	facts, err := ctx.GetHostFacts(ctx.GetHost())
	if err != nil {
		return fmt.Errorf("failed to gather facts before installation: %w", err)
	}

	if facts.PackageManager == nil || facts.PackageManager.Type == rn.PackageManagerUnknown {
		return fmt.Errorf("could not determine a valid package manager for host %s", ctx.GetHost().GetName())
	}
	pkgManager := facts.PackageManager

	logger.Infof("Uninstalling haproxy package...")
	packageName := "haproxy"
	removeCMD := fmt.Sprintf(pkgManager.RemoveCmd, packageName)
	if _, err := runner.Run(ctx.GoContext(), conn, removeCMD, s.Sudo); err != nil {
		return fmt.Errorf("failed to remove %s: %w", packageName, err)
	}

	logger.Infof("HAProxy cleanup finished on %s.", ctx.GetHost().GetName())
	return nil
}

func (s *CleanHAProxyStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name)
	logger.Info("Rollback is not applicable for a cleanup step.")
	return nil
}

var _ step.Step = (*CleanHAProxyStep)(nil)
