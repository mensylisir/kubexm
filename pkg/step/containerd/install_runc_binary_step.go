package containerd

import (
	"fmt"
	"path/filepath"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// RuncBinaryRemotePathCacheKey is the default cache key for runc binary remote path
const RuncBinaryRemotePathCacheKey = "runcBinaryRemotePath"

// InstallRuncBinaryStep copies the distributed runc binary to a system path and makes it executable.
type InstallRuncBinaryStep struct {
	meta                    spec.StepMeta
	RemoteBinaryPathCacheKey string // Task cache key for the path of the runc binary on the target node (e.g., /tmp/kubexm-binaries/runc.amd64)
	TargetSystemPath        string // System path to install runc to (e.g., /usr/local/sbin/runc)
	Sudo                    bool   // Whether to use sudo for file operations
}

// NewInstallRuncBinaryStep creates a new InstallRuncBinaryStep.
func NewInstallRuncBinaryStep(instanceName, remotePathCacheKey, targetSystemPath string, sudo bool) step.Step {
	if remotePathCacheKey == "" {
		remotePathCacheKey = RuncBinaryRemotePathCacheKey // Default from distribute step
	}
	if targetSystemPath == "" {
		targetSystemPath = "/usr/local/sbin/runc" // Default system path for runc
	}
	name := instanceName
	if name == "" {
		name = "InstallRuncBinary"
	}
	return &InstallRuncBinaryStep{
		meta: spec.StepMeta{
			Name:        name,
			Description: fmt.Sprintf("Installs runc binary to %s.", targetSystemPath),
		},
		RemoteBinaryPathCacheKey: remotePathCacheKey,
		TargetSystemPath:        targetSystemPath,
		Sudo:                    sudo,
	}
}

func (s *InstallRuncBinaryStep) Meta() *spec.StepMeta {
	return &s.meta
}

func (s *InstallRuncBinaryStep) Precheck(ctx step.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Precheck")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}
	exists, err := runnerSvc.Exists(ctx.GoContext(), conn, s.TargetSystemPath)
	if err != nil {
		logger.Warn("Failed to check for existing runc binary, will attempt installation.", "path", s.TargetSystemPath, "error", err)
		return false, nil
	}
	if exists {
		// Optional: Could add a version check if runc binary supports --version and expected version is known.
		// For now, existence implies it's correctly installed by this step's definition.
		logger.Info("Runc binary already exists at target system path.", "path", s.TargetSystemPath)
		return true, nil
	}
	logger.Info("Runc binary does not exist at target system path.", "path", s.TargetSystemPath)
	return false, nil
}

func (s *InstallRuncBinaryStep) Run(ctx step.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Run")
	remoteBinaryPathVal, found := ctx.GetTaskCache().Get(s.RemoteBinaryPathCacheKey)
	if !found {
		return fmt.Errorf("remote runc binary path not found in task cache with key '%s'", s.RemoteBinaryPathCacheKey)
	}
	remoteBinaryPath, ok := remoteBinaryPathVal.(string)
	if !ok || remoteBinaryPath == "" {
		return fmt.Errorf("invalid remote runc binary path in task cache: got '%v'", remoteBinaryPathVal)
	}
	logger.Info("Retrieved remote runc binary path from cache.", "path", remoteBinaryPath)

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	targetDir := filepath.Dir(s.TargetSystemPath)
	logger.Info("Ensuring target directory exists for runc binary.", "path", targetDir)
	if err := runnerSvc.Mkdirp(ctx.GoContext(), conn, targetDir, "0755", s.Sudo); err != nil {
		return fmt.Errorf("failed to create target directory %s for runc: %w", targetDir, err)
	}

	logger.Info("Installing runc binary.", "source", remoteBinaryPath, "destination", s.TargetSystemPath)
	// Use 'install' command if available for atomicity and setting permissions, or cp + chmod.
	// Runner's "Install" method might do this, or we use Run with cp/chmod.
	// For simplicity with current runner, using cp then chmod.
	cpCmd := fmt.Sprintf("cp -fp %s %s", remoteBinaryPath, s.TargetSystemPath)
	if _, errCp := runnerSvc.Run(ctx.GoContext(), conn, cpCmd, s.Sudo); errCp != nil {
		return fmt.Errorf("failed to copy runc binary from %s to %s: %w", remoteBinaryPath, s.TargetSystemPath, errCp)
	}

	logger.Info("Setting permissions for runc binary.", "path", s.TargetSystemPath)
	// runc needs to be executable, 0755 is standard.
	if errChmod := runnerSvc.Chmod(ctx.GoContext(), conn, s.TargetSystemPath, "0755", s.Sudo); errChmod != nil {
		return fmt.Errorf("failed to set permissions for runc binary %s: %w", s.TargetSystemPath, errChmod)
	}

	logger.Info("Runc binary installed successfully.", "path", s.TargetSystemPath)
	// Optionally remove the source from RemoteTempDir if it was a unique file not an archive extraction.
	// This depends on how DistributeRuncBinaryStep manages its source.
	return nil
}

func (s *InstallRuncBinaryStep) Rollback(ctx step.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Rollback")
	logger.Info("Attempting to remove installed runc binary for rollback.", "path", s.TargetSystemPath)
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		logger.Error("Failed to get connector for host during rollback.", "error", err)
		return nil
	}
	if err := runnerSvc.Remove(ctx.GoContext(), conn, s.TargetSystemPath, s.Sudo); err != nil {
		logger.Warn("Failed to remove runc binary during rollback (best effort).", "path", s.TargetSystemPath, "error", err)
	} else {
		logger.Info("Successfully removed runc binary (if it existed).", "path", s.TargetSystemPath)
	}
	return nil
}

var _ step.Step = (*InstallRuncBinaryStep)(nil)
