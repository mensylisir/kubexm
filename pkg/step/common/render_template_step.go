package common

import (
	"fmt"
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/util" // For util.RenderTemplate
)

// RenderTemplateStep renders a Go template and writes it to a remote file.
type RenderTemplateStep struct {
	meta            spec.StepMeta
	TemplateContent string      // The Go template string
	Data            interface{} // Data for the template
	RemoteDestPath  string      // Absolute path on the target host
	Permissions     string      // e.g., "0644"
	Sudo            bool        // Whether to use sudo for the remote write
}

// NewRenderTemplateStep creates a new RenderTemplateStep.
func NewRenderTemplateStep(instanceName, templateContent string, data interface{}, remoteDestPath, permissions string, sudo bool) step.Step {
	name := instanceName
	if name == "" {
		name = fmt.Sprintf("RenderTemplateTo-%s", remoteDestPath)
	}
	return &RenderTemplateStep{
		meta: spec.StepMeta{
			Name:        name,
			Description: fmt.Sprintf("Renders a template to %s", remoteDestPath),
		},
		TemplateContent: templateContent,
		Data:            data,
		RemoteDestPath:  remoteDestPath,
		Permissions:     permissions,
		Sudo:            sudo,
	}
}

func (s *RenderTemplateStep) Meta() *spec.StepMeta {
	return &s.meta
}

// Precheck for RenderTemplateStep:
// A robust precheck would render the template and compare its content with the remote file.
// For simplicity, this precheck will only check if the remote file exists.
// If it exists, we assume it's correct. This means changes to template data
// or content won't trigger a re-render unless the file is manually deleted.
// Returns:
//   - bool: true if the remote file exists (step considered done/skipped).
//   - error: Any error encountered during the check.
func (s *RenderTemplateStep) Precheck(ctx step.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Precheck")

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("Precheck: failed to get connector for host %s: %w", host.GetName(), err)
	}

	exists, err := runnerSvc.Exists(ctx.GoContext(), conn, s.RemoteDestPath)
	if err != nil {
		// If error checking existence (e.g. permission denied to parent dir), better to try Run.
		logger.Warn("Failed to check existence of remote file, assuming it needs to be rendered.", "path", s.RemoteDestPath, "error", err)
		return false, nil
	}

	if exists {
		// TODO: Implement content comparison for a more robust precheck.
		// This would involve:
		// 1. Render template locally (in memory).
		// 2. Read remote file content using runnerSvc.ReadFile().
		// 3. Compare.
		// For now, existence is sufficient to skip.
		logger.Info("Remote destination file already exists. Step considered done (no content check).", "path", s.RemoteDestPath)
		return true, nil
	}

	logger.Info("Remote destination file does not exist. Template needs to be rendered.", "path", s.RemoteDestPath)
	return false, nil
}

func (s *RenderTemplateStep) Run(ctx step.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Run")

	logger.Info("Rendering template.", "destination", s.RemoteDestPath)
	renderedContent, err := util.RenderTemplate(s.TemplateContent, s.Data)
	if err != nil {
		logger.Error(err, "Failed to render template string.")
		return fmt.Errorf("failed to render template for %s: %w", s.RemoteDestPath, err)
	}

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("Run: failed to get connector for host %s: %w", host.GetName(), err)
	}

	logger.Info("Writing rendered template to remote host.", "path", s.RemoteDestPath, "permissions", s.Permissions, "sudo", s.Sudo)
	err = runnerSvc.WriteFile(ctx.GoContext(), conn, []byte(renderedContent), s.RemoteDestPath, s.Permissions, s.Sudo)
	if err != nil {
		var cmdErr *connector.CommandError
		if errors.As(err, &cmdErr) {
			logger.Error(err, "Failed to write rendered template to remote host.", "stderr", cmdErr.Stderr, "stdout", cmdErr.Stdout)
			return fmt.Errorf("failed to write rendered template to %s on host %s (command: '%s', exit: %d): %w (stderr: %s)", s.RemoteDestPath, host.GetName(), cmdErr.Cmd, cmdErr.ExitCode, cmdErr, cmdErr.Stderr)
		}
		logger.Error(err, "Failed to write rendered template to remote host.")
		return fmt.Errorf("failed to write rendered template to %s on host %s: %w", s.RemoteDestPath, host.GetName(), err)
	}

	logger.Info("Template rendered and written successfully.")
	return nil
}

func (s *RenderTemplateStep) Rollback(ctx step.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Rollback")
	logger.Info("Attempting rollback: removing remote file.", "path", s.RemoteDestPath)

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		logger.Error(err, "Failed to get connector for host during rollback.")
		return fmt.Errorf("rollback: failed to get connector for host %s: %w", host.GetName(), err)
	}

	if err := runnerSvc.Remove(ctx.GoContext(), conn, s.RemoteDestPath, s.Sudo); err != nil {
		// Log as warning, as the file might not exist or path might be privileged.
		logger.Warn("Failed to remove remote file during rollback (best effort).", "path", s.RemoteDestPath, "error", err)
		// Consider if this should return an error or be advisory. For now, best effort.
	} else {
		logger.Info("Remote file removed successfully during rollback.")
	}
	return nil
}

var _ step.Step = (*RenderTemplateStep)(nil)

// Helper to check if an error is a CommandError (if not directly available in connector pkg)
// This can be placed in a utility package or within connector itself.
// For now, keeping it simple as direct type assertion or string check.
// func AsCommandError(err error, target **connector.CommandError) bool {
// 	 e, ok := err.(*connector.CommandError)
// 	 if ok {
// 	  *target = e
//   }
// 	 return ok
// }
