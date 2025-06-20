package preflight

import (
	"bytes" // For writing file content
	"fmt"
	"strings"
	"time" // For timestamp in config file

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec" // Added for spec.StepMeta
	"github.com/mensylisir/kubexm/pkg/step"
)

// SetSystemConfigStepSpec sets kernel parameters using sysctl and a configuration file.
type SetSystemConfigStepSpec struct {
	spec.StepMeta  `json:",inline"`
	Params         map[string]string `json:"params,omitempty"`
	ConfigFilePath string            `json:"configFilePath,omitempty"` // If empty, a default is used.
	Reload         bool              `json:"reload,omitempty"`         // Default true: whether to reload sysctl after writing file.
}

// NewSetSystemConfigStepSpec creates a new SetSystemConfigStepSpec.
func NewSetSystemConfigStepSpec(
	params map[string]string,
	configFilePath string,
	reload bool, // Simplified: directly pass the desired reload behavior
	name, description string, // Added for StepMeta
) *SetSystemConfigStepSpec {
	finalName := name
	effectivePath := configFilePath
	if effectivePath == "" {
		effectivePath = "/etc/sysctl.d/99-kubexms-preflight.conf" // Default config file path
	}
	if finalName == "" {
		finalName = fmt.Sprintf("Set System Kernel Parameters (sysctl) to file: %s", effectivePath)
	}

	paramSummary := []string{}
	for k, v := range params { // Use input params for initial description
		paramSummary = append(paramSummary, fmt.Sprintf("%s=%s", k, v))
	}
	finalDescription := description
	if finalDescription == "" {
		finalDescription = fmt.Sprintf("Sets sysctl params: [%s] to file %s and reloads (reload: %v).",
			strings.Join(paramSummary, ", "), effectivePath, reload)
	}

	return &SetSystemConfigStepSpec{
		StepMeta: spec.StepMeta{
			Name:        finalName,
			Description: finalDescription,
		},
		Params:         params,
		ConfigFilePath: effectivePath,
		Reload:         reload,
	}
}

// Name returns the step's name (implementing step.Step).
func (s *SetSystemConfigStepSpec) Name() string { return s.StepMeta.Name }

// Description returns the step's description (implementing step.Step).
func (s *SetSystemConfigStepSpec) Description() string { return s.StepMeta.Description }

// GetName returns the step's name for spec interface.
func (s *SetSystemConfigStepSpec) GetName() string { return s.StepMeta.Name }

// GetDescription returns the step's description for spec interface.
func (s *SetSystemConfigStepSpec) GetDescription() string { return s.StepMeta.Description }

// GetSpec returns the spec itself.
func (s *SetSystemConfigStepSpec) GetSpec() interface{} { return s }

// Meta returns the step's metadata.
func (s *SetSystemConfigStepSpec) Meta() *spec.StepMeta { return &s.StepMeta }

func (s *SetSystemConfigStepSpec) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Precheck")
    if host == nil { return false, fmt.Errorf("host is nil in Precheck for %s", s.GetName())}

	if len(s.Params) == 0 {
		logger.Debug("No sysctl params specified, precheck considered done.")
		return true, nil
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s for step %s: %w", host.GetName(), s.GetName(), err)
	}

	for key, expectedValue := range s.Params {
		cmd := fmt.Sprintf("sysctl -n %s", key)
		// Sudo typically not needed to read sysctl values.
		stdoutBytes, stderrBytes, execErr := conn.Exec(ctx.GoContext(), cmd, &connector.ExecOptions{Sudo: false})
		if execErr != nil {
			// If we can't read the current value (e.g. key doesn't exist), assume it's not set as desired.
			logger.Warn("Failed to read current value of sysctl key, assuming not set as expected.", "key", key, "error", execErr, "stderr", string(stderrBytes))
			return false, nil
		}
		currentValue := strings.TrimSpace(string(stdoutBytes))
		if currentValue != expectedValue {
			logger.Debug("Sysctl param mismatch.", "key", key, "current", currentValue, "expected", expectedValue)
			return false, nil
		}
	}
	logger.Info("All specified sysctl parameters already match desired values.")
	return true, nil
}

func (s *SetSystemConfigStepSpec) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Run")
    if host == nil { return fmt.Errorf("host is nil in Run for %s", s.GetName())}

	if len(s.Params) == 0 {
		logger.Info("No sysctl parameters to set.")
		return nil
	}

	conn, errConn := ctx.GetConnectorForHost(host)
	if errConn != nil {
		return fmt.Errorf("failed to get connector for host %s for step %s: %w", host.GetName(), s.GetName(), errConn)
	}

	var configFileContent bytes.Buffer
	configFileContent.WriteString(fmt.Sprintf("# Kernel parameters configured by KubeXMS Preflight (%s)\n", time.Now().Format(time.RFC3339)))
	for key, value := range s.Params {
		configFileContent.WriteString(fmt.Sprintf("%s = %s\n", key, value))
	}

	logger.Info("Writing sysctl parameters to file.", "file", s.ConfigFilePath)
	// Writing to /etc/sysctl.d/ or /etc/sysctl.conf usually requires sudo.
	// Assuming conn.WriteFile handles this if the connector is sudo-enabled.
	// If not, this would need to be a write to temp then sudo mv.
	// Assuming conn.CopyContent (from WriteFileStepSpec) handles sudo if needed for writing to /etc/
	errWrite := conn.CopyContent(ctx.GoContext(), configFileContent.String(), s.ConfigFilePath, connector.FileStat{
		Permissions: "0644",
		Sudo:        true, // Writing to /etc/sysctl.d usually requires sudo
	})
	if errWrite != nil {
		return fmt.Errorf("failed to write sysctl config file %s for step %s on host %s: %w", s.ConfigFilePath, s.GetName(), host.GetName(), errWrite)
	}
	logger.Info("Successfully wrote sysctl parameters to file.", "file", s.ConfigFilePath)

	if s.Reload {
		reloadCmd := ""
		// Determine appropriate reload command.
		if strings.HasPrefix(s.ConfigFilePath, "/etc/sysctl.d/") ||
		   strings.HasPrefix(s.ConfigFilePath, "/usr/lib/sysctl.d/") ||
		   strings.HasPrefix(s.ConfigFilePath, "/run/sysctl.d/") {
			reloadCmd = "sysctl --system" // Reloads all system config files including from /etc/sysctl.d
		} else if s.ConfigFilePath == "/etc/sysctl.conf" {
		    reloadCmd = "sysctl -p /etc/sysctl.conf" // Specifically load /etc/sysctl.conf
		} else {
			// For other files, try to load them specifically with -p.
			// This applies the settings but might not be persistent if not in a standard load path.
			reloadCmd = fmt.Sprintf("sysctl -p %s", s.ConfigFilePath)
		}

		logger.Info("Reloading sysctl configuration.", "command", reloadCmd)
		_, stderrReload, errReload := conn.Exec(ctx.GoContext(), reloadCmd, &connector.ExecOptions{Sudo: true})
		if errReload != nil {
			// Log as warning because the file write was successful. Some systems might report non-fatal errors
			// or if the file had no immediate effect (e.g., already set).
			logger.Warn("Sysctl reload command finished with error, check stderr for details. Verification will follow.", "command", reloadCmd, "stderr", string(stderrReload), "error", errReload)
		} else {
			logger.Info("Sysctl configuration reload command executed.", "command", reloadCmd)
		}
	} else {
		logger.Info("Skipping sysctl reload as per step configuration.")
	}

	logger.Info("Verifying sysctl parameters after apply...")
	allSet, checkErr := s.Precheck(ctx, host) // Re-use Precheck for verification
	if checkErr != nil {
	    // This means the verification check itself failed (e.g. couldn't run sysctl -n)
	    return fmt.Errorf("failed to verify sysctl params after apply for step %s on host %s: %w", s.GetName(), host.GetName(), checkErr)
	}
	if !allSet {
		// This means params are not matching expected values after apply/reload.
	    return fmt.Errorf("sysctl params not all set to desired values after apply/reload for step %s on host %s. Check previous logs for specific mismatches or reload errors", s.GetName(), host.GetName())
	}
	logger.Info("All sysctl parameters successfully set and verified.")
	return nil
}

func (s *SetSystemConfigStepSpec) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Rollback")
    if host == nil { return fmt.Errorf("host is nil in Rollback for %s", s.GetName())}

	logger.Info("Attempting to remove sysctl config file for rollback.", "file", s.ConfigFilePath)
	conn, errConn := ctx.GetConnectorForHost(host)
	if errConn != nil {
		logger.Error("Failed to get connector for host during rollback, cannot remove config file.", "error", errConn)
		return nil // Best-effort
	}

	// Use Exec for rm -f with sudo
	rmCmd := fmt.Sprintf("rm -f %s", s.ConfigFilePath)
	rmOpts := &connector.ExecOptions{Sudo: true} // Removing from /etc/sysctl.d usually needs sudo
	_, stderrRm, errRm := conn.Exec(ctx.GoContext(), rmCmd, rmOpts)

	if errRm != nil {
		logger.Error("Failed to remove sysctl config file during rollback (best effort).", "file", s.ConfigFilePath, "stderr", string(stderrRm), "error", errRm)
	} else {
		logger.Info("Successfully removed sysctl config file if it existed.", "file", s.ConfigFilePath)
		if s.Reload { // Only reload if original step intended a reload, to try to revert to previous state
		    reloadCmd := "sysctl --system"
		    logger.Info("Attempting to reload sysctl after removing config file.", "command", reloadCmd)
		    _, stderrReload, errReload := conn.Exec(ctx.GoContext(), reloadCmd, &connector.ExecOptions{Sudo: true})
		    if errReload != nil {
		        logger.Warn("Sysctl reload after config removal reported an error.", "command", reloadCmd, "stderr", string(stderrReload), "error", errReload)
		    } else {
		        logger.Info("Sysctl reloaded after config removal.", "command", reloadCmd)
		    }
		}
	}
	return nil
}

// Ensure SetSystemConfigStepSpec implements the step.Step interface.
var _ step.Step = (*SetSystemConfigStepSpec)(nil)
