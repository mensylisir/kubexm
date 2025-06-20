package common // Changed package name

import (
	"fmt"
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec" // Added for spec.StepMeta
	"github.com/mensylisir/kubexm/pkg/step" // For step.Step interface
)

// InstallPackagesStepSpec defines parameters for installing system packages.
type InstallPackagesStepSpec struct {
	spec.StepMeta `json:",inline"`
	Packages      []string `json:"packages,omitempty"`
}

// NewInstallPackagesStepSpec creates a new InstallPackagesStepSpec.
func NewInstallPackagesStepSpec(packages []string, name, description string) *InstallPackagesStepSpec {
	finalName := name
	if finalName == "" {
		finalName = "Install Packages"
	}
	finalDescription := description
	if finalDescription == "" {
		if len(packages) > 0 {
			finalDescription = fmt.Sprintf("Installs system packages: %s", strings.Join(packages, ", "))
		} else {
			finalDescription = "Installs specified system packages (none specified)."
		}
	}

	return &InstallPackagesStepSpec{
		StepMeta: spec.StepMeta{
			Name:        finalName,
			Description: finalDescription,
		},
		Packages: packages,
	}
}

// Name returns the step's name (implementing step.Step).
func (s *InstallPackagesStepSpec) Name() string { return s.StepMeta.Name }

// Description returns the step's description (implementing step.Step).
func (s *InstallPackagesStepSpec) Description() string { return s.StepMeta.Description }

// GetName returns the step's name for spec interface.
func (s *InstallPackagesStepSpec) GetName() string { return s.StepMeta.Name }

// GetDescription returns the step's description for spec interface.
func (s *InstallPackagesStepSpec) GetDescription() string { return s.StepMeta.Description }

// GetSpec returns the spec itself.
func (s *InstallPackagesStepSpec) GetSpec() interface{} { return s }

// Meta returns the step's metadata.
func (s *InstallPackagesStepSpec) Meta() *spec.StepMeta { return &s.StepMeta }

func (s *InstallPackagesStepSpec) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Rollback")
	// Rollback for package installation can be complex (e.g., versioning, dependencies).
	// A common approach is to make it a no-op or configurable.
	// For this default implementation, we'll make it a no-op.
	logger.Info("Rollback for InstallPackages is a no-op by default.")
	// Example if removal was desired:
	// if len(s.Packages) > 0 {
	//    runnerSvc := ctx.GetRunner()
	//    conn, _ := ctx.GetConnectorForHost(host)
	//    facts, _ := ctx.GetHostFacts(host)
	//    logger.Info("Attempting to remove packages as part of rollback.", "packages", s.Packages)
	//    return runnerSvc.RemovePackages(ctx.GoContext(), conn, facts, s.Packages...)
	// }
	return nil
}

func (s *InstallPackagesStepSpec) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Precheck")
	if len(s.Packages) == 0 {
		logger.Info("No packages specified to install, precheck considered done.")
		return true, nil
	}

    runnerSvc := ctx.GetRunner() // Assuming StepContext provides GetRunner()
    conn, err := ctx.GetConnectorForHost(host)
    if err != nil { return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err) }
    facts, err := ctx.GetHostFacts(host) // Assuming StepContext provides GetHostFacts()
    if err != nil { return false, fmt.Errorf("failed to get host facts for %s: %w", host.GetName(), err) }

    for _, pkg := range s.Packages {
        installed, err := runnerSvc.IsPackageInstalled(ctx.GoContext(), conn, facts, pkg)
        if err != nil {
			logger.Error("Failed to check if package is installed.", "package", pkg, "error", err)
			return false, fmt.Errorf("failed to check installation status for package %s on host %s: %w", pkg, host.GetName(), err)
		}
        if !installed {
            logger.Info("Package is not installed.", "package", pkg)
            return false, nil // Needs to run
        }
        logger.Debug("Package is already installed.", "package", pkg)
    }
    logger.Info("All specified packages are already installed.")
    return true, nil // All packages installed, step is done
}

func (s *InstallPackagesStepSpec) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Run")
	if len(s.Packages) == 0 {
		logger.Info("No packages specified to install.")
		return nil
	}

    runnerSvc := ctx.GetRunner()
    conn, err := ctx.GetConnectorForHost(host)
    if err != nil { return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err) }
    facts, err := ctx.GetHostFacts(host)
    if err != nil { return fmt.Errorf("failed to get host facts for %s: %w", host.GetName(), err) }

    logger.Info("Installing packages.", "packages", s.Packages)
    err = runnerSvc.InstallPackages(ctx.GoContext(), conn, facts, s.Packages...)
	if err != nil {
		return fmt.Errorf("failed to install packages on host %s: %w", host.GetName(), err)
	}
	logger.Info("Packages installed successfully.")
    return nil
}
