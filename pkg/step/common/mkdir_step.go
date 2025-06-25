package common

import (
	"fmt"
	"os" // For os.Stat and os.MkdirAll

	"github.com/mensylisir/kubexm/pkg/common" // Added for ControlNodeHostName
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// MkdirStep creates a directory on the target host.
// This step is typically expected to run on the control node if Path is a local path,
// or on a remote host if the StepContext's runner is used via a remote connector.
// For creating directories for downloaded resources on the control node, direct os calls are fine.
type MkdirStep struct {
	meta        spec.StepMeta
	Path        string
	Permissions os.FileMode // e.g., 0755
	Sudo        bool      // If true, uses runner to execute mkdir with sudo (for remote hosts)
	// If Sudo is false and this step is dispatched to the control node, it will use os.MkdirAll.
	// If Sudo is false and dispatched to remote, it uses runner to mkdir without sudo.
}

// NewMkdirStep creates a new MkdirStep.
func NewMkdirStep(instanceName, path string, permissions os.FileMode, sudo bool) step.Step {
	metaName := instanceName
	if metaName == "" {
		metaName = fmt.Sprintf("Mkdir-%s", path)
	}
	return &MkdirStep{
		meta: spec.StepMeta{
			Name:        metaName,
			Description: fmt.Sprintf("Ensure directory %s exists with permissions %o", path, permissions),
		},
		Path:        path,
		Permissions: permissions,
		Sudo:        sudo,
	}
}

func (s *MkdirStep) Meta() *spec.StepMeta {
	return &s.meta
}

func (s *MkdirStep) Precheck(ctx step.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Precheck")

	isControlNode := host.GetName() == common.ControlNodeHostName // Assuming common.ControlNodeHostName exists

	if isControlNode && !s.Sudo { // Local operation without sudo
		fi, err := os.Stat(s.Path)
		if err == nil {
			if fi.IsDir() {
				// TODO: Check permissions? For now, existence as dir is enough.
				logger.Infof("Directory %s already exists locally.", s.Path)
				return true, nil
			}
			logger.Warnf("Path %s exists locally but is not a directory. Step will attempt to run.", s.Path)
			return false, nil // Path exists but isn't a dir, let Run handle or fail
		}
		if os.IsNotExist(err) {
			logger.Infof("Directory %s does not exist locally. Step needs to run.", s.Path)
			return false, nil
		}
		logger.Errorf("Error statting local path %s: %v. Step will attempt to run.", s.Path, err)
		return false, err // Other error, let Run attempt and potentially fail clearly
	}

	// For remote hosts or local sudo, use runner to check
	runner := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("precheck MkdirStep: failed to get connector for host %s: %w", host.GetName(), err)
	}

	// Use runner.IsDir or runner.Exists. Let's assume runner.IsDir also implies existence.
	isDir, err := runner.IsDir(ctx.GoContext(), conn, s.Path) // Assumes Sudo context is handled by runner if needed for check
	if err != nil {
		// If error is because path doesn't exist, that's fine for precheck (means Run should proceed)
		// Need a way for runner.IsDir or runner.Exists to distinguish "not found" from "error".
		// For now, assume error means we should try to run.
		logger.Warnf("Error checking remote directory %s with runner: %v. Step will attempt to run.", s.Path, err)
		return false, nil
	}
	if isDir {
		// TODO: Check remote permissions? More complex.
		logger.Infof("Directory %s already exists on host %s.", s.Path, host.GetName())
		return true, nil
	}

	logger.Infof("Directory %s does not exist or is not a directory on host %s. Step needs to run.", s.Path, host.GetName())
	return false, nil
}

func (s *MkdirStep) Run(ctx step.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Run")
	isControlNode := host.GetName() == common.ControlNodeHostName

	if isControlNode && !s.Sudo { // Local operation without sudo
		logger.Infof("Creating directory %s locally with permissions %o.", s.Path, s.Permissions)
		err := os.MkdirAll(s.Path, s.Permissions)
		if err != nil {
			logger.Errorf("Failed to create local directory %s: %v", s.Path, err)
			return fmt.Errorf("failed to create local directory %s: %w", s.Path, err)
		}
		logger.Infof("Local directory %s created/ensured.", s.Path)
		return nil
	}

	// Remote operation or local with sudo: use runner
	runner := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("run MkdirStep: failed to get connector for host %s: %w", host.GetName(), err)
	}

	permStr := fmt.Sprintf("%o", s.Permissions)
	logger.Infof("Creating directory %s on host %s with permissions %s (sudo: %t).", s.Path, host.GetName(), permStr, s.Sudo)

	err = runner.Mkdirp(ctx.GoContext(), conn, s.Path, permStr, s.Sudo)
	if err != nil {
		logger.Errorf("Failed to create directory %s on host %s: %v", s.Path, host.GetName(), err)
		return fmt.Errorf("failed to create directory %s on host %s: %w", s.Path, host.GetName(), err)
	}

	logger.Infof("Directory %s created/ensured on host %s.", s.Path, host.GetName())
	return nil
}

func (s *MkdirStep) Rollback(ctx step.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Rollback")
	// Rollback for mkdir is typically to remove the directory.
	// This is potentially destructive if the directory wasn't empty or created solely by this step.
	// A common approach is to only remove if it's empty, or not roll back mkdir at all.
	// For simplicity, this rollback will attempt to remove it. User should be aware.

	isControlNode := host.GetName() == common.ControlNodeHostName

	logger.Warnf("Attempting to remove directory %s as rollback. This might be destructive if directory was not empty or not solely created by this step.", s.Path)

	if isControlNode && !s.Sudo {
		// Check if it's empty first? os.Remove will fail on non-empty. os.RemoveAll is more forceful.
		// For now, using os.Remove which is safer for non-empty.
		err := os.Remove(s.Path)
		if err != nil && !os.IsNotExist(err) {
			logger.Errorf("Failed to remove local directory %s during rollback: %v", s.Path, err)
			return fmt.Errorf("rollback: failed to remove local directory %s: %w", s.Path, err)
		}
		logger.Infof("Local directory %s removed or was not present for rollback.", s.Path)
		return nil
	}

	runner := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("rollback MkdirStep: failed to get connector for host %s: %w", host.GetName(), err)
	}

	// Use runner.Remove. It should handle if path is not there.
	err = runner.Remove(ctx.GoContext(), conn, s.Path, s.Sudo) // Assuming runner.Remove can remove directories
	if err != nil {
		logger.Errorf("Failed to remove directory %s on host %s during rollback: %v", s.Path, host.GetName(), err)
		return fmt.Errorf("rollback: failed to remove directory %s on host %s: %w", s.Path, host.GetName(), err)
	}
	logger.Infof("Directory %s removed or was not present on host %s for rollback.", s.Path, host.GetName())
	return nil
}

var _ step.Step = (*MkdirStep)(nil)
