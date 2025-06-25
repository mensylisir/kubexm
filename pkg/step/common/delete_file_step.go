package common

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// DeleteFileStep removes a file or directory on the target host.
type DeleteFileStep struct {
	meta     spec.StepMeta
	RemotePath string // Absolute path to the file/directory on the target host
	Sudo       bool   // Whether to use sudo for the remove operation
	Recursive  bool   // Whether to remove recursively (like rm -r)
}

// NewDeleteFileStep creates a new DeleteFileStep.
func NewDeleteFileStep(instanceName, remotePath string, sudo, recursive bool) step.Step {
	name := instanceName
	if name == "" {
		name = fmt.Sprintf("DeleteFile-%s", remotePath)
	}
	desc := fmt.Sprintf("Deletes file or directory %s on remote host", remotePath)
	if recursive {
		desc = fmt.Sprintf("Recursively deletes file or directory %s on remote host", remotePath)
	}

	return &DeleteFileStep{
		meta: spec.StepMeta{
			Name:        name,
			Description: desc,
		},
		RemotePath: remotePath,
		Sudo:       sudo,
		Recursive:  recursive,
	}
}

func (s *DeleteFileStep) Meta() *spec.StepMeta {
	return &s.meta
}

func (s *DeleteFileStep) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Precheck")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	exists, err := runnerSvc.Exists(ctx.GoContext(), conn, s.RemotePath)
	if err != nil {
		logger.Warn("Failed to check existence of remote path, assuming it might exist.", "path", s.RemotePath, "error", err)
		return false, nil // Let Run attempt removal
	}

	if !exists {
		logger.Info("Remote path does not exist. Step considered done.", "path", s.RemotePath)
		return true, nil
	}
	logger.Info("Remote path exists and needs removal.", "path", s.RemotePath)
	return false, nil
}

func (s *DeleteFileStep) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Run")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	logger.Info("Removing remote path.", "path", s.RemotePath, "recursive", s.Recursive)
	// Runner's Remove method should handle recursive based on a flag or by constructing `rm -rf`
	// For now, we assume runner.Remove handles it if the path is a directory and Recursive is true.
	// If runner.Remove is basic, this step would need to check if path is dir and add -r to command.
	// The current runner.Remove takes path and sudo. It doesn't explicitly take recursive.
	// So, this step might need to use CommandStep if Recursive is true and path is a directory.
	// For now, let's assume runner.Remove handles simple file and dir removal.
	// A more robust runner.Remove would inspect the path or take a recursive flag.

	// Let's refine this. If s.Recursive, we should use a command.
	if s.Recursive {
		rmCmd := fmt.Sprintf("rm -rf %s", s.RemotePath)
		_, stderr, errRm := runnerSvc.Run(ctx.GoContext(), conn, rmCmd, s.Sudo)
		if errRm != nil {
			return fmt.Errorf("failed to recursively remove %s: %w. Stderr: %s", s.RemotePath, errRm, string(stderr))
		}
	} else {
		// Simple file removal
		if err := runnerSvc.Remove(ctx.GoContext(), conn, s.RemotePath, s.Sudo); err != nil {
			// Check if error is because it's a directory and we didn't ask for recursive
			// This depends on how runner.Remove reports errors.
			// For now, just return the error.
			return fmt.Errorf("failed to remove %s: %w", s.RemotePath, err)
		}
	}

	logger.Info("Remote path removed successfully.", "path", s.RemotePath)
	return nil
}

func (s *DeleteFileStep) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Rollback")
	logger.Info("Rollback for DeleteFileStep is not applicable (would mean restoring the file/directory).")
	return nil
}

var _ step.Step = (*DeleteFileStep)(nil)
