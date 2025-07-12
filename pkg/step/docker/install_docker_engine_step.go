package docker

import (
	"fmt"
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// InstallDockerEngineStep installs Docker engine using the system package manager.
type InstallDockerEngineStep struct {
	meta         spec.StepMeta
	Packages     []string // Docker packages to install, e.g., ["docker-ce", "docker-ce-cli", "containerd.io"]
	Sudo         bool
	ExtraRepoSetupCmds []string // Optional commands to run before installation, e.g., for adding Docker's repository
}

// NewInstallDockerEngineStep creates a new InstallDockerEngineStep.
func NewInstallDockerEngineStep(instanceName string, packages []string, extraRepoCmds []string, sudo bool) step.Step {
	name := instanceName
	if name == "" {
		name = "InstallDockerEngine"
	}
	pkgs := packages
	if len(pkgs) == 0 {
		// Defaults might vary slightly by OS, but these are common for Debian/Ubuntu based.
		// Tasks should ideally determine the correct package names based on OS facts.
		pkgs = []string{"docker-ce", "docker-ce-cli", "containerd.io", "docker-buildx-plugin", "docker-compose-plugin"}
	}

	return &InstallDockerEngineStep{
		meta: spec.StepMeta{
			Name:        name,
			Description: fmt.Sprintf("Installs Docker Engine packages: %s", strings.Join(pkgs, ", ")),
		},
		Packages:     pkgs,
		ExtraRepoSetupCmds: extraRepoCmds,
		Sudo:         true, // Package installation usually requires sudo
	}
}

func (s *InstallDockerEngineStep) Meta() *spec.StepMeta {
	return &s.meta
}

func (s *InstallDockerEngineStep) Precheck(ctx step.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Precheck")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector: %w", err)
	}
	facts, err := ctx.GetHostFacts(host)
	if err != nil {
		return false, fmt.Errorf("failed to get host facts: %w", err)
	}

	// Check if a key Docker package is installed (e.g., "docker-ce" or the first one in the list)
	// A more robust check would iterate all s.Packages.
	if len(s.Packages) == 0 {
		logger.Info("No Docker packages specified to check for installation.")
		return true, nil // Or false if this state is considered an error for this step.
	}
	keyPackage := s.Packages[0]
	installed, err := runnerSvc.IsPackageInstalled(ctx.GoContext(), conn, facts, keyPackage)
	if err != nil {
		logger.Warn("Failed to check if Docker package is installed, will attempt installation.", "package", keyPackage, "error", err)
		return false, nil
	}
	if installed {
		// Optionally, could check `docker --version` here if a specific version is required.
		logger.Info("Key Docker package already installed.", "package", keyPackage)
		// If one is installed, assume all are for simplicity of precheck. Task can be more granular.
		return true, nil
	}
	logger.Info("Key Docker package not installed.", "package", keyPackage)
	return false, nil
}

func (s *InstallDockerEngineStep) Run(ctx step.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Run")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector: %w", err)
	}
	facts, err := ctx.GetHostFacts(host)
	if err != nil {
		return fmt.Errorf("failed to get host facts: %w", err)
	}

	// Run extra repository setup commands if any
	if len(s.ExtraRepoSetupCmds) > 0 {
		logger.Info("Running extra repository setup commands for Docker...")
		for _, cmd := range s.ExtraRepoSetupCmds {
			logger.Debug("Executing repo setup command", "command", cmd)
			_, stderr, errCmd := runnerSvc.Run(ctx.GoContext(), conn, cmd, s.Sudo)
			if errCmd != nil {
				return fmt.Errorf("failed to execute Docker repository setup command '%s': %w. Stderr: %s", cmd, errCmd, string(stderr))
			}
		}
		logger.Info("Docker repository setup commands executed. Updating package cache...")
		if errUpdate := runnerSvc.UpdatePackageCache(ctx.GoContext(), conn, facts); errUpdate != nil {
			// Log as warning, as InstallPackages might still work if cache is recent enough or packages are local.
			logger.Warn("Failed to update package cache after Docker repo setup (best effort).", "error", errUpdate)
		}
	}


	if len(s.Packages) == 0 {
		logger.Info("No Docker packages specified for installation.")
		return nil
	}

	logger.Info("Installing Docker Engine packages.", "packages", strings.Join(s.Packages, ", "))
	// The runner's InstallPackages method should handle sudo internally.
	if err := runnerSvc.InstallPackages(ctx.GoContext(), conn, facts, s.Packages...); err != nil {
		return fmt.Errorf("failed to install Docker Engine packages (%s): %w", strings.Join(s.Packages, ", "), err)
	}

	logger.Info("Docker Engine packages installed successfully.")
	return nil
}

func (s *InstallDockerEngineStep) Rollback(ctx step.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Rollback")
	logger.Info("Attempting to remove Docker Engine packages for rollback.")

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		logger.Error("Failed to get connector for rollback.", "error", err)
		return nil
	}
	facts, err := ctx.GetHostFacts(host)
	if err != nil {
		logger.Error("Failed to get host facts for rollback.", "error", err)
		return nil
	}

	if len(s.Packages) == 0 {
		logger.Info("No Docker packages were specified to remove for rollback.")
		return nil
	}

	// Runner's RemovePackages should handle sudo.
	if err := runnerSvc.RemovePackages(ctx.GoContext(), conn, facts, s.Packages...); err != nil {
		logger.Warn("Failed to remove Docker Engine packages during rollback (best effort).", "packages", strings.Join(s.Packages, ", "), "error", err)
	} else {
		logger.Info("Successfully removed Docker Engine packages (if they were installed).")
	}
	return nil
}

var _ step.Step = (*InstallDockerEngineStep)(nil)
