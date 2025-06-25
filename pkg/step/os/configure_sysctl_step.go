package os

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

const (
	defaultSysctlConfFile = "/etc/sysctl.d/99-kubexm.conf"
)

// ConfigureSysctlStep applies kernel sysctl parameters and ensures they persist.
type ConfigureSysctlStep struct {
	meta     spec.StepMeta
	Params   map[string]string // Key-value pairs of sysctl settings (e.g., "net.ipv4.ip_forward": "1")
	Sudo     bool
	ConfFile string // Path to the sysctl configuration file for persistence
}

// NewConfigureSysctlStep creates a new ConfigureSysctlStep.
func NewConfigureSysctlStep(instanceName string, params map[string]string, sudo bool, confFile string) step.Step {
	name := instanceName
	if name == "" {
		name = "ConfigureSysctlParams"
	}
	cf := confFile
	if cf == "" {
		cf = defaultSysctlConfFile
	}
	return &ConfigureSysctlStep{
		meta: spec.StepMeta{
			Name:        name,
			Description: fmt.Sprintf("Configures kernel sysctl parameters and persists them to %s", cf),
		},
		Params:   params,
		Sudo:     sudo,
		ConfFile: cf,
	}
}

func (s *ConfigureSysctlStep) Meta() *spec.StepMeta {
	return &s.meta
}

// getSysctlValue retrieves the current value of a sysctl parameter.
func (s *ConfigureSysctlStep) getSysctlValue(ctx step.StepContext, runnerSvc runner.Runner, conn connector.Connector, paramKey string) (string, error) {
	// sysctl -n key
	// Sudo might not be needed for reading, but some keys might require it.
	// For simplicity, using configured Sudo for the read as well.
	cmd := fmt.Sprintf("sysctl -n %s", paramKey)
	execOpts := &connector.ExecOptions{Sudo: s.Sudo, Check: true} // Check to handle non-zero if key not found

	stdout, _, err := runnerSvc.RunWithOptions(ctx.GoContext(), conn, cmd, execOpts)
	if err != nil {
		// If CommandError with non-zero exit, key might not exist or be readable.
		return "", fmt.Errorf("failed to get sysctl value for %s: %w", paramKey, err)
	}
	return strings.TrimSpace(string(stdout)), nil
}

// areParamsInConfFile checks if all specified sysctl params are correctly set in the configuration file.
func (s *ConfigureSysctlStep) areParamsInConfFile(ctx step.StepContext, runnerSvc runner.Runner, conn connector.Connector) (bool, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", ctx.GetHost().GetName())

	fileExists, err := runnerSvc.Exists(ctx.GoContext(), conn, s.ConfFile)
	if err != nil {
		logger.Warn("Failed to check existence of sysctl config file. Assuming params not configured persistently.", "file", s.ConfFile, "error", err)
		return false, nil
	}
	if !fileExists {
		logger.Info("Sysctl config file does not exist. Params not configured persistently.", "file", s.ConfFile)
		return false, nil
	}

	contentBytes, err := runnerSvc.ReadFile(ctx.GoContext(), conn, s.ConfFile)
	if err != nil {
		logger.Warn("Failed to read sysctl config file. Assuming params not configured persistently.", "file", s.ConfFile, "error", err)
		return false, nil
	}
	content := string(contentBytes)
	lines := strings.Split(content, "\n")

	configuredParams := make(map[string]string)
	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" || strings.HasPrefix(trimmedLine, "#") || strings.HasPrefix(trimmedLine, ";") {
			continue
		}
		parts := strings.SplitN(trimmedLine, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			configuredParams[key] = value
		}
	}

	for key, expectedValue := range s.Params {
		currentValue, found := configuredParams[key]
		if !found || strings.TrimSpace(currentValue) != strings.TrimSpace(expectedValue) {
			logger.Info("Sysctl param not correctly configured in persistent file.", "key", key, "expected", expectedValue, "found_in_file", currentValue)
			return false, nil
		}
	}
	return true, nil
}


func (s *ConfigureSysctlStep) Precheck(ctx step.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Precheck")
	if len(s.Params) == 0 {
		logger.Info("No sysctl parameters specified.")
		return true, nil
	}

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	// 1. Check current runtime values
	allRuntimeMatch := true
	for key, expectedValue := range s.Params {
		currentValue, errVal := s.getSysctlValue(ctx, runnerSvc, conn, key)
		if errVal != nil {
			// If key doesn't exist, it needs to be set.
			logger.Info("Failed to get current sysctl value or key does not exist, needs configuration.", "key", key, "error", errVal)
			allRuntimeMatch = false
			break
		}
		if strings.TrimSpace(currentValue) != strings.TrimSpace(expectedValue) {
			logger.Info("Current sysctl runtime value mismatch.", "key", key, "expected", expectedValue, "current", currentValue)
			allRuntimeMatch = false
			break
		}
		logger.Debug("Sysctl runtime value matches.", "key", key, "value", currentValue)
	}

	if !allRuntimeMatch {
		return false, nil // Runtime values don't match, Run is needed.
	}

	// 2. Check persistent configuration file
	allPersistentMatch, errPersist := s.areParamsInConfFile(ctx, runnerSvc, conn)
	if errPersist != nil {
		logger.Warn("Error checking sysctl persistence, assuming not persistent.", "error", errPersist)
		return false, nil // If persistence check fails, better to run.
	}

	if allPersistentMatch {
		logger.Info("All specified sysctl parameters are correctly set at runtime and configured for persistence.")
		return true, nil
	}

	logger.Info("Sysctl parameters are correct at runtime, but not all are configured for persistence. Step needs to run.")
	return false, nil
}

func (s *ConfigureSysctlStep) Run(ctx step.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Run")
	if len(s.Params) == 0 {
		logger.Info("No sysctl parameters to configure.")
		return nil
	}

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}
	execOpts := &connector.ExecOptions{Sudo: s.Sudo}

	// 1. Apply sysctl parameters at runtime
	for key, value := range s.Params {
		logger.Info("Applying sysctl parameter.", "key", key, "value", value)
		// sysctl -w key="value" (ensure value is quoted if it contains spaces, though not typical for sysctl)
		// sysctl -w key=value (more common)
		// Using key=value for safety, assuming values don't need complex quoting for sysctl command itself.
		sysctlCmd := fmt.Sprintf("sysctl -w %s=\"%s\"", key, value) // Quoting value for safety
		_, stderr, errSysctl := runnerSvc.RunWithOptions(ctx.GoContext(), conn, sysctlCmd, execOpts)
		if errSysctl != nil {
			logger.Error("Failed to apply sysctl parameter.", "key", key, "value", value, "command", sysctlCmd, "error", errSysctl, "stderr", string(stderr))
			return fmt.Errorf("failed to apply sysctl %s=%s: %w. Stderr: %s", key, value, errSysctl, string(stderr))
		}
		logger.Info("Sysctl parameter applied successfully.", "key", key, "value", value)
	}

	// 2. Ensure parameters are persistent
	logger.Info("Ensuring sysctl parameters are persistent.", "file", s.ConfFile)
	confDir := filepath.Dir(s.ConfFile)
	if errMkdir := runnerSvc.Mkdirp(ctx.GoContext(), conn, confDir, "0755", s.Sudo); errMkdir != nil {
		return fmt.Errorf("failed to create directory %s for sysctl config: %w", confDir, errMkdir)
	}

	// Read existing content to avoid duplicate lines, or simply append and let sysctl -p handle it.
	// A robust way is to build the desired content and overwrite, or manage lines carefully.
	// For this implementation, we will append if not present, or ensure the line matches.

	// Fetch existing content
	var existingContent string
	fileExists, _ := runnerSvc.Exists(ctx.GoContext(), conn, s.ConfFile)
	if fileExists {
		contentBytes, errRead := runnerSvc.ReadFile(ctx.GoContext(), conn, s.ConfFile)
		if errRead != nil {
			logger.Warn("Could not read existing sysctl conf file, may create new or append duplicates.", "file", s.ConfFile, "error", errRead)
		} else {
			existingContent = string(contentBytes)
		}
	}

	var contentBuilder strings.Builder
	contentBuilder.WriteString(existingContent)
	if !strings.HasSuffix(existingContent, "\n") && existingContent != "" {
		contentBuilder.WriteString("\n")
	}

	needsWrite := !fileExists
	currentLines := strings.Split(existingContent, "\n")
	existingParamsMap := make(map[string]string)
	for _, line := range currentLines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, ";") {
			continue
		}
		parts := strings.SplitN(trimmed, "=", 2)
		if len(parts) == 2 {
			existingParamsMap[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}

	// Build the new content ensuring our params are present and correct
	// This is a bit naive; a proper config management tool would handle this better.
	// We'll just ensure our lines are there. If a key exists with a different value, this won't update it.
	// A better approach might be to remove lines for s.Params keys and then add them.
	// For now: Add if not exactly present.
	finalLines := []string{}
	if fileExists { // If file exists, preserve non-kubexm lines
	    for _, line := range currentLines {
		    trimmed := strings.TrimSpace(line)
		    isOurParam := false
		    for key := range s.Params {
			    if strings.HasPrefix(trimmed, key+"=") || strings.HasPrefix(trimmed, key+" =") {
				    isOurParam = true
				    break
			    }
		    }
		    if !isOurParam {
			    finalLines = append(finalLines, line)
		    }
	    }
	}


	for key, value := range s.Params {
		expectedLine := fmt.Sprintf("%s = %s", key, value)
		finalLines = append(finalLines, expectedLine)
		// Simple check if line needs to be added (doesn't handle updates of existing values well)
		if val, ok := existingParamsMap[key]; !ok || val != value {
			needsWrite = true
		}
	}

	if needsWrite {
		finalContent := strings.Join(finalLines, "\n")
		// Ensure there's a trailing newline if content is not empty
		if finalContent != "" && !strings.HasSuffix(finalContent, "\n") {
			finalContent += "\n"
		}

		logger.Info("Writing sysctl parameters to persistent config file.", "file", s.ConfFile)
		errWrite := runnerSvc.WriteFile(ctx.GoContext(), conn, []byte(finalContent), s.ConfFile, "0644", s.Sudo)
		if errWrite != nil {
			return fmt.Errorf("failed to write sysctl params to %s: %w", s.ConfFile, errWrite)
		}
		logger.Info("Sysctl parameters written to persistent config.", "file", s.ConfFile)

		// Apply persistent settings (sysctl -p or sysctl --system)
		// Using --system is generally preferred if available and targets all /etc/sysctl.d files.
		// Using -p <file> targets only our file.
		applyCmd := fmt.Sprintf("sysctl -p %s", s.ConfFile)
		if s.ConfFile == defaultSysctlConfFile { // If using our default file, try to apply all system configs
			// Check if --system is supported
			// For simplicity, just use -p with our specific file.
		}
		logger.Info("Applying persistent sysctl settings from file.", "command", applyCmd)
		_, stderrApply, errApply := runnerSvc.RunWithOptions(ctx.GoContext(), conn, applyCmd, execOpts)
		if errApply != nil {
			// This can sometimes have non-fatal errors if some keys in other files are problematic.
			logger.Warn("sysctl -p command finished with an issue (might be ignorable).", "command", applyCmd, "error", errApply, "stderr", string(stderrApply))
		} else {
			logger.Info("Persistent sysctl settings applied.", "file", s.ConfFile)
		}
	} else {
		logger.Info("No changes needed for persistent sysctl config file.", "file", s.ConfFile)
	}

	return nil
}

func (s *ConfigureSysctlStep) Rollback(ctx step.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Rollback")
	// Rollback for sysctl is complex:
	// 1. Reverting runtime values: Need original values. This step doesn't store them.
	// 2. Reverting persistent file: Could remove lines added by this step, or restore a backup.
	// For now, a simple approach: if we created defaultSysctlConfFile, remove it.
	// Otherwise, log that manual intervention might be needed.
	if s.ConfFile == defaultSysctlConfFile {
		logger.Info("Attempting to remove sysctl configuration file as part of rollback.", "file", s.ConfFile)
		runnerSvc := ctx.GetRunner()
		conn, err := ctx.GetConnectorForHost(host)
		if err != nil {
			return fmt.Errorf("failed to get connector for host %s for sysctl rollback: %w", host.GetName(), err)
		}
		if errRem := runnerSvc.Remove(ctx.GoContext(), conn, s.ConfFile, s.Sudo); errRem != nil {
			logger.Warn("Failed to remove sysctl config file during rollback (best effort).", "file", s.ConfFile, "error", errRem)
		} else {
			logger.Info("Sysctl config file removed.", "file", s.ConfFile)
			// Optionally, try to run `sysctl --system` to reload without our file.
		}
	} else {
		logger.Warn("Sysctl parameters were applied to a non-default file. Manual review/rollback might be needed for persistent changes.", "file", s.ConfFile)
	}
	logger.Warn("Runtime sysctl values changed by this step are not automatically rolled back. A reboot or manual reset might be needed.")
	return nil
}

var _ step.Step = (*ConfigureSysctlStep)(nil)
