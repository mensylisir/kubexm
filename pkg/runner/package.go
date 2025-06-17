package runner

import (
	"context"
	"fmt"
	"strings"

	"github.com/kubexms/kubexms/pkg/connector" // Assuming this is the correct path
)

// PackageManagerType defines the type of package manager.
type PackageManagerType string

const (
	PackageManagerUnknown PackageManagerType = "unknown"
	PackageManagerApt     PackageManagerType = "apt"
	PackageManagerYum     PackageManagerType = "yum" // Covers dnf as commands are mostly compatible
	PackageManagerDnf     PackageManagerType = "dnf"
)

// PackageInfo holds details about the detected package manager.
type PackageInfo struct {
	Type         PackageManagerType
	UpdateCmd    string
	InstallCmd   string
	RemoveCmd    string
	AddRepoCmd   string // This might be more complex, involving adding files or specific commands
	PkgQueryCmd  string // Command to query if a package is installed
	CacheCleanCmd string
}

var (
	aptInfo = PackageInfo{
		Type:         PackageManagerApt,
		UpdateCmd:    "apt-get update -y",
		InstallCmd:   "apt-get install -y %s",      // %s for package names
		RemoveCmd:    "apt-get remove -y %s",       // %s for package names
		PkgQueryCmd:  "dpkg-query -W -f='${Status}' %s", // Example: dpkg-query -W -f='${Status}' nginx -> "install ok installed"
		CacheCleanCmd: "apt-get clean",
		// AddRepoCmd for apt usually involves `add-apt-repository` or manually adding .list files.
		// This is simplified here; a robust solution needs more logic.
	}
	yumDnfInfo = PackageInfo{ // DNF is largely command-compatible with YUM for basic operations
		Type:         PackageManagerYum, // Default to Yum, can be refined to Dnf by detection
		UpdateCmd:    "yum update -y", // or dnf update -y
		InstallCmd:   "yum install -y %s",
		RemoveCmd:    "yum remove -y %s",
		PkgQueryCmd:  "rpm -q %s",       // Example: rpm -q nginx -> "nginx-1.18.0-2.el8.x86_64" or "package nginx is not installed"
		CacheCleanCmd: "yum clean all", // or dnf clean all
	}
)

// detectPackageManager attempts to identify the package manager on the host.
// It caches the result in the Runner's Facts if not already detected.
func (r *Runner) detectPackageManager(ctx context.Context) (*PackageInfo, error) {
	if r.Facts == nil || r.Facts.OS == nil {
		return nil, fmt.Errorf("OS facts not available, cannot detect package manager")
	}
	// Check if already detected (e.g. stored in a new field in Facts if we add one)
	// For now, detect every time or rely on OS ID.

	switch strings.ToLower(r.Facts.OS.ID) {
	case "ubuntu", "debian", "raspbian", "linuxmint": // Add other Debian-based distros
		return &aptInfo, nil
	case "centos", "rhel", "fedora", "almalinux", "rocky": // Add other RHEL-based distros
		// Refine for DNF vs YUM if necessary. Fedora uses DNF. CentOS/RHEL 8+ use DNF.
		// We can check for `dnf` command first.
		if _, err := r.LookPath(ctx, "dnf"); err == nil {
			// Create a DNF specific variant if commands differ significantly beyond basic install/remove
			dnfSpecificInfo := yumDnfInfo // Copy base yum info
			dnfSpecificInfo.Type = PackageManagerDnf
			dnfSpecificInfo.UpdateCmd = "dnf update -y"
			dnfSpecificInfo.InstallCmd = "dnf install -y %s"
			dnfSpecificInfo.RemoveCmd = "dnf remove -y %s"
			dnfSpecificInfo.CacheCleanCmd = "dnf clean all"
			// PkgQueryCmd for dnf might be `dnf list installed %s`
			return &dnfSpecificInfo, nil
		}
		// If dnf not found, assume yum (for older CentOS/RHEL)
		return &yumDnfInfo, nil
	// TODO: Add cases for other package managers like pacman (Arch), zypper (SLES/OpenSUSE), etc.
	default:
		// Fallback: try to detect by command existence
		if _, err := r.LookPath(ctx, "apt-get"); err == nil {
			return &aptInfo, nil
		}
		if _, err := r.LookPath(ctx, "dnf"); err == nil {
			dnfSpecificInfo := yumDnfInfo
			dnfSpecificInfo.Type = PackageManagerDnf
			dnfSpecificInfo.UpdateCmd = "dnf update -y"
			dnfSpecificInfo.InstallCmd = "dnf install -y %s"
			dnfSpecificInfo.RemoveCmd = "dnf remove -y %s"
			dnfSpecificInfo.CacheCleanCmd = "dnf clean all"
			return &dnfSpecificInfo, nil
		}
		if _, err := r.LookPath(ctx, "yum"); err == nil {
			return &yumDnfInfo, nil
		}
		return nil, fmt.Errorf("unsupported OS or unable to detect package manager for OS ID: %s", r.Facts.OS.ID)
	}
}

// InstallPackages installs one or more packages.
func (r *Runner) InstallPackages(ctx context.Context, packages ...string) error {
	if r.Conn == nil {
		return fmt.Errorf("runner has no valid connector")
	}
	if len(packages) == 0 {
		return fmt.Errorf("no packages specified for installation")
	}

	pmInfo, err := r.detectPackageManager(ctx)
	if err != nil {
		return err
	}

	packageStr := strings.Join(packages, " ")
	cmd := fmt.Sprintf(pmInfo.InstallCmd, packageStr)

	// Package installation usually requires sudo.
	_, stderr, execErr := r.RunWithOptions(ctx, cmd, &connector.ExecOptions{Sudo: true})
	if execErr != nil {
		return fmt.Errorf("failed to install packages '%s' using %s: %w (stderr: %s)", packageStr, pmInfo.Type, execErr, string(stderr))
	}
	return nil
}

// RemovePackages removes one or more packages.
func (r *Runner) RemovePackages(ctx context.Context, packages ...string) error {
	if r.Conn == nil {
		return fmt.Errorf("runner has no valid connector")
	}
	if len(packages) == 0 {
		return fmt.Errorf("no packages specified for removal")
	}

	pmInfo, err := r.detectPackageManager(ctx)
	if err != nil {
		return err
	}

	packageStr := strings.Join(packages, " ")
	cmd := fmt.Sprintf(pmInfo.RemoveCmd, packageStr)

	// Package removal usually requires sudo.
	_, stderr, execErr := r.RunWithOptions(ctx, cmd, &connector.ExecOptions{Sudo: true})
	if execErr != nil {
		return fmt.Errorf("failed to remove packages '%s' using %s: %w (stderr: %s)", packageStr, pmInfo.Type, execErr, string(stderr))
	}
	return nil
}

// UpdatePackageCache updates the local package cache (e.g., apt-get update).
func (r *Runner) UpdatePackageCache(ctx context.Context) error {
	if r.Conn == nil {
		return fmt.Errorf("runner has no valid connector")
	}
	pmInfo, err := r.detectPackageManager(ctx)
	if err != nil {
		return err
	}

	cmd := pmInfo.UpdateCmd
	// Cache update usually requires sudo.
	_, stderr, execErr := r.RunWithOptions(ctx, cmd, &connector.ExecOptions{Sudo: true})
	if execErr != nil {
		return fmt.Errorf("failed to update package cache using %s: %w (stderr: %s)", pmInfo.Type, execErr, string(stderr))
	}
	return nil
}

// IsPackageInstalled checks if a single package is installed.
// Note: This is a basic check. Version comparison or more complex status checks might be needed.
func (r *Runner) IsPackageInstalled(ctx context.Context, packageName string) (bool, error) {
	if r.Conn == nil {
		return false, fmt.Errorf("runner has no valid connector")
	}
	if strings.TrimSpace(packageName) == "" {
		return false, fmt.Errorf("packageName cannot be empty")
	}

	pmInfo, err := r.detectPackageManager(ctx)
	if err != nil {
		return false, err
	}
	if pmInfo.PkgQueryCmd == "" {
		return false, fmt.Errorf("package query command not defined for %s", pmInfo.Type)
	}

	cmd := fmt.Sprintf(pmInfo.PkgQueryCmd, packageName)
	// Sudo is typically not required for querying package status.
	// The output of these commands varies. `rpm -q` exits 0 if installed.
	// `dpkg-query` exits 0 if package is known (installed or not), so output must be checked.

	stdout, _, execErr := r.RunWithOptions(ctx, cmd, &connector.ExecOptions{Sudo: false})

	if pmInfo.Type == PackageManagerApt {
		if execErr != nil { // dpkg-query returns non-zero if package is completely unknown
			return false, nil // Treat as not installed
		}
		// Example: "install ok installed"
		return strings.Contains(string(stdout), "install ok installed"), nil
	} else if pmInfo.Type == PackageManagerYum || pmInfo.Type == PackageManagerDnf {
		// rpm -q exits 0 if installed, non-zero otherwise.
		return execErr == nil, nil
	}

	// Fallback or unknown package manager type for query
	return false, fmt.Errorf("package installed check not fully implemented for %s or query failed: %v", pmInfo.Type, execErr)
}


// AddRepository adds a software repository.
// This is highly dependent on the package manager and OS.
// For apt: add-apt-repository, or editing sources.list.d.
// For yum/dnf: yum-config-manager --add-repo, or creating .repo files in /etc/yum.repos.d.
// This is a placeholder for a more complex implementation.
func (r *Runner) AddRepository(ctx context.Context, repoConfig string, isFilePath bool) error {
	if r.Conn == nil {
		return fmt.Errorf("runner has no valid connector")
	}
	pmInfo, err := r.detectPackageManager(ctx)
	if err != nil {
		return err
	}

	// This is a very simplified example and likely needs significant enhancement.
	if pmInfo.Type == PackageManagerApt {
		// Assuming repoConfig is a PPA string like "ppa:user/repo" for add-apt-repository
		// Or it could be the content of a .list file.
		if !isFilePath { // Assume it's a PPA or similar identifier
			// Ensure add-apt-repository is installed
			if _, err := r.LookPath(ctx, "add-apt-repository"); err != nil {
				if installErr := r.InstallPackages(ctx, "software-properties-common"); installErr != nil {
					return fmt.Errorf("failed to install software-properties-common (for add-apt-repository): %w", installErr)
				}
			}
			cmd := fmt.Sprintf("add-apt-repository -y %s", repoConfig)
			_, stderr, execErr := r.RunWithOptions(ctx, cmd, &connector.ExecOptions{Sudo: true})
			if execErr != nil {
				return fmt.Errorf("failed to add apt repository '%s': %w (stderr: %s)", repoConfig, execErr, string(stderr))
			}
			return r.UpdatePackageCache(ctx) // Update cache after adding repo
		}
		// If isFilePath is true, repoConfig is a path to a .list file content (not implemented here)
		return fmt.Errorf("AddRepository for apt with file path not yet implemented")

	} else if pmInfo.Type == PackageManagerYum || pmInfo.Type == PackageManagerDnf {
		// Assuming repoConfig is a URL to a .repo file or content of a .repo file
		if isFilePath { // repoConfig is content of a .repo file to be placed
			// Determine remote path, e.g., /etc/yum.repos.d/custom.repo
			// This needs a target filename.
			return fmt.Errorf("AddRepository for yum/dnf with file content not fully implemented (needs dest path)")
		} else { // Assume repoConfig is a URL to a .repo file
			cmd := ""
			if pmInfo.Type == PackageManagerDnf {
				cmd = fmt.Sprintf("dnf config-manager --add-repo %s", repoConfig)
			} else { // YUM
				// yum-config-manager might not be installed by default.
				// Need to install 'yum-utils' package first.
				if _, err := r.LookPath(ctx, "yum-config-manager"); err != nil {
					if installErr := r.InstallPackages(ctx, "yum-utils"); installErr != nil {
						return fmt.Errorf("failed to install yum-utils (for yum-config-manager): %w", installErr)
					}
				}
				cmd = fmt.Sprintf("yum-config-manager --add-repo %s", repoConfig)
			}
			_, stderr, execErr := r.RunWithOptions(ctx, cmd, &connector.ExecOptions{Sudo: true})
			if execErr != nil {
				return fmt.Errorf("failed to add yum/dnf repository from URL '%s': %w (stderr: %s)", repoConfig, execErr, string(stderr))
			}
			return nil // Cache update might be needed depending on config-manager behavior
		}
	}
	return fmt.Errorf("AddRepository not implemented for package manager type: %s", pmInfo.Type)
}

// TODO: Add more package management functions:
// - UpgradePackages(ctx context.Context, packages ...string) error (upgrade specific packages)
// - UpgradeAllPackages(ctx context.Context) error (system upgrade)
// - CleanPackageCache(ctx context.Context) error
