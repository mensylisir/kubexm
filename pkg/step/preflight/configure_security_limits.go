package preflight

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/kubexms/kubexms/pkg/connector" // Though not directly used for options here, good for consistency
	"github.com/kubexms/kubexms/pkg/runtime"
	"github.com/kubexms/kubexms/pkg/step"
	"github.com/kubexms/kubexms/pkg/step/spec"
)

// SecurityLimitEntry defines a raw line to be added to the security limits file.
type SecurityLimitEntry struct {
	Line string `json:"line"` // Raw line, e.g., "* soft nofile 1048576"
}

// ApplySecurityLimitsStepSpec defines the specification for applying security limits.
type ApplySecurityLimitsStepSpec struct {
	InitialEntries []SecurityLimitEntry `json:"initialEntries,omitempty"`
	SedCommands    []string             `json:"sedCommands,omitempty"`
	TargetConfFile string               `json:"targetConfFile,omitempty"` // Default /etc/security/limits.conf
}

// GetName returns the name of the step.
func (s *ApplySecurityLimitsStepSpec) GetName() string {
	return "Apply Security Limits"
}

// PopulateDefaults fills the spec with default values.
func (s *ApplySecurityLimitsStepSpec) PopulateDefaults() {
	if s.TargetConfFile == "" {
		s.TargetConfFile = "/etc/security/limits.conf"
	}

	if len(s.InitialEntries) == 0 {
		s.InitialEntries = []SecurityLimitEntry{
			// From common_config.sh (lines echoed into limits.conf)
			{Line: "* soft nofile 1048576"},
			{Line: "* hard nofile 1048576"},
			{Line: "* soft nproc 65535"},
			{Line: "* hard nproc 65535"},
			{Line: "root soft nofile 1048576"},
			{Line: "root hard nofile 1048576"},
			{Line: "root soft nproc 65535"},
			{Line: "root hard nproc 65535"},
		}
	}

	if len(s.SedCommands) == 0 {
		// The reference script (common_config.sh) primarily echoes lines rather than using sed for limits.conf.
		// If sed commands were typical for this file, they'd be added here.
		// For example:
		// s.SedCommands = []string{
		// 	fmt.Sprintf("sed -r -i 's@^#?(some_limit_pattern=).*@\\1new_value@g' %s", s.TargetConfFile),
		// }
		// Since the script doesn't use sed for limits.conf, this will remain empty by default.
	}
}

// ApplySecurityLimitsStepExecutor implements the logic for applying security limits.
type ApplySecurityLimitsStepExecutor struct{}

// Check determines if the security limits are already applied in the config file.
func (e *ApplySecurityLimitsStepExecutor) Check(ctx runtime.Context) (isDone bool, err error) {
	currentFullSpec, ok := ctx.Step().GetCurrentStepSpec()
	if !ok {
		return false, fmt.Errorf("StepSpec not found in context for ApplySecurityLimitsStep Check")
	}
	spec, ok := currentFullSpec.(*ApplySecurityLimitsStepSpec)
	if !ok {
		return false, fmt.Errorf("unexpected StepSpec type for ApplySecurityLimitsStep Check: %T", currentFullSpec)
	}
	spec.PopulateDefaults()

	hostCtxLogger := ctx.Logger.SugaredLogger().With("host", ctx.Host.Name, "step_spec", spec.GetName()).Sugar()

	configContentBytes, err := ctx.Host.Runner.ReadFile(ctx.GoContext, spec.TargetConfFile)
	if err != nil {
		// If the file doesn't exist, it's definitely not configured.
		hostCtxLogger.Warnf("Failed to read security limits file %s: %v. Assuming configuration is not done.", spec.TargetConfFile, err)
		return false, nil // Don't return error, just indicate not done.
	}
	configContent := string(configContentBytes)

	// Check for InitialEntries
	for _, entry := range spec.InitialEntries {
		// Normalize entry line for checking (e.g. trim spaces, though defaults are clean)
		trimmedEntryLine := strings.TrimSpace(entry.Line)
		if !strings.Contains(configContent, trimmedEntryLine) {
			hostCtxLogger.Infof("Security limits check: line '%s' not found in %s.", trimmedEntryLine, spec.TargetConfFile)
			return false, nil
		}
		hostCtxLogger.Debugf("Security limits check: line '%s' found in %s.", trimmedEntryLine, spec.TargetConfFile)
	}

	// Checking SedCommands' effects is complex as it requires parsing sed logic.
	// If SedCommands were used, a more sophisticated check of the resulting lines would be needed.
	// Since our defaults don't use sed for limits, this part is simpler for now.
	if len(spec.SedCommands) > 0 {
		hostCtxLogger.Warnf("SedCommands are present in spec, but Check logic currently only verifies InitialEntries. A more robust check for sed effects might be needed.")
	}

	hostCtxLogger.Infof("All expected initial security limit entries found in %s.", spec.TargetConfFile)
	return true, nil
}

// Execute applies the security limits.
func (e *ApplySecurityLimitsStepExecutor) Execute(ctx runtime.Context) *step.Result {
	startTime := time.Now()
	currentFullSpec, ok := ctx.Step().GetCurrentStepSpec()
	if !ok {
		return step.NewResult(ctx, startTime, fmt.Errorf("StepSpec not found in context for ApplySecurityLimitsStep Execute"))
	}
	spec, ok := currentFullSpec.(*ApplySecurityLimitsStepSpec)
	if !ok {
		return step.NewResult(ctx, startTime, fmt.Errorf("unexpected StepSpec type for ApplySecurityLimitsStep Execute: %T", currentFullSpec))
	}
	spec.PopulateDefaults()

	hostCtxLogger := ctx.Logger.SugaredLogger().With("host", ctx.Host.Name, "step_spec", spec.GetName()).Sugar()
	res := step.NewResult(ctx, startTime, nil)

	// 1. Read current TargetConfFile to check for existing lines
	hostCtxLogger.Infof("Checking and appending initial security limit entries to %s...", spec.TargetConfFile)
	configContentBytes, err := ctx.Host.Runner.ReadFile(ctx.GoContext, spec.TargetConfFile)
	var configContent string
	if err != nil {
		hostCtxLogger.Warnf("Could not read %s to check for existing lines (error: %v). Will proceed with appending. This may create duplicates if file exists but is unreadable.", spec.TargetConfFile, err)
		configContent = "" // Assume empty or non-existent
	} else {
		configContent = string(configContentBytes)
	}

	var entriesToAppend []string
	for _, entry := range spec.InitialEntries {
		trimmedEntryLine := strings.TrimSpace(entry.Line)
		// Simple check: if line doesn't exist, add it.
		if !strings.Contains(configContent, trimmedEntryLine) {
			entriesToAppend = append(entriesToAppend, trimmedEntryLine)
		} else {
			hostCtxLogger.Debugf("Line '%s' already found in %s, skipping append.", trimmedEntryLine, spec.TargetConfFile)
		}
	}

	if len(entriesToAppend) > 0 {
		// Using a single echo command with multiple lines. Sudo needed for /etc/security/limits.conf
		// Ensure lines are properly quoted if they contain special characters, though our defaults don't.
		echoCmd := fmt.Sprintf("printf '%%s\\n' '%s' >> %s", strings.Join(entriesToAppend, "' '"), spec.TargetConfFile)
		if _, _, err := ctx.Host.Runner.RunWithOptions(ctx.GoContext, echoCmd, &connector.ExecOptions{Sudo: true}); err != nil {
			res.Error = fmt.Errorf("failed to append initial entries to %s: %w", spec.TargetConfFile, err)
			res.Status = step.StatusFailed; hostCtxLogger.Errorf("Step failed: %v", res.Error); return res
		}
		hostCtxLogger.Infof("Appended %d initial entries to %s.", len(entriesToAppend), spec.TargetConfFile)
	} else {
		hostCtxLogger.Infof("No new initial entries needed to be appended to %s.", spec.TargetConfFile)
	}

	// 2. Apply Sed Commands (if any)
	if len(spec.SedCommands) > 0 {
		hostCtxLogger.Infof("Applying %d sed commands to %s...", len(spec.SedCommands), spec.TargetConfFile)
		for i, sedCmd := range spec.SedCommands {
			hostCtxLogger.Debugf("Executing sed command #%d: %s", i+1, sedCmd)
			if _, _, err := ctx.Host.Runner.RunWithOptions(ctx.GoContext, sedCmd, &connector.ExecOptions{Sudo: true}); err != nil {
				hostCtxLogger.Warnf("Sed command '%s' finished (possibly with non-critical errors if pattern not found): %v", sedCmd, err)
			}
		}
		hostCtxLogger.Infof("Sed commands applied to %s.", spec.TargetConfFile)
	}

	// 3. Deduplicate TargetConfFile
	hostCtxLogger.Infof("Deduplicating %s...", spec.TargetConfFile)
	tmpFileName := filepath.Join("/tmp", fmt.Sprintf("security_limits.dedup.%d.tmp", time.Now().UnixNano()))
	awkCmd := fmt.Sprintf("awk '!x[$0]++' %s > %s", spec.TargetConfFile, tmpFileName)
	mvCmd := fmt.Sprintf("mv %s %s", tmpFileName, spec.TargetConfFile)

	if _, _, err := ctx.Host.Runner.RunWithOptions(ctx.GoContext, awkCmd, &connector.ExecOptions{Sudo: true}); err != nil {
		res.Error = fmt.Errorf("failed to run awk for deduplication on %s: %w", spec.TargetConfFile, err)
		res.Status = step.StatusFailed; hostCtxLogger.Errorf("Step failed: %v", res.Error); return res
	}
	hostCtxLogger.Debugf("Deduplicated content written to %s", tmpFileName)

	if _, _, err := ctx.Host.Runner.RunWithOptions(ctx.GoContext, mvCmd, &connector.ExecOptions{Sudo: true}); err != nil {
		ctx.Host.Runner.Run(ctx.GoContext, fmt.Sprintf("rm -f %s", tmpFileName), true) // Attempt cleanup
		res.Error = fmt.Errorf("failed to move deduplicated file %s to %s: %w", tmpFileName, spec.TargetConfFile, err)
		res.Status = step.StatusFailed; hostCtxLogger.Errorf("Step failed: %v", res.Error); return res
	}
	hostCtxLogger.Infof("%s deduplicated successfully.", spec.TargetConfFile)

	// Security limits are typically applied at next login. No direct reload command.
	hostCtxLogger.Info("Security limits configuration updated. Changes will apply at next user login/session.")

	// Perform a post-execution check
	done, checkErr := e.Check(ctx) // Pass context
	if checkErr != nil {
		res.Error = fmt.Errorf("post-execution check failed: %w", checkErr)
		res.Status = step.StatusFailed
		hostCtxLogger.Errorf("Step failed verification: %v", res.Error)
		return res
	}
	if !done {
		errMsg := fmt.Sprintf("post-execution check indicates security limits in %s are not correctly applied", spec.TargetConfFile)
		res.Error = fmt.Errorf(errMsg)
		res.Status = step.StatusFailed
		hostCtxLogger.Errorf("Step failed verification: %s", errMsg)
		return res
	}

	res.Message = "Security limits applied successfully."
	// hostCtxLogger.Successf("Step succeeded: %s", res.Message) // Redundant
	return res
}

func init() {
	step.Register(step.GetSpecTypeName(&ApplySecurityLimitsStepSpec{}), &ApplySecurityLimitsStepExecutor{})
}
