package common

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/utils"
)

// EnsureDirectoryStepSpec defines parameters for ensuring a directory exists.
type EnsureDirectoryStepSpec struct {
	spec.StepMeta `json:",inline"`

	Path        string `json:"path,omitempty"`        // The directory path to ensure
	Permissions string `json:"permissions,omitempty"` // Optional: octal permissions to set if directory is created (e.g., "0755")
	Owner       string `json:"owner,omitempty"`       // Optional: owner for the directory (e.g., "user:group")
}

// NewEnsureDirectoryStepSpec creates a new EnsureDirectoryStepSpec.
func NewEnsureDirectoryStepSpec(name, description, path, permissions, owner string) *EnsureDirectoryStepSpec {
	finalName := name
	if finalName == "" {
		finalName = fmt.Sprintf("Ensure Directory %s", path)
	}
	finalDescription := description
	if finalDescription == "" {
		finalDescription = fmt.Sprintf("Ensures directory %s exists", path)
		if permissions != "" {
			finalDescription += fmt.Sprintf(" with permissions %s", permissions)
		}
		if owner != "" {
			finalDescription += fmt.Sprintf(" and owner %s", owner)
		}
	}

	return &EnsureDirectoryStepSpec{
		StepMeta: spec.StepMeta{
			Name:        finalName,
			Description: finalDescription,
		},
		Path:        path,
		Permissions: permissions,
		Owner:       owner,
	}
}

// Name returns the step's name.
func (s *EnsureDirectoryStepSpec) Name() string { return s.StepMeta.Name }

// Description returns the step's description.
func (s *EnsureDirectoryStepSpec) Description() string { return s.StepMeta.Description }

// GetName returns the step's name for spec interface.
func (s *EnsureDirectoryStepSpec) GetName() string { return s.StepMeta.Name }

// GetDescription returns the step's description for spec interface.
func (s *EnsureDirectoryStepSpec) GetDescription() string { return s.StepMeta.Description }

// GetSpec returns the spec itself.
func (s *EnsureDirectoryStepSpec) GetSpec() interface{} { return s }

// Meta returns the step's metadata.
func (s *EnsureDirectoryStepSpec) Meta() *spec.StepMeta { return &s.StepMeta }

// Precheck checks if the directory already exists.
// Note: It does not check permissions or owner here, Run will enforce them.
func (s *EnsureDirectoryStepSpec) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Precheck")

	if s.Path == "" {
		return false, fmt.Errorf("Path must be specified for EnsureDirectoryStep: %s", s.GetName())
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	exists, err := conn.Exists(ctx.GoContext(), s.Path)
	if err != nil {
		logger.Warn("Failed to check if directory exists, will attempt creation.", "path", s.Path, "error", err)
		return false, nil // Let Run attempt.
	}

	if exists {
		// Optionally, could check if it's actually a directory.
		// For now, if it exists, assume it's of the correct type or Run will fail if it's a file.
		logger.Info("Directory already exists.", "path", s.Path)
		// Even if it exists, Run might still be needed to ensure permissions/owner.
		// So, return false to let Run execute. If Run should be skipped, more checks are needed here.
		// For a simple "ensure exists", this could be true. But with perms/owner, Run should always execute if path exists.
		// Let's assume for now that if it exists, Run will ensure other properties.
		// A strict precheck would verify all properties.
		return false, nil // Let Run handle permissions and owner.
	}

	logger.Info("Directory does not exist. Creation needed.", "path", s.Path)
	return false, nil
}

// Run executes the directory creation and sets permissions/owner.
func (s *EnsureDirectoryStepSpec) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Run")

	if s.Path == "" {
		return fmt.Errorf("Path must be specified for EnsureDirectoryStep: %s", s.GetName())
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	sudo := utils.PathRequiresSudo(s.Path)
	execOpts := &connector.ExecOptions{Sudo: sudo}

	logger.Info("Ensuring directory exists.", "path", s.Path)
	mkdirCmd := fmt.Sprintf("mkdir -p %s", s.Path)
	_, stderrMkdir, errMkdir := conn.Exec(ctx.GoContext(), mkdirCmd, execOpts)
	if errMkdir != nil {
		return fmt.Errorf("failed to create directory %s (stderr: %s) on host %s: %w", s.Path, string(stderrMkdir), host.GetName(), errMkdir)
	}

	if s.Permissions != "" {
		logger.Info("Setting directory permissions.", "path", s.Path, "permissions", s.Permissions)
		chmodCmd := fmt.Sprintf("chmod %s %s", s.Permissions, s.Path)
		_, stderrChmod, errChmod := conn.Exec(ctx.GoContext(), chmodCmd, execOpts)
		if errChmod != nil {
			return fmt.Errorf("failed to set permissions %s on directory %s (stderr: %s) on host %s: %w", s.Permissions, s.Path, string(stderrChmod), host.GetName(), errChmod)
		}
	}

	if s.Owner != "" {
		logger.Info("Setting directory owner.", "path", s.Path, "owner", s.Owner)
		chownCmd := fmt.Sprintf("chown %s %s", s.Owner, s.Path)
		_, stderrChown, errChown := conn.Exec(ctx.GoContext(), chownCmd, execOpts)
		if errChown != nil {
			return fmt.Errorf("failed to set owner %s on directory %s (stderr: %s) on host %s: %w", s.Owner, s.Path, string(stderrChown), host.GetName(), errChown)
		}
	}

	logger.Info("Directory ensured successfully.", "path", s.Path)
	return nil
}

// Rollback is typically a no-op for EnsureDirectory, as removing a directory
// might unintentionally delete contents if other steps used it.
// It could be configured to rm/rmdir if specifically desired and known to be safe.
func (s *EnsureDirectoryStepSpec) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Rollback")
	logger.Info("EnsureDirectoryStep Rollback is a no-op by default.")
	// If conditional removal was desired:
	// if s.RemoveOnRollback && s.Path != "" { ... conn.Exec("rmdir %s", s.Path) ... }
	return nil
}

var _ step.Step = (*EnsureDirectoryStepSpec)(nil)
