package haproxy

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/common"
	rn "github.com/mensylisir/kubexm/internal/runner"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

type CleanHAProxyStep struct {
	step.Base
}

type CleanHAProxyStepBuilder struct {
	step.Builder[CleanHAProxyStepBuilder, *CleanHAProxyStep]
}

func NewCleanHAProxyStepBuilder(ctx runtime.ExecutionContext, instanceName string) *CleanHAProxyStepBuilder {
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

func (s *CleanHAProxyStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector")
		return result, err
	}

	remoteConfigDir := common.HAProxyDefaultConfDirTarget
	logger.Infof("Removing HAProxy config directory: %s", remoteConfigDir)
	if err := runner.Remove(ctx.GoContext(), conn, remoteConfigDir, s.Sudo, true); err != nil {
		logger.Warnf("Failed to remove config directory %s (may not exist): %v", remoteConfigDir, err)
	}

	facts, err := ctx.GetHostFacts(ctx.GetHost())
	if err != nil {
		result.MarkFailed(err, "failed to gather facts before installation")
		return result, err
	}

	if facts.PackageManager == nil || facts.PackageManager.Type == rn.PackageManagerUnknown {
		result.MarkFailed(err, "could not determine a valid package manager")
		return result, err
	}
	pkgManager := facts.PackageManager

	logger.Infof("Uninstalling haproxy package...")
	packageName := "haproxy"
	removeCMD := fmt.Sprintf(pkgManager.RemoveCmd, packageName)
	if _, err := runner.Run(ctx.GoContext(), conn, removeCMD, s.Sudo); err != nil {
		result.MarkFailed(err, "failed to remove haproxy")
		return result, err
	}

	logger.Infof("HAProxy cleanup finished on %s.", ctx.GetHost().GetName())
	result.MarkCompleted("HAProxy cleanup completed successfully")
	return result, nil
}

func (s *CleanHAProxyStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name)
	logger.Info("Rollback is not applicable for a cleanup step.")
	return nil
}

var _ step.Step = (*CleanHAProxyStep)(nil)
