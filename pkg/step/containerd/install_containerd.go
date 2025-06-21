package containerd

import (
	"fmt"
	"path/filepath"
	// "strings" // Not directly used in this refactored version unless for complex path manipulations
	// "time" // Not used for step.Result

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec" // Added for StepMeta
	"github.com/mensylisir/kubexm/pkg/step"
	// commonstep "github.com/mensylisir/kubexm/pkg/step/common" // Not used directly
)

// InstallContainerdStep installs containerd binaries and systemd unit file
// from a previously extracted archive.
type InstallContainerdStep struct {
	meta                             spec.StepMeta
	SourceExtractedPathSharedDataKey string
	SystemdUnitFileSourceRelPath     string
	SystemdUnitFileTargetPath        string
	BinariesToCopy                   map[string]string // map: source_relative_path -> target_system_path
	Sudo                             bool              // Whether to use sudo for file operations
}

// NewInstallContainerdStep creates a new InstallContainerdStep.
// If binariesToCopy is nil or empty, default binaries will be set.
// If systemd paths are empty, defaults will be used.
func NewInstallContainerdStep(
	instanceName string, // For Meta.Name
	sourceExtractedPathKey string,
	binariesToCopy map[string]string,
	systemdSourceRelPath, systemdTargetPath string,
	sudo bool,
) step.Step {
	metaName := instanceName
	if metaName == "" {
		metaName = "InstallContainerdFromExtracted"
	}
	s := &InstallContainerdStep{
		meta: spec.StepMeta{
			Name: metaName,
			Description: fmt.Sprintf("Installs containerd binaries and service file using extracted content from cache key '%s'.", sourceExtractedPathKey),
		},
		SourceExtractedPathSharedDataKey: sourceExtractedPathKey,
		SystemdUnitFileSourceRelPath:     systemdSourceRelPath,
		SystemdUnitFileTargetPath:        systemdTargetPath,
		BinariesToCopy:                   binariesToCopy,
		Sudo:                             sudo,
	}
	s.populateDefaults() // Call populateDefaults after struct creation
	return s
}

func (s *InstallContainerdStep) populateDefaults() {
	if s.SourceExtractedPathSharedDataKey == "" {
		// This key should ideally be explicitly passed by the task.
		// Using a default here can lead to hidden dependencies.
		// For robustness, tasks should define the keys they use.
		// If a default is absolutely needed, it should be a well-known constant.
		s.SourceExtractedPathSharedDataKey = "DefaultExtractedContainerdPath" // Example default key
	}
	if s.SystemdUnitFileSourceRelPath == "" {
		s.SystemdUnitFileSourceRelPath = "containerd.service" // Often at root of extracted archive, or usr/lib/systemd/system/containerd.service within archive
	}
	if s.SystemdUnitFileTargetPath == "" {
		s.SystemdUnitFileTargetPath = "/usr/lib/systemd/system/containerd.service"
	}
	if len(s.BinariesToCopy) == 0 {
		s.BinariesToCopy = map[string]string{
			"bin/containerd":              "/usr/local/bin/containerd",
			"bin/containerd-shim":         "/usr/local/bin/containerd-shim",
			"bin/containerd-shim-runc-v1": "/usr/local/bin/containerd-shim-runc-v1",
			"bin/containerd-shim-runc-v2": "/usr/local/bin/containerd-shim-runc-v2",
			"bin/ctr":                     "/usr/local/bin/ctr",
			// runc is now handled by a separate InstallRuncBinaryStep
			// "bin/runc":                      "/usr/local/sbin/runc",
		}
	}
}

// Meta returns the step's metadata.
func (s *InstallContainerdStep) Meta() *spec.StepMeta {
	return &s.meta
}

func (s *InstallContainerdStep) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Precheck")

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s for step %s: %w", host.GetName(), s.meta.Name, err)
	}

	for _, targetPath := range s.BinariesToCopy {
		exists, errExists := runnerSvc.Exists(ctx.GoContext(), conn, targetPath)
		if errExists != nil {
			// If we can't check, assume it's not there and let Run try.
			logger.Warn("Failed to check existence of target binary, Run will attempt installation.", "path", targetPath, "error", errExists)
			return false, nil
		}
		if !exists {
			logger.Debug("Target binary does not exist.", "path", targetPath)
			return false, nil
		}
	}
	logger.Debug("All target binaries exist.")

	if s.SystemdUnitFileTargetPath != "" {
		exists, errExists := runnerSvc.Exists(ctx.GoContext(), conn, s.SystemdUnitFileTargetPath)
		if errExists != nil {
			logger.Warn("Failed to check existence of systemd unit file, Run will attempt installation.", "path", s.SystemdUnitFileTargetPath, "error", errExists)
			return false, nil
		}
		if !exists {
			logger.Debug("Systemd unit file does not exist.", "path", s.SystemdUnitFileTargetPath)
			return false, nil
		}
		logger.Debug("Systemd unit file exists.", "path", s.SystemdUnitFileTargetPath)
	}

	logger.Info("Containerd installation (binaries and service file) appears complete.")
	return true, nil
}

func (s *InstallContainerdStep) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Run")

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s for step %s: %w", host.GetName(), s.meta.Name, err)
	}

	extractedPathVal, found := ctx.TaskCache().Get(s.SourceExtractedPathSharedDataKey)
	if !found {
		return fmt.Errorf("path to extracted containerd not found in Task Cache using key '%s' for step %s on host %s", s.SourceExtractedPathSharedDataKey, s.meta.Name, host.GetName())
	}
	extractedPath, typeOk := extractedPathVal.(string)
	if !typeOk || extractedPath == "" {
		return fmt.Errorf("invalid extracted containerd path in Task Cache (not a string or empty) for key '%s' for step %s on host %s", s.SourceExtractedPathSharedDataKey, s.meta.Name, host.GetName())
	}
	logger.Info("Using extracted containerd files from.", "path", extractedPath)

	// Operations like mkdir, cp, chmod will use runnerSvc.Run with sudo if s.Sudo is true.
	execOpts := &connector.ExecOptions{Sudo: s.Sudo}

	for srcRelPath, targetSystemPath := range s.BinariesToCopy {
		sourceBinaryPath := filepath.Join(extractedPath, srcRelPath)
		targetDir := filepath.Dir(targetSystemPath)
		binaryName := filepath.Base(targetSystemPath)

		logger.Info("Ensuring target directory exists for binary.", "path", targetDir, "binary", binaryName)
		if err := runnerSvc.Mkdirp(ctx.GoContext(), conn, targetDir, "0755", s.Sudo); err != nil {
			return fmt.Errorf("failed to create target directory %s for %s for step %s on host %s: %w", targetDir, binaryName, s.meta.Name, host.GetName(), err)
		}

		// Check source file existence using runner
		srcExists, errSrcExist := runnerSvc.Exists(ctx.GoContext(), conn, sourceBinaryPath)
		if errSrcExist != nil {
			logger.Warn("Could not verify existence of source binary, attempting copy anyway.", "path", sourceBinaryPath, "error", errSrcExist)
		} else if !srcExists {
			return fmt.Errorf("source binary %s not found in extracted path %s for step %s on host %s", srcRelPath, extractedPath, s.meta.Name, host.GetName())
		}

		logger.Info("Copying binary.", "source", sourceBinaryPath, "destination", targetSystemPath)
		// Using runner.Run for cp, as runner.WriteFile is for content from control node.
		// runner.UploadFile could be used if sourceBinaryPath was local to control node.
		cpCmd := fmt.Sprintf("cp -fp %s %s", sourceBinaryPath, targetSystemPath)
		if _, errCp := runnerSvc.Run(ctx.GoContext(), conn, cpCmd, s.Sudo); errCp != nil { // Assuming Run captures stderr internally or CommandError has it
			return fmt.Errorf("failed to copy binary %s to %s for step %s on host %s: %w", sourceBinaryPath, targetSystemPath, s.meta.Name, host.GetName(), errCp)
		}

		logger.Info("Setting permissions for binary.", "path", targetSystemPath)
		if errChmod := runnerSvc.Chmod(ctx.GoContext(), conn, targetSystemPath, "0755", s.Sudo); errChmod != nil {
			return fmt.Errorf("failed to set permissions for %s for step %s on host %s: %w", targetSystemPath, s.meta.Name, host.GetName(), errChmod)
		}
		logger.Info("Binary installed and permissions set.", "binary", binaryName, "path", targetSystemPath)
	}

	if s.SystemdUnitFileTargetPath != "" && s.SystemdUnitFileSourceRelPath != "" {
		sourceServiceFile := filepath.Join(extractedPath, s.SystemdUnitFileSourceRelPath)
		targetServiceFileDir := filepath.Dir(s.SystemdUnitFileTargetPath)

		logger.Info("Ensuring target directory for systemd unit file exists.", "path", targetServiceFileDir)
		if err := runnerSvc.Mkdirp(ctx.GoContext(), conn, targetServiceFileDir, "0755", s.Sudo); err != nil {
			return fmt.Errorf("failed to create target directory %s for systemd unit file for step %s on host %s: %w", targetServiceFileDir, s.meta.Name, host.GetName(), err)
		}

		srcSvcExists, errSrcSvcExist := runnerSvc.Exists(ctx.GoContext(), conn, sourceServiceFile)
		if errSrcSvcExist != nil {
			logger.Warn("Could not verify existence of source systemd file, attempting copy anyway.", "path", sourceServiceFile, "error", errSrcSvcExist)
		} else if !srcSvcExists {
			logger.Warn("Source systemd unit file not found in extracted archive. Skipping installation of service file.", "sourcePath", sourceServiceFile, "archivePath", extractedPath)
		}

		if srcSvcExists {
			logger.Info("Copying systemd unit file.", "source", sourceServiceFile, "destination", s.SystemdUnitFileTargetPath)
			cpCmd := fmt.Sprintf("cp -f %s %s", sourceServiceFile, s.SystemdUnitFileTargetPath)
			if _, errCpSvc := runnerSvc.Run(ctx.GoContext(), conn, cpCmd, s.Sudo); errCpSvc != nil {
				return fmt.Errorf("failed to copy systemd unit file from %s to %s for step %s on host %s: %w", sourceServiceFile, s.SystemdUnitFileTargetPath, s.meta.Name, host.GetName(), errCpSvc)
			}

			logger.Info("Setting permissions for systemd unit file.", "path", s.SystemdUnitFileTargetPath)
			if errChmodSvc := runnerSvc.Chmod(ctx.GoContext(), conn, s.SystemdUnitFileTargetPath, "0644", s.Sudo); errChmodSvc != nil {
				return fmt.Errorf("failed to set permissions for systemd unit file %s for step %s on host %s: %w", s.SystemdUnitFileTargetPath, s.meta.Name, host.GetName(), errChmodSvc)
			}
			logger.Info("Systemd unit file installed successfully.", "path", s.SystemdUnitFileTargetPath)
		}
	} else {
		logger.Info("No systemd unit file source or target path specified. Skipping systemd unit file installation.")
	}
	return nil
}

func (s *InstallContainerdStep) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Rollback")

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s for rollback of step %s: %w", host.GetName(), s.meta.Name, err)
	}

	for _, targetPath := range s.BinariesToCopy {
		logger.Info("Attempting to remove binary for rollback.", "path", targetPath)
		if errRm := runnerSvc.Remove(ctx.GoContext(), conn, targetPath, s.Sudo); errRm != nil {
			logger.Error("Failed to remove binary during rollback.", "path", targetPath, "error", errRm)
			// Continue rollback, best-effort
		}
	}

	if s.SystemdUnitFileTargetPath != "" {
		logger.Info("Attempting to remove systemd unit file for rollback.", "path", s.SystemdUnitFileTargetPath)
		if errRmSvc := runnerSvc.Remove(ctx.GoContext(), conn, s.SystemdUnitFileTargetPath, s.Sudo); errRmSvc != nil {
			logger.Error("Failed to remove systemd unit file during rollback.", "path", s.SystemdUnitFileTargetPath, "error", errRmSvc)
			// Continue rollback, best-effort
		}
	}
	logger.Info("Rollback attempt for containerd installation finished.")
	return nil
}

// Ensure InstallContainerdStep implements the step.Step interface.
var _ step.Step = (*InstallContainerdStep)(nil)
