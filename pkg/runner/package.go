package runner

import (
	"context"
	"fmt"
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector"
)

// Constants for PackageInfo, already defined in interface.go and runner.go (via interface embedding)
// They are good here for reference or if this file were to be used more independently.
// However, to avoid re-declaration if these files are part of the same package build,
// these might be removed if they are truly identical to those in interface.go's type block.
// For this refactor, assuming the type definitions in interface.go are canonical for the package.

// detectPackageManager is now a private method of defaultRunner, located in runner.go

// InstallPackages installs one or more packages.
func (r *defaultRunner) InstallPackages(ctx context.Context, conn connector.Connector, facts *Facts, packages ...string) error {
	if conn == nil {
		return fmt.Errorf("connector cannot be nil")
	}
	if facts == nil || facts.PackageManager == nil {
		return fmt.Errorf("package manager facts not available")
	}
	if len(packages) == 0 {
		return fmt.Errorf("no packages specified for installation")
	}

	pmInfo := facts.PackageManager
	packageStr := strings.Join(packages, " ")
	cmd := fmt.Sprintf(pmInfo.InstallCmd, packageStr)

	_, _, execErr := r.RunWithOptions(ctx, conn, cmd, &connector.ExecOptions{Sudo: true})
	if execErr != nil {
		return fmt.Errorf("failed to install packages '%s' using %s: %w", packageStr, pmInfo.Type, execErr)
	}
	return nil
}

// RemovePackages removes one or more packages.
func (r *defaultRunner) RemovePackages(ctx context.Context, conn connector.Connector, facts *Facts, packages ...string) error {
	if conn == nil {
		return fmt.Errorf("connector cannot be nil")
	}
	if facts == nil || facts.PackageManager == nil {
		return fmt.Errorf("package manager facts not available")
	}
	if len(packages) == 0 {
		return fmt.Errorf("no packages specified for removal")
	}

	pmInfo := facts.PackageManager
	packageStr := strings.Join(packages, " ")
	cmd := fmt.Sprintf(pmInfo.RemoveCmd, packageStr)

	_, _, execErr := r.RunWithOptions(ctx, conn, cmd, &connector.ExecOptions{Sudo: true})
	if execErr != nil {
		return fmt.Errorf("failed to remove packages '%s' using %s: %w", packageStr, pmInfo.Type, execErr)
	}
	return nil
}

// UpdatePackageCache updates the local package cache (e.g., apt-get update).
func (r *defaultRunner) UpdatePackageCache(ctx context.Context, conn connector.Connector, facts *Facts) error {
	if conn == nil {
		return fmt.Errorf("connector cannot be nil")
	}
	if facts == nil || facts.PackageManager == nil {
		return fmt.Errorf("package manager facts not available")
	}

	pmInfo := facts.PackageManager
	cmd := pmInfo.UpdateCmd
	_, _, execErr := r.RunWithOptions(ctx, conn, cmd, &connector.ExecOptions{Sudo: true})
	if execErr != nil {
		return fmt.Errorf("failed to update package cache using %s: %w", pmInfo.Type, execErr)
	}
	return nil
}

// IsPackageInstalled checks if a single package is installed.
func (r *defaultRunner) IsPackageInstalled(ctx context.Context, conn connector.Connector, facts *Facts, packageName string) (bool, error) {
	if conn == nil {
		return false, fmt.Errorf("connector cannot be nil")
	}
	if facts == nil || facts.PackageManager == nil {
		return false, fmt.Errorf("package manager facts not available")
	}
	if strings.TrimSpace(packageName) == "" {
		return false, fmt.Errorf("packageName cannot be empty")
	}

	pmInfo := facts.PackageManager
	if pmInfo.PkgQueryCmd == "" {
		return false, fmt.Errorf("package query command not defined for %s", pmInfo.Type)
	}

	cmd := fmt.Sprintf(pmInfo.PkgQueryCmd, packageName)
	stdout, _, execErr := r.RunWithOptions(ctx, conn, cmd, &connector.ExecOptions{Sudo: false})

	if pmInfo.Type == PackageManagerApt {
		if execErr != nil {
			return false, nil
		}
		return strings.Contains(string(stdout), "install ok installed"), nil
	} else if pmInfo.Type == PackageManagerYum || pmInfo.Type == PackageManagerDnf {
		return execErr == nil, nil
	}
	return false, fmt.Errorf("package installed check not fully implemented for %s or query failed: %v", pmInfo.Type, execErr)
}

// AddRepository adds a software repository.
func (r *defaultRunner) AddRepository(ctx context.Context, conn connector.Connector, facts *Facts, repoConfig string, isFilePath bool) error {
	if conn == nil {
		return fmt.Errorf("connector cannot be nil")
	}
	if facts == nil || facts.PackageManager == nil {
		return fmt.Errorf("package manager facts not available")
	}

	pmInfo := facts.PackageManager
	if pmInfo.Type == PackageManagerApt {
		if !isFilePath {
			if _, err := r.LookPath(ctx, conn, "add-apt-repository"); err != nil {
				// Pass facts to InstallPackages
				if installErr := r.InstallPackages(ctx, conn, facts, "software-properties-common"); installErr != nil {
					return fmt.Errorf("failed to install software-properties-common (for add-apt-repository): %w", installErr)
				}
			}
			cmd := fmt.Sprintf("add-apt-repository -y %s", repoConfig)
			_, _, execErr := r.RunWithOptions(ctx, conn, cmd, &connector.ExecOptions{Sudo: true})
			if execErr != nil {
				return fmt.Errorf("failed to add apt repository '%s': %w", repoConfig, execErr)
			}
			return r.UpdatePackageCache(ctx, conn, facts) // Pass facts
		}
		return fmt.Errorf("AddRepository for apt with file path not yet implemented")

	} else if pmInfo.Type == PackageManagerYum || pmInfo.Type == PackageManagerDnf {
		if isFilePath {
			return fmt.Errorf("AddRepository for yum/dnf with file content not fully implemented (needs dest path)")
		} else {
			cmd := ""
			if pmInfo.Type == PackageManagerDnf {
				cmd = fmt.Sprintf("dnf config-manager --add-repo %s", repoConfig)
			} else {
				if _, err := r.LookPath(ctx, conn, "yum-config-manager"); err != nil {
					// Pass facts to InstallPackages
					if installErr := r.InstallPackages(ctx, conn, facts, "yum-utils"); installErr != nil {
						return fmt.Errorf("failed to install yum-utils (for yum-config-manager): %w", installErr)
					}
				}
				cmd = fmt.Sprintf("yum-config-manager --add-repo %s", repoConfig)
			}
			_, _, execErr := r.RunWithOptions(ctx, conn, cmd, &connector.ExecOptions{Sudo: true})
			if execErr != nil {
				return fmt.Errorf("failed to add yum/dnf repository from URL '%s': %w", repoConfig, execErr)
			}
			return nil
		}
	}
	return fmt.Errorf("AddRepository not implemented for package manager type: %s", pmInfo.Type)
}
