package containerd

import (
	"fmt"
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	// "github.com/mensylisir/kubexm/pkg/utils" // Not strictly needed for this step if not using PathRequiresSudo
)

// ClearContainerdDataStepSpec defines parameters for clearing containerd's data store.
type ClearContainerdDataStepSpec struct {
	spec.StepMeta `json:",inline"`

	DataStorePath  string `json:"dataStorePath,omitempty"`
	ServiceName    string `json:"serviceName,omitempty"`
	RemoveRootPath bool   `json:"removeRootPath,omitempty"` // If true, remove DataStorePath itself. False (default), remove contents.
	Sudo           bool   `json:"sudo,omitempty"`
}

// NewClearContainerdDataStepSpec creates a new ClearContainerdDataStepSpec.
func NewClearContainerdDataStepSpec(name, description string) *ClearContainerdDataStepSpec {
	finalName := name
	if finalName == "" {
		finalName = "Clear Containerd Data Store"
	}
	finalDescription := description
	// Description will be refined in populateDefaults.

	return &ClearContainerdDataStepSpec{
		StepMeta: spec.StepMeta{
			Name:        finalName,
			Description: finalDescription,
		},
		// Defaults will be set in populateDefaults
	}
}

// Name returns the step's name.
func (s *ClearContainerdDataStepSpec) Name() string { return s.StepMeta.Name }

// Description returns the step's description.
func (s *ClearContainerdDataStepSpec) Description() string { return s.StepMeta.Description }

// GetName returns the step's name for spec interface.
func (s *ClearContainerdDataStepSpec) GetName() string { return s.StepMeta.Name }

// GetDescription returns the step's description for spec interface.
func (s *ClearContainerdDataStepSpec) GetDescription() string { return s.StepMeta.Description }

// GetSpec returns the spec itself.
func (s *ClearContainerdDataStepSpec) GetSpec() interface{} { return s }

// Meta returns the step's metadata.
func (s *ClearContainerdDataStepSpec) Meta() *spec.StepMeta { return &s.StepMeta }

func (s *ClearContainerdDataStepSpec) populateDefaults(logger runtime.Logger) {
	if s.DataStorePath == "" {
		s.DataStorePath = "/var/lib/containerd"
		logger.Debug("DataStorePath defaulted.", "path", s.DataStorePath)
	}
	if s.ServiceName == "" {
		s.ServiceName = "containerd"
		logger.Debug("ServiceName defaulted.", "name", s.ServiceName)
	}
	// RemoveRootPath defaults to false (zero value for bool).
	// Sudo defaults to true for these operations.
	if !s.Sudo { // If explicitly set to false by user, respect it. Otherwise, default to true.
	    // This logic is a bit tricky with bool zero values. If factory doesn't set it, it's false.
	    // Let's assume the factory should set the default Sudo=true.
	    // For now, if it's false, we make it true as per prompt's "Defaults to true".
	    s.Sudo = true
		logger.Debug("Sudo defaulted to true.")
	}


	if s.StepMeta.Description == "" {
		action := "Clears contents of"
		if s.RemoveRootPath {
			action = "Removes root path"
		}
		s.StepMeta.Description = fmt.Sprintf("%s containerd data store at %s after stopping service %s.",
			action, s.DataStorePath, s.ServiceName)
	}
}

// Precheck determines if the containerd data directory is already empty or non-existent.
func (s *ClearContainerdDataStepSpec) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Precheck")
	s.populateDefaults(logger)

	if s.DataStorePath == "" {
		return false, fmt.Errorf("DataStorePath must be specified for %s", s.GetName())
	}
	// Basic safety check for DataStorePath
	if s.DataStorePath == "/" || s.DataStorePath == "/var" || s.DataStorePath == "/var/lib" {
		return false, fmt.Errorf("DataStorePath is too broad and unsafe for removal: %s", s.DataStorePath)
	}


	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	exists, err := conn.Exists(ctx.GoContext(), s.DataStorePath)
	if err != nil {
		logger.Warn("Failed to check data store path existence, assuming data needs clearing.", "path", s.DataStorePath, "error", err)
		return false, nil // Let Run attempt.
	}

	if !exists {
		logger.Info("Data store path does not exist. No data to clear.", "path", s.DataStorePath)
		return true, nil
	}

	// If path exists, check if it's empty (if not removing root itself)
	if !s.RemoveRootPath {
		// `ls -A` lists all entries except . and .. If it outputs anything, directory is not empty.
		lsCmd := fmt.Sprintf("ls -A %s", s.DataStorePath)
		// Sudo might be needed to list contents of /var/lib/containerd if current user can't access.
		execOpts := &connector.ExecOptions{Sudo: s.Sudo}
		stdout, _, lsErr := conn.Exec(ctx.GoContext(), lsCmd, execOpts)
		if lsErr != nil {
			// If listing fails, we can't be sure if it's empty. Assume not done.
			logger.Warn("Failed to list contents of data store path, assuming data needs clearing.", "path", s.DataStorePath, "error", lsErr)
			return false, nil
		}
		if strings.TrimSpace(string(stdout)) == "" {
			logger.Info("Data store path exists but is empty. No data to clear.", "path", s.DataStorePath)
			return true, nil
		}
		logger.Info("Data store path exists and is not empty. Data needs clearing.", "path", s.DataStorePath)
	} else { // RemoveRootPath is true, and path exists.
	    logger.Info("Data store path exists and RemoveRootPath is true. Data needs clearing (by removing root path).", "path", s.DataStorePath)
	}

	return false, nil
}

// Run stops containerd and clears its data directory.
func (s *ClearContainerdDataStepSpec) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Run")
	s.populateDefaults(logger)

	if s.DataStorePath == "" {
		return fmt.Errorf("DataStorePath must be specified for %s", s.GetName())
	}
	// Basic safety check for DataStorePath
	if s.DataStorePath == "/" || s.DataStorePath == "/var" || s.DataStorePath == "/var/lib" {
		return fmt.Errorf("DataStorePath is too broad and unsafe for removal: %s", s.DataStorePath)
	}


	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}
	execOpts := &connector.ExecOptions{Sudo: s.Sudo}

	// Stop Service
	if s.ServiceName != "" {
		logger.Info("Stopping containerd service before clearing data.", "service", s.ServiceName)
		stopCmd := fmt.Sprintf("systemctl stop %s", s.ServiceName)
		_, stderrStop, errStop := conn.Exec(ctx.GoContext(), stopCmd, execOpts)
		if errStop != nil {
			// Log as warning because service might already be stopped or not exist.
			logger.Warn("Failed to stop service (might be already stopped or not exist).", "service", s.ServiceName, "stderr", string(stderrStop), "error", errStop)
		} else {
			logger.Info("Service stopped.", "service", s.ServiceName)
		}
	}

	// Clear Data
	// Check if path exists before trying to remove its contents or itself
	exists, errExist := conn.Exists(ctx.GoContext(), s.DataStorePath)
	if errExist != nil {
		return fmt.Errorf("failed to check existence of %s before clearing: %w", s.DataStorePath, errExist)
	}

	if exists {
		var clearCmd string
		if s.RemoveRootPath {
			clearCmd = fmt.Sprintf("rm -rf %s", s.DataStorePath)
			logger.Info("Removing containerd data store root path.", "path", s.DataStorePath, "command", clearCmd)
		} else {
			// Ensure trailing slash for removing contents `/*`
			cleanPath := strings.TrimSuffix(s.DataStorePath, "/") + "/"
			clearCmd = fmt.Sprintf("rm -rf %s*", cleanPath) // Remove contents
			logger.Info("Clearing contents of containerd data store.", "path", s.DataStorePath, "command", clearCmd)
		}

		_, stderrClear, errClear := conn.Exec(ctx.GoContext(), clearCmd, execOpts)
		if errClear != nil {
			return fmt.Errorf("failed to clear containerd data at %s (stderr: %s): %w", s.DataStorePath, string(stderrClear), errClear)
		}
		logger.Info("Containerd data store cleared successfully.", "path", s.DataStorePath)
	} else {
		logger.Info("Containerd data store path does not exist, no clearing needed.", "path", s.DataStorePath)
	}

	return nil
}

// Rollback for clearing data is not supported.
func (s *ClearContainerdDataStepSpec) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Rollback")
	logger.Warn("Rollback for clearing containerd data is not supported to prevent accidental data restoration or further issues.")
	return nil
}

var _ step.Step = (*ClearContainerdDataStepSpec)(nil)
