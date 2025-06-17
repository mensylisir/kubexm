package containerd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/kubexms/kubexms/pkg/runner" // For runner.PackageManagerType and DetectPackageManager
	"github.com/kubexms/kubexms/pkg/runtime"
	"github.com/kubexms/kubexms/pkg/step"
)

// InstallContainerdStep installs containerd.io package.
// It assumes that prerequisite steps like adding repository keys or specific repo files
// are handled either by this step's logic (if simple enough) or by preceding steps.
type InstallContainerdStep struct {
	// Version specifies the version of containerd.io to install.
	// If empty, the package manager will install the latest available version.
	// Format: e.g., "1.6.9-1" (apt) or "1.6.9" (yum/dnf might need full NVR).
	Version string
	// TODO: Add fields for repository URL, GPG key URL if this step should manage them.
	// For now, assumes repos are configured by a separate, preceding step or system setup.
}

// Name returns a human-readable name for the step.
func (s *InstallContainerdStep) Name() string {
	if s.Version != "" {
		return fmt.Sprintf("Install containerd.io (version %s)", s.Version)
	}
	return "Install containerd.io (latest)"
}

// Check determines if containerd.io is already installed (and optionally at the correct version).
func (s *InstallContainerdStep) Check(ctx *runtime.Context) (isDone bool, err error) {
	if ctx.Host.Runner == nil {
		return false, fmt.Errorf("runner not available in context for host %s", ctx.Host.Name)
	}
	hostCtxLogger := ctx.Logger.SugaredLogger.With("host", ctx.Host.Name, "step", s.Name()).Sugar()

	pkgName := "containerd.io"
	installed, err := ctx.Host.Runner.IsPackageInstalled(ctx.GoContext, pkgName)
	if err != nil {
		return false, fmt.Errorf("failed to check if package %s is installed on host %s: %w", pkgName, ctx.Host.Name, err)
	}
	if !installed {
		hostCtxLogger.Debugf("Package %s is not installed.", pkgName)
		return false, nil
	}

	if s.Version == "" {
		hostCtxLogger.Infof("Package %s is installed (latest version or version not specified for check).", pkgName)
		return true, nil // Installed, and no specific version requested.
	}

	// Specific version is required, try to check it.
	hostCtxLogger.Debugf("Package %s is installed, checking version against required '%s'.", pkgName, s.Version)
	var versionCmd string
	pmInfo, detectErr := ctx.Host.Runner.DetectPackageManager(ctx.GoContext)
	if detectErr != nil {
		hostCtxLogger.Warnf("Could not detect package manager to verify containerd version: %v. Assuming not correct version to be safe.", detectErr)
		return false, nil // Cannot verify version, let Run attempt to install specific version.
	}

	switch pmInfo.Type {
	case runner.PackageManagerApt:
		versionCmd = fmt.Sprintf("apt-cache policy %s | grep 'Installed:' | awk '{print $2}'", pkgName)
	case runner.PackageManagerYum, runner.PackageManagerDnf:
		versionCmd = fmt.Sprintf("rpm -q --queryformat '%%{VERSION}-%%{RELEASE}' %s", pkgName)
	default:
		hostCtxLogger.Warnf("Version check not implemented for package manager type %s. Assuming not correct version.", pmInfo.Type)
		return false, nil // Cannot verify version.
	}

	stdout, stderr, execErr := ctx.Host.Runner.Run(ctx.GoContext, versionCmd, false)
	if execErr != nil {
		hostCtxLogger.Warnf("Failed to get installed version of %s (command: '%s'): %v. Stderr: %s. Assuming not correct version.", pkgName, versionCmd, execErr, string(stderr))
		return false, nil // Error getting version, let Run handle it.
	}
	installedVersion := strings.TrimSpace(string(stdout))

	// Robust version comparison can be complex (e.g. "1.6.9-1" vs "1.6.9", or "5:1.6.9-1" vs "1.6.9-1")
	// For APT, `apt-cache policy` might return "(none)" if not installed, or the exact version string.
	// For RPM, `rpm -q` returns N-V-R.
	// A common approach is to check if the required version is a prefix of the installed one,
	// or an exact match.
	if strings.HasPrefix(installedVersion, s.Version) {
		hostCtxLogger.Infof("Package %s version %s is installed and matches required version %s.", pkgName, installedVersion, s.Version)
		return true, nil // Correct version installed
	}
	hostCtxLogger.Infof("Package %s version %s is installed, but required version is %s. Will attempt to install specific version.", pkgName, installedVersion, s.Version)
	return false, nil // Installed but not the right version
}

// Run installs containerd.io.
func (s *InstallContainerdStep) Run(ctx *runtime.Context) *step.Result {
	startTime := time.Now()
	res := step.NewResult(s.Name(), ctx.Host.Name, startTime, nil)
	hostCtxLogger := ctx.Logger.SugaredLogger.With("host", ctx.Host.Name, "step", s.Name()).Sugar()

	if ctx.Host.Runner == nil {
		res.Error = fmt.Errorf("runner not available in context for host %s", ctx.Host.Name)
		res.Status = "Failed"; res.Message = res.Error.Error()
		hostCtxLogger.Errorf("Step failed: %v", res.Error)
		return res
	}

	// For now, assume the repository is already configured by a preceding step.
	// Future: Add repo setup logic here or ensure it's handled by a dedicated step.
	hostCtxLogger.Infof("Ensuring package cache is updated before installing containerd.io...")
	if err := ctx.Host.Runner.UpdatePackageCache(ctx.GoContext); err != nil {
		res.Error = fmt.Errorf("failed to update package cache on host %s: %w", ctx.Host.Name, err)
		res.Status = "Failed"; res.Message = res.Error.Error()
		hostCtxLogger.Errorf("Step failed: %v", res.Error)
		return res
	}
	hostCtxLogger.Successf("Package cache updated.")

	pkgToInstall := "containerd.io"
	versionForCmd := s.Version // Use the raw version string from the step

	if versionForCmd != "" {
		pmInfo, detectErr := ctx.Host.Runner.DetectPackageManager(ctx.GoContext)
		if detectErr != nil {
			hostCtxLogger.Warnf("Could not detect package manager to format versioned package name for %s on host %s: %v. Installing '%s' without specific version string format.", pkgName, ctx.Host.Name, detectErr, pkgName)
			// proceed with pkgToInstall = "containerd.io" and let package manager handle if version is appended weirdly
		} else {
			if pmInfo.Type == runner.PackageManagerApt {
				// For apt, "name=version"
				pkgToInstall = fmt.Sprintf("%s=%s", pkgName, versionForCmd)
			} else if pmInfo.Type == runner.PackageManagerYum || pmInfo.Type == runner.PackageManagerDnf {
				// For yum/dnf, "name-version" (may include release, arch etc.)
				// User should provide version string that yum/dnf understands.
				pkgToInstall = fmt.Sprintf("%s-%s", pkgName, versionForCmd)
			} else {
				hostCtxLogger.Warnf("Package manager type %s on host %s does not have specific version formatting logic; using raw version string if provided with package name.", pmInfo.Type, ctx.Host.Name)
				// Default to name-version, or just name if version is complex for this PM
				pkgToInstall = fmt.Sprintf("%s-%s", pkgName, versionForCmd)
			}
		}
		hostCtxLogger.Infof("Attempting to install %s on host %s...", pkgToInstall, ctx.Host.Name)
	} else {
		hostCtxLogger.Infof("Attempting to install latest version of %s on host %s...", pkgToInstall, ctx.Host.Name)
	}


	if err := ctx.Host.Runner.InstallPackages(ctx.GoContext, pkgToInstall); err != nil {
		res.Error = fmt.Errorf("failed to install package %s on host %s: %w", pkgToInstall, ctx.Host.Name, err)
		res.Status = "Failed"; res.Message = res.Error.Error()
		hostCtxLogger.Errorf("Step failed: %v", res.Error)
		return res
	}

	res.EndTime = time.Now()
	res.Status = "Succeeded"
	res.Message = fmt.Sprintf("Package %s installed successfully on host %s.", pkgToInstall, ctx.Host.Name)
	hostCtxLogger.Successf("Step succeeded: %s", res.Message)
	return res
}

var _ step.Step = &InstallContainerdStep{}
