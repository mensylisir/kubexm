package common

import (
	"fmt"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type InstallPackagesStep struct {
	step.Base
	Packages          []string
	packagesToInstall []string
	RemoveOnRollback  bool
}

func (b *InstallPackagesStepBuilder) WithRemoveOnRollback(remove bool) *InstallPackagesStepBuilder {
	b.Step.RemoveOnRollback = remove
	return b
}

type InstallPackagesStepBuilder struct {
	step.Builder[InstallPackagesStepBuilder, *InstallPackagesStep]
}

func NewInstallPackagesStepBuilder(ctx runtime.ExecutionContext, instanceName string, packages []string) *InstallPackagesStepBuilder {
	cs := &InstallPackagesStep{
		Packages: packages,
	}
	cs.Base.Meta.Name = instanceName
	cs.Base.Meta.Description = fmt.Sprintf("[%s]>>Install package [%s]", instanceName, strings.Join(packages, ","))
	cs.Base.Sudo = false
	cs.Base.IgnoreError = false
	cs.Base.Timeout = 30 * time.Second
	return new(InstallPackagesStepBuilder).Init(cs)
}

func (s *InstallPackagesStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *InstallPackagesStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", ctx.GetHost().GetName(), err)
	}
	facts, err := ctx.GetHostFacts(ctx.GetHost())
	if err != nil {
		return false, fmt.Errorf("failed to get host facts for %s: %w", ctx.GetHost().GetName(), err)
	}
	s.packagesToInstall = []string{}

	for _, pkg := range s.Packages {
		installed, err := runnerSvc.IsPackageInstalled(ctx.GoContext(), conn, facts, pkg)
		if err != nil {
			return false, fmt.Errorf("failed to check if package %s is installed on %s: %w", pkg, ctx.GetHost().GetName(), err)
		}
		if !installed {
			s.packagesToInstall = append(s.packagesToInstall, pkg)
		}
	}

	if len(s.packagesToInstall) == 0 {
		logger.Info("All packages already installed.")
		return true, nil
	}

	logger.Info("Some packages need to be installed.", "packages", s.packagesToInstall)
	return false, nil
}

func (s *InstallPackagesStep) Run(ctx runtime.ExecutionContext) (*step.StepResult, error) {
	result := step.NewStepResult(s.Meta().Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	if len(s.packagesToInstall) == 0 {
		logger.Info("No new packages to install.")
		result.MarkSkipped("No new packages to install")
		return result, nil
	}
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil, fmt.Errorf("failed to get connector for host %s: %w", ctx.GetHost().GetName(), err)
	}
	facts, err := ctx.GetHostFacts(ctx.GetHost())
	if err != nil {
		return nil, fmt.Errorf("failed to get host facts for %s: %w", ctx.GetHost().GetName(), err)
	}

	logger.Info("Installing packages.", "packages", s.packagesToInstall)
	err = runnerSvc.InstallPackages(ctx.GoContext(), conn, facts, s.packagesToInstall...)
	if err != nil {
		return nil, fmt.Errorf("failed to install packages %v on %s: %w", s.packagesToInstall, ctx.GetHost().GetName(), err)
	}
	result.MarkCompleted(fmt.Sprintf("Installed packages: %v", s.packagesToInstall))
	return result, nil
}

func (s *InstallPackagesStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	if !s.RemoveOnRollback {
		logger.Info("Rollback for InstallPackagesStep is disabled by configuration. No action taken.")
		return nil
	}
	logger.Info("Attempting to remove packages for rollback.", "packages", s.Packages)

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return fmt.Errorf("rollback: failed to get connector for host %s: %w", ctx.GetHost().GetName(), err)
	}
	facts, err := ctx.GetHostFacts(ctx.GetHost())
	if err != nil {
		logger.Error(err, "Rollback: failed to get host facts, cannot safely remove packages.")
		return nil
	}

	err = runnerSvc.RemovePackages(ctx.GoContext(), conn, facts, s.Packages...)
	if err != nil {
		logger.Error(err, "Failed to remove packages during rollback.", "packages", s.Packages)
	} else {
		logger.Info("Successfully removed packages for rollback.", "packages", s.Packages)
	}

	return nil
}

var _ step.Step = (*InstallPackagesStep)(nil)
