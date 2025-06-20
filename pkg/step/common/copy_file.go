package common

import (
	"fmt"
	"path/filepath" // For ensuring destination directory

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/utils" // For PathRequiresSudo
)

// CopyFileStepSpec defines parameters for copying a file.
// It can handle local-to-remote or remote-to-remote copies.
type CopyFileStepSpec struct {
	spec.StepMeta `json:",inline"`

	SourcePath      string `json:"sourcePath,omitempty"`
	DestinationPath string `json:"destinationPath,omitempty"`
	SourceIsLocal   bool   `json:"sourceIsLocal,omitempty"` // True if SourcePath is on agent, false if on target host
	Permissions     string `json:"permissions,omitempty"`   // e.g., "0644"
	Owner           string `json:"owner,omitempty"`         // e.g., "user:group"
	Sudo            bool   `json:"sudo,omitempty"`          // Use sudo for remote operations
}

// NewCopyFileStepSpec creates a new CopyFileStepSpec.
func NewCopyFileStepSpec(name, description, sourcePath, destinationPath string, sourceIsLocal bool) *CopyFileStepSpec {
	finalName := name
	if finalName == "" {
		if sourceIsLocal {
			finalName = fmt.Sprintf("Copy local %s to remote %s", filepath.Base(sourcePath), destinationPath)
		} else {
			finalName = fmt.Sprintf("Copy remote %s to %s", sourcePath, destinationPath)
		}
	}
	finalDescription := description
	if finalDescription == "" {
		if sourceIsLocal {
			finalDescription = fmt.Sprintf("Copies local file %s to remote destination %s.", sourcePath, destinationPath)
		} else {
			finalDescription = fmt.Sprintf("Copies file from %s to %s on the remote host.", sourcePath, destinationPath)
		}
	}

	return &CopyFileStepSpec{
		StepMeta: spec.StepMeta{
			Name:        finalName,
			Description: finalDescription,
		},
		SourcePath:      sourcePath,
		DestinationPath: destinationPath,
		SourceIsLocal:   sourceIsLocal,
		// Permissions, Owner, Sudo can be set explicitly after creation if needed
	}
}

// Name returns the step's name.
func (s *CopyFileStepSpec) Name() string { return s.StepMeta.Name }

// Description returns the step's description.
func (s *CopyFileStepSpec) Description() string { return s.StepMeta.Description }

// GetName returns the step's name for spec interface.
func (s *CopyFileStepSpec) GetName() string { return s.StepMeta.Name }

// GetDescription returns the step's description for spec interface.
func (s *CopyFileStepSpec) GetDescription() string { return s.StepMeta.Description }

// GetSpec returns the spec itself.
func (s *CopyFileStepSpec) GetSpec() interface{} { return s }

// Meta returns the step's metadata.
func (s *CopyFileStepSpec) Meta() *spec.StepMeta { return &s.StepMeta }

// populateDefaults can set default values.
func (s *CopyFileStepSpec) populateDefaults(logger runtime.Logger) {
	if s.Sudo == false && (s.Owner != "" || utils.PathRequiresSudo(s.DestinationPath) || (s.Permissions != "" && utils.PathRequiresSudo(s.DestinationPath))) {
		// If owner is set, or destination path is privileged, sudo is likely needed for chown/chmod or writing.
		// This is a basic heuristic.
		// s.Sudo = true // Decided against auto-setting sudo based on path, should be explicit.
		// logger.Debug("Sudo defaulted to true due to privileged destination or owner/permissions settings.")
	}
	if s.StepMeta.Description == "" { // If factory was called with empty description
		action := "Copies"
		if s.SourceIsLocal { action += " local" }
		action += fmt.Sprintf(" file from %s to %s", s.SourcePath, s.DestinationPath)
		if s.Permissions != "" { action += fmt.Sprintf(" with perms %s", s.Permissions) }
		if s.Owner != "" { action += fmt.Sprintf(" owned by %s", s.Owner) }
		s.StepMeta.Description = action + "."
	}
}

// Precheck checks if the destination file already exists.
// A more robust precheck might compare checksums if SourceIsLocal=false and source is accessible,
// or if a checksum is provided in the spec.
func (s *CopyFileStepSpec) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Precheck")
	s.populateDefaults(logger)

	if s.SourcePath == "" || s.DestinationPath == "" {
		return false, fmt.Errorf("SourcePath and DestinationPath must be specified for %s", s.GetName())
	}

	if s.SourceIsLocal {
		// Prechecking local source file existence from here is tricky as this code runs on the target.
		// The agent/tool initiating the copy should ideally ensure local source exists before defining this step.
		// For now, we'll primarily check the destination.
		logger.Debug("SourceIsLocal is true. Local source file existence check is deferred to Run phase by the agent.")
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	if !s.SourceIsLocal { // If source is remote, check its existence
		sourceExists, err := conn.Exists(ctx.GoContext(), s.SourcePath)
		if err != nil {
			logger.Warn("Failed to check remote source file existence, will attempt copy.", "path", s.SourcePath, "error", err)
			return false, nil
		}
		if !sourceExists {
			return false, fmt.Errorf("remote source file %s does not exist on host %s", s.SourcePath, host.GetName())
		}
	}

	destExists, err := conn.Exists(ctx.GoContext(), s.DestinationPath)
	if err != nil {
		logger.Warn("Failed to check destination file existence, will attempt copy.", "path", s.DestinationPath, "error", err)
		return false, nil
	}

	if destExists {
		// TODO: Implement checksum comparison if a Checksum field is added to the spec.
		// For now, if destination exists, assume it's the correct file to avoid re-copying.
		// This makes the step idempotent on destination existence only.
		logger.Info("Destination file already exists. Assuming it's up-to-date.", "path", s.DestinationPath)
		return true, nil
	}

	logger.Info("Destination file does not exist. Copy needed.", "path", s.DestinationPath)
	return false, nil
}

// Run executes the file copy operation.
func (s *CopyFileStepSpec) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Run")
	s.populateDefaults(logger)

	if s.SourcePath == "" || s.DestinationPath == "" {
		return fmt.Errorf("SourcePath and DestinationPath must be specified for %s", s.GetName())
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	// Ensure destination directory exists for remote-to-remote copy
	if !s.SourceIsLocal {
	    destDir := filepath.Dir(s.DestinationPath)
	    mkdirCmd := fmt.Sprintf("mkdir -p %s", destDir)
	    // Use Sudo field from spec for mkdir if remote-to-remote
	    execOptsMkdir := &connector.ExecOptions{Sudo: s.Sudo || utils.PathRequiresSudo(destDir)}
	    _, stderrMkdir, errMkdir := conn.Exec(ctx.GoContext(), mkdirCmd, execOptsMkdir)
	    if errMkdir != nil {
		    return fmt.Errorf("failed to create destination directory %s (stderr: %s) on host %s: %w", destDir, string(stderrMkdir), host.GetName(), errMkdir)
	    }
	}


	if s.SourceIsLocal {
		logger.Info("Copying local file to remote destination.", "localSource", s.SourcePath, "remoteDest", s.DestinationPath)
		// The connector's Copy method should handle sudo for the final placement and chmod/chown.
		// FileTransferOptions Sudo field might be used by the connector implementation.
		// If not, the connector itself might need to be sudo-enabled.
		err = conn.Copy(ctx.GoContext(), s.SourcePath, s.DestinationPath, &connector.FileTransferOptions{
			Permissions: s.Permissions,
			Owner:       s.Owner,
			Sudo:        s.Sudo, // Pass the step's Sudo hint to the copy operation
		})
		if err != nil {
			return fmt.Errorf("failed to copy local file %s to remote %s on host %s: %w", s.SourcePath, s.DestinationPath, host.GetName(), err)
		}
	} else { // Remote-to-remote copy
		logger.Info("Copying remote file to remote destination.", "remoteSource", s.SourcePath, "remoteDest", s.DestinationPath)
		cpCmd := fmt.Sprintf("cp -f %s %s", s.SourcePath, s.DestinationPath)
		execOpts := &connector.ExecOptions{Sudo: s.Sudo}
		_, stderrCp, errCp := conn.Exec(ctx.GoContext(), cpCmd, execOpts)
		if errCp != nil {
			return fmt.Errorf("failed to copy remote file from %s to %s (stderr: %s) on host %s: %w", s.SourcePath, s.DestinationPath, string(stderrCp), host.GetName(), errCp)
		}

		if s.Permissions != "" {
			chmodCmd := fmt.Sprintf("chmod %s %s", s.Permissions, s.DestinationPath)
			_, stderrChmod, errChmod := conn.Exec(ctx.GoContext(), chmodCmd, execOpts) // Use same Sudo as cp
			if errChmod != nil {
				return fmt.Errorf("failed to set permissions %s on %s (stderr: %s) on host %s: %w", s.Permissions, s.DestinationPath, string(stderrChmod), host.GetName(), errChmod)
			}
			logger.Debug("Permissions set.", "path", s.DestinationPath, "permissions", s.Permissions)
		}

		if s.Owner != "" {
			chownCmd := fmt.Sprintf("chown %s %s", s.Owner, s.DestinationPath)
			_, stderrChown, errChown := conn.Exec(ctx.GoContext(), chownCmd, execOpts) // Use same Sudo as cp
			if errChown != nil {
				return fmt.Errorf("failed to set owner %s on %s (stderr: %s) on host %s: %w", s.Owner, s.DestinationPath, string(stderrChown), host.GetName(), errChown)
			}
			logger.Debug("Owner set.", "path", s.DestinationPath, "owner", s.Owner)
		}
	}

	logger.Info("File copied successfully.", "destination", s.DestinationPath)
	return nil
}

// Rollback removes the destination file.
func (s *CopyFileStepSpec) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Rollback")
	s.populateDefaults(logger) // Ensure DestinationPath is available if defaulted

	if s.DestinationPath == "" {
		logger.Info("DestinationPath is empty, nothing to roll back.")
		return nil
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s for rollback: %w", host.GetName(), err)
	}

	logger.Info("Attempting to remove destination file.", "path", s.DestinationPath)
	rmCmd := fmt.Sprintf("rm -f %s", s.DestinationPath)
	// Use Sudo field from spec for removal.
	execOpts := &connector.ExecOptions{Sudo: s.Sudo || utils.PathRequiresSudo(s.DestinationPath)}
	_, stderrRm, errRm := conn.Exec(ctx.GoContext(), rmCmd, execOpts)

	if errRm != nil {
		logger.Error("Failed to remove destination file during rollback (best effort).", "path", s.DestinationPath, "stderr", string(stderrRm), "error", errRm)
	} else {
		logger.Info("Destination file removed successfully.", "path", s.DestinationPath)
	}
	return nil
}

var _ step.Step = (*CopyFileStepSpec)(nil)
