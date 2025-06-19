package containerd

import (
	"context" // Required by runtime.Context
	"fmt"
	"strings"
	"time"

	"github.com/kubexms/kubexms/pkg/connector" // For connector.ExecOptions
	"github.com/kubexms/kubexms/pkg/runner"    // For runner.PackageManagerApt etc.
	"github.com/kubexms/kubexms/pkg/runtime"
	"github.com/kubexms/kubexms/pkg/spec"
	"github.com/kubexms/kubexms/pkg/step"
)

// InstallContainerdStepSpec defines parameters for installing containerd.io.
type InstallContainerdStepSpec struct {
	Version  string
	StepName string // Optional: for a custom name for this specific step instance
}

// GetName returns the step name.
func (s *InstallContainerdStepSpec) GetName() string {
	if s.StepName != "" { return s.StepName }
	if s.Version != "" { return fmt.Sprintf("Install containerd.io (version %s)", s.Version) }
	return "Install containerd.io (latest)"
}
var _ spec.StepSpec = &InstallContainerdStepSpec{}

// InstallContainerdStepExecutor implements the logic for InstallContainerdStepSpec.
type InstallContainerdStepExecutor struct{}

func init() {
	step.Register(step.GetSpecTypeName(&InstallContainerdStepSpec{}), &InstallContainerdStepExecutor{})
}

// Check determines if containerd.io is installed and matches the specified version.
func (e *InstallContainerdStepExecutor) Check(s spec.StepSpec, ctx *runtime.Context) (isDone bool, err error) {
	spec, ok := s.(*InstallContainerdStepSpec)
	if !ok {
		return false, fmt.Errorf("unexpected spec type %T for InstallContainerdStepExecutor Check method", s)
	}
	if ctx.Host.Runner == nil {
		return false, fmt.Errorf("runner not available in context for host %s", ctx.Host.Name)
	}
	hostCtxLogger := ctx.Logger.SugaredLogger.With("host", ctx.Host.Name, "step_spec", spec.GetName()).Sugar()


	pkgName := "containerd.io"
	installed, err := ctx.Host.Runner.IsPackageInstalled(ctx.GoContext, pkgName)
	if err != nil {
		return false, fmt.Errorf("failed to check if package %s is installed on host %s: %w", pkgName, ctx.Host.Name, err)
	}
	if !installed {
		hostCtxLogger.Debugf("Package %s is not installed.", pkgName)
		return false, nil
	}

	if spec.Version == "" {
		hostCtxLogger.Infof("Package %s is installed (latest version or version not specified for check).", pkgName)
		return true, nil
	}

	hostCtxLogger.Debugf("Package %s is installed, checking version against required '%s'.", pkgName, spec.Version)

	// Version specific check. Assumes runner has a public DetectPackageManager method.
	pmInfo, detectErr := ctx.Host.Runner.DetectPackageManager(ctx.GoContext)
	if detectErr != nil {
		hostCtxLogger.Warnf("Could not detect package manager to verify %s version %s: %v. Assuming check passes to avoid re-install if not strictly needed.", pkgName, spec.Version, detectErr)
		// If we cannot detect PM, we cannot reliably check version.
		// Depending on strictness, could return false to force Run, or true to be lenient.
		// Let's be lenient here for Check, Run will try to install specific version anyway.
		return true, nil
	}

	var versionCmd string
	switch pmInfo.Type {
	case runner.PackageManagerApt:
		versionCmd = fmt.Sprintf("apt-cache policy %s | grep 'Installed:' | awk '{print $2}'", pkgName)
	case runner.PackageManagerYum, runner.PackageManagerDnf:
		versionCmd = fmt.Sprintf("rpm -q --queryformat '%%{VERSION}-%%{RELEASE}' %s", pkgName)
	default:
		hostCtxLogger.Warnf("Version check for %s not implemented for package manager type %s. Assuming check passes.", pkgName, pmInfo.Type)
		return true, nil
	}

	// Sudo false for version check commands.
	stdoutBytes, stderrBytes, execErr := ctx.Host.Runner.RunWithOptions(ctx.GoContext, versionCmd, &connector.ExecOptions{Sudo: false})
	if execErr != nil {
		hostCtxLogger.Warnf("Failed to get installed version of %s (command: '%s'): %v. Stderr: %s. Assuming version match for check phase to be safe or needs update.", pkgName, versionCmd, execErr, string(stderrBytes))
		// If we can't get the version, we can't confirm it's correct.
		// Returning false means Run will execute.
		return false, nil
	}
	installedVersion := strings.TrimSpace(string(stdoutBytes))

	// Version comparison: simple prefix match. e.g. spec.Version "1.6.9" matches installed "1.6.9-1" or "1.6.9".
	// Trim "v" prefix from spec.Version for comparison if it exists.
	requiredVersionNoV := strings.TrimPrefix(spec.Version, "v")
	if strings.HasPrefix(installedVersion, requiredVersionNoV) || installedVersion == requiredVersionNoV {
		hostCtxLogger.Infof("Package %s version %s is installed and matches required version %s.", pkgName, installedVersion, spec.Version)
		return true, nil
	}

	hostCtxLogger.Infof("Package %s version %s is installed, but required version is %s. Main installation logic will run.", pkgName, installedVersion, spec.Version)
	return false, nil // Installed but not the right version, so not "done".
}

// Execute installs containerd.io.
func (e *InstallContainerdStepExecutor) Execute(s spec.StepSpec, ctx *runtime.Context) *step.Result {
	spec, ok := s.(*InstallContainerdStepSpec)
	if !ok {
		myErr := fmt.Errorf("Execute: unexpected spec type %T for InstallContainerdStepExecutor", s)
		stepName := "InstallContainerd (type error)"
		if s != nil { stepName = s.GetName() }
		return step.NewResult(stepName, ctx.Host.Name, time.Now(), myErr)
	}

	startTime := time.Now()
	res := step.NewResult(spec.GetName(), ctx.Host.Name, startTime, nil)
	hostCtxLogger := ctx.Logger.SugaredLogger.With("host", ctx.Host.Name, "step_spec", spec.GetName()).Sugar()

	if ctx.Host.Runner == nil {
		res.Error = fmt.Errorf("runner not available in context for host %s", ctx.Host.Name)
		res.Status = "Failed"; res.Message = res.Error.Error(); hostCtxLogger.Errorf("Step failed: %v", res.Error); return res
	}

	hostCtxLogger.Infof("Updating package cache before installing containerd.io...")
	if err := ctx.Host.Runner.UpdatePackageCache(ctx.GoContext); err != nil {
		res.Error = fmt.Errorf("failed to update package cache: %w", err)
		res.Status = "Failed"; res.Message = res.Error.Error(); hostCtxLogger.Errorf("Step failed: %v", res.Error); return res
	}
	hostCtxLogger.Successf("Package cache updated.")

	pkgToInstall := "containerd.io"
	pkgNameForLog := "containerd.io"
	versionForCmd := strings.TrimPrefix(spec.Version, "v") // Remove "v" for commands like yum/dnf

	if spec.Version != "" { // If a specific version is requested
		pmInfo, detectErr := ctx.Host.Runner.DetectPackageManager(ctx.GoContext)
		if detectErr != nil {
			hostCtxLogger.Warnf("Could not detect package manager to format versioned package name for containerd.io %s: %v. Installing '%s' without version formatting.", spec.Version, detectErr, pkgName)
			// Attempt to install with version string as is, might work for some PMs or if user knows the exact format.
			if versionForCmd != "" { pkgToInstall = fmt.Sprintf("%s-%s", pkgName, versionForCmd) } // A common guess
			pkgNameForLog = pkgToInstall
		} else {
			if pmInfo.Type == runner.PackageManagerApt {
				pkgToInstall = fmt.Sprintf("containerd.io=%s", spec.Version) // APT uses original version string
			} else if pmInfo.Type == runner.PackageManagerYum || pmInfo.Type == runner.PackageManagerDnf {
				pkgToInstall = fmt.Sprintf("containerd.io-%s", versionForCmd)
			} else { // Default or unknown
				pkgToInstall = fmt.Sprintf("containerd.io-%s", versionForCmd)
			}
			pkgNameForLog = pkgToInstall
		}
		hostCtxLogger.Infof("Attempting to install %s...", pkgNameForLog)
	} else {
		hostCtxLogger.Infof("Attempting to install latest version of %s...", pkgNameForLog)
	}

	if err := ctx.Host.Runner.InstallPackages(ctx.GoContext, pkgToInstall); err != nil {
		res.Error = fmt.Errorf("failed to install package %s: %w", pkgNameForLog, err)
		res.Status = "Failed"; res.Message = res.Error.Error(); hostCtxLogger.Errorf("Step failed: %v", res.Error); return res
	}

	res.EndTime = time.Now(); res.Status = "Succeeded"
	res.Message = fmt.Sprintf("Package %s installed successfully.", pkgNameForLog)
	hostCtxLogger.Successf("Step succeeded: %s", res.Message)
	return res
}
var _ step.StepExecutor = &InstallContainerdStepExecutor{}
