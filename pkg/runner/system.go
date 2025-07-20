package runner

import (
	"context"
	"fmt"
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector"
)

func (r *defaultRunner) LoadModule(ctx context.Context, conn connector.Connector, facts *Facts, moduleName string, params ...string) error {
	if conn == nil {
		return fmt.Errorf("connector cannot be nil")
	}
	if strings.TrimSpace(moduleName) == "" {
		return fmt.Errorf("moduleName cannot be empty for LoadModule")
	}
	for _, char := range moduleName {
		isLetter := (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z')
		isDigit := (char >= '0' && char <= '9')
		isAllowedSymbol := char == '_' || char == '-'
		if !(isLetter || isDigit || isAllowedSymbol) {
			return fmt.Errorf("invalid characters in moduleName: %s", moduleName)
		}
	}
	for _, param := range params {
		if strings.TrimSpace(param) == "" {
			return fmt.Errorf("module parameters cannot be empty strings")
		}
	}

	cmdParts := []string{"modprobe", moduleName}
	if len(params) > 0 {
		cmdParts = append(cmdParts, params...)
	}
	cmd := strings.Join(cmdParts, " ")

	_, stderr, err := r.RunWithOptions(ctx, conn, cmd, &connector.ExecOptions{Sudo: true})
	if err != nil {
		return fmt.Errorf("failed to load module %s with params %v: %w (stderr: %s)", moduleName, params, err, string(stderr))
	}
	return nil
}

func (r *defaultRunner) IsModuleLoaded(ctx context.Context, conn connector.Connector, moduleName string) (bool, error) {
	if conn == nil {
		return false, fmt.Errorf("connector cannot be nil")
	}
	if strings.TrimSpace(moduleName) == "" {
		return false, fmt.Errorf("moduleName cannot be empty for IsModuleLoaded")
	}
	for _, char := range moduleName {
		isLetter := (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z')
		isDigit := (char >= '0' && char <= '9')
		isAllowedSymbol := char == '_' || char == '-'
		if !(isLetter || isDigit || isAllowedSymbol) {
			return false, fmt.Errorf("invalid characters in moduleName: %s", moduleName)
		}
	}

	cmd := fmt.Sprintf("test -d /sys/module/%s", moduleName)
	loaded, err := r.Check(ctx, conn, cmd, false)
	if err != nil {
		// This implies an issue with running `test` or the connection, not that the module isn't loaded.
		return false, fmt.Errorf("error checking for module %s: %w", moduleName, err)
	}
	return loaded, nil
}

func (r *defaultRunner) ConfigureModuleOnBoot(ctx context.Context, conn connector.Connector, facts *Facts, moduleName string, params ...string) error {
	if conn == nil {
		return fmt.Errorf("connector cannot be nil")
	}
	if strings.TrimSpace(moduleName) == "" {
		return fmt.Errorf("moduleName cannot be empty for ConfigureModuleOnBoot")
	}
	// Basic validation  moduleName
	for _, char := range moduleName {
		isLetter := (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z')
		isDigit := (char >= '0' && char <= '9')
		isAllowedSymbol := char == '_' || char == '-'
		if !(isLetter || isDigit || isAllowedSymbol) {
			return fmt.Errorf("invalid characters in moduleName: %s", moduleName)
		}
	}
	modulesLoadDir := "/etc/modules-load.d"
	if err := r.Mkdirp(ctx, conn, modulesLoadDir, "0755", true); err != nil {
		return fmt.Errorf("failed to ensure directory %s exists: %w", modulesLoadDir, err)
	}

	confFilePath := fmt.Sprintf("/etc/modules-load.d/%s.conf", moduleName)
	content := moduleName + "\n"

	err := r.WriteFile(ctx, conn, []byte(content), confFilePath, "0644", true)
	if err != nil {
		return fmt.Errorf("failed to write module config %s: %w", confFilePath, err)
	}

	if len(params) > 0 {
		modprobeDir := "/etc/modprobe.d"
		if err := r.Mkdirp(ctx, conn, modprobeDir, "0755", true); err != nil {
			return fmt.Errorf("failed to ensure directory %s exists: %w", modprobeDir, err)
		}
		optionsFilePath := fmt.Sprintf("/etc/modprobe.d/%s.conf", moduleName)
		optionsContent := fmt.Sprintf("options %s %s\n", moduleName, strings.Join(params, " "))
		errOptions := r.WriteFile(ctx, conn, []byte(optionsContent), optionsFilePath, "0644", true)
		if errOptions != nil {
			return fmt.Errorf("failed to write module options to %s: %w", optionsFilePath, errOptions)
		}
	}

	return nil
}

func (r *defaultRunner) SetSysctl(ctx context.Context, conn connector.Connector, key, value string, persistent bool) error {
	if conn == nil {
		return fmt.Errorf("connector cannot be nil")
	}
	if strings.TrimSpace(key) == "" {
		return fmt.Errorf("key cannot be empty for SetSysctl")
	}
	if strings.ContainsAny(key, ";|&`\n") {
		return fmt.Errorf("invalid characters in sysctl key: %s", key)
	}
	if strings.ContainsAny(value, "`\n") {
		return fmt.Errorf("invalid characters in sysctl value: %s", value)
	}
	cmdTemporary := fmt.Sprintf("sysctl -w %s=\"%s\"", key, value)
	_, stderr, err := r.RunWithOptions(ctx, conn, cmdTemporary, &connector.ExecOptions{Sudo: true})
	if err != nil {
		return fmt.Errorf("failed to set sysctl %s=%s temporarily: %w (stderr: %s)", key, value, err, string(stderr))
	}

	if persistent {
		sysctlConfFile := "/etc/sysctl.d/99-kubexm-runner.conf"
		lineToAdd := fmt.Sprintf("%s = %s", key, value)
		sysctlDDir := "/etc/sysctl.d"
		if errMkdir := r.Mkdirp(ctx, conn, sysctlDDir, "0755", true); errMkdir != nil {
			return fmt.Errorf("failed to ensure directory %s exists: %w", sysctlDDir, errMkdir)
		}

		checkCmd := fmt.Sprintf("grep -Fxq -- %s %s", lineToAdd, sysctlConfFile)
		exists, _ := r.Check(ctx, conn, checkCmd, false)

		if !exists {
			echoCmd := fmt.Sprintf("echo %s | tee -a %s", lineToAdd, sysctlConfFile)
			_, stderrPersist, errPersist := r.RunWithOptions(ctx, conn, echoCmd, &connector.ExecOptions{Sudo: true})
			if errPersist != nil {
				return fmt.Errorf("failed to persist sysctl setting '%s' to %s: %w (stderr: %s)", lineToAdd, sysctlConfFile, errPersist, string(stderrPersist))
			}
		}

		applyAllCmd := "sysctl -p"
		_, stderrApply, errApply := r.RunWithOptions(ctx, conn, applyAllCmd, &connector.ExecOptions{Sudo: true})
		if errApply != nil {
			return fmt.Errorf("failed to apply sysctl settings with 'sysctl -p': %w (stderr: %s)", errApply, string(stderrApply))
		}
	}
	return nil
}

func (r *defaultRunner) SetTimezone(ctx context.Context, conn connector.Connector, facts *Facts, timezone string) error {
	if conn == nil {
		return fmt.Errorf("connector cannot be nil")
	}
	if strings.TrimSpace(timezone) == "" {
		return fmt.Errorf("timezone cannot be empty for SetTimezone")
	}

	if strings.ContainsAny(timezone, ";|&`$\n") || strings.HasPrefix(timezone, "/") || strings.Contains(timezone, "..") {
		return fmt.Errorf("invalid characters or format in timezone string: %s", timezone)
	}

	if _, err := r.LookPath(ctx, conn, "timedatectl"); err == nil {
		cmd := fmt.Sprintf("timedatectl set-timezone %s", timezone)
		_, stderr, execErr := r.RunWithOptions(ctx, conn, cmd, &connector.ExecOptions{Sudo: true})
		if execErr != nil {
			return fmt.Errorf("failed to set timezone to %s using timedatectl: %w (stderr: %s)", timezone, execErr, string(stderr))
		}
		return nil
	}

	return fmt.Errorf("SetTimezone: 'timedatectl' command not found, and fallback method is not fully implemented for this OS/configuration")
}

func (r *defaultRunner) DisableSwap(ctx context.Context, conn connector.Connector, facts *Facts) error {
	if conn == nil {
		return fmt.Errorf("connector cannot be nil")
	}

	cmdSwapoff := "swapoff -a"
	_, stderrSwapoff, errSwapoff := r.RunWithOptions(ctx, conn, cmdSwapoff, &connector.ExecOptions{Sudo: true})
	if errSwapoff != nil {
		return fmt.Errorf("failed to execute 'swapoff -a': %w (stderr: %s)", errSwapoff, string(stderrSwapoff))
	}
	fstabPath := "/etc/fstab"
	backupExtension := ".kubexm-runner.bak"
	sedPattern := `s@^\([^#][^ ]*[[:space:]]\+[^[:space:]]\+[[:space:]]\+swap[[:space:]]\+.*\)@#\1@g`
	cmdFstab := fmt.Sprintf("sed -i'%s' -E \"%s\" %s", backupExtension, sedPattern, fstabPath)

	_, stderrFstab, errFstab := r.RunWithOptions(ctx, conn, cmdFstab, &connector.ExecOptions{Sudo: true})
	if errFstab != nil {
		return fmt.Errorf("failed to comment out swap entries in %s: %w (stderr: %s)", fstabPath, errFstab, string(stderrFstab))
	}

	return nil
}

func (r *defaultRunner) IsSwapEnabled(ctx context.Context, conn connector.Connector) (bool, error) {
	if conn == nil {
		return false, fmt.Errorf("connector cannot be nil")
	}
	cmd := "cat /proc/swaps"
	stdout, stderr, err := r.RunWithOptions(ctx, conn, cmd, &connector.ExecOptions{Sudo: false})
	if err != nil {
		// If /proc/swaps doesn't exist or cat fails, it's an error condition for the check.
		// However, some systems might not have /proc/swaps if swap is entirely disabled at kernel level.
		// A CommandError might indicate `cat /proc/swaps` failed because the file doesn't exist,
		// which could imply swap is not even configured/supported.
		// For now, treat any error from cat as an inability to determine, or a problem.
		return false, fmt.Errorf("failed to read /proc/swaps: %w (stderr: %s)", err, string(stderr))
	}

	lines := strings.Split(strings.TrimSpace(string(stdout)), "\n")
	if len(lines) > 1 {
		for i, line := range lines {
			if i == 0 {
				continue
			}
			if strings.TrimSpace(line) != "" {
				return true, nil // Found an actual swap entry
			}
		}
	}
	return false, nil
}
