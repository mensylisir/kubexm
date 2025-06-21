package preflight

import (
	"bytes" // For writing file content
	"fmt"
	"strings"
	"time" // For timestamp in config file

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec" // Added for StepMeta
	"github.com/mensylisir/kubexm/pkg/step"
)

// SetSystemConfigStep sets kernel parameters using sysctl and a configuration file.
type SetSystemConfigStep struct {
	meta           spec.StepMeta
	Params         map[string]string
	ConfigFilePath string // If empty, a default is used.
	Reload         bool   // Whether to reload sysctl after writing file.
	Sudo           bool   // Sudo for writing file and reloading sysctl.
}

// NewSetSystemConfigStep creates a new SetSystemConfigStep.
func NewSetSystemConfigStep(
	instanceName string,
	params map[string]string,
	configFilePath string,
	reload bool,
	sudo bool,
) step.Step {
	effectivePath := configFilePath
	if effectivePath == "" {
		effectivePath = "/etc/sysctl.d/99-kubexms-preflight.conf"
	}
	name := instanceName
	if name == "" {
		name = fmt.Sprintf("SetSystemKernelParamsToFile-%s", strings.ReplaceAll(effectivePath, "/", "_"))
	}

	paramSummary := []string{}
	for k, v := range params {
		paramSummary = append(paramSummary, fmt.Sprintf("%s=%s", k, v))
	}
	desc := fmt.Sprintf("Sets sysctl params: [%s] to file %s and reloads (reload: %v).",
		strings.Join(paramSummary, ", "), effectivePath, reload)

	return &SetSystemConfigStep{
		meta: spec.StepMeta{
			Name:        name,
			Description: desc,
		},
		Params:         params,
		ConfigFilePath: effectivePath,
		Reload:         reload,
		Sudo:           sudo,
	}
}

// Meta returns the step's metadata.
func (s *SetSystemConfigStep) Meta() *spec.StepMeta {
	return &s.meta
}

func (s *SetSystemConfigStep) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Precheck")
	if host == nil {
		return false, fmt.Errorf("host is nil in Precheck for %s", s.meta.Name)
	}

	if len(s.Params) == 0 {
		logger.Debug("No sysctl params specified, precheck considered done.")
		return true, nil
	}

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s for step %s: %w", host.GetName(), s.meta.Name, err)
	}

	for key, expectedValue := range s.Params {
		cmd := fmt.Sprintf("sysctl -n %s", key)
		// Sudo typically not needed to read sysctl values.
		stdoutBytes, stderrBytes, execErr := runnerSvc.RunWithOptions(ctx.GoContext(), conn, cmd, &connector.ExecOptions{Sudo: false, Check: true})
		if execErr != nil {
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

func (s *SetSystemConfigStep) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Run")
	if host == nil {
		return fmt.Errorf("host is nil in Run for %s", s.meta.Name)
	}

	if len(s.Params) == 0 {
		logger.Info("No sysctl parameters to set.")
		return nil
	}

	runnerSvc := ctx.GetRunner()
	conn, errConn := ctx.GetConnectorForHost(host)
	if errConn != nil {
		return fmt.Errorf("failed to get connector for host %s for step %s: %w", host.GetName(), s.meta.Name, errConn)
	}

	var configFileContent bytes.Buffer
	configFileContent.WriteString(fmt.Sprintf("# Kernel parameters configured by KubeXMS Preflight (%s)\n", time.Now().Format(time.RFC3339)))
	for key, value := range s.Params {
		configFileContent.WriteString(fmt.Sprintf("%s = %s\n", key, value))
	}

	logger.Info("Writing sysctl parameters to file.", "file", s.ConfigFilePath)
	errWrite := runnerSvc.WriteFile(ctx.GoContext(), conn, configFileContent.Bytes(), s.ConfigFilePath, "0644", s.Sudo)
	if errWrite != nil {
		return fmt.Errorf("failed to write sysctl config file %s for step %s on host %s: %w", s.ConfigFilePath, s.meta.Name, host.GetName(), errWrite)
	}
	logger.Info("Successfully wrote sysctl parameters to file.", "file", s.ConfigFilePath)

	if s.Reload {
		reloadCmd := ""
		if strings.HasPrefix(s.ConfigFilePath, "/etc/sysctl.d/") ||
			strings.HasPrefix(s.ConfigFilePath, "/usr/lib/sysctl.d/") ||
			strings.HasPrefix(s.ConfigFilePath, "/run/sysctl.d/") {
			reloadCmd = "sysctl --system"
		} else if s.ConfigFilePath == "/etc/sysctl.conf" {
			reloadCmd = "sysctl -p /etc/sysctl.conf"
		} else {
			reloadCmd = fmt.Sprintf("sysctl -p %s", s.ConfigFilePath)
		}

		logger.Info("Reloading sysctl configuration.", "command", reloadCmd)
		if _, errReload := runnerSvc.Run(ctx.GoContext(), conn, reloadCmd, s.Sudo); errReload != nil {
			logger.Warn("Sysctl reload command finished with error, check logs for details. Verification will follow.", "command", reloadCmd, "error", errReload)
		} else {
			logger.Info("Sysctl configuration reload command executed.", "command", reloadCmd)
		}
	} else {
		logger.Info("Skipping sysctl reload as per step configuration.")
	}

	logger.Info("Verifying sysctl parameters after apply...")
	allSet, checkErr := s.Precheck(ctx, host)
	if checkErr != nil {
		return fmt.Errorf("failed to verify sysctl params after apply for step %s on host %s: %w", s.meta.Name, host.GetName(), checkErr)
	}
	if !allSet {
		return fmt.Errorf("sysctl params not all set to desired values after apply/reload for step %s on host %s. Check previous logs for specific mismatches or reload errors", s.meta.Name, host.GetName())
	}
	logger.Info("All sysctl parameters successfully set and verified.")
	return nil
}

func (s *SetSystemConfigStep) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Rollback")
	if host == nil {
		return fmt.Errorf("host is nil in Rollback for %s", s.meta.Name)
	}

	logger.Info("Attempting to remove sysctl config file for rollback.", "file", s.ConfigFilePath)
	runnerSvc := ctx.GetRunner()
	conn, errConn := ctx.GetConnectorForHost(host)
	if errConn != nil {
		logger.Error("Failed to get connector for host during rollback, cannot remove config file.", "error", errConn)
		return nil // Best-effort
	}

	if errRemove := runnerSvc.Remove(ctx.GoContext(), conn, s.ConfigFilePath, s.Sudo); errRemove != nil {
		logger.Error("Failed to remove sysctl config file during rollback (best effort).", "file", s.ConfigFilePath, "error", errRemove)
	} else {
		logger.Info("Successfully removed sysctl config file if it existed.", "file", s.ConfigFilePath)
		if s.Reload {
			reloadCmd := "sysctl --system"
			logger.Info("Attempting to reload sysctl after removing config file.", "command", reloadCmd)
			if _, errReload := runnerSvc.Run(ctx.GoContext(), conn, reloadCmd, s.Sudo); errReload != nil {
				logger.Warn("Sysctl reload after config removal reported an error.", "command", reloadCmd, "error", errReload)
			} else {
				logger.Info("Sysctl reloaded after config removal.", "command", reloadCmd)
			}
		}
	}
	return nil
}

// Ensure SetSystemConfigStep implements the step.Step interface.
var _ step.Step = (*SetSystemConfigStep)(nil)
