package preflight

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/kubexms/kubexms/pkg/runner"
	"github.com/kubexms/kubexms/pkg/runtime"
	"github.com/kubexms/kubexms/pkg/step"
)

// SetSystemConfigStep sets kernel parameters using sysctl.
// It can write to a specific sysctl configuration file and apply the changes.
type SetSystemConfigStep struct {
	// Params is a map of kernel parameter keys to their desired values.
	// E.g., {"net.ipv4.ip_forward": "1"}
	Params map[string]string
	// ConfigFilePath is the path to the sysctl config file to write.
	// If empty, a default path like "/etc/sysctl.d/99-kubexms-preflight.conf" will be used.
	ConfigFilePath string
	// Reload specifies whether to run `sysctl --system` (or `sysctl -p <file>`) after writing the config.
	// This applies the settings. Defaults to true.
	Reload *bool // Pointer to distinguish between not set (use default) and explicitly false.
}

// Name returns a human-readable name for the step.
func (s *SetSystemConfigStep) Name() string {
	return fmt.Sprintf("Set System Kernel Parameters (sysctl) to file: %s", s.effectiveConfigFilePath())
}

// effectiveConfigFilePath returns the actual config file path to be used,
// providing a default if ConfigFilePath is not set.
func (s *SetSystemConfigStep) effectiveConfigFilePath() string {
	if s.ConfigFilePath != "" {
		return s.ConfigFilePath
	}
	return "/etc/sysctl.d/99-kubexms-preflight.conf" // Default file
}

// shouldReload returns true if sysctl configuration should be reloaded.
// Defaults to true if not explicitly set.
func (s *SetSystemConfigStep) shouldReload() bool {
	if s.Reload == nil {
		return true // Default to reloading
	}
	return *s.Reload
}

// Check determines if all specified kernel parameters are already set to the desired values.
func (s *SetSystemConfigStep) Check(ctx *runtime.Context) (isDone bool, err error) {
	if len(s.Params) == 0 {
		ctx.Logger.Debugf("No sysctl params specified for step '%s' on host %s, considering done.", s.Name(), ctx.Host.Name)
		return true, nil
	}
	if ctx.Host.Runner == nil {
		return false, fmt.Errorf("runner not available in context for host %s", ctx.Host.Name)
	}
	hostCtxLogger := ctx.Logger.SugaredLogger.With("host", ctx.Host.Name, "step", s.Name()).Sugar()


	for key, expectedValue := range s.Params {
		// sysctl -n <key> reads the current value
		cmd := fmt.Sprintf("sysctl -n %s", key)
		// Sudo not usually needed for reading sysctl values.
		stdout, stderr, execErr := ctx.Host.Runner.Run(ctx.GoContext, cmd, false)
		if execErr != nil {
			// Some keys might not exist until a module is loaded, or on certain OSes.
			// Consider this a failure to check, meaning not "done".
			hostCtxLogger.Warnf("Failed to read current value of sysctl key '%s': %v (stderr: %s). Assuming not set as expected.", key, execErr, string(stderr))
			return false, fmt.Errorf("failed to read current value of sysctl key '%s': %w (stderr: %s)", key, execErr, string(stderr))
		}
		currentValue := strings.TrimSpace(string(stdout))

		// Values from sysctl can sometimes have multiple fields separated by tabs or spaces.
		// E.g., `sysctl -n net.ipv4.tcp_mem` might return "123 456 789".
		// If expectedValue is simple, direct comparison is fine.
		// If expectedValue might also contain spaces (e.g. "1 2 3"), direct compare is okay.
		// For more robustness, one might compare fields individually if the format is known.
		// Current logic: direct string comparison after trimming space.
		if currentValue != expectedValue {
			hostCtxLogger.Debugf("Check: Sysctl param %s is currently %q, want %q.", key, currentValue, expectedValue)
			return false, nil // Mismatch found, not done.
		}
		hostCtxLogger.Debugf("Check: Sysctl param %s is already %q.", key, currentValue)
	}
	hostCtxLogger.Infof("All specified sysctl parameters already match desired values.")
	return true, nil // All params match
}

// Run applies the kernel parameters by writing them to a configuration file
// and then reloading the sysctl configuration.
func (s *SetSystemConfigStep) Run(ctx *runtime.Context) *step.Result {
	startTime := time.Now()
	res := step.NewResult(s.Name(), ctx.Host.Name, startTime, nil)
	hostCtxLogger := ctx.Logger.SugaredLogger.With("host", ctx.Host.Name, "step", s.Name()).Sugar()
	var errorsCollected []string
	var appliedParams []string

	if len(s.Params) == 0 {
		res.Status = "Succeeded"
		res.Message = "No sysctl parameters specified to set."
		hostCtxLogger.Infof(res.Message)
		res.EndTime = time.Now()
		return res
	}
	if ctx.Host.Runner == nil {
		res.Error = fmt.Errorf("runner not available in context for host %s", ctx.Host.Name)
		res.Status = "Failed"; res.Message = res.Error.Error()
		hostCtxLogger.Errorf("Step failed: %v", res.Error)
		return res
	}

	var configFileContent strings.Builder
	configFileContent.WriteString(fmt.Sprintf("# Kernel parameters configured by KubeXMS Preflight (%s)\n", time.Now().Format(time.RFC3339)))
	for key, value := range s.Params {
		// Basic sanitization/validation can be added here if keys/values come from less trusted sources.
		configFileContent.WriteString(fmt.Sprintf("%s = %s\n", key, value))
		appliedParams = append(appliedParams, fmt.Sprintf("%s=%s", key, value))
	}

	filePath := s.effectiveConfigFilePath()
	hostCtxLogger.Infof("Writing sysctl parameters to %s: %s", filePath, strings.Join(appliedParams, ", "))

	// Write content to the file. Permissions for sysctl config files are typically 0644.
	// Sudo is required to write to system directories like /etc/sysctl.d/.
	err := ctx.Host.Runner.WriteFile(ctx.GoContext, []byte(configFileContent.String()), filePath, "0644", true)
	if err != nil {
		errMsg := fmt.Sprintf("failed to write sysctl config file %s: %v", filePath, err)
		errorsCollected = append(errorsCollected, errMsg)
		// No point in reloading if write failed.
	} else {
		hostCtxLogger.Successf("Successfully wrote sysctl parameters to %s.", filePath)
		if s.shouldReload() {
			reloadCmd := ""
			// `sysctl --system` reloads from all standard locations like /etc/sysctl.d/, /usr/lib/sysctl.d, /etc/sysctl.conf
			// `sysctl -p <file>` applies only the specified file.
			// If we write to a file in /etc/sysctl.d/, using `sysctl --system` is often preferred.
			if strings.HasPrefix(filePath, "/etc/sysctl.d/") || strings.HasPrefix(filePath, "/usr/lib/sysctl.d/") {
				reloadCmd = "sysctl --system"
			} else if filePath == "/etc/sysctl.conf" { // If writing directly to the main file
				reloadCmd = "sysctl -p /etc/sysctl.conf" // or just "sysctl -p"
			} else { // Custom file path not in standard dirs
				reloadCmd = fmt.Sprintf("sysctl -p %s", filePath)
			}

			hostCtxLogger.Infof("Reloading sysctl configuration using: '%s'", reloadCmd)
			// Sudo is required for `sysctl -p` or `sysctl --system` to apply changes.
			_, stderrReload, errReload := ctx.Host.Runner.Run(ctx.GoContext, reloadCmd, true)
			if errReload != nil {
				errMsg := fmt.Sprintf("failed to reload sysctl configuration with '%s': %v (stderr: %s)", reloadCmd, errReload, string(stderrReload))
				errorsCollected = append(errorsCollected, errMsg)
			} else {
				hostCtxLogger.Successf("Sysctl configuration reloaded successfully using '%s'.", reloadCmd)
			}
		} else {
			hostCtxLogger.Infof("Skipping sysctl reload as per step configuration.")
		}
	}

	res.EndTime = time.Now()
	if len(errorsCollected) > 0 {
		res.Status = "Failed"
		res.Error = fmt.Errorf(strings.Join(errorsCollected, "; "))
		res.Message = res.Error.Error()
		hostCtxLogger.Errorf("Step finished with errors: %s", res.Message)
	} else {
		// Final verification after apply (if reload was done or params are immediately effective)
		// This is important because `sysctl -p <file>` might not error if a key in the file is invalid.
		hostCtxLogger.Infof("Verifying sysctl parameters after apply...")
		allSet, checkErr := s.Check(ctx) // Re-run Check logic
		if checkErr != nil {
			res.Status = "Failed"
			res.Error = fmt.Errorf("failed to verify sysctl params after apply: %w", checkErr)
			res.Message = res.Error.Error()
			hostCtxLogger.Errorf("Step verification failed: %s", res.Message)
		} else if !allSet {
			// This means some parameters didn't stick or weren't readable correctly.
			// It might be useful to log which specific parameters failed verification.
			res.Status = "Failed"
			res.Error = fmt.Errorf("sysctl params not all set to desired values after apply and reload attempt")
			res.Message = res.Error.Error()
			hostCtxLogger.Errorf("Step verification failed: %s. Some parameters may not have applied correctly.", res.Message)
		} else {
			res.Status = "Succeeded"
			res.Message = fmt.Sprintf("All sysctl parameters (%s) successfully set and verified.", strings.Join(appliedParams, ", "))
			hostCtxLogger.Successf("Step succeeded: %s", res.Message)
		}
	}
	return res
}

var _ step.Step = &SetSystemConfigStep{}
