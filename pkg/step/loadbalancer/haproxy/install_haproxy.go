package haproxy

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/runner"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type InstallHAProxyStep struct {
	step.Base
}
type InstallHAProxyStepBuilder struct {
	step.Builder[InstallHAProxyStepBuilder, *InstallHAProxyStep]
}

func NewInstallHAProxyStepBuilder(ctx runtime.Context, instanceName string) *InstallHAProxyStepBuilder {
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

func (s *InstallHAProxyStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	facts, err := ctx.GetHostFacts(ctx.GetHost())
	if err != nil {
		return fmt.Errorf("failed to gather facts before installation: %w", err)
	}

	if facts.PackageManager == nil || facts.PackageManager.Type == runner.PackageManagerUnknown {
		return fmt.Errorf("could not determine a valid package manager for host %s", ctx.GetHost().GetName())
	}
	pkgManager := facts.PackageManager

	packageName := "haproxy"
	installCmd := fmt.Sprintf(pkgManager.InstallCmd, packageName)

	if pkgManager.UpdateCmd != "" {
		logger.Infof("Executing package manager update command: %s", pkgManager.UpdateCmd)
		if _, err := runnerSvc.Run(ctx.GoContext(), conn, pkgManager.UpdateCmd, s.Sudo); err != nil {
			return fmt.Errorf("package manager update command failed: %w", err)
		}
	}

	logger.Infof("Executing installation command: %s", installCmd)
	if _, err := runnerSvc.Run(ctx.GoContext(), conn, installCmd, s.Sudo); err != nil {
		return fmt.Errorf("failed to install %s: %w", packageName, err)
	}
	logger.Infof("'%s' installed successfully.", packageName)

	return nil
}

func (s *InstallHAProxyStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Rollback for InstallHaproxyStep: uninstalling is not implemented by default. Skipping.")
	return nil
}

var _ step.Step = (*InstallHAProxyStep)(nil)
