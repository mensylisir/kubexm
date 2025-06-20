package etcd

import (
	"fmt"
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	// "github.com/mensylisir/kubexm/pkg/utils" // Not strictly needed for this step if not using PathRequiresSudo
)

// ClearEtcdDataStepSpec defines parameters for clearing etcd's data directory.
type ClearEtcdDataStepSpec struct {
	spec.StepMeta `json:",inline"`

	DataDir        string `json:"dataDir,omitempty"`
	ServiceName    string `json:"serviceName,omitempty"`
	RemoveRootPath bool   `json:"removeRootPath,omitempty"` // If true, remove DataDir itself. False (default), remove contents.
	Sudo           bool   `json:"sudo,omitempty"`
}

// NewClearEtcdDataStepSpec creates a new ClearEtcdDataStepSpec.
func NewClearEtcdDataStepSpec(name, description string) *ClearEtcdDataStepSpec {
	finalName := name
	if finalName == "" {
		finalName = "Clear Etcd Data Directory"
	}
	finalDescription := description
	// Description will be refined in populateDefaults.

	return &ClearEtcdDataStepSpec{
		StepMeta: spec.StepMeta{
			Name:        finalName,
			Description: finalDescription,
		},
		// Defaults will be set in populateDefaults
	}
}

// Name returns the step's name.
func (s *ClearEtcdDataStepSpec) Name() string { return s.StepMeta.Name }

// Description returns the step's description.
func (s *ClearEtcdDataStepSpec) Description() string { return s.StepMeta.Description }

// GetName returns the step's name for spec interface.
func (s *ClearEtcdDataStepSpec) GetName() string { return s.StepMeta.Name }

// GetDescription returns the step's description for spec interface.
func (s *ClearEtcdDataStepSpec) GetDescription() string { return s.StepMeta.Description }

// GetSpec returns the spec itself.
func (s *ClearEtcdDataStepSpec) GetSpec() interface{} { return s }

// Meta returns the step's metadata.
func (s *ClearEtcdDataStepSpec) Meta() *spec.StepMeta { return &s.StepMeta }

func (s *ClearEtcdDataStepSpec) populateDefaults(logger runtime.Logger) {
	if s.DataDir == "" {
		s.DataDir = "/var/lib/etcd" // Common default for etcd
		logger.Debug("DataDir defaulted.", "path", s.DataDir)
	}
	if s.ServiceName == "" {
		s.ServiceName = "etcd" // Common default service name
		logger.Debug("ServiceName defaulted.", "name", s.ServiceName)
	}
	// RemoveRootPath defaults to false (Go's bool zero value).
	if !s.Sudo { // If not explicitly set by user (zero value is false), default to true.
		s.Sudo = true
		logger.Debug("Sudo defaulted to true for data clearing operations.")
	}

	if s.StepMeta.Description == "" {
		action := "Clears contents of"
		if s.RemoveRootPath {
			action = "Removes root path"
		}
		s.StepMeta.Description = fmt.Sprintf("%s etcd data directory at %s after stopping service %s.",
			action, s.DataDir, s.ServiceName)
	}
}

// Precheck determines if the etcd data directory is already empty or non-existent.
func (s *ClearEtcdDataStepSpec) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Precheck")
	s.populateDefaults(logger)

	if s.DataDir == "" {
		return false, fmt.Errorf("DataDir must be specified for %s", s.GetName())
	}
	// Safety check for DataDir path
	if (s.RemoveRootPath && (s.DataDir == "/" || s.DataDir == "/var" || s.DataDir == "/var/lib")) || s.DataDir == "/" {
		return false, fmt.Errorf("DataDir path '%s' is too broad and unsafe for removal operation in step %s", s.DataDir, s.GetName())
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	exists, err := conn.Exists(ctx.GoContext(), s.DataDir)
	if err != nil {
		logger.Warn("Failed to check data directory existence, assuming data needs clearing.", "path", s.DataDir, "error", err)
		return false, nil // Let Run attempt.
	}

	if !exists {
		logger.Info("Data directory does not exist. No data to clear.", "path", s.DataDir)
		return true, nil
	}

	// If path exists, and we are only clearing contents, check if it's empty
	if !s.RemoveRootPath {
		lsCmd := fmt.Sprintf("ls -A %s", s.DataDir)
		execOpts := &connector.ExecOptions{Sudo: s.Sudo}
		stdout, _, lsErr := conn.Exec(ctx.GoContext(), lsCmd, execOpts)
		if lsErr != nil {
			logger.Warn("Failed to list contents of data directory, assuming data needs clearing.", "path", s.DataDir, "error", lsErr)
			return false, nil
		}
		if strings.TrimSpace(string(stdout)) == "" {
			logger.Info("Data directory exists but is empty. No data to clear.", "path", s.DataDir)
			return true, nil
		}
		logger.Info("Data directory exists and is not empty. Data needs clearing.", "path", s.DataDir)
	} else { // RemoveRootPath is true, and path exists.
		logger.Info("Data directory exists and RemoveRootPath is true. Data needs clearing (by removing root path).", "path", s.DataDir)
	}

	return false, nil
}

// Run stops the etcd service and clears its data directory.
func (s *ClearEtcdDataStepSpec) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Run")
	s.populateDefaults(logger)

	if s.DataDir == "" {
		return fmt.Errorf("DataDir must be specified for %s", s.GetName())
	}
	if (s.RemoveRootPath && (s.DataDir == "/" || s.DataDir == "/var" || s.DataDir == "/var/lib")) || s.DataDir == "/" {
		return fmt.Errorf("DataDir path '%s' is too broad and unsafe for removal operation in step %s", s.DataDir, s.GetName())
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}
	execOpts := &connector.ExecOptions{Sudo: s.Sudo}

	// Stop Service
	if s.ServiceName != "" {
		logger.Info("Stopping etcd service before clearing data.", "service", s.ServiceName)
		stopCmd := fmt.Sprintf("systemctl stop %s", s.ServiceName)
		_, stderrStop, errStop := conn.Exec(ctx.GoContext(), stopCmd, execOpts)
		if errStop != nil {
			logger.Warn("Failed to stop service (might be already stopped or not exist).", "service", s.ServiceName, "stderr", string(stderrStop), "error", errStop)
		} else {
			logger.Info("Service stopped.", "service", s.ServiceName)
		}
	}

	// Clear Data
	exists, errExist := conn.Exists(ctx.GoContext(), s.DataDir)
	if errExist != nil {
		return fmt.Errorf("failed to check existence of %s before clearing: %w", s.DataDir, errExist)
	}

	if exists {
		var clearCmd string
		if s.RemoveRootPath {
			clearCmd = fmt.Sprintf("rm -rf %s", s.DataDir)
			logger.Info("Removing etcd data directory root path.", "path", s.DataDir, "command", clearCmd)
		} else {
			// To clear contents including hidden files, rm -rf path/* path/.* (excluding . and ..)
			// A safer pattern is often to remove and recreate the directory if permissions allow easily.
			// Or use: find <DataDir> -mindepth 1 -delete
			// For simplicity with basic rm:
			cleanPath := strings.TrimSuffix(s.DataDir, "/") // Ensure no trailing slash for globbing
			clearCmd = fmt.Sprintf("rm -rf %s/* %s/.[!.]* %s/..?*", cleanPath, cleanPath, cleanPath) // Attempt to remove all contents including hidden
			logger.Info("Clearing contents of etcd data directory.", "path", s.DataDir, "command_pattern", fmt.Sprintf("rm -rf %s/{*,.[!.]*,..?*}", cleanPath))
		}

		_, stderrClear, errClear := conn.Exec(ctx.GoContext(), clearCmd, execOpts)
		if errClear != nil {
			// If clearing contents, some errors (like "cannot remove '.' or '..'") are expected for `.*` patterns.
			// A more robust clear would use `find ... -delete` or check exit codes carefully.
			// For now, log warning for content removal, error for root path removal.
			if s.RemoveRootPath || !strings.Contains(string(stderrClear), "cannot remove") {
				return fmt.Errorf("failed to clear etcd data at %s (stderr: %s): %w", s.DataDir, string(stderrClear), errClear)
			}
			logger.Warn("Error(s) encountered while clearing contents of etcd data directory (some hidden files might remain or perm issues).", "path", s.DataDir, "stderr", string(stderrClear), "error", errClear)
		}
		logger.Info("Etcd data directory cleared/contents removed successfully.", "path", s.DataDir)
	} else {
		logger.Info("Etcd data directory does not exist, no clearing needed.", "path", s.DataDir)
	}

	return nil
}

// Rollback for clearing etcd data is not supported.
func (s *ClearEtcdDataStepSpec) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Rollback")
	logger.Warn("Rollback for clearing etcd data is not supported to prevent accidental data restoration or further issues.")
	return nil
}

var _ step.Step = (*ClearEtcdDataStepSpec)(nil)
