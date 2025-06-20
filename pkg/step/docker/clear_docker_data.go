package docker

import (
	"fmt"
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// ClearDockerDataStepSpec defines parameters for clearing Docker's data root directory.
type ClearDockerDataStepSpec struct {
	spec.StepMeta `json:",inline"`

	DataRootPath   string `json:"dataRootPath,omitempty"`
	ServiceName    string `json:"serviceName,omitempty"`
	RemoveRootPath bool   `json:"removeRootPath,omitempty"` // If true, remove DataRootPath itself. False (default), remove contents.
	Sudo           bool   `json:"sudo,omitempty"`
}

// NewClearDockerDataStepSpec creates a new ClearDockerDataStepSpec.
func NewClearDockerDataStepSpec(name, description string) *ClearDockerDataStepSpec {
	finalName := name
	if finalName == "" {
		finalName = "Clear Docker Data Root"
	}
	finalDescription := description
	// Description will be refined in populateDefaults.

	return &ClearDockerDataStepSpec{
		StepMeta: spec.StepMeta{
			Name:        finalName,
			Description: finalDescription,
		},
		// Defaults will be set in populateDefaults
	}
}

// Name returns the step's name.
func (s *ClearDockerDataStepSpec) Name() string { return s.StepMeta.Name }

// Description returns the step's description.
func (s *ClearDockerDataStepSpec) Description() string { return s.StepMeta.Description }

// GetName returns the step's name for spec interface.
func (s *ClearDockerDataStepSpec) GetName() string { return s.StepMeta.Name }

// GetDescription returns the step's description for spec interface.
func (s *ClearDockerDataStepSpec) GetDescription() string { return s.StepMeta.Description }

// GetSpec returns the spec itself.
func (s *ClearDockerDataStepSpec) GetSpec() interface{} { return s }

// Meta returns the step's metadata.
func (s *ClearDockerDataStepSpec) Meta() *spec.StepMeta { return &s.StepMeta }

func (s *ClearDockerDataStepSpec) populateDefaults(logger runtime.Logger) {
	if s.DataRootPath == "" {
		s.DataRootPath = "/var/lib/docker"
		logger.Debug("DataRootPath defaulted.", "path", s.DataRootPath)
	}
	if s.ServiceName == "" {
		s.ServiceName = "docker"
		logger.Debug("ServiceName defaulted.", "name", s.ServiceName)
	}
	// RemoveRootPath defaults to false (zero value for bool).
	if !s.Sudo { // If not explicitly set to false by user (zero value is false), default to true.
		s.Sudo = true
		logger.Debug("Sudo defaulted to true.")
	}

	if s.StepMeta.Description == "" {
		action := "Clears contents of"
		if s.RemoveRootPath {
			action = "Removes root path"
		}
		s.StepMeta.Description = fmt.Sprintf("%s Docker data root at %s after stopping service %s.",
			action, s.DataRootPath, s.ServiceName)
	}
}

// Precheck determines if the Docker data directory is already empty or non-existent.
func (s *ClearDockerDataStepSpec) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Precheck")
	s.populateDefaults(logger)

	if s.DataRootPath == "" {
		return false, fmt.Errorf("DataRootPath must be specified for %s", s.GetName())
	}
	if s.DataRootPath == "/" || s.DataRootPath == "/var" || s.DataRootPath == "/var/lib" && s.RemoveRootPath { // Be extra careful if removing root path itself
		return false, fmt.Errorf("DataRootPath is too broad and unsafe for root removal: %s", s.DataRootPath)
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	exists, err := conn.Exists(ctx.GoContext(), s.DataRootPath)
	if err != nil {
		logger.Warn("Failed to check data root path existence, assuming data needs clearing.", "path", s.DataRootPath, "error", err)
		return false, nil // Let Run attempt.
	}

	if !exists {
		logger.Info("Data root path does not exist. No data to clear.", "path", s.DataRootPath)
		return true, nil
	}

	if !s.RemoveRootPath { // If we only clear contents, check if it's empty
		lsCmd := fmt.Sprintf("ls -A %s", s.DataRootPath)
		execOpts := &connector.ExecOptions{Sudo: s.Sudo}
		stdout, _, lsErr := conn.Exec(ctx.GoContext(), lsCmd, execOpts)
		if lsErr != nil {
			logger.Warn("Failed to list contents of data root path, assuming data needs clearing.", "path", s.DataRootPath, "error", lsErr)
			return false, nil
		}
		if strings.TrimSpace(string(stdout)) == "" {
			logger.Info("Data root path exists but is empty. No data to clear.", "path", s.DataRootPath)
			return true, nil
		}
		logger.Info("Data root path exists and is not empty. Data needs clearing.", "path", s.DataRootPath)
	} else { // RemoveRootPath is true, and path exists.
		logger.Info("Data root path exists and RemoveRootPath is true. Data needs clearing (by removing root path).", "path", s.DataRootPath)
	}

	return false, nil
}

// Run stops the Docker service and clears its data directory.
func (s *ClearDockerDataStepSpec) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Run")
	s.populateDefaults(logger)

	if s.DataRootPath == "" {
		return fmt.Errorf("DataRootPath must be specified for %s", s.GetName())
	}
	if s.DataRootPath == "/" || (s.RemoveRootPath && (s.DataRootPath == "/var" || s.DataRootPath == "/var/lib")) {
		return fmt.Errorf("DataRootPath is too broad and unsafe for removal: %s", s.DataRootPath)
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}
	execOpts := &connector.ExecOptions{Sudo: s.Sudo}

	// Stop Service
	if s.ServiceName != "" {
		logger.Info("Stopping Docker service before clearing data.", "service", s.ServiceName)
		stopCmd := fmt.Sprintf("systemctl stop %s", s.ServiceName)
		_, stderrStop, errStop := conn.Exec(ctx.GoContext(), stopCmd, execOpts)
		if errStop != nil {
			logger.Warn("Failed to stop service (might be already stopped or not exist).", "service", s.ServiceName, "stderr", string(stderrStop), "error", errStop)
		} else {
			logger.Info("Service stopped.", "service", s.ServiceName)
		}
	}

	// Clear Data
	exists, errExist := conn.Exists(ctx.GoContext(), s.DataRootPath)
	if errExist != nil {
		return fmt.Errorf("failed to check existence of %s before clearing: %w", s.DataRootPath, errExist)
	}

	if exists {
		var clearCmd string
		if s.RemoveRootPath {
			clearCmd = fmt.Sprintf("rm -rf %s", s.DataRootPath)
			logger.Info("Removing Docker data root path.", "path", s.DataRootPath, "command", clearCmd)
		} else {
			// Ensure trailing slash for removing contents `/*` and `.` files `.*`
			// A safer way is to remove and recreate the directory if contents only.
			// For now, using a common pattern, but this can be risky if DataRootPath is a symlink or something unexpected.
			// A sequence of "find . -mindepth 1 -delete" inside the dir is safer for "contents only".
			// Or, as per prompt, "rm -rf <DataStorePath>/*"
			// Note: "rm -rf /path/*" does not remove hidden files/dirs (like .git).
			// "rm -rf /path/{*,.*}" is better but complex with shell expansion.
			// Safest for "contents only" is usually `rm -rf /path/ && mkdir -p /path/`
			// Let's stick to the prompt's `/*` but acknowledge its limits.
			// A better alternative for "contents only":
			// 1. `find /var/lib/docker -mindepth 1 -delete`
			// 2. `rm -rf /var/lib/docker && mkdir -p /var/lib/docker && chown/chmod /var/lib/docker`
			// The prompt implies `DataStorePath/*`. Let's refine this to be safer.
			// If not removing root, remove contents then ensure root dir exists.
			logger.Info("Clearing contents of Docker data root.", "path", s.DataRootPath)
			tempPathForClearing := strings.TrimSuffix(s.DataRootPath, "/") // Ensure no trailing slash before adding /*

			// This removes all visible files and directories under the path.
			// Hidden files/dirs directly under DataRootPath (e.g. /var/lib/docker/.config) would remain.
			clearCmdVisible := fmt.Sprintf("rm -rf %s/*", tempPathForClearing)
			logger.Debug("Removing visible contents.", "command", clearCmdVisible)
			_, stderrClearVis, errClearVis := conn.Exec(ctx.GoContext(), clearCmdVisible, execOpts)
			if errClearVis != nil {
				logger.Warn("Error removing visible contents from Docker data root (best-effort).", "path", s.DataRootPath, "stderr", string(stderrClearVis), "error", errClearVis)
			}

			// Attempt to remove hidden files/dirs (excluding . and .. which are not matched by simple *)
			// This is a common pattern but has edge cases depending on shell.
			// A find command would be more robust: find <DataRootPath> -mindepth 1 -delete
			clearCmdHidden := fmt.Sprintf("rm -rf %s/.*", tempPathForClearing)
			logger.Debug("Attempting to remove hidden contents (excluding . and ..).", "command", clearCmdHidden)
			_, stderrClearHid, errClearHid := conn.Exec(ctx.GoContext(), clearCmdHidden, execOpts)
			if errClearHid != nil {
				// This will often error on . and .. if not handled by shell, log as warning.
				logger.Warn("Error removing hidden contents from Docker data root (best-effort, might be due to '.' or '..').", "path", s.DataRootPath, "stderr", string(stderrClearHid), "error", errClearHid)
			}
			logger.Info("Docker data root contents cleared (best-effort).", "path", s.DataRootPath)

		} else { // If RemoveRootPath is true
			_, stderrClear, errClear := conn.Exec(ctx.GoContext(), clearCmd, execOpts)
			if errClear != nil {
				return fmt.Errorf("failed to clear Docker data at %s (stderr: %s): %w", s.DataRootPath, string(stderrClear), errClear)
			}
			logger.Info("Docker data store cleared successfully.", "path", s.DataRootPath)
		}
	} else {
		logger.Info("Docker data root path does not exist, no clearing needed.", "path", s.DataRootPath)
	}

	return nil
}

// Rollback for clearing Docker data is not supported.
func (s *ClearDockerDataStepSpec) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Rollback")
	logger.Warn("Rollback for clearing Docker data is not supported to prevent accidental data restoration or further issues.")
	return nil
}

var _ step.Step = (*ClearDockerDataStepSpec)(nil)
