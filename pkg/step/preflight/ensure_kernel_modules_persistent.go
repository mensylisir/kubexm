package preflight

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/kubexms/kubexms/pkg/connector" // For ExecOptions, though not heavily used here
	"github.com/kubexms/kubexms/pkg/runtime"
	"github.com/kubexms/kubexms/pkg/step"
	"github.com/kubexms/kubexms/pkg/step/spec"
)

// EnsureKernelModulesPersistentStepSpec defines the specification for ensuring kernel modules are configured to load on boot.
type EnsureKernelModulesPersistentStepSpec struct {
	Modules      []string `json:"modules,omitempty"`
	ConfFilePath string   `json:"confFilePath,omitempty"`
}

// GetName returns the name of the step.
func (s *EnsureKernelModulesPersistentStepSpec) GetName() string {
	return "Ensure Kernel Modules Load on Boot"
}

// PopulateDefaults fills the spec with default values.
func (s *EnsureKernelModulesPersistentStepSpec) PopulateDefaults() {
	if len(s.Modules) == 0 {
		s.Modules = []string{
			"br_netfilter",
			"overlay",
			"ip_vs",
			"ip_vs_rr",
			"ip_vs_wrr",
			"ip_vs_sh",
			"nf_conntrack_placeholder", // Placeholder to be resolved
		}
	}
	if s.ConfFilePath == "" {
		// Using a different default name to distinguish from the previous 'load' step if used together.
		s.ConfFilePath = "/etc/modules-load.d/kubexms-persistent-modules.conf"
	}
}

// EnsureKernelModulesPersistentStepExecutor implements the logic.
type EnsureKernelModulesPersistentStepExecutor struct{}

// resolveModules_internal is a helper to determine the actual module names to persist,
// especially handling placeholders like "nf_conntrack_placeholder".
// Note: Renamed to avoid conflict if another `resolveModules` exists in the package.
func (e *EnsureKernelModulesPersistentStepExecutor) resolveModulesInternal(ctx *runtime.Context, desiredModules []string) ([]string, error) {
	resolved := []string{}
	nfPlaceholder := "nf_conntrack_placeholder"
	nfResolved := false

	for _, moduleName := range desiredModules {
		if moduleName == nfPlaceholder {
			if nfResolved {
				continue
			}
			// Try nf_conntrack_ipv4 first
			// Using RunWithOptions with AllowFailure to check modinfo's exit status. Sudo not needed for modinfo.
			_, _, errIPv4 := ctx.Host.Runner.RunWithOptions(ctx.GoContext, "modinfo nf_conntrack_ipv4", &connector.ExecOptions{Sudo: false, AllowFailure: true})
			if errIPv4 == nil {
				resolved = append(resolved, "nf_conntrack_ipv4")
				nfResolved = true
				ctx.Logger.Debugf("Resolved '%s' to 'nf_conntrack_ipv4' based on modinfo.", nfPlaceholder)
				continue
			}
			ctx.Logger.Debugf("modinfo nf_conntrack_ipv4 failed (or module not found): %v. Trying nf_conntrack.", errIPv4)

			// Try nf_conntrack next
			_, _, errNf := ctx.Host.Runner.RunWithOptions(ctx.GoContext, "modinfo nf_conntrack", &connector.ExecOptions{Sudo: false, AllowFailure: true})
			if errNf == nil {
				resolved = append(resolved, "nf_conntrack")
				nfResolved = true
				ctx.Logger.Debugf("Resolved '%s' to 'nf_conntrack' based on modinfo.", nfPlaceholder)
				continue
			}
			ctx.Logger.Debugf("modinfo nf_conntrack also failed (or module not found): %v.", errNf)
			return nil, fmt.Errorf("failed to resolve placeholder '%s': neither nf_conntrack_ipv4 nor nf_conntrack found via modinfo", nfPlaceholder)
		} else {
			// For non-placeholder modules, verify they exist with modinfo before adding to resolved list.
			// This ensures we only try to persist modules that are actually available on the system.
			_, _, errModinfo := ctx.Host.Runner.RunWithOptions(ctx.GoContext, "modinfo "+moduleName, &connector.ExecOptions{Sudo: false, AllowFailure: true})
			if errModinfo != nil {
				ctx.Logger.Warnf("Module '%s' specified for persistence does not seem to be available (modinfo failed: %v). It will not be added to the config.", moduleName, errModinfo)
			} else {
				resolved = append(resolved, moduleName)
			}
		}
	}
	if !nfResolved && contains(desiredModules, nfPlaceholder) {
		return nil, fmt.Errorf("failed to resolve placeholder '%s': neither nf_conntrack_ipv4 nor nf_conntrack seems available", nfPlaceholder)
	}

	// Sort for consistent comparison and file content
	sort.Strings(resolved)
	return resolved, nil
}

// Helper function to check if a slice contains a string
func contains(slice []string, str string) bool {
	for _, item := range slice {
		if item == str {
			return true
		}
	}
	return false
}

// Check determines if the kernel module persistence file is correctly configured.
func (e *EnsureKernelModulesPersistentStepExecutor) Check(s spec.StepSpec, ctx *runtime.Context) (isDone bool, err error) {
	stepSpec, ok := s.(*EnsureKernelModulesPersistentStepSpec)
	if !ok {
		return false, fmt.Errorf("unexpected spec type %T", s)
	}
	stepSpec.PopulateDefaults()
	hostCtxLogger := ctx.Logger.SugaredLogger.With("host", ctx.Host.Name, "step_spec", stepSpec.GetName()).Sugar()

	expectedModules, err := e.resolveModulesInternal(ctx, stepSpec.Modules)
	if err != nil {
		hostCtxLogger.Warnf("Could not resolve module list for checking persistence file: %v", err)
		return false, nil // Cannot determine expected state, so not "done".
	}
	if len(expectedModules) == 0 && len(stepSpec.Modules) > 0 && !contains(stepSpec.Modules, "nf_conntrack_placeholder"){
		 // If non-placeholder modules were specified but none resolved, something is wrong.
		 hostCtxLogger.Warnf("No modules resolved for persistence, but spec contained non-placeholder modules. Check module availability.")
		 return false, nil
	}


	confContentBytes, err := ctx.Host.Runner.ReadFile(ctx.GoContext, stepSpec.ConfFilePath)
	if err != nil {
		hostCtxLogger.Infof("Failed to read kernel module config file %s: %v. Configuration is not done.", stepSpec.ConfFilePath, err)
		return false, nil // Conf file doesn't exist or unreadable
	}

	confLines := strings.Split(strings.TrimSpace(string(confContentBytes)), "\n")
	actualModulesInConf := []string{}
	for _, line := range confLines {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" || strings.HasPrefix(trimmedLine, "#") || strings.HasPrefix(trimmedLine, ";") {
			continue
		}
		actualModulesInConf = append(actualModulesInConf, trimmedLine)
	}
	sort.Strings(actualModulesInConf) // Sort for consistent comparison

	if len(expectedModules) == 0 && len(actualModulesInConf) == 0 && (len(stepSpec.Modules) == 0 || (len(stepSpec.Modules) == 1 && stepSpec.Modules[0] == "nf_conntrack_placeholder" && expectedModules == nil)) {
		hostCtxLogger.Debugf("No modules specified or resolved, and config file %s is effectively empty. Considered done.", stepSpec.ConfFilePath)
		return true, nil
	}

	if len(expectedModules) != len(actualModulesInConf) {
		hostCtxLogger.Infof("Number of modules in %s (%d) does not match expected (%d). Expected: %v, Actual: %v",
			stepSpec.ConfFilePath, len(actualModulesInConf), len(expectedModules), expectedModules, actualModulesInConf)
		return false, nil
	}

	for i := range expectedModules {
		if expectedModules[i] != actualModulesInConf[i] {
			hostCtxLogger.Infof("Module list in %s differs from expected. Expected: %v, Actual: %v",
				stepSpec.ConfFilePath, expectedModules, actualModulesInConf)
			return false, nil
		}
	}

	hostCtxLogger.Infof("Kernel module persistence file %s is correctly configured with modules: %v.", stepSpec.ConfFilePath, expectedModules)
	return true, nil
}

// Execute ensures kernel modules are configured to load on boot.
func (e *EnsureKernelModulesPersistentStepExecutor) Execute(s spec.StepSpec, ctx *runtime.Context) *step.Result {
	stepSpec, ok := s.(*EnsureKernelModulesPersistentStepSpec)
	if !ok {
		myErr := fmt.Errorf("Execute: unexpected spec type %T", s)
		stepName := "EnsureKernelModulesPersistent (type error)"; if s != nil { stepName = s.GetName() }
		return step.NewResult(stepName, ctx.Host.Name, time.Now(), myErr)
	}
	stepSpec.PopulateDefaults()

	stepName := stepSpec.GetName()
	startTime := time.Now()
	res := step.NewResult(stepName, ctx.Host.Name, startTime, nil)
	hostCtxLogger := ctx.Logger.SugaredLogger.With("host", ctx.Host.Name, "step_spec", stepName).Sugar()

	resolvedModules, err := e.resolveModulesInternal(ctx, stepSpec.Modules)
	if err != nil {
		res.Error = fmt.Errorf("failed to resolve module list for persistence: %w", err)
		res.SetFailed(res.Error.Error()); hostCtxLogger.Errorf("Step failed: %v", res.Error); return res
	}

	if len(resolvedModules) == 0 {
		hostCtxLogger.Warnf("No kernel modules resolved to persist. If modules were specified, this might indicate they are not available on the system. Config file %s will not be written or will be empty.", stepSpec.ConfFilePath)
		// Depending on strictness, this could be an error. If stepSpec.Modules was empty, this is fine.
		// If stepSpec.Modules had items but none resolved, it's a problem.
		if len(stepSpec.Modules) > 0 && ! (len(stepSpec.Modules) == 1 && stepSpec.Modules[0] == "nf_conntrack_placeholder") {
			res.Error = fmt.Errorf("no modules could be resolved for persistence out of specified: %v", stepSpec.Modules)
			res.SetFailed(res.Error.Error()); hostCtxLogger.Errorf("Step failed: %v", res.Error); return res
		}
	}


	confDir := filepath.Dir(stepSpec.ConfFilePath)
	if err := ctx.Host.Runner.Mkdirp(ctx.GoContext, confDir, "0755", true); err != nil {
		res.Error = fmt.Errorf("failed to create directory %s: %w", confDir, err)
		res.SetFailed(res.Error.Error()); hostCtxLogger.Errorf("Step failed: %v", res.Error); return res
	}

	confFileContent := ""
	if len(resolvedModules) > 0 {
		confFileContent = strings.Join(resolvedModules, "\n") + "\n" // Ensure trailing newline
	} else {
		// If no modules, write an empty file or a file with a comment.
		// This ensures that an existing file with old modules is overwritten.
		confFileContent = "# No kernel modules specified or resolved for persistence by KubexMS.\n"
	}

	hostCtxLogger.Infof("Writing kernel module persistence configuration to %s with modules: %v", stepSpec.ConfFilePath, resolvedModules)
	if err := ctx.Host.Runner.WriteFile(ctx.GoContext, []byte(confFileContent), stepSpec.ConfFilePath, "0644", true); err != nil {
		res.Error = fmt.Errorf("failed to write kernel module config file %s: %w", stepSpec.ConfFilePath, err)
		res.SetFailed(res.Error.Error()); hostCtxLogger.Errorf("Step failed: %v", res.Error); return res
	}
	hostCtxLogger.Infof("Kernel module persistence file %s written successfully.", stepSpec.ConfFilePath)

	done, checkErr := e.Check(s, ctx)
	if checkErr != nil {
		res.Error = fmt.Errorf("post-execution check failed: %w", checkErr)
		res.SetFailed(res.Error.Error()); hostCtxLogger.Errorf("Step failed verification: %v", res.Error); return res
	}
	if !done {
		errMsg := "post-execution check indicates kernel module persistence file is not correctly configured"
		res.Error = fmt.Errorf(errMsg)
		res.SetFailed(errMsg); hostCtxLogger.Errorf("Step failed verification: %s", errMsg); return res
	}

	res.SetSucceeded("Kernel module persistence configured successfully.")
	hostCtxLogger.Successf("Step succeeded: %s", res.Message)
	return res
}

func init() {
	step.Register(step.GetSpecTypeName(&EnsureKernelModulesPersistentStepSpec{}), &EnsureKernelModulesPersistentStepExecutor{})
}
