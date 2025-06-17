package runner

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/kubexms/kubexms/pkg/connector"
)

// Helper to setup runner with a specific OS for package tests
func newTestRunnerForPackage(t *testing.T, osID string) (*Runner, *MockConnector) {
	mockConn := NewMockConnector()
	mockConn.GetOSFunc = func(ctx context.Context) (*connector.OS, error) {
		return &connector.OS{ID: osID, Arch: "amd64", Kernel: "test-kernel"}, nil
	}
	// Default Exec for NewRunner fact gathering & other commands if not overridden
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd
		mockConn.LastExecOptions = options
		if isFactGatheringCommand(cmd) { return []byte("dummy"), nil, nil }
		// Fallback for package commands if not specifically mocked in a test
		return []byte(""), nil, nil
	}
	// Default LookPath for commands that might be checked by detectPackageManager or AddRepository
	mockConn.LookPathFunc = func(ctx context.Context, file string) (string, error) {
		if isFactGatheringCommandLookup(file) { return "/usr/bin/" + file, nil}
		switch file {
		case "apt-get":
			if osID == "ubuntu" || osID == "debian" { return "/usr/bin/apt-get", nil }
		case "yum":
			if osID == "centos" || osID == "rhel" { return "/usr/bin/yum", nil }
		case "dnf":
			if osID == "fedora" || osID == "rhel" { return "/usr/bin/dnf", nil } // RHEL 8+ might have dnf
		case "add-apt-repository":
			if osID == "ubuntu" || osID == "debian" { return "/usr/bin/add-apt-repository", nil }
		case "yum-config-manager":
			if osID == "centos" || osID == "rhel" { return "/usr/bin/yum-config-manager", nil }
		// dnf uses 'dnf config-manager', so 'dnf' itself is the key check.
		default:
			return "", fmt.Errorf("LookPath mock: command %s not found for OS %s in package test setup", file, osID)
		}
		return "", fmt.Errorf("LookPath mock: unhandled case for file %s on OS %s", file, osID)
	}

	r, err := NewRunner(context.Background(), mockConn)
	if err != nil {
		t.Fatalf("Failed to create runner for package tests (OS: %s): %v", osID, err)
	}
	return r, mockConn
}


func TestRunner_InstallPackages_Apt(t *testing.T) {
	r, mockConn := newTestRunnerForPackage(t, "ubuntu")
	packages := []string{"nginx", "vim"}
	expectedCmd := fmt.Sprintf(aptInfo.InstallCmd, strings.Join(packages, " "))

	var cmdExecuted string
	// Override ExecFunc for this specific test
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd
		mockConn.LastExecOptions = options
		if isFactGatheringCommand(cmd) { return []byte("dummy"), nil, nil }
		if cmd == expectedCmd && options.Sudo {
			cmdExecuted = cmd
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("InstallPackages apt: unexpected cmd %s or sudo %v", cmd, options.Sudo)
	}

	err := r.InstallPackages(context.Background(), packages...)
	if err != nil {
		t.Fatalf("InstallPackages() for apt error = %v", err)
	}
	if cmdExecuted != expectedCmd {
		t.Errorf("InstallPackages() for apt command = %q, want %q", cmdExecuted, expectedCmd)
	}
}

func TestRunner_InstallPackages_Yum(t *testing.T) {
	r, mockConn := newTestRunnerForPackage(t, "centos")
	packages := []string{"httpd", "tmux"}
	expectedCmd := fmt.Sprintf(yumDnfInfo.InstallCmd, strings.Join(packages, " "))


	var cmdExecuted string
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd
		mockConn.LastExecOptions = options
		if isFactGatheringCommand(cmd) { return []byte("dummy"), nil, nil }
		if cmd == expectedCmd && options.Sudo {
			cmdExecuted = cmd
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("InstallPackages yum: unexpected cmd %s or sudo %v", cmd, options.Sudo)
	}
	err := r.InstallPackages(context.Background(), packages...)
	if err != nil {
		t.Fatalf("InstallPackages() for yum error = %v", err)
	}
	if cmdExecuted != expectedCmd {
		t.Errorf("InstallPackages() for yum command = %q, want %q", cmdExecuted, expectedCmd)
	}
}

func TestRunner_RemovePackages_Dnf(t *testing.T) {
	r, mockConn := newTestRunnerForPackage(t, "fedora")
	packages := []string{"old-package"}

	dnfSpecificInfo := yumDnfInfo
	dnfSpecificInfo.Type = PackageManagerDnf
	dnfSpecificInfo.RemoveCmd = "dnf remove -y %s"
	// This line was missing in the original request, needed for dnf-specific command:
	dnfSpecificInfo.InstallCmd = "dnf install -y %s"
	dnfSpecificInfo.UpdateCmd = "dnf update -y"
	dnfSpecificInfo.CacheCleanCmd = "dnf clean all"

	expectedCmd := fmt.Sprintf(dnfSpecificInfo.RemoveCmd, strings.Join(packages, " "))


	var cmdExecuted string
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd
		mockConn.LastExecOptions = options
		if isFactGatheringCommand(cmd) { return []byte("dummy"), nil, nil }
		if cmd == expectedCmd && options.Sudo {
			cmdExecuted = cmd
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("RemovePackages dnf: unexpected cmd %s or sudo %v", cmd, options.Sudo)
	}
	err := r.RemovePackages(context.Background(), packages...)
	if err != nil {
		t.Fatalf("RemovePackages() for dnf error = %v", err)
	}
	if cmdExecuted != expectedCmd {
		t.Errorf("RemovePackages() for dnf command = %q, want %q", cmdExecuted, expectedCmd)
	}
}

func TestRunner_UpdatePackageCache_Apt(t *testing.T) {
	r, mockConn := newTestRunnerForPackage(t, "ubuntu")
	expectedCmd := aptInfo.UpdateCmd
	var cmdExecuted string
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd
		mockConn.LastExecOptions = options
		if isFactGatheringCommand(cmd) { return []byte("dummy"), nil, nil }
		if cmd == expectedCmd && options.Sudo {
			cmdExecuted = cmd
			return nil, nil, nil
		}
		return nil, nil, errors.New("UpdatePackageCache apt: unexpected cmd")
	}
	err := r.UpdatePackageCache(context.Background())
	if err != nil {
		t.Fatalf("UpdatePackageCache() for apt error = %v", err)
	}
	if cmdExecuted != expectedCmd {
		t.Errorf("UpdatePackageCache apt cmd = %q, want %q", cmdExecuted, expectedCmd)
	}
}

func TestRunner_IsPackageInstalled_Apt_Installed(t *testing.T) {
	r, mockConn := newTestRunnerForPackage(t, "ubuntu")
	pkg := "nginx"
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd
		mockConn.LastExecOptions = options
		if isFactGatheringCommand(cmd) { return []byte("dummy"), nil, nil }
		// dpkg-query -W -f='${Status}' nginx
		if strings.Contains(cmd, "dpkg-query -W") && strings.Contains(cmd, pkg) {
			return []byte("install ok installed"), nil, nil
		}
		return nil, nil, errors.New("IsPackageInstalled apt: unexpected cmd")
	}
	installed, err := r.IsPackageInstalled(context.Background(), pkg)
	if err != nil {t.Fatalf("IsPackageInstalled apt error = %v", err)}
	if !installed {t.Error("IsPackageInstalled apt = false, want true")}
}

func TestRunner_IsPackageInstalled_Yum_NotInstalled(t *testing.T) {
	r, mockConn := newTestRunnerForPackage(t, "centos")
	pkg := "nonexistent-pkg"
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd
		mockConn.LastExecOptions = options
		if isFactGatheringCommand(cmd) { return []byte("dummy"), nil, nil }
		// rpm -q nonexistent-pkg
		if strings.Contains(cmd, "rpm -q") && strings.Contains(cmd, pkg) {
			return nil, []byte("package nonexistent-pkg is not installed"), &connector.CommandError{ExitCode: 1}
		}
		return nil, nil, errors.New("IsPackageInstalled yum: unexpected cmd")
	}
	installed, err := r.IsPackageInstalled(context.Background(), pkg)
	if err != nil {t.Fatalf("IsPackageInstalled yum error = %v", err)}
	if installed {t.Error("IsPackageInstalled yum = true, want false")}
}

func TestRunner_AddRepository_Apt_PPA(t *testing.T) {
	r, mockConn := newTestRunnerForPackage(t, "ubuntu")
	repoPPA := "ppa:graphics-drivers/ppa"

	var addRepoCmd, updateCmd, installPropsCmd string

	// Mock LookPath to indicate add-apt-repository is initially not found, then found after "install"
	var addAptRepoFound bool
	mockConn.LookPathFunc = func(ctx context.Context, file string) (string, error) {
		if isFactGatheringCommandLookup(file) { return "/usr/bin/" + file, nil}
		if file == "apt-get" { return "/usr/bin/apt-get", nil}
		if file == "add-apt-repository" {
			if addAptRepoFound {
				return "/usr/bin/add-apt-repository", nil
			}
			return "", errors.New("add-apt-repository not found initially")
		}
		return "", fmt.Errorf("LookPath mock for AddRepoApt: unhandled command %s", file)
	}

	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd
		mockConn.LastExecOptions = options
		if isFactGatheringCommand(cmd) { return []byte("dummy"), nil, nil }

		// software-properties-common install
		if strings.Contains(cmd, "apt-get install -y software-properties-common") && options.Sudo {
			installPropsCmd = cmd
			addAptRepoFound = true // Simulate it's found after install
			return nil, nil, nil
		}
		if strings.Contains(cmd, "add-apt-repository -y "+repoPPA) && options.Sudo {
			addRepoCmd = cmd
			return nil, nil, nil
		}
		if cmd == aptInfo.UpdateCmd && options.Sudo {
			updateCmd = cmd
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("AddRepository apt PPA: unexpected cmd: %s", cmd)
	}

	err := r.AddRepository(context.Background(), repoPPA, false)
	if err != nil {
		t.Fatalf("AddRepository apt PPA error = %v", err)
	}
	if installPropsCmd == "" {t.Error("software-properties-common install command not called")}
	if addRepoCmd == "" {t.Error("add-apt-repository command not called")}
	if updateCmd == "" {t.Error("apt-get update not called after add-apt-repository")}
}
