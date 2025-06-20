package preflight

import (
	// "context" // No longer directly used if runtime.StepContext is used
	"fmt"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/connector" // For connector.ExecOptions
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
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
func (e *SetSystemConfigStepExecutor) Check(ctx runtime.StepContext) (isDone bool, err error) { // Changed to StepContext
	logger := ctx.GetLogger()
	currentHost := ctx.GetHost()
	goCtx := ctx.GoContext()

	if currentHost == nil {
		logger.Error("Current host not found in context for Check")
		return false, fmt.Errorf("current host is required for SetSystemConfigStep Check")
	}
	logger = logger.With("host", currentHost.GetName())

	rawSpec, ok := ctx.StepCache().GetCurrentStepSpec()
	if !ok {
		logger.Error("StepSpec not found in context")
		return false, fmt.Errorf("StepSpec not found in context for SetSystemConfigStepExecutor Check method")
	}
	spec, ok := rawSpec.(*SetSystemConfigStepSpec)
	if !ok {
		logger.Error("Unexpected StepSpec type", "type", fmt.Sprintf("%T", rawSpec))
		return false, fmt.Errorf("unexpected spec type %T for SetSystemConfigStepExecutor Check method", rawSpec)
	}
	logger = logger.With("step", spec.GetName())

	if len(spec.Params) == 0 {
		logger.Debug("No sysctl params specified, considering done.")
		return true, nil
	}

	conn, err := ctx.GetConnectorForHost(currentHost)
	if err != nil {
		logger.Error("Failed to get connector for host", "error", err)
		return false, fmt.Errorf("failed to get connector for host %s: %w", currentHost.GetName(), err)
	}

	for key, expectedValue := range spec.Params {
		cmd := fmt.Sprintf("sysctl -n %s", key)
		stdoutBytes, stderrBytes, execErr := conn.RunCommand(goCtx, cmd, &connector.ExecOptions{Sudo: false}) // Use connector
		if execErr != nil {
			logger.Warn("Failed to read current value of sysctl key. Assuming not set as expected.", "key", key, "error", execErr, "stderr", string(stderrBytes))
			return false, fmt.Errorf("failed to read current value of sysctl key '%s' on host %s: %w (stderr: %s)", key, currentHost.GetName(), execErr, string(stderrBytes))
		}
		currentValue := strings.TrimSpace(string(stdoutBytes))
		if currentValue != expectedValue {
			logger.Debug("Sysctl param mismatch.", "key", key, "current", currentValue, "expected", expectedValue)
			return false, nil
		}
		logger.Debug("Sysctl param matches.", "key", key, "value", currentValue)
	}
	logger.Info("All specified sysctl parameters already match desired values.")
	return true, nil
}

// Execute applies sysctl params.
func (e *SetSystemConfigStepExecutor) Execute(ctx runtime.StepContext) *step.Result { // Changed to StepContext
	startTime := time.Now()
	logger := ctx.GetLogger()
	currentHost := ctx.GetHost()
	goCtx := ctx.GoContext()

	res := step.NewResult(ctx, currentHost, startTime, nil)

	if currentHost == nil {
		logger.Error("Current host not found in context for Execute")
		res.Error = fmt.Errorf("current host is required for SetSystemConfigStep Execute")
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	logger = logger.With("host", currentHost.GetName())

	rawSpec, ok := ctx.StepCache().GetCurrentStepSpec()
	if !ok {
		logger.Error("StepSpec not found in context")
		res.Error = fmt.Errorf("StepSpec not found in context for SetSystemConfigStepExecutor Execute method")
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	spec, ok := rawSpec.(*SetSystemConfigStepSpec)
	if !ok {
		logger.Error("Unexpected StepSpec type", "type", fmt.Sprintf("%T", rawSpec))
		res.Error = fmt.Errorf("unexpected spec type %T for SetSystemConfigStepExecutor Execute method", rawSpec)
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	logger = logger.With("step", spec.GetName())

	var errorsCollected []string
	var appliedParams []string

	if len(spec.Params) == 0 {
		res.Status = step.StatusSucceeded; res.Message = "No sysctl parameters to set."
		logger.Info(res.Message)
		res.EndTime = time.Now(); return res
	}

	conn, errConn := ctx.GetConnectorForHost(currentHost)
	if errConn != nil {
		logger.Error("Failed to get connector for host", "error", errConn)
		res.Error = fmt.Errorf("failed to get connector for host %s: %w", currentHost.GetName(), errConn)
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}

	var configFileContent strings.Builder
	configFileContent.WriteString(fmt.Sprintf("# Kernel parameters configured by KubeXMS Preflight (%s)\n", time.Now().Format(time.RFC3339)))
	for key, value := range spec.Params {
		configFileContent.WriteString(fmt.Sprintf("%s = %s\n", key, value))
		appliedParams = append(appliedParams, fmt.Sprintf("%s=%s", key, value))
	}

	filePath := spec.effectiveConfigFilePath()
	logger.Info("Writing sysctl parameters.", "file", filePath, "params", strings.Join(appliedParams, ", "))

	errWrite := conn.WriteFile(goCtx, []byte(configFileContent.String()), filePath, "0644") // Sudo handled by connector if path needs it
	if errWrite != nil {
		errMsg := fmt.Sprintf("failed to write sysctl config file %s: %v", filePath, errWrite)
		logger.Error(errMsg)
		errorsCollected = append(errorsCollected, errMsg)
	} else {
		logger.Info("Successfully wrote sysctl parameters.", "file", filePath)
		if spec.shouldReload() {
			reloadCmd := ""
			if strings.HasPrefix(filePath, "/etc/sysctl.d/") || strings.HasPrefix(filePath, "/usr/lib/sysctl.d/") || filePath == "/run/sysctl.d/" {
				reloadCmd = "sysctl --system"
			} else if filePath == "/etc/sysctl.conf" {
			    reloadCmd = "sysctl -p /etc/sysctl.conf"
			} else {
				reloadCmd = fmt.Sprintf("sysctl -p %s", filePath)
			}

			logger.Info("Reloading sysctl configuration.", "command", reloadCmd)
			_, stderrReload, errReload := conn.RunCommand(goCtx, reloadCmd, &connector.ExecOptions{Sudo: true}) // Use connector
			if errReload != nil {
				errMsg := fmt.Sprintf("failed to reload sysctl configuration with '%s': %v (stderr: %s)", reloadCmd, errReload, string(stderrReload))
				logger.Error(errMsg)
				errorsCollected = append(errorsCollected, errMsg)
			} else {
				logger.Info("Sysctl configuration reloaded successfully.", "command", reloadCmd)
			}
		} else {
			logger.Info("Skipping sysctl reload as per step configuration.")
		}
	}
	res.EndTime = time.Now()

	if len(errorsCollected) > 0 {
		res.Status = step.StatusFailed; res.Error = fmt.Errorf(strings.Join(errorsCollected, "; ")); res.Message = res.Error.Error()
		logger.Error("Step finished with errors.", "message", res.Message)
	} else {
		logger.Info("Verifying sysctl parameters after apply...")
		allSet, checkErr := e.Check(ctx)
		if checkErr != nil {
			logger.Error("Failed to verify sysctl params after apply.", "error", checkErr)
			res.Status = step.StatusFailed; res.Error = fmt.Errorf("failed to verify sysctl params after apply: %w", checkErr)
			res.Message = res.Error.Error()
		} else if !allSet {
			logger.Error("Sysctl params not all set to desired values after apply and reload attempt.")
			res.Status = step.StatusFailed; res.Error = fmt.Errorf("sysctl params not all set to desired values after apply and reload attempt")
			res.Message = res.Error.Error()
		} else {
			res.Status = step.StatusSucceeded
			res.Message = fmt.Sprintf("All sysctl parameters (%s) successfully set and verified.", strings.Join(appliedParams, ", "))
			logger.Info("Step succeeded.", "message", res.Message)
		}
	}
	return res
}
var _ step.StepExecutor = &SetSystemConfigStepExecutor{}
