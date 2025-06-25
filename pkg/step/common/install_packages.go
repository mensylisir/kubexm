package common

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/spec" // Required for spec.StepMeta
	"github.com/mensylisir/kubexm/pkg/step" // Required for step.Step interface
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
func (s *InstallPackagesStep) Rollback(ctx step.StepContext, host connector.Host) error {
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
func (s *InstallPackagesStep) Precheck(ctx step.StepContext, host connector.Host) (bool, error) {
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
		logger.Info("Package already installed.", "package", pkg)
	}
	logger.Info("All packages already installed.", "packages", s.Packages)
	return true, nil // All packages are already installed
}

// Run executes the package installation.
func (s *InstallPackagesStep) Run(ctx step.StepContext, host connector.Host) error {
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

// UninstallPackageStep defines a step to uninstall one or more packages.
type UninstallPackageStep struct {
	Packages []string
	Purge    bool // Whether to also remove configuration files (e.g., apt-get purge)
	meta     spec.StepMeta
}

// NewUninstallPackageStep creates a new UninstallPackageStep.
func NewUninstallPackageStep(packages []string, purge bool, instanceName string) step.Step {
	metaName := instanceName
	if metaName == "" {
		metaName = "UninstallPackages"
	}
	return &UninstallPackageStep{
		Packages: packages,
		Purge:    purge,
		meta: spec.StepMeta{
			Name:        metaName,
			Description: fmt.Sprintf("Uninstall packages: %v (Purge: %v)", packages, purge),
		},
	}
}

// Meta returns the step's metadata.
func (s *UninstallPackageStep) Meta() *spec.StepMeta {
	return &s.meta
}

// Precheck verifies if all specified packages are already uninstalled.
func (s *UninstallPackageStep) Precheck(ctx step.StepContext, host connector.Host) (bool, error) {
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

	allUninstalled := true
	for _, pkg := range s.Packages {
		installed, err := runnerSvc.IsPackageInstalled(ctx.GoContext(), conn, facts, pkg)
		if err != nil {
			// If error checking, assume it might be installed and let Run proceed.
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

// Run executes the package uninstallation.
func (s *UninstallPackageStep) Run(ctx step.StepContext, host connector.Host) error {
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

	logger.Info("Uninstalling packages.", "packages", s.Packages, "purge", s.Purge)
	// Runner's RemovePackages method needs to be aware of the Purge option.
	// This might involve passing it as a parameter or the runner constructing
	// the correct command (e.g., `apt-get purge` vs `apt-get remove`).
	// For now, assume runner.RemovePackages can handle it or does a non-purging remove.
	// TODO: Enhance runner.RemovePackages to accept a purge flag.
	err = runnerSvc.RemovePackages(ctx.GoContext(), conn, facts, s.Packages...) // Pass s.Purge if runner method supports it
	if err != nil {
		return fmt.Errorf("failed to uninstall packages %v on %s: %w", s.Packages, host.GetName(), err)
	}
	logger.Info("Packages uninstalled successfully.")
	return nil
}

// Rollback for UninstallPackageStep might attempt to reinstall the packages.
// This is complex as versions and sources might be unknown. For now, no-op.
func (s *UninstallPackageStep) Rollback(ctx step.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Rollback")
	logger.Info("Rollback for UninstallPackageStep is not implemented (would involve re-installing).")
	return nil
}

var _ step.Step = (*UninstallPackageStep)(nil)
