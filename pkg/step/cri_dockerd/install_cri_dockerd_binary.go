package cri_dockerd

import (
	"fmt"
	"path/filepath"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/utils" // For PathRequiresSudo
)

// InstallCriDockerdBinaryStepSpec defines parameters for installing the cri-dockerd binary.
type InstallCriDockerdBinaryStepSpec struct {
	spec.StepMeta `json:",inline"`

	SourceBinaryPathCacheKey string `json:"sourceBinaryPathCacheKey,omitempty"` // Required
	TargetBinaryPath         string `json:"targetBinaryPath,omitempty"`
	Permissions              string `json:"permissions,omitempty"`
	Sudo                     bool   `json:"sudo,omitempty"`
}

// NewInstallCriDockerdBinaryStepSpec creates a new InstallCriDockerdBinaryStepSpec.
func NewInstallCriDockerdBinaryStepSpec(name, description, sourceBinaryPathCacheKey string) *InstallCriDockerdBinaryStepSpec {
	finalName := name
	if finalName == "" {
		finalName = "Install cri-dockerd Binary"
	}
	finalDescription := description
	// Description refined in populateDefaults

	if sourceBinaryPathCacheKey == "" {
		// This is a required field.
	}

	return &InstallCriDockerdBinaryStepSpec{
		StepMeta: spec.StepMeta{
			Name:        finalName,
			Description: finalDescription,
		},
		SourceBinaryPathCacheKey: sourceBinaryPathCacheKey,
		// Defaults in populateDefaults
	}
}

// Name returns the step's name.
func (s *InstallCriDockerdBinaryStepSpec) Name() string { return s.StepMeta.Name }

// Description returns the step's description.
func (s *InstallCriDockerdBinaryStepSpec) Description() string { return s.StepMeta.Description }

// GetName returns the step's name for spec interface.
func (s *InstallCriDockerdBinaryStepSpec) GetName() string { return s.StepMeta.Name }

// GetDescription returns the step's description for spec interface.
func (s *InstallCriDockerdBinaryStepSpec) GetDescription() string { return s.StepMeta.Description }

// GetSpec returns the spec itself.
func (s *InstallCriDockerdBinaryStepSpec) GetSpec() interface{} { return s }

// Meta returns the step's metadata.
func (s *InstallCriDockerdBinaryStepSpec) Meta() *spec.StepMeta { return &s.StepMeta }

func (s *InstallCriDockerdBinaryStepSpec) populateDefaults(logger runtime.Logger) {
	if s.TargetBinaryPath == "" {
		s.TargetBinaryPath = "/usr/local/bin/cri-dockerd"
		logger.Debug("TargetBinaryPath defaulted.", "path", s.TargetBinaryPath)
	}
	if s.Permissions == "" {
		s.Permissions = "0755"
		logger.Debug("Permissions defaulted.", "permissions", s.Permissions)
	}
	if !s.Sudo { // Default to true if not explicitly set false
		s.Sudo = true
		logger.Debug("Sudo defaulted to true.")
	}

	if s.StepMeta.Description == "" {
		s.StepMeta.Description = fmt.Sprintf("Installs cri-dockerd binary from cached path (key '%s') to %s with permissions %s.",
			s.SourceBinaryPathCacheKey, s.TargetBinaryPath, s.Permissions)
	}
}

// Precheck determines if the cri-dockerd binary is already installed.
func (s *InstallCriDockerdBinaryStepSpec) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Precheck")
	s.populateDefaults(logger)

	if s.SourceBinaryPathCacheKey == "" {
		return false, fmt.Errorf("SourceBinaryPathCacheKey must be specified for %s", s.GetName())
	}
	if s.TargetBinaryPath == "" {
		return false, fmt.Errorf("TargetBinaryPath must be specified for %s", s.GetName())
	}

	// Check if source path is even in cache. If not, Run will fail.
	// For precheck, if the target already exists and is correct, we don't need the source.
	sourcePathVal, sourceFound := ctx.StepCache().Get(s.SourceBinaryPathCacheKey)
	if !sourceFound {
		logger.Warn("Source binary path not found in cache. If target binary does not exist, Run will fail.", "key", s.SourceBinaryPathCacheKey)
		// Continue to check target, as it might already be there from a previous run.
	} else {
		if _, ok := sourcePathVal.(string); !ok || sourcePathVal.(string) == "" {
			logger.Warn("Source binary path in cache is invalid or empty. If target binary does not exist, Run will fail.", "key", s.SourceBinaryPathCacheKey)
		}
	}


	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	exists, err := conn.Exists(ctx.GoContext(), s.TargetBinaryPath)
	if err != nil {
		logger.Warn("Failed to check target binary existence, assuming installation is needed.", "path", s.TargetBinaryPath, "error", err)
		return false, nil // Let Run attempt.
	}

	if exists {
		logger.Info("Target binary already exists. Assuming installed.", "path", s.TargetBinaryPath)
		// TODO: Optionally, verify checksum of target binary against source if available.
		// This would require source binary to be accessible or its checksum cached.
		return true, nil
	}

	logger.Info("Target binary does not exist. Installation needed.", "path", s.TargetBinaryPath)
	return false, nil
}

// Run installs the cri-dockerd binary.
func (s *InstallCriDockerdBinaryStepSpec) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Run")
	s.populateDefaults(logger)

	if s.SourceBinaryPathCacheKey == "" {
		return fmt.Errorf("SourceBinaryPathCacheKey must be specified for %s", s.GetName())
	}
	if s.TargetBinaryPath == "" {
		return fmt.Errorf("TargetBinaryPath must be specified for %s", s.GetName())
	}

	sourceBinaryPathVal, found := ctx.StepCache().Get(s.SourceBinaryPathCacheKey)
	if !found {
		return fmt.Errorf("source cri-dockerd binary path not found in StepCache using key '%s'", s.SourceBinaryPathCacheKey)
	}
	sourceBinaryPath, ok := sourceBinaryPathVal.(string)
	if !ok || sourceBinaryPath == "" {
		return fmt.Errorf("invalid or empty source cri-dockerd binary path in StepCache (key '%s')", s.SourceBinaryPathCacheKey)
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}
	execOpts := &connector.ExecOptions{Sudo: s.Sudo}

	// Ensure target directory exists
	targetDir := filepath.Dir(s.TargetBinaryPath)
	logger.Debug("Ensuring target directory for binary exists.", "path", targetDir)
	mkdirCmd := fmt.Sprintf("mkdir -p %s", targetDir)
	_, stderrMkdir, errMkdir := conn.Exec(ctx.GoContext(), mkdirCmd, execOpts)
	if errMkdir != nil {
		return fmt.Errorf("failed to create target directory %s for binary (stderr: %s): %w", targetDir, string(stderrMkdir), errMkdir)
	}

	logger.Info("Copying cri-dockerd binary.", "source", sourceBinaryPath, "destination", s.TargetBinaryPath)
	cpCmd := fmt.Sprintf("cp -f %s %s", sourceBinaryPath, s.TargetBinaryPath)
	_, stderrCp, errCp := conn.Exec(ctx.GoContext(), cpCmd, execOpts)
	if errCp != nil {
		return fmt.Errorf("failed to copy binary from %s to %s (stderr: %s): %w", sourceBinaryPath, s.TargetBinaryPath, string(stderrCp), errCp)
	}

	if s.Permissions != "" {
		chmodCmd := fmt.Sprintf("chmod %s %s", s.Permissions, s.TargetBinaryPath)
		logger.Info("Setting permissions on installed binary.", "path", s.TargetBinaryPath, "permissions", s.Permissions)
		_, stderrChmod, errChmod := conn.Exec(ctx.GoContext(), chmodCmd, execOpts)
		if errChmod != nil {
			return fmt.Errorf("failed to set permissions %s on %s (stderr: %s): %w", s.Permissions, s.TargetBinaryPath, string(stderrChmod), errChmod)
		}
	}

	logger.Info("cri-dockerd binary installed successfully.", "path", s.TargetBinaryPath)
	return nil
}

// Rollback removes the installed cri-dockerd binary.
func (s *InstallCriDockerdBinaryStepSpec) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Rollback")
	s.populateDefaults(logger) // Ensure TargetBinaryPath is populated

	if s.TargetBinaryPath == "" {
		logger.Info("TargetBinaryPath is empty, cannot perform rollback.")
		return nil
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s for rollback: %w", host.GetName(), err)
	}

	logger.Info("Attempting to remove installed cri-dockerd binary.", "path", s.TargetBinaryPath)
	rmCmd := fmt.Sprintf("rm -f %s", s.TargetBinaryPath)
	execOpts := &connector.ExecOptions{Sudo: s.Sudo}
	_, stderrRm, errRm := conn.Exec(ctx.GoContext(), rmCmd, execOpts)

	if errRm != nil {
		logger.Error("Failed to remove binary during rollback (best effort).", "path", s.TargetBinaryPath, "stderr", string(stderrRm), "error", errRm)
	} else {
		logger.Info("Binary removed successfully.", "path", s.TargetBinaryPath)
	}
	return nil
}

var _ step.Step = (*InstallCriDockerdBinaryStepSpec)(nil)
