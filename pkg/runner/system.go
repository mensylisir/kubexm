package runner

import (
	"context"
	"fmt"
	"strings" // Needed for TrimSpace
	// "time" // May be needed later

	"github.com/mensylisir/kubexm/pkg/connector"
)

// --- System & Kernel Methods ---

// util.ShellEscape is defined in file.go and accessible within the package.

func (r *defaultRunner) LoadModule(ctx context.Context, conn connector.Connector, facts *Facts, moduleName string, params ...string) error {
	if conn == nil {
		return fmt.Errorf("connector cannot be nil")
	}
	if strings.TrimSpace(moduleName) == "" {
		return fmt.Errorf("moduleName cannot be empty for LoadModule")
	}
	// Basic validation for moduleName
	for _, char := range moduleName {
		isLetter := (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z')
		isDigit := (char >= '0' && char <= '9')
		isAllowedSymbol := char == '_' || char == '-'
		if !(isLetter || isDigit || isAllowedSymbol) {
			return fmt.Errorf("invalid characters in moduleName: %s", moduleName)
		}
	}
	// Basic validation for params (e.g. ensure they look like key=value or just keys)
	for _, param := range params {
		if strings.TrimSpace(param) == "" {
			return fmt.Errorf("module parameters cannot be empty strings")
		}
		// A more robust validation would check param format (e.g., no dangerous shell characters)
		// For now, assume params are simple.
	}

	cmdParts := []string{"modprobe", moduleName}
	if len(params) > 0 {
		// Parameters should be escaped if they contain spaces or special characters.
		// For simplicity now, assuming parameters are "safe" or that underlying util.ShellEscape handles it if used.
		// However, modprobe parameters are typically simple.
		cmdParts = append(cmdParts, params...)
	}
	cmd := strings.Join(cmdParts, " ")

	// modprobe usually requires sudo
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

	// Validate moduleName to prevent injection if it were used in a less safe command.
	// For `test -d /sys/module/...`, it's relatively safe.
	// A simple validation: ensure it's a typical module name (alphanumeric, underscores, hyphens).
	// This is a basic check.
	for _, char := range moduleName {
		isLetter := (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z')
		isDigit := (char >= '0' && char <= '9')
		isAllowedSymbol := char == '_' || char == '-'
		if !(isLetter || isDigit || isAllowedSymbol) {
			return false, fmt.Errorf("invalid characters in moduleName: %s", moduleName)
		}
	}

	cmd := fmt.Sprintf("test -d /sys/module/%s", moduleName)
	// No sudo typically needed to check /sys
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
	// Basic validation for moduleName
	for _, char := range moduleName {
		isLetter := (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z')
		isDigit := (char >= '0' && char <= '9')
		isAllowedSymbol := char == '_' || char == '-'
		if !(isLetter || isDigit || isAllowedSymbol) {
			return fmt.Errorf("invalid characters in moduleName: %s", moduleName)
		}
	}
	// Note: params handling (writing to /etc/modprobe.d/) is not implemented in this version.

	// Ensure /etc/modules-load.d directory exists (usually does, but good practice)
	// Mkdirp will use sudo if the main operation (WriteFile) uses sudo.
	// The WriteFile below will use sudo.
	modulesLoadDir := "/etc/modules-load.d"
	if err := r.Mkdirp(ctx, conn, modulesLoadDir, "0755", true); err != nil {
		return fmt.Errorf("failed to ensure directory %s exists: %w", modulesLoadDir, err)
	}

	confFilePath := fmt.Sprintf("/etc/modules-load.d/%s.conf", moduleName)
	content := moduleName + "\n" // File content is just the module name

	// WriteFile will use sudo. Permissions 0644 are standard for .conf files.
	err := r.WriteFile(ctx, conn, []byte(content), confFilePath, "0644", true)
	if err != nil {
		return fmt.Errorf("failed to write module config %s: %w", confFilePath, err)
	}

	if len(params) > 0 {
		// Implement writing options to /etc/modprobe.d/moduleName.conf
		// Content: "options moduleName param1=val1 param2=val2"
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
	// Basic validation for key and value to prevent trivial command injection.
	// Sysctl keys are typically dot/slash separated alphanum. Values can be varied.
	// This is not foolproof security, just a basic sanity check.
	if strings.ContainsAny(key, ";|&`\n") {
		return fmt.Errorf("invalid characters in sysctl key: %s", key)
	}
	if strings.ContainsAny(value, "`\n") { // Value might legitimately contain spaces or some symbols.
		return fmt.Errorf("invalid characters in sysctl value: %s", value)
	}

	// Temporary set
	// Use util.ShellEscape for key and value just in case, though sysctl -w is usually safe.
	// sysctl keys don't typically need escaping, but values might if they contain shell metachars.
	// However, for sysctl, the value is usually passed directly.
	// Let's assume key and value are "simple" enough not to break `sysctl -w`.
	// A safer way for value is `sysctl -w key="%s"` but that depends on sysctl version.
	// Sticking to `key=value`.
	cmdTemporary := fmt.Sprintf("sysctl -w %s=\"%s\"", key, value) // Quoting value can help.
	_, stderr, err := r.RunWithOptions(ctx, conn, cmdTemporary, &connector.ExecOptions{Sudo: true})
	if err != nil {
		return fmt.Errorf("failed to set sysctl %s=%s temporarily: %w (stderr: %s)", key, value, err, string(stderr))
	}

	if persistent {
		sysctlConfFile := "/etc/sysctl.d/99-kubexm-runner.conf" // Central file for this runner's settings
		lineToAdd := fmt.Sprintf("%s = %s", key, value)

		// Ensure /etc/sysctl.d directory exists
		sysctlDDir := "/etc/sysctl.d"
		if errMkdir := r.Mkdirp(ctx, conn, sysctlDDir, "0755", true); errMkdir != nil {
			return fmt.Errorf("failed to ensure directory %s exists: %w", sysctlDDir, errMkdir)
		}

		// Idempotency: Check if the exact line exists.
		// Grep for exact line match. -x means whole line, -F means fixed string.
		checkCmd := fmt.Sprintf("grep -Fxq -- %s %s", lineToAdd, sysctlConfFile)
		exists, _ := r.Check(ctx, conn, checkCmd, false) // Ignore error from Check, if grep fails, assume not found or file doesn't exist.

		if !exists {
			// Append the setting. Use `tee -a` for robustness with sudo.
			// Need to escape lineToAdd for the echo command.
			echoCmd := fmt.Sprintf("echo %s | tee -a %s", lineToAdd, sysctlConfFile)
			_, stderrPersist, errPersist := r.RunWithOptions(ctx, conn, echoCmd, &connector.ExecOptions{Sudo: true})
			if errPersist != nil {
				return fmt.Errorf("failed to persist sysctl setting '%s' to %s: %w (stderr: %s)", lineToAdd, sysctlConfFile, errPersist, string(stderrPersist))
			}
		}

		// Apply persistent settings
		// `sysctl -p <file>` is safer than `sysctl -p` if other files might have errors.
		// Some systems might not have the file initially, `sysctl -p file` might error.
		// `sysctl -p` (all files) is more common if we are sure our file is correctly formatted.
		// If sysctlConfFile might not exist yet (e.g. first write), `sysctl -p` (all) is safer.
		// Let's use `sysctl -p` to be more general.
		applyAllCmd := "sysctl -p" // applyCmd was removed as it was unused.
		_, stderrApply, errApply := r.RunWithOptions(ctx, conn, applyAllCmd, &connector.ExecOptions{Sudo: true})
		if errApply != nil {
			// Not all `sysctl -p` errors are fatal for our specific change if it was written correctly.
			// For example, another sysctl file having an error.
			// But if our key itself causes `sysctl -p` to fail, that's an issue.
			// We'll return the error for now.
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

	// Validate timezone string format (basic validation)
	// Timezones are typically "Area/Location", e.g., "America/New_York", "Europe/London", "UTC"
	// This check is very basic and might need to be more robust or rely on the system tool's validation.
	if strings.ContainsAny(timezone, ";|&`$\n") || strings.HasPrefix(timezone, "/") || strings.Contains(timezone, "..") {
		return fmt.Errorf("invalid characters or format in timezone string: %s", timezone)
	}

	// Prefer timedatectl if available (common on systemd systems)
	if _, err := r.LookPath(ctx, conn, "timedatectl"); err == nil {
		cmd := fmt.Sprintf("timedatectl set-timezone %s", timezone)
		_, stderr, execErr := r.RunWithOptions(ctx, conn, cmd, &connector.ExecOptions{Sudo: true})
		if execErr != nil {
			return fmt.Errorf("failed to set timezone to %s using timedatectl: %w (stderr: %s)", timezone, execErr, string(stderr))
		}
		return nil
	}

	// Fallback for non-systemd systems (more complex, involves symlinking /etc/localtime)
	// For now, if timedatectl is not found, return an error indicating limited support.
	// A full implementation would:
	// 1. Check if /usr/share/zoneinfo/[timezone] exists.
	// 2. sudo rm /etc/localtime
	// 3. sudo ln -s /usr/share/zoneinfo/[timezone] /etc/localtime
	// 4. Optionally, update /etc/timezone file.
	return fmt.Errorf("SetTimezone: 'timedatectl' command not found, and fallback method is not fully implemented for this OS/configuration")
}

func (r *defaultRunner) DisableSwap(ctx context.Context, conn connector.Connector, facts *Facts) error {
	if conn == nil {
		return fmt.Errorf("connector cannot be nil")
	}

	// Disable swap for the current session
	cmdSwapoff := "swapoff -a"
	_, stderrSwapoff, errSwapoff := r.RunWithOptions(ctx, conn, cmdSwapoff, &connector.ExecOptions{Sudo: true})
	if errSwapoff != nil {
		// Not always fatal if swap wasn't on, but good to report.
		// Some systems might return error if no swap is configured.
		// We can choose to log this as a warning or return the error.
		// For now, let's return it, as failure to swapoff could be significant.
		return fmt.Errorf("failed to execute 'swapoff -a': %w (stderr: %s)", errSwapoff, string(stderrSwapoff))
	}

	// Make it persistent by commenting out swap entries in /etc/fstab
	// This sed command looks for lines containing "swap" as the third field (fs_vfstype)
	// or lines starting with UUID/LABEL that have "none swap"
	// It creates a backup .fstab.kubexm-runner.bak
	// Using a more specific sed pattern to avoid commenting out non-swap lines that might contain "swap"
	// Pattern 1: Match lines like "/dev/sdb1 none swap sw 0 0" or "UUID=... none swap sw 0 0"
	// Pattern 2: Match lines like "/path/to/swapfile swap swap defaults 0 0" (less common for 'none' as mount point)
	// A common pattern is `<device_spec> <mount_point> <fs_type> <options> <dump> <pass>`
	// For swap, mount_point is often 'none' and fs_type is 'swap'.
	// sed -i.bak -E 's@(^([^#]\S+\s+none\s+swap\s+.*))@#\1@g' /etc/fstab
	// This regex is a bit safer:
	// ^([^#]\S+\s+(none|\S+)\s+swap\s+.*)
	//  ^ - start of line
	//  ([^#]\S+\s+ - not starting with #, then device spec and space
	//  (none|\S+)\s+ - mount point (can be 'none' or an actual path for swapfile) and space
	//  swap\s+ - fstype 'swap' and space
	//  .*) - rest of the line
	// This should capture most common swap entries.
	fstabPath := "/etc/fstab"
	// Using a temp variable for the backup extension to avoid issues with util.ShellEscape if used later.
	backupExtension := ".kubexm-runner.bak"
	// Escape for sed: single quotes around sed script, internal single quotes need to be `'\''`.
	// The pattern itself does not contain single quotes here.
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

	// Check /proc/swaps. If it has more than one line (header + data), swap is enabled.
	// This is a common and reliable way on Linux.
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
	// If /proc/swaps exists and is readable:
	// - If it's empty or only has a header, len(lines) might be 0 or 1 after TrimSpace.
	// - If there's at least one active swap entry beyond the header, len(lines) > 1.
	// Header line is typically "Filename Type Size Used Priority"
	if len(lines) > 1 {
		// Further check: ensure the lines after header are not empty or just comments
		for i, line := range lines {
			if i == 0 {
				continue
			} // Skip header
			if strings.TrimSpace(line) != "" {
				return true, nil // Found an actual swap entry
			}
		}
	}

	// If only header or empty after trim, or only empty lines after header
	return false, nil
}
