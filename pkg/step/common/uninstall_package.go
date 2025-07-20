package common

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"strings"
	"time"
)

type UninstallPackageStep struct {
	step.Base
	Packages []string
	Purge    bool
}

type UninstallPackageStepBuilder struct {
	step.Builder[UninstallPackageStepBuilder, *UninstallPackageStep]
}

func NewUninstallPackageStepBuilder(instanceName string, packages []string) *UninstallPackageStepBuilder {
	cs := &UninstallPackageStep{
		Packages: packages,
	}
	cs.Base.Meta.Name = instanceName
	cs.Base.Meta.Description = fmt.Sprintf("[%s]>>Remove package [%s]", instanceName, strings.Join(packages, ","))
	cs.Base.Sudo = false
	cs.Base.IgnoreError = false
	cs.Base.Timeout = 30 * time.Second
	return new(UninstallPackageStepBuilder).Init(cs)
}

func (b *UninstallPackageStepBuilder) WithPurge(purge bool) *UninstallPackageStepBuilder {
	b.Step.Purge = purge
	return b
}

func (s *UninstallPackageStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *UninstallPackageStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
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

	allUninstalled := true
	for _, pkg := range s.Packages {
		installed, err := runnerSvc.IsPackageInstalled(ctx.GoContext(), conn, facts, pkg)
		if err != nil {
			logger.Warn("Failed to check if package is installed, assuming it might be present.", "package", pkg, "error", err)
			return false, nil
		}
		if installed {
			logger.Info("Package still installed.", "package", pkg)
			allUninstalled = false
			break
		}
		logger.Info("Package already uninstalled.", "package", pkg)
	}

	if allUninstalled {
		logger.Info("All specified packages already uninstalled.")
		return true, nil
	}
	return false, nil
}

func (s *UninstallPackageStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", ctx.GetHost().GetName(), err)
	}
	facts, err := ctx.GetHostFacts(ctx.GetHost())
	if err != nil {
		return fmt.Errorf("failed to get host facts for %s: %w", ctx.GetHost().GetName(), err)
	}
	var toRemove []string
	for _, pkg := range s.Packages {
		installed, err := runnerSvc.IsPackageInstalled(ctx.GoContext(), conn, facts, pkg)
		if err != nil {
			return err
		}
		if installed {
			toRemove = append(toRemove, pkg)
		}
	}
	if len(toRemove) == 0 {
		logger.Info("No packages from the list are currently installed. Nothing to do.")
		return nil
	}
	logger.Info("Uninstalling packages.", "packages", toRemove, "purge", s.Purge)
	err = runnerSvc.RemovePackages(ctx.GoContext(), conn, facts, toRemove...)
	if err != nil {
		return fmt.Errorf("failed to uninstall packages %v on %s: %w", toRemove, ctx.GetHost().GetName(), err)
	}
	logger.Info("Packages uninstalled successfully.")
	return nil
}

func (s *UninstallPackageStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Rollback for UninstallPackageStep is not implemented (would involve re-installing).")
	return nil
}

var _ step.Step = (*UninstallPackageStep)(nil)
