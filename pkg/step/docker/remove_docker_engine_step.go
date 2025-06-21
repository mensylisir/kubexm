package docker

import (
	"fmt"
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// RemoveDockerEngineStep removes Docker engine packages using the system package manager.
type RemoveDockerEngineStep struct {
	meta     spec.StepMeta
	Packages []string // Docker packages to remove, e.g., ["docker-ce", "docker-ce-cli", "containerd.io"]
	Purge    bool     // Whether to purge configuration files during removal (e.g., apt-get purge)
	Sudo     bool
}

// NewRemoveDockerEngineStep creates a new RemoveDockerEngineStep.
func NewRemoveDockerEngineStep(instanceName string, packages []string, purge, sudo bool) step.Step {
	name := instanceName
	if name == "" {
		name = "RemoveDockerEngine"
	}
	pkgs := packages
	if len(pkgs) == 0 {
		pkgs = []string{"docker-ce", "docker-ce-cli", "containerd.io", "docker-buildx-plugin", "docker-compose-plugin"}
	}

	return &RemoveDockerEngineStep{
		meta: spec.StepMeta{
			Name:        name,
			Description: fmt.Sprintf("Removes Docker Engine packages: %s (Purge: %v)", strings.Join(pkgs, ", "), purge),
		},
		Packages: pkgs,
		Purge:    purge, // Runner's RemovePackages should ideally support a purge option
		Sudo:     true,
	}
}

func (s *RemoveDockerEngineStep) Meta() *spec.StepMeta {
	return &s.meta
}

func (s *RemoveDockerEngineStep) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
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

	if len(s.Packages) == 0 {
		logger.Info("No Docker packages specified to check for removal.")
		return true, nil
	}
	keyPackage := s.Packages[0]
	installed, err := runnerSvc.IsPackageInstalled(ctx.GoContext(), conn, facts, keyPackage)
	if err != nil {
		logger.Warn("Failed to check if Docker package is installed, assuming it might be present.", "package", keyPackage, "error", err)
		return false, nil // Let Run attempt removal
	}
	if !installed {
		logger.Info("Key Docker package already not installed.", "package", keyPackage)
		// Assuming if one is not installed, all are (for simplicity of precheck)
		return true, nil
	}
	logger.Info("Key Docker package is installed and needs removal.", "package", keyPackage)
	return false, nil
}

func (s *RemoveDockerEngineStep) Run(ctx runtime.StepContext, host connector.Host) error {
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

	if len(s.Packages) == 0 {
		logger.Info("No Docker packages specified for removal.")
		return nil
	}

	logger.Info("Removing Docker Engine packages.", "packages", strings.Join(s.Packages, ", "), "purge", s.Purge)
	// The runner's RemovePackages method needs to support a 'purge' option.
	// If it doesn't, this step might need to construct specific apt/yum purge commands.
	// For now, assume runner.RemovePackages handles it or does a non-purging remove.
	// A more complete implementation would check facts.OS.PackageManager and use specific purge flags.
	if err := runnerSvc.RemovePackages(ctx.GoContext(), conn, facts, s.Packages... /*, s.Purge */); err != nil {
		return fmt.Errorf("failed to remove Docker Engine packages (%s): %w", strings.Join(s.Packages, ", "), err)
	}

	logger.Info("Docker Engine packages removed successfully.")
	return nil
}

func (s *RemoveDockerEngineStep) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Rollback")
	logger.Info("Rollback for RemoveDockerEngineStep is not applicable (would mean reinstalling).")
	return nil
}

var _ step.Step = (*RemoveDockerEngineStep)(nil)
