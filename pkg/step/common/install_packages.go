package common

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec" // Required for spec.StepMeta
	"github.com/mensylisir/kubexm/pkg/step"  // Required for step.Step interface
)

// InstallPackagesStep defines a step to install one or more packages.
type InstallPackagesStep struct {
	Packages []string
	meta     spec.StepMeta
}

// NewInstallPackagesStep creates a new InstallPackagesStep.
// instanceName is a specific name for this instance of the step, e.g., "Install Nginx Package".
// If instanceName is empty, a default name will be generated.
func NewInstallPackagesStep(packages []string, instanceName string) step.Step { // Return type is step.Step
	metaName := instanceName
	if metaName == "" {
		metaName = "InstallPackages"
	}
	return &InstallPackagesStep{
		Packages: packages,
		meta: spec.StepMeta{
			Name:        metaName,
			Description: fmt.Sprintf("Install packages: %v", packages),
		},
	}
}

// Meta returns the step's metadata.
func (s *InstallPackagesStep) Meta() *spec.StepMeta {
	return &s.meta
}

// Rollback for InstallPackagesStep might attempt to remove the packages.
// For this example, it's a no-op.
func (s *InstallPackagesStep) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Rollback")
	logger.Info("Attempting rollback (no-op for this version).", "packages", s.Packages)
	// Example of actual rollback:
	// runnerSvc := ctx.GetRunner()
	// conn, err := ctx.GetConnectorForHost(host)
	// if err != nil { return err }
	// facts, err := ctx.GetHostFacts(host)
	// if err != nil { return err }
	// return runnerSvc.RemovePackages(ctx.GoContext(), conn, facts, s.Packages...)
	return nil
}

// Precheck verifies if all specified packages are already installed.
func (s *InstallPackagesStep) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Precheck")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}
	facts, err := ctx.GetHostFacts(host)
	if err != nil {
		return false, fmt.Errorf("failed to get host facts for %s: %w", host.GetName(), err)
	}

	for _, pkg := range s.Packages {
		installed, err := runnerSvc.IsPackageInstalled(ctx.GoContext(), conn, facts, pkg)
		if err != nil {
			return false, fmt.Errorf("failed to check if package %s is installed on %s: %w", pkg, host.GetName(), err)
		}
		if !installed {
			logger.Info("Package not installed.", "package", pkg)
			return false, nil // At least one package is not installed
		}
		logger.V(1).Info("Package already installed.", "package", pkg)
	}
	logger.Info("All packages already installed.", "packages", s.Packages)
	return true, nil // All packages are already installed
}

// Run executes the package installation.
func (s *InstallPackagesStep) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Run")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}
	facts, err := ctx.GetHostFacts(host)
	if err != nil {
		return fmt.Errorf("failed to get host facts for %s: %w", host.GetName(), err)
	}

	logger.Info("Installing packages.", "packages", s.Packages)
	err = runnerSvc.InstallPackages(ctx.GoContext(), conn, facts, s.Packages...)
	if err != nil {
		return fmt.Errorf("failed to install packages %v on %s: %w", s.Packages, host.GetName(), err)
	}
	return nil
}

// Ensure InstallPackagesStep implements the Step interface.
var _ step.Step = (*InstallPackagesStep)(nil) // Interface check uses step.Step
