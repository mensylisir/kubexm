package runner

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/mensylisir/kubexm/pkg/connector"
)

// isFactGatheringCommand is a helper used in some mock ExecFuncs
func isFactGatheringCommand(cmd string) bool {
	factCmds := []string{"hostname", "uname -r", "nproc", "grep MemTotal", "ip -4 route", "ip -6 route", "command -v", "test -e /etc/init.d"}
	for _, fc := range factCmds {
		if strings.Contains(cmd, fc) {
			return true
		}
	}
	return false
}


// Helper to setup runner with a specific OS for package tests
func newTestRunnerForPackage(t *testing.T, osID string) (Runner, *Facts, *MockConnector) { // Updated signature
	mockConn := NewMockConnector()
	// Setup mockConn.GetOSFunc and mockConn.ExecFunc for basic fact gathering
	mockConn.GetOSFunc = func(ctx context.Context) (*connector.OS, error) {
		return &connector.OS{ID: osID, Arch: "amd64", Kernel: "test-kernel"}, nil
	}
	// This ExecFunc needs to cover commands used by GatherFacts, including those for detectPackageManager/detectInitSystem
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd
		mockConn.LastExecOptions = options
		if mockConn.ExecHistory == nil { mockConn.ExecHistory = []string{} }
		mockConn.ExecHistory = append(mockConn.ExecHistory, cmd)

		if strings.Contains(cmd, "hostname") { return []byte("pkg-test-host"), nil, nil }
		if strings.Contains(cmd, "nproc") { return []byte("1"), nil, nil }
		if strings.Contains(cmd, "grep MemTotal") { return []byte("1024000"), nil, nil } // 1GB
		if strings.Contains(cmd, "ip -4 route get 8.8.8.8") { return []byte("8.8.8.8 dev eth0 src 1.1.1.1"), nil, nil }
		if strings.Contains(cmd, "ip -6 route get") { return nil, nil, fmt.Errorf("no ipv6") }

		// Mock commands for detectPackageManager (these are run via LookPath by detectPackageManager, not Exec directly usually)
		// However, if LookPath fails and detectPackageManager tries something else via Exec, this might be hit.
		// The LookPathFunc mock is more critical for detectPackageManager.
		// For detectInitSystem, 'test -e /etc/init.d' is an Exec.
		if strings.HasPrefix(cmd, "test -e /etc/init.d") { return nil, nil, errors.New("no /etc/init.d for this osID mock or systemd found first")}


		// Fallback for other commands
		// fmt.Printf("newTestRunnerForPackage default ExecFunc called for: %s\n", cmd)
		return []byte("default exec output for package test fact gathering"), nil, nil
	}
	// LookPathFunc is crucial for detectPackageManager and AddRepository
	mockConn.LookPathFunc = func(ctx context.Context, file string) (string, error) {
		 // General tools that might be looked up by GatherFacts or specific methods
		if file == "hostname" || file == "uname" || file == "nproc" || file == "grep" || file == "awk" || file == "ip" || file == "cat" {
			return "/usr/bin/" + file, nil
		}
		// Package manager tools
		if file == "apt-get" && (osID == "ubuntu" || osID == "debian" || osID == "raspbian" || osID == "linuxmint") { return "/usr/bin/apt-get", nil }
		if file == "yum" && (osID == "centos" || osID == "rhel" || osID == "fedora"  || osID == "almalinux" || osID == "rocky" ) { return "/usr/bin/yum", nil } // Added fedora etc. to yum path for simplicity if dnf fails
		if file == "dnf" && (osID == "fedora" || osID == "rhel" || osID == "almalinux" || osID == "rocky") { return "/usr/bin/dnf", nil }
		// Init system tools
		if file == "systemctl" { return "/usr/bin/systemctl", nil }
		if file == "service" { return "/usr/sbin/service", nil }
		// Repo tools
		if file == "add-apt-repository" && (osID == "ubuntu" || osID == "debian") {return "/usr/bin/add-apt-repository", nil}
		if file == "yum-config-manager" && (osID == "centos" || osID == "rhel") {return "/usr/bin/yum-config-manager", nil}

		// fmt.Printf("LookPath mock (package test) called for: %s on OS %s - returning not found by default\n", file, osID)
		return "", fmt.Errorf("LookPath mock (package): command %s not found for OS %s", file, osID)
	}

	runnerInstance := NewRunner() // Corrected call
	facts, err := runnerInstance.GatherFacts(context.Background(), mockConn)
	if err != nil {
		t.Fatalf("newTestRunnerForPackage: Failed to gather facts for OS %s: %v", osID, err)
	}
	if facts == nil {
        t.Fatalf("newTestRunnerForPackage: GatherFacts returned nil facts for OS %s", osID)
    }
	// It's crucial that facts.PackageManager is populated correctly by the mocked GatherFacts.
	if facts.PackageManager == nil {
		 // This indicates an issue in the mock setup for detectPackageManager (likely LookPath or OS ID mapping)
         t.Logf("Warning: newTestRunnerForPackage for OS '%s': facts.PackageManager is nil. Check mock LookPath for apt-get/yum/dnf.", osID)
	}
	if facts.InitSystem == nil {
		t.Logf("Warning: newTestRunnerForPackage for OS '%s': facts.InitSystem is nil. Check mock LookPath for systemctl/service.", osID)
	}
	return runnerInstance, facts, mockConn
}


func TestRunner_InstallPackages_Apt(t *testing.T) {
	r, facts, mockConn := newTestRunnerForPackage(t, "ubuntu")
	if facts.PackageManager == nil || facts.PackageManager.Type != PackageManagerApt {
		t.Fatalf("Test setup error: expected Apt package manager for ubuntu, got: %v", facts.PackageManager)
	}
	pmInfo := facts.PackageManager

	packages := []string{"nginx", "vim"}
	expectedCmd := fmt.Sprintf(pmInfo.InstallCmd, strings.Join(packages, " "))

	var cmdExecuted string
	// Override ExecFunc for this specific test
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd
		mockConn.LastExecOptions = options
		// Allow fact-gathering commands from the helper if any are re-triggered by nested calls (unlikely for InstallPackages)
		if isFactGatheringCommand(cmd) { return []byte("dummy"), nil, nil }
		if cmd == expectedCmd && options.Sudo {
			cmdExecuted = cmd
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("InstallPackages apt: unexpected cmd %s or sudo %v", cmd, options.Sudo)
	}

	err := r.InstallPackages(context.Background(), mockConn, facts, packages...)
	if err != nil {
		t.Fatalf("InstallPackages() for apt error = %v", err)
	}
	if cmdExecuted != expectedCmd {
		t.Errorf("InstallPackages() for apt command = %q, want %q", cmdExecuted, expectedCmd)
	}
}

func TestRunner_InstallPackages_Yum(t *testing.T) {
	r, facts, mockConn := newTestRunnerForPackage(t, "centos")
	if facts.PackageManager == nil || facts.PackageManager.Type != PackageManagerYum {
		t.Fatalf("Test setup error: expected Yum package manager for centos, got: %v", facts.PackageManager)
	}
	pmInfo := facts.PackageManager

	packages := []string{"httpd", "tmux"}
	expectedCmd := fmt.Sprintf(pmInfo.InstallCmd, strings.Join(packages, " "))


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
	err := r.InstallPackages(context.Background(), mockConn, facts, packages...)
	if err != nil {
		t.Fatalf("InstallPackages() for yum error = %v", err)
	}
	if cmdExecuted != expectedCmd {
		t.Errorf("InstallPackages() for yum command = %q, want %q", cmdExecuted, expectedCmd)
	}
}

func TestRunner_RemovePackages_Dnf(t *testing.T) {
	r, facts, mockConn := newTestRunnerForPackage(t, "fedora")
	if facts.PackageManager == nil || facts.PackageManager.Type != PackageManagerDnf {
		t.Fatalf("Test setup error: expected Dnf package manager for fedora, got: %v", facts.PackageManager)
	}
	pmInfo := facts.PackageManager

	packages := []string{"old-package"}
	expectedCmd := fmt.Sprintf(pmInfo.RemoveCmd, strings.Join(packages, " "))


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
	err := r.RemovePackages(context.Background(), mockConn, facts, packages...)
	if err != nil {
		t.Fatalf("RemovePackages() for dnf error = %v", err)
	}
	if cmdExecuted != expectedCmd {
		t.Errorf("RemovePackages() for dnf command = %q, want %q", cmdExecuted, expectedCmd)
	}
}

func TestRunner_UpdatePackageCache_Apt(t *testing.T) {
	r, facts, mockConn := newTestRunnerForPackage(t, "ubuntu")
	if facts.PackageManager == nil || facts.PackageManager.Type != PackageManagerApt {
		t.Fatalf("Test setup error: expected Apt package manager for ubuntu, got: %v", facts.PackageManager)
	}
	expectedCmd := facts.PackageManager.UpdateCmd
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
	err := r.UpdatePackageCache(context.Background(), mockConn, facts)
	if err != nil {
		t.Fatalf("UpdatePackageCache() for apt error = %v", err)
	}
	if cmdExecuted != expectedCmd {
		t.Errorf("UpdatePackageCache apt cmd = %q, want %q", cmdExecuted, expectedCmd)
	}
}

func TestRunner_IsPackageInstalled_Apt_Installed(t *testing.T) {
	r, facts, mockConn := newTestRunnerForPackage(t, "ubuntu")
	if facts.PackageManager == nil {
		t.Fatalf("Test setup error: PackageManager facts are nil for ubuntu")
	}
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
	installed, err := r.IsPackageInstalled(context.Background(), mockConn, facts, pkg) // Corrected call
	if err != nil {t.Fatalf("IsPackageInstalled apt error = %v", err)}
	if !installed {t.Error("IsPackageInstalled apt = false, want true")}
}

func TestRunner_IsPackageInstalled_Yum_NotInstalled(t *testing.T) {
	r, facts, mockConn := newTestRunnerForPackage(t, "centos") // Corrected: assign facts
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
	installed, err := r.IsPackageInstalled(context.Background(), mockConn, facts, pkg) // Corrected call
	if err != nil {t.Fatalf("IsPackageInstalled yum error = %v", err)}
	if installed {t.Error("IsPackageInstalled yum = true, want false")}
}

func TestRunner_AddRepository_Apt_PPA(t *testing.T) {
	r, facts, mockConn := newTestRunnerForPackage(t, "ubuntu") // Corrected: assign facts
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
		// Check against facts.PackageManager.UpdateCmd as aptInfo is not available here
		if facts != nil && facts.PackageManager != nil && cmd == facts.PackageManager.UpdateCmd && options.Sudo {
			updateCmd = cmd
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("AddRepository apt PPA: unexpected cmd: %s", cmd)
	}

	err := r.AddRepository(context.Background(), mockConn, facts, repoPPA, false) // Corrected call
	if err != nil {
		t.Fatalf("AddRepository apt PPA error = %v", err)
	}
	if installPropsCmd == "" {t.Error("software-properties-common install command not called")}
	if addRepoCmd == "" {t.Error("add-apt-repository command not called")}
	if updateCmd == "" {t.Error("apt-get update not called after add-apt-repository")}
}
