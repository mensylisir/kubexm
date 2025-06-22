package common

import (
	"fmt"
	"os" // For os.Stat to check local file

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	// Assuming runtime.StepContext will be passed, which implements step.StepContext
	// No direct import of runtime needed here if we use step.StepContext interface in methods.
)

// UploadFileStep defines a step to upload a file from the local machine (control node)
// to a specified path on a remote host.
type UploadFileStep struct {
	meta             spec.StepMeta
	LocalSrcPath     string // Absolute path to the source file on the machine running kubexm
	RemoteDestPath   string // Absolute path to the destination on the target host
	Permissions      string // e.g., "0644"
	Sudo             bool   // Whether to use sudo for the remote write (e.g., writing to a privileged location)
	AllowMissingSrc  bool   // If true, do not error if the local source file is missing (useful for optional uploads)
}

// NewUploadFileStep creates a new UploadFileStep.
func NewUploadFileStep(instanceName, localSrcPath, remoteDestPath, permissions string, sudo, allowMissingSrc bool) step.Step {
	name := instanceName
	if name == "" {
		name = fmt.Sprintf("UploadFile:%s_to_%s", localSrcPath, remoteDestPath)
	}
	return &UploadFileStep{
		meta: spec.StepMeta{
			Name:        name,
			Description: fmt.Sprintf("Upload local file %s to remote %s", localSrcPath, remoteDestPath),
		},
		LocalSrcPath:    localSrcPath,
		RemoteDestPath:  remoteDestPath,
		Permissions:     permissions,
		Sudo:            sudo,
		AllowMissingSrc: allowMissingSrc,
	}
}

func (s *UploadFileStep) Meta() *spec.StepMeta {
	return &s.meta
}

func (s *UploadFileStep) Precheck(ctx step.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Precheck")

	// 1. Check if local source file exists (if not AllowMissingSrc)
	if _, err := os.Stat(s.LocalSrcPath); err != nil {
		if os.IsNotExist(err) {
			if s.AllowMissingSrc {
				logger.Info("Local source file does not exist, but AllowMissingSrc is true. Step will be skipped.", "path", s.LocalSrcPath)
				return true, nil // Skip if local source is missing and allowed
			}
			logger.Error("Local source file does not exist.", "path", s.LocalSrcPath, "error", err)
			return false, fmt.Errorf("local source file %s for step %s does not exist: %w", s.LocalSrcPath, s.meta.Name, err)
		}
		// Other error with os.Stat
		logger.Error("Failed to stat local source file.", "path", s.LocalSrcPath, "error", err)
		return false, fmt.Errorf("failed to stat local source file %s for step %s: %w", s.LocalSrcPath, s.meta.Name, err)
	}

	// 2. Check if remote file exists and matches (e.g., by checksum if desired)
	// For simplicity, this precheck will consider the step "done" if the remote file exists.
	// A more robust precheck would compare checksums.
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("Precheck: failed to get connector for host %s: %w", host.GetName(), err)
	}

	exists, err := runnerSvc.Exists(ctx.GoContext(), conn, s.RemoteDestPath)
	if err != nil {
		logger.Warn("Failed to check existence of remote file, assuming it needs upload.", "path", s.RemoteDestPath, "error", err)
		return false, nil // Proceed with run
	}
	if exists {
		// TODO: Add checksum comparison for more robust precheck.
		// For now, if it exists, we assume it's correct.
		logger.Info("Remote file already exists. Step considered done.", "path", s.RemoteDestPath)
		return true, nil
	}

	logger.Info("Remote file does not exist. Step needs to run.", "path", s.RemoteDestPath)
	return false, nil
}

func (s *UploadFileStep) Run(ctx step.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Run")

	// Re-check local source existence in case it was deleted between Precheck and Run,
	// or if precheck was skipped.
	localFileInfo, err := os.Stat(s.LocalSrcPath)
	if err != nil {
		if os.IsNotExist(err) {
			if s.AllowMissingSrc {
				logger.Info("Local source file does not exist, but AllowMissingSrc is true. Skipping upload.", "path", s.LocalSrcPath)
				return nil
			}
			logger.Error("Local source file does not exist.", "path", s.LocalSrcPath, "error", err)
			return fmt.Errorf("run: local source file %s for step %s does not exist: %w", s.LocalSrcPath, s.meta.Name, err)
		}
		logger.Error("Run: Failed to stat local source file.", "path", s.LocalSrcPath, "error", err)
		return fmt.Errorf("run: failed to stat local source file %s for step %s: %w", s.LocalSrcPath, s.meta.Name, err)
	}

	if localFileInfo.IsDir() {
		return fmt.Errorf("local source path %s is a directory, UploadFileStep only supports single files", s.LocalSrcPath)
	}

	content, err := os.ReadFile(s.LocalSrcPath)
	if err != nil {
		return fmt.Errorf("failed to read local source file %s: %w", s.LocalSrcPath, err)
	}

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("run: failed to get connector for host %s: %w", host.GetName(), err)
	}

	logger.Info("Uploading file.", "local", s.LocalSrcPath, "remote", s.RemoteDestPath, "permissions", s.Permissions, "sudo", s.Sudo)
	err = runnerSvc.WriteFile(ctx.GoContext(), conn, content, s.RemoteDestPath, s.Permissions, s.Sudo)
	if err != nil {
		return fmt.Errorf("failed to upload file from %s to %s on host %s: %w", s.LocalSrcPath, s.RemoteDestPath, host.GetName(), err)
	}

	logger.Info("File uploaded successfully.")
	return nil
}

func (s *UploadFileStep) Rollback(ctx step.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Rollback")
	logger.Info("Attempting rollback: removing remote file.", "path", s.RemoteDestPath)

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		// Log error but proceed if possible, or return if connector is essential
		logger.Error("Failed to get connector for host during rollback, cannot remove remote file.", "error", err)
		return fmt.Errorf("rollback: failed to get connector for host %s: %w", host.GetName(), err)
	}

	// Sudo for remove should ideally match sudo for write, or be configurable.
	// Assuming s.Sudo applies to removal as well if the path is privileged.
	if err := runnerSvc.Remove(ctx.GoContext(), conn, s.RemoteDestPath, s.Sudo); err != nil {
		logger.Warn("Failed to remove remote file during rollback (best effort).", "path", s.RemoteDestPath, "error", err)
		// Decide if this error should fail the rollback or be advisory
	} else {
		logger.Info("Remote file removed successfully during rollback.")
	}
	return nil // Best effort for rollback
}

var _ step.Step = (*UploadFileStep)(nil)
