package docker

import (
	"fmt"
	"path/filepath"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// InstallCriDockerdBinaryStep copies the extracted cri-dockerd binary to /usr/local/bin
// and its systemd unit files to /etc/systemd/system/.
type InstallCriDockerdBinaryStep struct {
	meta                    spec.StepMeta
	ExtractedDirCacheKey    string // Task cache key for the path of the directory containing extracted cri-dockerd content
	TargetBinaryDir         string // System path for the binary, e.g., /usr/local/bin
	TargetSystemdDir        string // System path for systemd units, e.g., /etc/systemd/system
	Sudo                    bool   // Whether to use sudo for file operations
}

// NewInstallCriDockerdBinaryStep creates a new InstallCriDockerdBinaryStep.
func NewInstallCriDockerdBinaryStep(instanceName, extractedDirCacheKey, targetBinaryDir, targetSystemdDir string, sudo bool) step.Step {
	if extractedDirCacheKey == "" {
		extractedDirCacheKey = CriDockerdExtractedDirCacheKey
	}
	if targetBinaryDir == "" {
		targetBinaryDir = "/usr/local/bin"
	}
	if targetSystemdDir == "" {
		targetSystemdDir = "/etc/systemd/system"
	}
	name := instanceName
	if name == "" {
		name = "InstallCriDockerdBinaryAndUnits"
	}
	return &InstallCriDockerdBinaryStep{
		meta: spec.StepMeta{
			Name:        name,
			Description: fmt.Sprintf("Installs cri-dockerd binary to %s and systemd units to %s.", targetBinaryDir, targetSystemdDir),
		},
		ExtractedDirCacheKey:   extractedDirCacheKey,
		TargetBinaryDir:        targetBinaryDir,
		TargetSystemdDir:       targetSystemdDir,
		Sudo:                   sudo,
	}
}

func (s *InstallCriDockerdBinaryStep) Meta() *spec.StepMeta {
	return &s.meta
}

func (s *InstallCriDockerdBinaryStep) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Precheck")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector: %w", err)
	}

	criDockerdBinaryPath := filepath.Join(s.TargetBinaryDir, "cri-dockerd")
	serviceFilePath := filepath.Join(s.TargetSystemdDir, "cri-dockerd.service")
	socketFilePath := filepath.Join(s.TargetSystemdDir, "cri-dockerd.socket")

	binaryExists, _ := runnerSvc.Exists(ctx.GoContext(), conn, criDockerdBinaryPath)
	serviceFileExists, _ := runnerSvc.Exists(ctx.GoContext(), conn, serviceFilePath)
	socketFileExists, _ := runnerSvc.Exists(ctx.GoContext(), conn, socketFilePath)

	if binaryExists && serviceFileExists && socketFileExists {
		logger.Info("cri-dockerd binary and systemd units already installed.")
		return true, nil
	}

	logger.Info("cri-dockerd binary or systemd units not found. Installation needed.")
	if !binaryExists { logger.Debug("Binary missing", "path", criDockerdBinaryPath)}
	if !serviceFileExists { logger.Debug("Service file missing", "path", serviceFilePath)}
	if !socketFileExists { logger.Debug("Socket file missing", "path", socketFilePath)}

	return false, nil
}

func (s *InstallCriDockerdBinaryStep) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Run")
	extractedDirVal, found := ctx.TaskCache().Get(s.ExtractedDirCacheKey)
	if !found {
		return fmt.Errorf("extracted cri-dockerd directory path not found in task cache with key '%s'", s.ExtractedDirCacheKey)
	}
	extractedDir, ok := extractedDirVal.(string)
	if !ok || extractedDir == "" {
		return fmt.Errorf("invalid extracted cri-dockerd directory path in task cache: got '%v'", extractedDirVal)
	}
	logger.Info("Retrieved extracted cri-dockerd directory path from cache.", "path", extractedDir)

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector: %w", err)
	}

	// Ensure target directories exist
	for _, dir := range []string{s.TargetBinaryDir, s.TargetSystemdDir} {
		logger.Info("Ensuring target directory exists.", "path", dir)
		if err := runnerSvc.Mkdirp(ctx.GoContext(), conn, dir, "0755", s.Sudo); err != nil {
			return fmt.Errorf("failed to create target directory %s: %w", dir, err)
		}
	}

	// Install cri-dockerd binary
	sourceBinaryPath := filepath.Join(extractedDir, "cri-dockerd")
	targetBinaryPath := filepath.Join(s.TargetBinaryDir, "cri-dockerd")
	logger.Info("Installing cri-dockerd binary.", "source", sourceBinaryPath, "destination", targetBinaryPath)
	cpCmdBinary := fmt.Sprintf("cp -fp %s %s", sourceBinaryPath, targetBinaryPath)
	if _, errCp := runnerSvc.Run(ctx.GoContext(), conn, cpCmdBinary, s.Sudo); errCp != nil {
		return fmt.Errorf("failed to copy cri-dockerd binary from %s to %s: %w", sourceBinaryPath, targetBinaryPath, errCp)
	}
	if errChmod := runnerSvc.Chmod(ctx.GoContext(), conn, targetBinaryPath, "0755", s.Sudo); errChmod != nil {
		return fmt.Errorf("failed to set permissions for cri-dockerd binary %s: %w", targetBinaryPath, errChmod)
	}

	// Install systemd unit files
	// cri-dockerd archives typically include a "packaging/systemd/" directory.
	systemdSourceBase := filepath.Join(extractedDir, "packaging", "systemd")
	filesToCopy := map[string]string{
		"cri-dockerd.service": filepath.Join(s.TargetSystemdDir, "cri-dockerd.service"),
		"cri-dockerd.socket":  filepath.Join(s.TargetSystemdDir, "cri-dockerd.socket"),
	}

	for srcName, targetPath := range filesToCopy {
		sourcePath := filepath.Join(systemdSourceBase, srcName)
		logger.Info("Installing systemd unit file.", "source", sourcePath, "destination", targetPath)

		srcExists, _ := runnerSvc.Exists(ctx.GoContext(), conn, sourcePath)
		if !srcExists {
			return fmt.Errorf("source systemd file %s not found in extracted path %s", srcName, systemdSourceBase)
		}

		cpCmdUnit := fmt.Sprintf("cp -f %s %s", sourcePath, targetPath)
		if _, errCp := runnerSvc.Run(ctx.GoContext(), conn, cpCmdUnit, s.Sudo); errCp != nil {
			return fmt.Errorf("failed to copy systemd unit %s to %s: %w", srcName, targetPath, errCp)
		}
		if errChmod := runnerSvc.Chmod(ctx.GoContext(), conn, targetPath, "0644", s.Sudo); errChmod != nil {
			return fmt.Errorf("failed to set permissions for systemd unit %s: %w", targetPath, errChmod)
		}
	}

	logger.Info("cri-dockerd binary and systemd units installed successfully. Run 'systemctl daemon-reload'.")
	return nil
}

func (s *InstallCriDockerdBinaryStep) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Rollback")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		logger.Error("Failed to get connector for rollback.", "error", err)
		return nil
	}

	itemsToRemove := []string{
		filepath.Join(s.TargetBinaryDir, "cri-dockerd"),
		filepath.Join(s.TargetSystemdDir, "cri-dockerd.service"),
		filepath.Join(s.TargetSystemdDir, "cri-dockerd.socket"),
	}

	for _, itemPath := range itemsToRemove {
		logger.Info("Attempting to remove item for rollback.", "path", itemPath)
		if err := runnerSvc.Remove(ctx.GoContext(), conn, itemPath, s.Sudo); err != nil {
			logger.Warn("Failed to remove item during rollback (best effort).", "path", itemPath, "error", err)
		}
	}
	logger.Info("Rollback attempt for installing cri-dockerd finished.")
	return nil
}

var _ step.Step = (*InstallCriDockerdBinaryStep)(nil)
