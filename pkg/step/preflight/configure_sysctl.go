package preflight

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/kubexms/kubexms/pkg/connector"
	"github.com/kubexms/kubexms/pkg/runtime"
	"github.com/kubexms/kubexms/pkg/step"
	"github.com/kubexms/kubexms/pkg/step/spec"
)

// SysctlSetting defines a key-value pair for sysctl.
type SysctlSetting struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// ApplySysctlSettingsStepSpec defines the specification for applying sysctl settings.
type ApplySysctlSettingsStepSpec struct {
	InitialSettings []SysctlSetting `json:"initialSettings,omitempty"`
	SedCommands     []string        `json:"sedCommands,omitempty"`
	TargetConfFile  string          `json:"targetConfFile,omitempty"` // Default /etc/sysctl.conf
}

// GetName returns the name of the step.
func (s *ApplySysctlSettingsStepSpec) GetName() string {
	return "Apply Sysctl Settings"
}

// PopulateDefaults fills the spec with default values from the bash script.
// This method should be called after the spec is unmarshalled from any user input.
func (s *ApplySysctlSettingsStepSpec) PopulateDefaults() {
	if s.TargetConfFile == "" {
		s.TargetConfFile = "/etc/sysctl.conf"
	}

	// Defaults if InitialSettings and SedCommands are empty
	if len(s.InitialSettings) == 0 {
		s.InitialSettings = []SysctlSetting{
			{Key: "net.ipv4.ip_forward", Value: "1"},
			{Key: "net.ipv6.conf.all.forwarding", Value: "1"},
			{Key: "net.bridge.bridge-nf-call-iptables", Value: "1"},
			{Key: "net.bridge.bridge-nf-call-ip6tables", Value: "1"},
			{Key: "net.ipv4.conf.all.rp_filter", Value: "0"}, // From common_config.sh, different from direct sed
			{Key: "net.ipv4.conf.default.rp_filter", Value: "0"}, // From common_config.sh
			{Key: "fs.may_detach_mounts", Value: "1"},
			{Key: "vm.overcommit_memory", Value: "1"},
			// {Key: "vm.panic_on_oom", Value: "0"}, // This is often debated, ensure it's desired
			{Key: "vm.max_map_count", Value: "262144"}, // For Elasticsearch, etc.
			// net.core.somaxconn = 32768 (Handled by sed)
			// vm.swappiness = 1 (Often set, but original script doesn't directly echo this)
		}
	}

	if len(s.SedCommands) == 0 {
		s.SedCommands = []string{
			fmt.Sprintf("sed -r -i 's@^#?(net.ipv4.conf.all.rp_filter=).*@\\10@g' %s", s.TargetConfFile),
			fmt.Sprintf("sed -r -i 's@^#?(net.ipv4.conf.default.rp_filter=).*@\\10@g' %s", s.TargetConfFile),
			fmt.Sprintf("sed -r -i 's@^#?(net.core.somaxconn=).*@\\165536@g' %s", s.TargetConfFile), // Script has 65536, common_config has 1024, choosing higher
			// The script also has these, which are covered by InitialSettings or other steps:
			// sed -r -i 's@^#?(net.ipv4.ip_forward=).*@\\11@g' /etc/sysctl.conf
			// sed -r -i 's@^#?(net.bridge.bridge-nf-call-iptables=).*@\\11@g' /etc/sysctl.conf
		}
	}
}

// ApplySysctlSettingsStepExecutor implements the logic for applying sysctl settings.
type ApplySysctlSettingsStepExecutor struct{}

// Check determines if the sysctl settings are already applied.
func (e *ApplySysctlSettingsStepExecutor) Check(ctx runtime.Context) (isDone bool, err error) {
	currentFullSpec, ok := ctx.Step().GetCurrentStepSpec()
	if !ok {
		return false, fmt.Errorf("StepSpec not found in context for ApplySysctlSettingsStep Check")
	}
	spec, ok := currentFullSpec.(*ApplySysctlSettingsStepSpec)
	if !ok {
		return false, fmt.Errorf("unexpected StepSpec type for ApplySysctlSettingsStep Check: %T", currentFullSpec)
	}
	spec.PopulateDefaults() // Ensure defaults are populated

	hostCtxLogger := ctx.Logger.SugaredLogger().With("host", ctx.Host.Name, "step_spec", spec.GetName()).Sugar()

	// Check live values for InitialSettings
	for _, setting := range spec.InitialSettings {
		// sysctl -n strips whitespace, so our value should also be stripped for comparison.
		expectedValue := strings.TrimSpace(setting.Value)
		// sysctl keys sometimes have spaces, sometimes dots. Runner.Run should handle it.
		// Using RunWithOptions with Sudo false as sysctl read usually doesn't need it.
		cmd := fmt.Sprintf("sysctl -n %s", strings.TrimSpace(setting.Key))
		stdout, _, err := ctx.Host.Runner.RunWithOptions(ctx.GoContext, cmd, &connector.ExecOptions{Sudo: false, TrimOutput: true})
		if err != nil {
			// If a key doesn't exist, sysctl might error. This implies it's not set.
			hostCtxLogger.Warnf("Failed to get sysctl value for key '%s': %v. Assuming not set correctly.", setting.Key, err)
			return false, nil
		}
		currentValue := strings.TrimSpace(stdout)

		if currentValue != expectedValue {
			hostCtxLogger.Infof("Sysctl check failed for key '%s': expected '%s', got '%s'.", setting.Key, expectedValue, currentValue)
			return false, nil
		}
		hostCtxLogger.Debugf("Sysctl check passed for key '%s': value is '%s'.", setting.Key, currentValue)
	}

	// Checking SedCommands is complex as it requires parsing sed.
	// The primary check is on live values. If sysctl.conf was correctly modified
	// and `sysctl -p` was run, live values should reflect it.
	// A more thorough check could read sysctl.conf and try to match sed patterns, but that's extensive.
	hostCtxLogger.Infof("All checked live sysctl values match expected initial settings. Sed commands effect is assumed if live values are correct after 'sysctl -p'.")
	return true, nil
}

// Execute applies the sysctl settings.
func (e *ApplySysctlSettingsStepExecutor) Execute(ctx runtime.Context) *step.Result {
	startTime := time.Now()
	currentFullSpec, ok := ctx.Step().GetCurrentStepSpec()
	if !ok {
		return step.NewResult(ctx, startTime, fmt.Errorf("StepSpec not found in context for ApplySysctlSettingsStep Execute"))
	}
	spec, ok := currentFullSpec.(*ApplySysctlSettingsStepSpec)
	if !ok {
		return step.NewResult(ctx, startTime, fmt.Errorf("unexpected StepSpec type for ApplySysctlSettingsStep Execute: %T", currentFullSpec))
	}
	spec.PopulateDefaults() // Ensure defaults are populated

	hostCtxLogger := ctx.Logger.SugaredLogger().With("host", ctx.Host.Name, "step_spec", spec.GetName()).Sugar()
	res := step.NewResult(ctx, startTime, nil)

	// 1. Append Initial Settings to TargetConfFile
	hostCtxLogger.Infof("Appending initial sysctl settings to %s...", spec.TargetConfFile)
	confContentBytes, err := ctx.Host.Runner.ReadFile(ctx.GoContext, spec.TargetConfFile)
	var confContent string
	if err != nil {
		hostCtxLogger.Warnf("Could not read %s to check for existing lines (error: %v). Will proceed with appending. This may create duplicates if file exists but is unreadable by current user.", spec.TargetConfFile, err)
		confContent = "" // Assume empty or non-existent, so all lines will be new
	} else {
		confContent = string(confContentBytes)
	}

	var settingsToAppend []string
	for _, setting := range spec.InitialSettings {
		line := fmt.Sprintf("%s = %s", strings.TrimSpace(setting.Key), strings.TrimSpace(setting.Value))
		// Check if line (exactly) or key= (more loosely) already exists and is not commented.
		// A simple check for exact line:
		if !strings.Contains(confContent, line) { // This is a basic check. A more robust one would parse key-value.
			settingsToAppend = append(settingsToAppend, line)
		} else {
			hostCtxLogger.Debugf("Line '%s' already found or similar in %s, skipping append.", line, spec.TargetConfFile)
		}
	}
	if len(settingsToAppend) > 0 {
		// Using a single echo command with multiple lines. Sudo needed for /etc/sysctl.conf
		echoCmd := fmt.Sprintf("printf '%%s\\n' '%s' >> %s", strings.Join(settingsToAppend, "' '"), spec.TargetConfFile)
		if _, _, err := ctx.Host.Runner.RunWithOptions(ctx.GoContext, echoCmd, &connector.ExecOptions{Sudo: true}); err != nil {
			res.Error = fmt.Errorf("failed to append initial settings to %s: %w", spec.TargetConfFile, err)
			res.Status = step.StatusFailed; hostCtxLogger.Errorf("Step failed: %v", res.Error); return res
		}
		hostCtxLogger.Infof("Appended %d initial settings to %s.", len(settingsToAppend), spec.TargetConfFile)
	} else {
		hostCtxLogger.Infof("No new initial settings needed to be appended to %s.", spec.TargetConfFile)
	}

	// 2. Apply Sed Commands
	hostCtxLogger.Infof("Applying %d sed commands to %s...", len(spec.SedCommands), spec.TargetConfFile)
	for i, sedCmd := range spec.SedCommands {
		hostCtxLogger.Debugf("Executing sed command #%d: %s", i+1, sedCmd)
		// Sudo is required for sed commands modifying /etc/sysctl.conf
		if _, _, err := ctx.Host.Runner.RunWithOptions(ctx.GoContext, sedCmd, &connector.ExecOptions{Sudo: true}); err != nil {
			// Some sed commands might "fail" if pattern not found, but that's not an error for us.
			// Runner.Run should ideally not error if exit code is 0.
			hostCtxLogger.Warnf("Sed command '%s' finished (possibly with non-critical errors if pattern not found): %v", sedCmd, err)
		}
	}
	hostCtxLogger.Infof("Sed commands applied to %s.", spec.TargetConfFile)

	// 3. Deduplicate TargetConfFile
	hostCtxLogger.Infof("Deduplicating %s...", spec.TargetConfFile)
	// Create a temporary file name. Using timestamp for uniqueness.
	// Note: mktemp on remote host via runner would be safer.
	// For simplicity, constructing a path; ensure Runner.Run can handle this.
	// A more robust solution would use `mktemp` on the target host.
	// Let's assume /tmp is writable by the user or sudo will handle it.
	tmpFileName := filepath.Join("/tmp", fmt.Sprintf("sysctl.dedup.%d.tmp", time.Now().UnixNano()))

	awkCmd := fmt.Sprintf("awk '!x[$0]++' %s > %s", spec.TargetConfFile, tmpFileName)
	mvCmd := fmt.Sprintf("mv %s %s", tmpFileName, spec.TargetConfFile)
	// Ensure these operations are done with appropriate permissions (sudo)
	if _, _, err := ctx.Host.Runner.RunWithOptions(ctx.GoContext, awkCmd, &connector.ExecOptions{Sudo: true}); err != nil {
		res.Error = fmt.Errorf("failed to run awk for deduplication on %s: %w", spec.TargetConfFile, err)
		res.Status = step.StatusFailed; hostCtxLogger.Errorf("Step failed: %v", res.Error); return res
	}
	hostCtxLogger.Debugf("Deduplicated content written to %s", tmpFileName)

	if _, _, err := ctx.Host.Runner.RunWithOptions(ctx.GoContext, mvCmd, &connector.ExecOptions{Sudo: true}); err != nil {
		ctx.Host.Runner.Run(ctx.GoContext, fmt.Sprintf("rm -f %s", tmpFileName), true)
		res.Error = fmt.Errorf("failed to move deduplicated file %s to %s: %w", tmpFileName, spec.TargetConfFile, err)
		res.Status = step.StatusFailed; hostCtxLogger.Errorf("Step failed: %v", res.Error); return res
	}
	hostCtxLogger.Infof("%s deduplicated successfully.", spec.TargetConfFile)


	// 4. Apply Settings Live
	hostCtxLogger.Infof("Applying sysctl settings live (sysctl -p %s)...", spec.TargetConfFile)
	sysctlCmd := fmt.Sprintf("sysctl -p %s", spec.TargetConfFile)
	stdoutSysctl, stderrSysctl, errSysctl := ctx.Host.Runner.RunWithOptions(ctx.GoContext, sysctlCmd, &connector.ExecOptions{Sudo: true})
	if errSysctl != nil {
		hostCtxLogger.Warnf("'sysctl -p' command finished with error: %v. Stdout: %s, Stderr: %s. Some settings may not have applied.", errSysctl, stdoutSysctl, stderrSysctl)
	} else {
		hostCtxLogger.Infof("'sysctl -p %s' executed successfully. Live settings updated.", spec.TargetConfFile)
		if stdoutSysctl != "" { hostCtxLogger.Debugf("sysctl -p stdout: %s", stdoutSysctl) }
		if stderrSysctl != "" { hostCtxLogger.Debugf("sysctl -p stderr: %s", stderrSysctl) }
	}

	done, checkErr := e.Check(ctx) // Pass context
	if checkErr != nil {
		res.Error = fmt.Errorf("post-execution check failed: %w", checkErr)
		res.Status = step.StatusFailed
		hostCtxLogger.Errorf("Step failed verification: %v", res.Error)
		return res
	}
	if !done {
		errMsg := "post-execution check indicates sysctl settings are not correctly applied"
		res.Error = fmt.Errorf(errMsg)
		res.Status = step.StatusFailed
		hostCtxLogger.Errorf("Step failed verification: %s", errMsg)
		return res
	}

	res.Message = "Sysctl settings applied successfully."
	// hostCtxLogger.Successf("Step succeeded: %s", res.Message) // Redundant as NewResult sets status
	return res
}

func init() {
	step.Register(step.GetSpecTypeName(&ApplySysctlSettingsStepSpec{}), &ApplySysctlSettingsStepExecutor{})
}
