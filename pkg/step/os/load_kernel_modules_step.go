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
	defaultModulesConfFile = "/etc/modules-load.d/kubexm.conf"
)

// LoadKernelModulesStep loads specified kernel modules and ensures they are loaded on boot.
type LoadKernelModulesStep struct {
	meta        spec.StepMeta
	Modules     []string // List of module names to load
	Sudo        bool
	ConfFile    string // Path to the modules-load.d configuration file
}

// NewLoadKernelModulesStep creates a new LoadKernelModulesStep.
func NewLoadKernelModulesStep(instanceName string, modules []string, sudo bool, confFile string) step.Step {
	name := instanceName
	if name == "" {
		name = "LoadKernelModules"
	}
	cf := confFile
	if cf == "" {
		cf = defaultModulesConfFile
	}
	return &LoadKernelModulesStep{
		meta: spec.StepMeta{
			Name:        name,
			Description: fmt.Sprintf("Loads kernel modules: %v and ensures persistence in %s", modules, cf),
		},
		Modules:  modules,
		Sudo:     sudo,
		ConfFile: cf,
	}
}

func (s *LoadKernelModulesStep) Meta() *spec.StepMeta {
	return &s.meta
}

// isModuleLoaded checks if a kernel module is currently loaded.
func (s *LoadKernelModulesStep) isModuleLoaded(ctx step.StepContext, runnerSvc runner.Runner, conn connector.Connector, moduleName string) (bool, error) {
	// lsmod | grep module_name
	// Sudo typically not needed for lsmod.
	lsmodCmd := fmt.Sprintf("lsmod | grep -w ^%s", moduleName) // -w for whole word, ^ for start of line
	execOpts := &connector.ExecOptions{Sudo: false, Check: true} // Check=true allows non-zero exit without Go error

	stdout, _, err := runnerSvc.RunWithOptions(ctx.GoContext(), conn, lsmodCmd, execOpts)

	if err != nil {
		if cmdErr, ok := err.(*connector.CommandError); ok && cmdErr.ExitCode == 1 { // grep not found
			return false, nil
		}
		// Other errors (e.g., lsmod command not found)
		return false, fmt.Errorf("failed to check if module %s is loaded: %w", moduleName, err)
	}
	return strings.TrimSpace(string(stdout)) != "", nil
}

// areModulesInConfFile checks if all specified modules are present in the configuration file.
func (s *LoadKernelModulesStep) areModulesInConfFile(ctx step.StepContext, runnerSvc runner.Runner, conn connector.Connector) (bool, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", ctx.GetHost().GetName())

	fileExists, err := runnerSvc.Exists(ctx.GoContext(), conn, s.ConfFile)
	if err != nil {
		logger.Warn("Failed to check existence of modules config file. Assuming modules not configured persistently.", "file", s.ConfFile, "error", err)
		return false, nil // If cannot check, assume not configured
	}
	if !fileExists {
		logger.Info("Modules config file does not exist. Modules not configured persistently.", "file", s.ConfFile)
		return false, nil
	}

	contentBytes, err := runnerSvc.ReadFile(ctx.GoContext(), conn, s.ConfFile)
	if err != nil {
		logger.Warn("Failed to read modules config file. Assuming modules not configured persistently.", "file", s.ConfFile, "error", err)
		return false, nil
	}
	content := string(contentBytes)
	lines := strings.Split(content, "\n")

	configuredModules := make(map[string]bool)
	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine != "" && !strings.HasPrefix(trimmedLine, "#") {
			configuredModules[trimmedLine] = true
		}
	}

	for _, mod := range s.Modules {
		if !configuredModules[mod] {
			logger.Info("Module not found in persistent config.", "module", mod, "file", s.ConfFile)
			return false, nil
		}
	}
	return true, nil
}

func (s *LoadKernelModulesStep) Precheck(ctx step.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Precheck")
	if len(s.Modules) == 0 {
		logger.Info("No modules specified to load.")
		return true, nil
	}

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	allLoaded := true
	for _, modName := range s.Modules {
		loaded, errLoadCheck := s.isModuleLoaded(ctx, runnerSvc, conn, modName)
		if errLoadCheck != nil {
			logger.Warn("Error checking if module is loaded, assuming not loaded.", "module", modName, "error", errLoadCheck)
			allLoaded = false
			break
		}
		if !loaded {
			logger.Info("Module not currently loaded.", "module", modName)
			allLoaded = false
			break
		}
		logger.Debug("Module already loaded.", "module", modName)
	}

	if !allLoaded {
		return false, nil // At least one module not loaded, Run is needed.
	}

	// All modules are loaded, now check persistence
	allPersistent, errPersistCheck := s.areModulesInConfFile(ctx, runnerSvc, conn)
	if errPersistCheck != nil {
		logger.Warn("Error checking module persistence, assuming not persistent.", "error", errPersistCheck)
		return false, nil // If persistence check fails, better to run.
	}

	if allPersistent {
		logger.Info("All specified kernel modules are already loaded and configured for persistence.")
		return true, nil
	}

	logger.Info("Modules are loaded, but not all are configured for persistence. Step needs to run.")
	return false, nil
}

func (s *LoadKernelModulesStep) Run(ctx step.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Run")
	if len(s.Modules) == 0 {
		logger.Info("No modules specified to load.")
		return nil
	}

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}
	execOpts := &connector.ExecOptions{Sudo: s.Sudo}

	// 1. Load modules using modprobe
	for _, modName := range s.Modules {
		logger.Info("Loading kernel module.", "module", modName)
		// Check if already loaded to avoid unnecessary modprobe calls that might warn/error
		isLoaded, _ := s.isModuleLoaded(ctx, runnerSvc, conn, modName)
		if isLoaded {
			logger.Debug("Module already loaded, skipping modprobe.", "module", modName)
			continue
		}

		modprobeCmd := fmt.Sprintf("modprobe %s", modName)
		_, stderr, errModprobe := runnerSvc.RunWithOptions(ctx.GoContext(), conn, modprobeCmd, execOpts)
		if errModprobe != nil {
			// modprobe can fail if module is built-in or already loaded.
			// Check stderr for common messages.
			// For now, log error and continue, persistence is important.
			logger.Error("modprobe command failed.", "module", modName, "command", modprobeCmd, "error", errModprobe, "stderr", string(stderr))
			return fmt.Errorf("failed to load module %s: %w. Stderr: %s", modName, errModprobe, string(stderr))
		}
		logger.Info("Kernel module loaded successfully.", "module", modName)
	}

	// 2. Ensure modules are loaded on boot by adding them to ConfFile
	logger.Info("Ensuring modules are configured for persistence.", "file", s.ConfFile)

	// Ensure directory for conf file exists
	confDir := filepath.Dir(s.ConfFile)
	if errMkdir := runnerSvc.Mkdirp(ctx.GoContext(), conn, confDir, "0755", s.Sudo); errMkdir != nil {
		return fmt.Errorf("failed to create directory %s for modules config: %w", confDir, errMkdir)
	}

	// Create/append modules to the conf file.
	// This command appends each module on a new line if not already present.
	// A more robust way might be to read, check, and then write the whole file.
	// For simplicity using echo and tee.
	var commands []string
	for _, modName := range s.Modules {
		// Check if module is already in the file to avoid duplicates (basic grep check)
		// grep -qxF 'module_name' /etc/modules-load.d/kubexm.conf || echo 'module_name' >> /etc/modules-load.d/kubexm.conf
		// The `tee -a` approach is simpler to implement via a single command for all modules.
		// We will build up the content and write it once.
	}

	// Read existing content if file exists
	var existingContent string
	fileExists, _ := runnerSvc.Exists(ctx.GoContext(), conn, s.ConfFile)
	if fileExists {
		existingBytes, errRead := runnerSvc.ReadFile(ctx.GoContext(), conn, s.ConfFile)
		if errRead != nil {
			logger.Warn("Could not read existing modules file, may overwrite or append duplicates.", "file", s.ConfFile, "error", errRead)
		} else {
			existingContent = string(existingBytes)
		}
	}

	modulesInFile := make(map[string]bool)
	for _, line := range strings.Split(existingContent, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			modulesInFile[trimmed] = true
		}
	}

	var contentBuilder strings.Builder
	contentBuilder.WriteString(existingContent)
	if !strings.HasSuffix(existingContent, "\n") && existingContent != "" {
		contentBuilder.WriteString("\n") // Ensure newline if file exists and doesn't end with one
	}

	added := false
	for _, modName := range s.Modules {
		if !modulesInFile[modName] {
			contentBuilder.WriteString(modName)
			contentBuilder.WriteString("\n")
			added = true
			logger.Debug("Module will be added to persistent config.", "module", modName)
		} else {
			logger.Debug("Module already in persistent config.", "module", modName)
		}
	}

	if added || !fileExists { // Write if new modules were added or if the file didn't exist
		finalContent := contentBuilder.String()
		logger.Info("Writing modules to persistent config file.", "file", s.ConfFile, "content_length", len(finalContent))
		errWrite := runnerSvc.WriteFile(ctx.GoContext(), conn, []byte(finalContent), s.ConfFile, "0644", s.Sudo)
		if errWrite != nil {
			return fmt.Errorf("failed to write modules to %s: %w", s.ConfFile, errWrite)
		}
		logger.Info("Modules successfully written to persistent config.", "file", s.ConfFile)
	} else {
		logger.Info("No changes needed for persistent modules config file.", "file", s.ConfFile)
	}

	return nil
}

func (s *LoadKernelModulesStep) Rollback(ctx step.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Rollback")
	// Rollback could involve:
	// 1. Removing modules from the ConfFile (if they were added by this step).
	// 2. `rmmod` the modules (if they were loaded by this step and not before).
	// This is complex to do perfectly. A simpler rollback is to just log.
	// For now, no-op, as unintended rmmod can be disruptive.
	logger.Info("Rollback for LoadKernelModulesStep is non-trivial and not implemented. Manual check may be needed if issues arise.")
	// If we wanted to remove from ConfFile, we'd need to know which lines *this step* added.
	return nil
}

var _ step.Step = (*LoadKernelModulesStep)(nil)
