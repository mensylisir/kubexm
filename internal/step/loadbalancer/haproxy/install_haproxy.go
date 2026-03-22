package haproxy

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runner"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

type InstallHAProxyStep struct {
	step.Base
}
type InstallHAProxyStepBuilder struct {
	step.Builder[InstallHAProxyStepBuilder, *InstallHAProxyStep]
}

func NewInstallHAProxyStepBuilder(ctx runtime.ExecutionContext, instanceName string) *InstallHAProxyStepBuilder {
	s := &InstallHAProxyStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install HAProxy package", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute
	b := new(InstallHAProxyStepBuilder).Init(s)
	return b
}
func (s *InstallHAProxyStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *InstallHAProxyStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	if _, err := runnerSvc.Run(ctx.GoContext(), conn, "which haproxy", false); err != nil {
		logger.Info("HAProxy binary not found. Step needs to run to install it.")
		return false, nil
	}

	logger.Info("HAProxy is already installed.")
	return true, nil
}

func (s *InstallHAProxyStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector")
		return result, err
	}

	facts, err := ctx.GetHostFacts(ctx.GetHost())
	if err != nil {
		result.MarkFailed(err, "failed to gather facts before installation")
		return result, err
	}

	if facts.PackageManager == nil || facts.PackageManager.Type == runner.PackageManagerUnknown {
		result.MarkFailed(err, "could not determine a valid package manager")
		return result, err
	}
	pkgManager := facts.PackageManager

	packageName := "haproxy"
	installCmd := fmt.Sprintf(pkgManager.InstallCmd, packageName)

	if pkgManager.UpdateCmd != "" {
		logger.Infof("Executing package manager update command: %s", pkgManager.UpdateCmd)
		if _, err := runnerSvc.Run(ctx.GoContext(), conn, pkgManager.UpdateCmd, s.Sudo); err != nil {
			result.MarkFailed(err, "package manager update command failed")
			return result, err
		}
	}

	logger.Infof("Executing installation command: %s", installCmd)
	if _, err := runnerSvc.Run(ctx.GoContext(), conn, installCmd, s.Sudo); err != nil {
		result.MarkFailed(err, "failed to install haproxy")
		return result, err
	}
	logger.Infof("'%s' installed successfully.", packageName)

	result.MarkCompleted("HAProxy installed successfully")
	return result, nil
}

func (s *InstallHAProxyStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Rollback for InstallHaproxyStep: uninstalling is not implemented by default. Skipping.")
	return nil
}

var _ step.Step = (*InstallHAProxyStep)(nil)
