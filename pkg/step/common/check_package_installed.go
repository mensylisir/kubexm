package common

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	// "github.com/Masterminds/semver/v3" // For robust version comparison, if added
)

// CheckPackageInstalledStepSpec defines parameters for checking package installation status and version.
type CheckPackageInstalledStepSpec struct {
	spec.StepMeta `json:",inline"`

	PackageManagerCmd     string `json:"packageManagerCmd,omitempty"`
	PackageName           string `json:"packageName,omitempty"` // Required
	MinVersion            string `json:"minVersion,omitempty"`
	ExactVersion          string `json:"exactVersion,omitempty"`
	OutputInstalledCacheKey string `json:"outputInstalledCacheKey,omitempty"`
	OutputVersionCacheKey   string `json:"outputVersionCacheKey,omitempty"`
	Sudo                  bool   `json:"sudo,omitempty"` // For package query commands, usually not needed
}

// NewCheckPackageInstalledStepSpec creates a new CheckPackageInstalledStepSpec.
func NewCheckPackageInstalledStepSpec(name, description, packageName string) *CheckPackageInstalledStepSpec {
	finalName := name
	if finalName == "" {
		finalName = fmt.Sprintf("Check Package Installed: %s", packageName)
	}
	finalDescription := description
	// Description refined in populateDefaults

	if packageName == "" {
		// This is a required field.
	}

	return &CheckPackageInstalledStepSpec{
		StepMeta: spec.StepMeta{
			Name:        finalName,
			Description: finalDescription,
		},
		PackageName: packageName,
	}
}

// Name returns the step's name.
func (s *CheckPackageInstalledStepSpec) Name() string { return s.StepMeta.Name }

// Description returns the step's description.
func (s *CheckPackageInstalledStepSpec) Description() string { return s.StepMeta.Description }

// GetName returns the step's name for spec interface.
func (s *CheckPackageInstalledStepSpec) GetName() string { return s.StepMeta.Name }

// GetDescription returns the step's description for spec interface.
func (s *CheckPackageInstalledStepSpec) GetDescription() string { return s.StepMeta.Description }

// GetSpec returns the spec itself.
func (s *CheckPackageInstalledStepSpec) GetSpec() interface{} { return s }

// Meta returns the step's metadata.
func (s *CheckPackageInstalledStepSpec) Meta() *spec.StepMeta { return &s.StepMeta }

func (s *CheckPackageInstalledStepSpec) populateDefaults(logger runtime.Logger) {
	// Sudo defaults to false (zero value) for query commands.
	if s.StepMeta.Description == "" {
		desc := fmt.Sprintf("Checks if package '%s' is installed", s.PackageName)
		if s.ExactVersion != "" {
			desc += fmt.Sprintf(" with exact version %s", s.ExactVersion)
		} else if s.MinVersion != "" {
			desc += fmt.Sprintf(" with minimum version %s", s.MinVersion)
		}
		s.StepMeta.Description = desc + "."
	}
}

// compareVersions checks if actualVersion meets minVersion and exactVersion constraints.
// This is a simplified comparison. For robust semver, use a library.
func compareVersions(actualVersion, minVersion, exactVersion string, logger runtime.Logger) (bool, string) {
	if actualVersion == "" {
		return false, "Actual version not found"
	}

	if exactVersion != "" {
		if actualVersion == exactVersion {
			return true, ""
		}
		return false, fmt.Sprintf("version mismatch: actual %s, required exact %s", actualVersion, exactVersion)
	}
	if minVersion != "" {
		// Simplified comparison: assumes versions are dot-separated numbers and compares lexicographically.
		// For proper semver comparison, use a library like Masterminds/semver.
		// e.g., vActual, _ := semver.NewVersion(actualVersion); vMin, _ := semver.NewVersion(minVersion);
		// return !vActual.LessThan(vMin), ""
		if strings.Compare(actualVersion, minVersion) >= 0 {
			return true, ""
		}
		return false, fmt.Sprintf("version too low: actual %s, minimum required %s", actualVersion, minVersion)
	}
	return true, "" // No version constraint to check against
}


// Precheck attempts to use cached information if available.
func (s *CheckPackageInstalledStepSpec) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Precheck")
	s.populateDefaults(logger)

	if s.PackageName == "" {
		return false, fmt.Errorf("PackageName must be specified for %s", s.GetName())
	}

	if s.OutputInstalledCacheKey != "" {
		cachedInstalledVal, found := ctx.StepCache().Get(s.OutputInstalledCacheKey)
		if found {
			isInstalled, ok := cachedInstalledVal.(bool)
			if !ok {
				logger.Warn("Invalid cached installed status type, re-running check.", "key", s.OutputInstalledCacheKey)
				return false, nil
			}

			if !isInstalled { // If cached as not installed
				if s.MinVersion == "" && s.ExactVersion == "" { // And no version requirement
					logger.Info("Package cached as not installed. Precheck done.", "package", s.PackageName)
					return true, nil // Effectively "done" as per cached state of not being installed and no version needed
				}
				// If a version is required, and it's cached as not installed, Run needs to verify this.
				logger.Info("Package cached as not installed, but version check might be required by Run.", "package", s.PackageName)
				return false, nil
			}

			// Cached as installed, now check version if needed
			if s.MinVersion == "" && s.ExactVersion == "" {
				logger.Info("Package cached as installed, no version constraints. Precheck done.", "package", s.PackageName)
				return true, nil
			}

			if s.OutputVersionCacheKey != "" {
				cachedVersionVal, versionFound := ctx.StepCache().Get(s.OutputVersionCacheKey)
				if versionFound {
					cachedVersion, versionOk := cachedVersionVal.(string)
					if !versionOk {
						logger.Warn("Invalid cached version type, re-running check.", "key", s.OutputVersionCacheKey)
						return false, nil
					}
					versionOk, _ := compareVersions(cachedVersion, s.MinVersion, s.ExactVersion, logger)
					if versionOk {
						logger.Info("Package cached as installed with matching version. Precheck done.", "package", s.PackageName, "version", cachedVersion)
						return true, nil
					}
					logger.Info("Package cached as installed but version mismatch. Re-running check.", "package", s.PackageName, "cachedVersion", cachedVersion)
				} else {
					logger.Info("Package cached as installed, but version not in cache. Re-running check.", "package", s.PackageName)
				}
			} else {
				logger.Info("Package cached as installed, but OutputVersionCacheKey not set. Re-running version check if needed.", "package", s.PackageName)
			}
		}
	}
	return false, nil // Default to run the check
}


// Run performs the package installation check.
func (s *CheckPackageInstalledStepSpec) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Run")
	s.populateDefaults(logger)

	if s.PackageName == "" {
		return fmt.Errorf("PackageName must be specified for %s", s.GetName())
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}
	execOpts := &connector.ExecOptions{Sudo: s.Sudo}

	pmCmdToUse := s.PackageManagerCmd
	if pmCmdToUse == "" {
		facts, errFacts := ctx.GetHostFacts(host)
		if errFacts != nil {
			return fmt.Errorf("failed to get host facts to determine package manager for %s: %w", host.GetName(), errFacts)
		}
		// Simplified OS detection for package manager commands
		osID := strings.ToLower(facts.OS.ID)
		switch osID {
		case "ubuntu", "debian":
			// dpkg-query output for installed: "install ok installed <version>"
			// For not installed, it returns non-zero.
			pmCmdToUse = `dpkg-query -W -f='${Status}\n${Version}' ` + s.PackageName + ` || echo "not-installed"`
		case "centos", "rhel", "fedora", "almalinux", "rocky":
			// rpm -q returns 0 and version if installed, non-zero if not.
			// Using --queryformat to get a clean version string.
			pmCmdToUse = `rpm -q --queryformat '%{VERSION}-%{RELEASE}' ` + s.PackageName
		default:
			return fmt.Errorf("unsupported OS '%s' for auto-detecting package manager on host %s", osID, host.GetName())
		}
		logger.Debug("Auto-detected package manager command.", "osID", osID, "command", pmCmdToUse)
	} else {
	    // Replace placeholder if user provided a command template
	    pmCmdToUse = strings.ReplaceAll(pmCmdToUse, "%s", s.PackageName)
	}


	logger.Info("Checking package installation status.", "command", pmCmdToUse)
	stdout, stderr, execErr := conn.Exec(ctx.GoContext(), pmCmdToUse, execOpts)

	installed := false
	foundVersion := ""

	if execErr == nil { // Command succeeded, implies package is likely installed for rpm/dpkg query types
		installed = true
		outputStr := strings.TrimSpace(string(stdout))

		// Parse version based on typical outputs
		// dpkg-query example: "install ok installed\n1.2.3-4ubuntu1"
		if strings.Contains(outputStr, "install ok installed") {
			lines := strings.Split(outputStr, "\n")
			if len(lines) > 1 {
				foundVersion = strings.TrimSpace(lines[1])
			}
		} else { // Assume output is version (e.g., from rpm -q --qf)
			foundVersion = outputStr
		}
	} else { // Command failed
		logger.Info("Package query command failed, assuming package not installed.", "package", s.PackageName, "error", execErr, "stderr", string(stderr))
		installed = false
		// For dpkg, if we used `|| echo "not-installed"`, and `not-installed` is in stdout, it means not installed.
		if strings.Contains(strings.TrimSpace(string(stdout)), "not-installed") {
			installed = false
		} else if _, ok := execErr.(*connector.CommandError); ok {
			// rpm -q exits 1 if not installed. Other errors are actual problems.
			// For now, any error means not installed for simplicity of this check step.
		} else {
		    // Unexpected execution error
		    return fmt.Errorf("failed to execute package query '%s' (stderr: %s): %w", pmCmdToUse, string(stderr), execErr)
		}
	}

	if s.OutputInstalledCacheKey != "" {
		ctx.StepCache().Set(s.OutputInstalledCacheKey, installed)
		logger.Debug("Stored installed status in cache.", "key", s.OutputInstalledCacheKey, "status", installed)
	}
	if s.OutputVersionCacheKey != "" && foundVersion != "" {
		ctx.StepCache().Set(s.OutputVersionCacheKey, foundVersion)
		logger.Debug("Stored found version in cache.", "key", s.OutputVersionCacheKey, "version", foundVersion)
	}

	if !installed {
		msg := fmt.Sprintf("Package '%s' is not installed.", s.PackageName)
		if s.MinVersion != "" || s.ExactVersion != "" { // If a version was required, non-installation is a failure of criteria
			logger.Error(msg + " Version requirements not met.")
			// This step's role is to check and report, not to fail the pipeline if package not found.
			// If this check *must* pass, a subsequent validation step or task logic should handle it.
		} else {
			logger.Info(msg)
		}
		return nil // Not an error for the step itself if package is simply not there.
	}

	// Package is installed, check version constraints
	logger.Info("Package is installed.", "package", s.PackageName, "foundVersion", foundVersion)
	versionOk, reason := compareVersions(foundVersion, s.MinVersion, s.ExactVersion, logger)
	if !versionOk {
		logger.Error(fmt.Sprintf("Package '%s' version check failed: %s", s.PackageName, reason))
		// Again, this step reports; it doesn't fail the pipeline based on criteria.
	} else {
		logger.Info(fmt.Sprintf("Package '%s' version check passed.", s.PackageName), "details", reason)
	}
	return nil
}

// Rollback for CheckPackageInstalledStep is a no-op.
func (s *CheckPackageInstalledStepSpec) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Rollback")
	logger.Debug("CheckPackageInstalledStep has no rollback action.")
	return nil
}

var _ step.Step = (*CheckPackageInstalledStepSpec)(nil)
