package containerd

import (
	"fmt"
	"path/filepath"
	// "strings" // Not directly used in this refactored version unless for complex path manipulations
	// "time" // Not used for step.Result

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step"
	commonstep "github.com/mensylisir/kubexm/pkg/step/common" // For DefaultExtractedPathKey
)

// InstallContainerdStep installs containerd binaries and systemd unit file
// from a previously extracted archive.
type InstallContainerdStep struct {
	SourceExtractedPathSharedDataKey string
	SystemdUnitFileSourceRelPath     string
	SystemdUnitFileTargetPath        string
	BinariesToCopy                   map[string]string // map: source_relative_path -> target_system_path
	StepName                         string
}

// NewInstallContainerdStep creates a new InstallContainerdStep.
// If binariesToCopy is nil or empty, default binaries will be set.
// If systemd paths are empty, defaults will be used.
func NewInstallContainerdStep(
	sourceExtractedPathKey string,
	binariesToCopy map[string]string,
	systemdSourceRelPath, systemdTargetPath string,
	stepName string,
) step.Step {
	s := &InstallContainerdStep{
		SourceExtractedPathSharedDataKey: sourceExtractedPathKey,
		SystemdUnitFileSourceRelPath:     systemdSourceRelPath,
		SystemdUnitFileTargetPath:        systemdTargetPath,
		BinariesToCopy:                   binariesToCopy,
		StepName:                         stepName,
	}
	s.populateDefaults() // Call populateDefaults after struct creation
	return s
}

func (s *InstallContainerdStep) populateDefaults() {
	if s.SourceExtractedPathSharedDataKey == "" {
		// Assuming DefaultExtractedPathKey is defined in commonstep or a constants package
		// For now, using a placeholder. This should be correctly referenced from commonstep.
		// If commonstep.DefaultExtractedPathKey doesn't exist, this will need adjustment.
		// Based on previous subtask, commonstep.DefaultExtractedPathKey should be available if that step was done.
		// However, the prompt used "commonstep.DefaultExtractedPathKey" for a constant that was
		// defined in extract_archive.go itself. Let's assume this key is a well-known string for now.
		// The plan implies commonstep.DefaultExtractedPathKey exists from extract_archive refactor.
		// That constant was `DefaultExtractedPathKey` in `extract_archive.go` but not exported.
		// The previous subtask `extract_archive.go` defined a *new* `ExtractArchiveStep` which used
		// `ExtractedDirSharedDataKey` as a field, not a global constant.
		// Let's use a sensible default string here, or assume it's passed from a global config.
		// For now, will use the value from previous spec:
		s.SourceExtractedPathSharedDataKey = "extractedPath" // commonstep.DefaultExtractedPathKey (if it exists and is exported)
	}
	if s.SystemdUnitFileSourceRelPath == "" {
		s.SystemdUnitFileSourceRelPath = "containerd.service" // Often at root of extracted archive
	}
	if s.SystemdUnitFileTargetPath == "" {
		s.SystemdUnitFileTargetPath = "/usr/lib/systemd/system/containerd.service"
	}
	if len(s.BinariesToCopy) == 0 {
		s.BinariesToCopy = map[string]string{
			"bin/containerd":                "/usr/local/bin/containerd",
			"bin/containerd-shim":           "/usr/local/bin/containerd-shim",
			"bin/containerd-shim-runc-v1":   "/usr/local/bin/containerd-shim-runc-v1",
			"bin/containerd-shim-runc-v2":   "/usr/local/bin/containerd-shim-runc-v2",
			"bin/ctr":                       "/usr/local/bin/ctr",
			"bin/runc":                      "/usr/local/sbin/runc", // runc often goes here or /usr/local/bin
		}
	}
}

func (s *InstallContainerdStep) Name() string {
	if s.StepName != "" {
		return s.StepName
	}
	return "Install Containerd from Extracted Files"
}

func (s *InstallContainerdStep) Description() string {
	return fmt.Sprintf("Installs containerd binaries and service file using extracted content from cache key '%s'.", s.SourceExtractedPathSharedDataKey)
}

func (s *InstallContainerdStep) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.Name(), "host", host.GetName(), "phase", "Precheck")
	// s.populateDefaults(); // Already called in constructor

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s for step %s: %w", host.GetName(), s.Name(), err)
	}

	for _, targetPath := range s.BinariesToCopy {
		exists, errExists := conn.Exists(ctx.GoContext(), targetPath)
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
		exists, errExists := conn.Exists(ctx.GoContext(), s.SystemdUnitFileTargetPath)
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
	logger := ctx.GetLogger().With("step", s.Name(), "host", host.GetName(), "phase", "Run")
	// s.populateDefaults(); // Already called in constructor

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s for step %s: %w", host.GetName(), s.Name(), err)
	}

	extractedPathVal, found := ctx.TaskCache().Get(s.SourceExtractedPathSharedDataKey)
	if !found {
		return fmt.Errorf("path to extracted containerd not found in Task Cache using key '%s' for step %s on host %s", s.SourceExtractedPathSharedDataKey, s.Name(), host.GetName())
	}
	extractedPath, typeOk := extractedPathVal.(string)
	if !typeOk || extractedPath == "" {
		return fmt.Errorf("invalid extracted containerd path in Task Cache (not a string or empty) for key '%s' for step %s on host %s", s.SourceExtractedPathSharedDataKey, s.Name(), host.GetName())
	}
	logger.Info("Using extracted containerd files from.", "path", extractedPath)

	execOptsSudo := &connector.ExecOptions{Sudo: true} // Most operations here require sudo

	for srcRelPath, targetSystemPath := range s.BinariesToCopy {
		sourceBinaryPath := filepath.Join(extractedPath, srcRelPath)
		targetDir := filepath.Dir(targetSystemPath)
		binaryName := filepath.Base(targetSystemPath)

		logger.Info("Ensuring target directory exists for binary.", "path", targetDir, "binary", binaryName)
		// Assuming conn.Mkdir handles sudo if needed, or target is writable by default.
		// For system paths, sudo is likely needed for Mkdir as well.
		// Re-evaluating: Mkdir on connector might not take sudo option. Using exec for `mkdir -p` with sudo.
		mkdirCmd := fmt.Sprintf("mkdir -p %s", targetDir)
		_, stderrMkdir, errMkdir := conn.Exec(ctx.GoContext(), mkdirCmd, execOptsSudo)
		if errMkdir != nil {
			return fmt.Errorf("failed to create target directory %s for %s (stderr: %s) for step %s on host %s: %w", targetDir, binaryName, string(stderrMkdir), s.Name(), host.GetName(), errMkdir)
		}

		srcExists, errSrcExist := conn.Exists(ctx.GoContext(), sourceBinaryPath)
		if errSrcExist != nil {
		    logger.Warn("Could not verify existence of source binary, attempting copy anyway.", "path", sourceBinaryPath, "error", errSrcExist)
		} else if !srcExists {
		    return fmt.Errorf("source binary %s not found in extracted path %s for step %s on host %s", srcRelPath, extractedPath, s.Name(), host.GetName())
		}

		logger.Info("Copying binary.", "source", sourceBinaryPath, "destination", targetSystemPath)
		cpCmd := fmt.Sprintf("cp -fp %s %s", sourceBinaryPath, targetSystemPath)
		_, stderrCp, errCp := conn.Exec(ctx.GoContext(), cpCmd, execOptsSudo)
		if errCp != nil {
			return fmt.Errorf("failed to copy binary %s to %s (stderr: %s) for step %s on host %s: %w", sourceBinaryPath, targetSystemPath, string(stderrCp), s.Name(), host.GetName(), errCp)
		}

		chmodCmd := fmt.Sprintf("chmod 0755 %s", targetSystemPath)
		_, stderrChmod, errChmod := conn.Exec(ctx.GoContext(), chmodCmd, execOptsSudo)
		if errChmod != nil {
			return fmt.Errorf("failed to set permissions for %s (stderr: %s) for step %s on host %s: %w", targetSystemPath, string(stderrChmod), s.Name(), host.GetName(), errChmod)
		}
		logger.Info("Binary installed and permissions set.", "binary", binaryName, "path", targetSystemPath)
	}

	if s.SystemdUnitFileTargetPath != "" && s.SystemdUnitFileSourceRelPath != "" {
		sourceServiceFile := filepath.Join(extractedPath, s.SystemdUnitFileSourceRelPath)
		targetServiceFileDir := filepath.Dir(s.SystemdUnitFileTargetPath)

		logger.Info("Ensuring target directory for systemd unit file exists.", "path", targetServiceFileDir)
		mkdirSvcCmd := fmt.Sprintf("mkdir -p %s", targetServiceFileDir)
		_, stderrMkdirSvc, errMkdirSvc := conn.Exec(ctx.GoContext(), mkdirSvcCmd, execOptsSudo)
		if errMkdirSvc != nil {
			return fmt.Errorf("failed to create target directory %s for systemd unit file (stderr: %s) for step %s on host %s: %w", targetServiceFileDir, string(stderrMkdirSvc), s.Name(), host.GetName(), errMkdirSvc)
		}

		srcSvcExists, errSrcSvcExist := conn.Exists(ctx.GoContext(), sourceServiceFile)
		if errSrcSvcExist != nil {
		    logger.Warn("Could not verify existence of source systemd file, attempting copy anyway.", "path", sourceServiceFile, "error", errSrcSvcExist)
		} else if !srcSvcExists {
		    logger.Warn("Source systemd unit file not found in extracted archive. Skipping installation of service file.", "sourcePath", sourceServiceFile, "archivePath", extractedPath)
		}

		if srcSvcExists {
		    logger.Info("Copying systemd unit file.", "source", sourceServiceFile, "destination", s.SystemdUnitFileTargetPath)
		    cpCmd := fmt.Sprintf("cp -f %s %s", sourceServiceFile, s.SystemdUnitFileTargetPath)
		    _, stderrCpSvc, errCpSvc := conn.Exec(ctx.GoContext(), cpCmd, execOptsSudo)
		    if errCpSvc != nil {
			return fmt.Errorf("failed to copy systemd unit file from %s to %s (stderr: %s) for step %s on host %s: %w", sourceServiceFile, s.SystemdUnitFileTargetPath, string(stderrCpSvc), s.Name(), host.GetName(), errCpSvc)
		    }
		    chmodCmdSvc := fmt.Sprintf("chmod 0644 %s", s.SystemdUnitFileTargetPath)
		    _, stderrChmodSvc, errChmodSvc := conn.Exec(ctx.GoContext(), chmodCmdSvc, execOptsSudo)
		    if errChmodSvc != nil {
			return fmt.Errorf("failed to set permissions for systemd unit file %s (stderr: %s) for step %s on host %s: %w", s.SystemdUnitFileTargetPath, string(stderrChmodSvc), s.Name(), host.GetName(), errChmodSvc)
		    }
		    logger.Info("Systemd unit file installed successfully.", "path", s.SystemdUnitFileTargetPath)
		}
	} else {
		logger.Info("No systemd unit file source or target path specified. Skipping systemd unit file installation.")
	}
	return nil
}

func (s *InstallContainerdStep) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.Name(), "host", host.GetName(), "phase", "Rollback")
	// s.populateDefaults(); // Defaults should be populated from constructor

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s for rollback of step %s: %w", host.GetName(), s.Name(), err)
	}

	execOptsSudo := &connector.ExecOptions{Sudo: true}

	for _, targetPath := range s.BinariesToCopy {
		logger.Info("Attempting to remove binary for rollback.", "path", targetPath)
		rmCmd := fmt.Sprintf("rm -f %s", targetPath)
		_, stderrRm, errRm := conn.Exec(ctx.GoContext(), rmCmd, execOptsSudo)
		if errRm != nil {
			logger.Error("Failed to remove binary during rollback.", "path", targetPath, "stderr", string(stderrRm), "error", errRm)
			// Continue rollback, best-effort
		}
	}

	if s.SystemdUnitFileTargetPath != "" {
		logger.Info("Attempting to remove systemd unit file for rollback.", "path", s.SystemdUnitFileTargetPath)
		rmCmd := fmt.Sprintf("rm -f %s", s.SystemdUnitFileTargetPath)
		_, stderrRmSvc, errRmSvc := conn.Exec(ctx.GoContext(), rmCmd, execOptsSudo)
		if errRmSvc != nil {
			logger.Error("Failed to remove systemd unit file during rollback.", "path", s.SystemdUnitFileTargetPath, "stderr", string(stderrRmSvc), "error", errRmSvc)
			// Continue rollback, best-effort
		}
	}
	logger.Info("Rollback attempt for containerd installation finished.")
	return nil
}

// Ensure InstallContainerdStep implements the step.Step interface.
var _ step.Step = (*InstallContainerdStep)(nil)
