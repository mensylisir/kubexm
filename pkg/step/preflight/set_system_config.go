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
func (e *SetSystemConfigStepExecutor) Check(s spec.StepSpec, ctx *runtime.Context) (isDone bool, err error) {
	spec, ok := s.(*SetSystemConfigStepSpec)
	if !ok {
		return false, fmt.Errorf("unexpected spec type %T for SetSystemConfigStepExecutor Check method", s)
	}
	if len(spec.Params) == 0 {
		if ctx.Logger != nil {
			ctx.Logger.Debugf("No sysctl params specified for step '%s' on host %s, considering done.", spec.GetName(), ctx.Host.Name)
		}
		return true, nil
	}
	if ctx.Host.Runner == nil {
		return false, fmt.Errorf("runner not available in context for host %s", ctx.Host.Name)
	}
	hostCtxLogger := ctx.Logger.SugaredLogger.With("host", ctx.Host.Name, "step_spec", spec.GetName()).Sugar()

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
func (e *SetSystemConfigStepExecutor) Execute(s spec.StepSpec, ctx *runtime.Context) *step.Result {
	spec, ok := s.(*SetSystemConfigStepSpec)
	if !ok {
		myErr := fmt.Errorf("Execute: unexpected spec type %T for SetSystemConfigStepExecutor", s)
		stepName := "SetSystemConfig (type error)"
		if s != nil { stepName = s.GetName() }
		return step.NewResult(stepName, ctx.Host.Name, time.Now(), myErr)
	}

	startTime := time.Now()
	res := step.NewResult(spec.GetName(), ctx.Host.Name, startTime, nil)
	hostCtxLogger := ctx.Logger.SugaredLogger.With("host", ctx.Host.Name, "step_spec", spec.GetName()).Sugar()
	var errorsCollected []string
	var appliedParams []string

	if len(spec.Params) == 0 {
		res.Status = "Succeeded"; res.Message = "No sysctl parameters to set."
		hostCtxLogger.Infof(res.Message)
		res.EndTime = time.Now(); return res
	}
	if ctx.Host.Runner == nil {
		res.Error = fmt.Errorf("runner not available in context for host %s", ctx.Host.Name)
		res.Status = "Failed"; res.Message = res.Error.Error(); hostCtxLogger.Errorf("Step failed: %v", res.Error); return res
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
	res.EndTime = time.Now()

	if len(errorsCollected) > 0 {
		res.Status = "Failed"; res.Error = fmt.Errorf(strings.Join(errorsCollected, "; ")); res.Message = res.Error.Error()
		hostCtxLogger.Errorf("Step finished with errors: %s", res.Message)
	} else {
		hostCtxLogger.Infof("Verifying sysctl parameters after apply...")
		allSet, checkErr := e.Check(s, ctx)
		if checkErr != nil {
			res.Status = "Failed"; res.Error = fmt.Errorf("failed to verify sysctl params after apply: %w", checkErr)
			res.Message = res.Error.Error()
			hostCtxLogger.Errorf("Step verification failed: %s", res.Message)
		} else if !allSet {
			res.Status = "Failed"; res.Error = fmt.Errorf("sysctl params not all set to desired values after apply and reload attempt")
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
var _ step.StepExecutor = &SetSystemConfigStepExecutor{}
