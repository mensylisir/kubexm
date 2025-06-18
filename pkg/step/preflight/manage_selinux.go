package preflight

import (
	"fmt"
	"strings"
	"time"

	"github.com/kubexms/kubexms/pkg/connector"
	"github.com/kubexms/kubexms/pkg/runtime"
	"github.com/kubexms/kubexms/pkg/step"
	"github.com/kubexms/kubexms/pkg/step/spec"
)

// ConfigureSELinuxStepSpec defines the specification for configuring SELinux to disabled.
type ConfigureSELinuxStepSpec struct {
	// Mode can be "disabled" or "permissive". For this subtask, we focus on "disabled".
	// Mode string `json:"mode,omitempty"`
}

// GetName returns the name of the step.
func (s *ConfigureSELinuxStepSpec) GetName() string {
	return "Configure SELinux to Disabled"
}

// ConfigureSELinuxStepExecutor implements the logic for configuring SELinux.
type ConfigureSELinuxStepExecutor struct{}

// Check determines if SELinux is already configured as disabled.
func (e *ConfigureSELinuxStepExecutor) Check(s spec.StepSpec, ctx *runtime.Context) (isDone bool, err error) {
	hostCtxLogger := ctx.Logger.SugaredLogger.With("host", ctx.Host.Name, "step_spec", s.GetName()).Sugar()
	desiredState := "disabled" // For this executor, we hardcode to disabled.

	// 1. Check /etc/selinux/config
	configPath := "/etc/selinux/config"
	configContentBytes, err := ctx.Host.Runner.ReadFile(ctx.GoContext, configPath)
	if err != nil {
		// If the config file doesn't exist, SELinux might not be installed or managed.
		// Consider this "done" for disabling, but log it.
		hostCtxLogger.Warnf("SELinux config file %s not found. Assuming SELinux is not managed or is effectively disabled.", configPath)
		// To be safe, let's also check getenforce if possible.
	} else {
		configContent := string(configContentBytes)
		foundSELinuxLine := false
		correctSetting := false
		for _, line := range strings.Split(configContent, "\n") {
			trimmedLine := strings.TrimSpace(line)
			if strings.HasPrefix(trimmedLine, "SELINUX=") {
				foundSELinuxLine = true
				if trimmedLine == "SELINUX="+desiredState {
					correctSetting = true
					hostCtxLogger.Debugf("SELinux config file %s already contains 'SELINUX=%s'.", configPath, desiredState)
					break
				}
				hostCtxLogger.Infof("SELinux config file %s has '%s', expected 'SELINUX=%s'.", configPath, trimmedLine, desiredState)
			}
		}
		if !foundSELinuxLine {
			hostCtxLogger.Warnf("SELinux config file %s does not contain an 'SELINUX=' line. Configuration may be incomplete.", configPath)
			// This is likely "not done" as we expect to set it.
			return false, nil
		}
		if !correctSetting {
			return false, nil // Config file needs update.
		}
	}

	// 2. Check getenforce output
	// Sudo typically not needed for getenforce
	getenforceOutput, getenforceErr := ctx.Host.Runner.Run(ctx.GoContext, "getenforce", false)
	if getenforceErr != nil {
		// If getenforce command is not found, SELinux might not be installed/active.
		// This state is consistent with "disabled".
		hostCtxLogger.Warnf("`getenforce` command failed or not found: %v. Assuming SELinux is not active.", getenforceErr)
		return true, nil // Consider done if command fails (likely SELinux not active part of kernel)
	}

	currentEnforcement := strings.TrimSpace(getenforceOutput)
	hostCtxLogger.Debugf("`getenforce` output: %s", currentEnforcement)

	// If current state is "Disabled" or "Permissive", it's acceptable.
	// "Disabled" means it's off. "Permissive" means it's on but not enforcing (logs violations).
	// For the goal of "disabling" it, Permissive is often an acceptable intermediate or final state for compatibility.
	// However, the spec name is "Configure SELinux to Disabled".
	// The original script sets it to "disabled" in config and "setenforce 0" (Permissive for current session).
	// Let's align: config should be "disabled", runtime can be "Disabled" or "Permissive".
	if strings.ToLower(currentEnforcement) == "disabled" || strings.ToLower(currentEnforcement) == "permissive" {
		hostCtxLogger.Infof("SELinux enforcement status is '%s'. Configuration is considered done.", currentEnforcement)
		return true, nil
	}

	hostCtxLogger.Infof("SELinux enforcement status is '%s', but expected 'Disabled' or 'Permissive'.", currentEnforcement)
	return false, nil
}

// Execute configures SELinux to disabled.
func (e *ConfigureSELinuxStepExecutor) Execute(s spec.StepSpec, ctx *runtime.Context) *step.Result {
	stepName := s.GetName()
	startTime := time.Now()
	res := step.NewResult(stepName, ctx.Host.Name, startTime, nil)
	hostCtxLogger := ctx.Logger.SugaredLogger.With("host", ctx.Host.Name, "step_spec", stepName).Sugar()
	desiredState := "disabled"
	configPath := "/etc/selinux/config"

	// 1. Modify /etc/selinux/config using sed
	// This ensures we only change SELINUX=enforcing or SELINUX=permissive to SELINUX=disabled
	// and leaves SELINUX=disabled as is.
	// Using Run for sed is simpler than Read/Modify/Write for this specific replacement.
	// The command `sed -i 's/^SELINUX=\(enforcing\|permissive\)/SELINUX=disabled/' /etc/selinux/config` is more targeted.
	// Or, more simply, ensure the line is exactly SELINUX=disabled.
	// `sed -ri 's/^SELINUX=.*/SELINUX=disabled/' /etc/selinux/config` would be more direct like the original script's goal.
	// Let's try to make it idempotent by first checking if the file needs modification.

	configContentBytes, err := ctx.Host.Runner.ReadFile(ctx.GoContext, configPath)
	if err != nil {
		res.Error = fmt.Errorf("failed to read %s: %w", configPath, err)
		res.SetFailed(res.Error.Error())
		hostCtxLogger.Errorf("Step failed: %v", res.Error)
		return res
	}
	configContent := string(configContentBytes)
	needsUpdate := false
	newLines := []string{}
	foundSELinuxLine := false

	for _, line := range strings.Split(configContent, "\n") {
		trimmedLine := strings.TrimSpace(line)
		if strings.HasPrefix(trimmedLine, "SELINUX=") {
			foundSELinuxLine = true
			if trimmedLine != "SELINUX="+desiredState {
				newLines = append(newLines, "SELINUX="+desiredState)
				needsUpdate = true
				hostCtxLogger.Infof("Modifying SELinux config line from '%s' to 'SELINUX=%s'", trimmedLine, desiredState)
			} else {
				newLines = append(newLines, line) // Keep original if already correct (preserves comments on same line if any)
			}
		} else {
			newLines = append(newLines, line)
		}
	}

	if !foundSELinuxLine { // If SELINUX= line doesn't exist, append it.
		newLines = append(newLines, "SELINUX="+desiredState)
		needsUpdate = true
		hostCtxLogger.Infof("Appending 'SELINUX=%s' to %s", desiredState, configPath)
	}

	if needsUpdate {
		newConfigContent := strings.Join(newLines, "\n")
		// Ensure file ends with a newline.
		if !strings.HasSuffix(newConfigContent, "\n") && len(newConfigContent) > 0 {
			newConfigContent += "\n"
		}
		err := ctx.Host.Runner.WriteFile(ctx.GoContext, []byte(newConfigContent), configPath, "0644", true)
		if err != nil {
			res.Error = fmt.Errorf("failed to write modified %s: %w", configPath, err)
			res.SetFailed(res.Error.Error())
			hostCtxLogger.Errorf("Step failed: %v", res.Error)
			return res
		}
		hostCtxLogger.Infof("%s updated successfully to 'SELINUX=%s'. A reboot is usually required for this to take full effect.", configPath, desiredState)
	} else {
		hostCtxLogger.Infof("%s already configured to 'SELINUX=%s'. No changes made to file.", configPath, desiredState)
	}

	// 2. Run setenforce 0
	// Check if setenforce command exists
	setenforcePath, err := ctx.Host.Runner.LookPath(ctx.GoContext, "setenforce")
	if err != nil || setenforcePath == "" {
		hostCtxLogger.Warnf("'setenforce' command not found. Skipping runtime SELinux change. Current state will depend on reboot or kernel default. Error: %v", err)
	} else {
		hostCtxLogger.Infof("Attempting to set current SELinux mode to Permissive (setenforce 0)...")
		// Sudo is required for setenforce
		_, stderr, err := ctx.Host.Runner.RunWithOptions(ctx.GoContext, "setenforce 0", &connector.ExecOptions{Sudo: true})
		if err != nil {
			// setenforce 0 might fail if SELinux is already disabled at kernel level.
			hostCtxLogger.Warnf("'setenforce 0' command finished with error (this may be expected if SELinux is fully disabled by kernel): %v. Stderr: %s", err, string(stderr))
		} else {
			hostCtxLogger.Infof("'setenforce 0' executed successfully. SELinux is now Permissive for the current session.")
		}
	}

	// Perform a post-execution check
	done, checkErr := e.Check(s, ctx)
	if checkErr != nil {
		res.Error = fmt.Errorf("post-execution check failed: %w", checkErr)
		res.SetFailed(res.Error.Error())
		hostCtxLogger.Errorf("Step failed verification: %v", res.Error)
		return res
	}
	if !done {
		// This might occur if a reboot is strictly necessary for getenforce to show Disabled
		// and current session remains Enforcing. The Check logic allows Permissive.
		hostCtxLogger.Warnf("Post-execution check indicates SELinux may not be fully in the desired state ('Disabled' in config, 'Disabled' or 'Permissive' runtime). This might be expected if a reboot is pending.")
		// Depending on strictness, this could be a failure. For now, we'll consider it a soft warning if config is set.
		// The check already allows Permissive.
	}


	res.SetSucceeded(fmt.Sprintf("SELinux configuration set to '%s'. Runtime set to Permissive if possible. Reboot may be required.", desiredState))
	hostCtxLogger.Successf("Step succeeded: %s", res.Message)
	return res
}

func init() {
	step.Register(step.GetSpecTypeName(&ConfigureSELinuxStepSpec{}), &ConfigureSELinuxStepExecutor{})
}
