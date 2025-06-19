package preflight

import (
	"context" // Required by runtime.Context
	"fmt"
	"strings"
	"time"

	"github.com/kubexms/kubexms/pkg/connector" // For connector.ExecOptions
	"github.com/kubexms/kubexms/pkg/runtime"
	"github.com/kubexms/kubexms/pkg/spec"
	"github.com/kubexms/kubexms/pkg/step"
)

// SetSystemConfigStepSpec defines parameters for setting sysctl kernel parameters.
type SetSystemConfigStepSpec struct {
	Params         map[string]string
	ConfigFilePath string // If empty, a default like /etc/sysctl.d/99-kubexms-preflight.conf is used
	Reload         *bool  // Pointer to distinguish between not set (use default true) and explicitly false.
	StepName       string // Optional: for a custom name for this specific step instance
}

// GetName returns the step name.
func (s *SetSystemConfigStepSpec) GetName() string {
	if s.StepName != "" { return s.StepName }
	return fmt.Sprintf("Set System Kernel Parameters (sysctl) to file: %s", s.effectiveConfigFilePath())
}

// effectiveConfigFilePath returns the actual config file path to be used.
func (s *SetSystemConfigStepSpec) effectiveConfigFilePath() string {
	if s.ConfigFilePath != "" { return s.ConfigFilePath }
	return "/etc/sysctl.d/99-kubexms-preflight.conf" // Default file
}

// shouldReload determines if sysctl configuration should be reloaded.
func (s *SetSystemConfigStepSpec) shouldReload() bool {
	if s.Reload == nil {
		return true // Default to true if not specified
	}
	return *s.Reload
}
var _ spec.StepSpec = &SetSystemConfigStepSpec{}

// SetSystemConfigStepExecutor implements the logic for SetSystemConfigStepSpec.
type SetSystemConfigStepExecutor struct{}

func init() {
	step.Register(step.GetSpecTypeName(&SetSystemConfigStepSpec{}), &SetSystemConfigStepExecutor{})
}

// Check determines if sysctl params are already set.
func (e *SetSystemConfigStepExecutor) Check(ctx runtime.Context) (isDone bool, err error) {
	currentFullSpec, ok := ctx.Step().GetCurrentStepSpec()
	if !ok {
		return false, fmt.Errorf("StepSpec not found in context for SetSystemConfigStep Check")
	}
	spec, ok := currentFullSpec.(*SetSystemConfigStepSpec)
	if !ok {
		return false, fmt.Errorf("unexpected StepSpec type for SetSystemConfigStep Check: %T", currentFullSpec)
	}

	if len(spec.Params) == 0 {
		logger := ctx.Logger // Use logger from context
		if logger != nil && logger.SugaredLogger != nil {
			logger.SugaredLogger.Debugf("No sysctl params specified for step '%s' on host %s, considering done.", spec.GetName(), ctx.Host.Name)
		} else {
			fmt.Printf("Warning: Logger not available for step '%s'\n", spec.GetName())
		}
		return true, nil
	}
	if ctx.Host == nil || ctx.Host.Runner == nil { // Added ctx.Host nil check
		return false, fmt.Errorf("host or runner not available in context")
	}
	hostCtxLogger := ctx.Logger.SugaredLogger().With("host", ctx.Host.Name, "step_spec", spec.GetName()).Sugar()

	for key, expectedValue := range spec.Params {
		cmd := fmt.Sprintf("sysctl -n %s", key)
		// Sudo not usually needed for reading sysctl values.
		stdoutBytes, stderrBytes, execErr := ctx.Host.Runner.RunWithOptions(ctx.GoContext, cmd, &connector.ExecOptions{Sudo: false})
		if execErr != nil {
			hostCtxLogger.Warnf("Failed to read current value of sysctl key '%s': %v (stderr: %s). Assuming not set as expected.", key, execErr, string(stderrBytes))
			// If we can't read the current value, we can't confirm it's correctly set.
			// So, it's not "done", and an error occurred during the check.
			return false, fmt.Errorf("failed to read current value of sysctl key '%s' on host %s: %w (stderr: %s)", key, ctx.Host.Name, execErr, string(stderrBytes))
		}
		currentValue := strings.TrimSpace(string(stdoutBytes))
		if currentValue != expectedValue {
			hostCtxLogger.Debugf("Check: Sysctl param %s is currently %q, want %q.", key, currentValue, expectedValue)
			return false, nil // Mismatch found, not done.
		}
		hostCtxLogger.Debugf("Check: Sysctl param %s is already %q.", key, currentValue)
	}
	hostCtxLogger.Infof("All specified sysctl parameters already match desired values.")
	return true, nil // All params match
}

// Execute applies sysctl params.
func (e *SetSystemConfigStepExecutor) Execute(ctx runtime.Context) *step.Result {
	startTime := time.Now()
	currentFullSpec, ok := ctx.Step().GetCurrentStepSpec()
	if !ok {
		return step.NewResult(ctx, startTime, fmt.Errorf("StepSpec not found in context for SetSystemConfigStep Execute"))
	}
	spec, ok := currentFullSpec.(*SetSystemConfigStepSpec)
	if !ok {
		return step.NewResult(ctx, startTime, fmt.Errorf("unexpected StepSpec type for SetSystemConfigStep Execute: %T", currentFullSpec))
	}

	res := step.NewResult(ctx, startTime, nil) // Initialize with nil error
	hostCtxLogger := ctx.Logger.SugaredLogger().With("host", ctx.Host.Name, "step_spec", spec.GetName()).Sugar()
	var errorsCollected []string
	var appliedParams []string

	if len(spec.Params) == 0 {
		res.Message = "No sysctl parameters to set."
		// Status is already Succeeded by NewResult(ctx, startTime, nil)
		hostCtxLogger.Infof(res.Message)
		res.EndTime = time.Now(); return res // Update EndTime if returning early
	}
	if ctx.Host == nil || ctx.Host.Runner == nil { // Added ctx.Host nil check
		res.Error = fmt.Errorf("host or runner not available in context")
		res.Status = step.StatusFailed; res.Message = res.Error.Error(); hostCtxLogger.Errorf("Step failed: %v", res.Error); return res
	}

	var configFileContent strings.Builder
	configFileContent.WriteString(fmt.Sprintf("# Kernel parameters configured by KubeXMS Preflight (%s)\n", time.Now().Format(time.RFC3339)))
	for key, value := range spec.Params {
		configFileContent.WriteString(fmt.Sprintf("%s = %s\n", key, value))
		appliedParams = append(appliedParams, fmt.Sprintf("%s=%s", key, value))
	}

	filePath := spec.effectiveConfigFilePath()
	hostCtxLogger.Infof("Writing sysctl parameters to %s: %s", filePath, strings.Join(appliedParams, ", "))

	// Sudo is required to write to system directories like /etc/sysctl.d/.
	err := ctx.Host.Runner.WriteFile(ctx.GoContext, []byte(configFileContent.String()), filePath, "0644", true)
	if err != nil {
		errMsg := fmt.Sprintf("failed to write sysctl config file %s: %v", filePath, err)
		errorsCollected = append(errorsCollected, errMsg)
	} else {
		hostCtxLogger.Successf("Successfully wrote sysctl parameters to %s.", filePath)
		if spec.shouldReload() {
			reloadCmd := ""
			if strings.HasPrefix(filePath, "/etc/sysctl.d/") || strings.HasPrefix(filePath, "/usr/lib/sysctl.d/") || filePath == "/run/sysctl.d/" {
				reloadCmd = "sysctl --system"
			} else if filePath == "/etc/sysctl.conf" {
			    reloadCmd = "sysctl -p /etc/sysctl.conf" // or just "sysctl -p"
			} else {
				reloadCmd = fmt.Sprintf("sysctl -p %s", filePath)
			}

			hostCtxLogger.Infof("Reloading sysctl configuration using: '%s'", reloadCmd)
			// Sudo is required for `sysctl -p` or `sysctl --system`.
			_, stderrReload, errReload := ctx.Host.Runner.RunWithOptions(ctx.GoContext, reloadCmd, &connector.ExecOptions{Sudo: true})
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
	// res.EndTime is set by NewResult, but update to be precise after all actions
	res.EndTime = time.Now()

	if len(errorsCollected) > 0 {
		res.Error = fmt.Errorf(strings.Join(errorsCollected, "; ")) // Set main error
		res.Status = step.StatusFailed // Explicitly set Failed status
		res.Message = res.Error.Error()
		hostCtxLogger.Errorf("Step finished with errors: %s", res.Message)
	} else {
		hostCtxLogger.Infof("Verifying sysctl parameters after apply...")
		allSet, checkErr := e.Check(ctx) // Pass context
		if checkErr != nil {
			res.Error = fmt.Errorf("failed to verify sysctl params after apply: %w", checkErr)
			res.Status = step.StatusFailed // Set status if checkErr occurred
			res.Message = res.Error.Error()
			hostCtxLogger.Errorf("Step verification failed: %s", res.Message)
		} else if !allSet {
			res.Error = fmt.Errorf("sysctl params not all set to desired values after apply and reload attempt")
			res.Status = step.StatusFailed // Set status if not all set
			res.Message = res.Error.Error()
			hostCtxLogger.Errorf("Step verification failed: %s. Some parameters may not have applied correctly.", res.Message)
		} else {
			// Status is already Succeeded if errorsCollected is empty and checkErr is nil
			res.Message = fmt.Sprintf("All sysctl parameters (%s) successfully set and verified.", strings.Join(appliedParams, ", "))
			hostCtxLogger.Successf("Step succeeded: %s", res.Message)
		}
	}
	return res
}
var _ step.StepExecutor = &SetSystemConfigStepExecutor{}
