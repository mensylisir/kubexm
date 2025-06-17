package containerd

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/kubexms/kubexms/pkg/connector"
	"github.com/kubexms/kubexms/pkg/runner"
	"github.com/kubexms/kubexms/pkg/runtime"
	"github.com/kubexms/kubexms/pkg/step"
)

// Helper for containerd tests, using the shared newTestContextForStep
func newTestContextForContainerd(t *testing.T, mockConn *step.MockStepConnector, osID string, osArch string) *runtime.Context {
	t.Helper()
	if mockConn == nil {
		mockConn = step.NewMockStepConnector()
	}

	osInfo := &connector.OS{ID: osID, Arch: osArch, VersionID: "test-ver"}
	// Override GetOSFunc for package manager detection within the step
	mockConn.GetOSFunc = func(ctx context.Context) (*connector.OS, error) {
		return osInfo, nil
	}

	// Mock LookPath for package manager tool detection (apt-get, yum, dnf)
	// and version checking tools (apt-cache, rpm)
	mockConn.LookPathFunc = func(ctx context.Context, file string) (string, error) {
		if (osID == "ubuntu" || osID == "debian") {
			if file == "apt-get" { return "/usr/bin/apt-get", nil }
			if file == "apt-cache" { return "/usr/bin/apt-cache", nil}
		} else if (osID == "centos" || osID == "rhel" || osID == "fedora") {
			if file == "yum" { return "/usr/bin/yum", nil }
			if file == "dnf" { return "/usr/bin/dnf", nil } // Fedora or RHEL8+
			if file == "rpm" { return "/bin/rpm", nil }
		}
		// Default for other lookups if any step logic needs them
		// return "/usr/bin/" + file, nil
		return "", errors.New("LookPath mock: command '" + file + "' not configured for OS " + osID)
	}

	facts := &runner.Facts{OS: osInfo, Hostname: "containerd-test-host"}
	return step.newTestContextForStep(t, mockConn, facts)
}

func TestInstallContainerdStep_Run_Success_Apt(t *testing.T) {
	mockConn := step.NewMockStepConnector()
	ctx := newTestContextForContainerd(t, mockConn, "ubuntu", "amd64")
	s := InstallContainerdStep{Version: "1.6.9-1"} // Specific APT version

	var updateCalled, installCalled bool
	mockConn.ExecFunc = func(ctxGo context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.ExecHistory = append(mockConn.ExecHistory, cmd)
		if cmd == "apt-get update -y" && options.Sudo {
			updateCalled = true
			return nil, nil, nil
		}
		// Construct expected package name with version for apt
		expectedInstallPkg := fmt.Sprintf("containerd.io=%s", s.Version)
		if cmd == fmt.Sprintf("apt-get install -y %s", expectedInstallPkg) && options.Sudo {
			installCalled = true
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("InstallContainerd apt unexpected cmd: %s", cmd)
	}

	res := s.Run(ctx)
	if res.Status != "Succeeded" {
		t.Errorf("Run status = %s, want Succeeded. Msg: %s, Err: %v", res.Status, res.Message, res.Error)
	}
	if !updateCalled {t.Error("apt-get update not called")}
	if !installCalled {t.Error("apt-get install for containerd.io with version not called correctly")}
}

func TestInstallContainerdStep_Run_Success_Yum_NoVersion(t *testing.T) {
	mockConn := step.NewMockStepConnector()
	ctx := newTestContextForContainerd(t, mockConn, "centos", "amd64")
	s := InstallContainerdStep{} // No version, install latest

	var updateCalled, installCalled bool
	mockConn.ExecFunc = func(ctxGo context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.ExecHistory = append(mockConn.ExecHistory, cmd)
		if cmd == "yum update -y" && options.Sudo { // Assuming yum for centos in mock LookPath
			updateCalled = true
			return nil, nil, nil
		}
		// Expected install command for yum with no version
		expectedInstallCmd := "yum install -y containerd.io"
		if cmd == expectedInstallCmd && options.Sudo {
			installCalled = true
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("InstallContainerd yum unexpected cmd: %s", cmd)
	}
	res := s.Run(ctx)
	if res.Status != "Succeeded" {
		t.Errorf("Run status = %s, want Succeeded. Msg: %s, Err: %v", res.Status, res.Message, res.Error)
	}
	if !updateCalled {t.Error("yum update not called")}
	if !installCalled {t.Error("yum install for containerd.io (latest) not called correctly")}
}


func TestInstallContainerdStep_Check_Installed_CorrectVersion_Apt(t *testing.T) {
	mockConn := step.NewMockStepConnector()
	ctx := newTestContextForContainerd(t, mockConn, "ubuntu", "amd64")
	// Version for check should match what apt-cache policy might return for the prefix.
	s := InstallContainerdStep{Version: "1.6.9"}

	// This function will be called multiple times by the Check logic
	callCount := 0
	mockConn.ExecFunc = func(ctxGo context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.ExecHistory = append(mockConn.ExecHistory, cmd)
		callCount++
		if callCount == 1 && strings.Contains(cmd, "dpkg-query -W -f='${Status}' containerd.io") {
			return []byte("install ok installed"), nil, nil // Simulate installed
		}
		if callCount == 2 && strings.Contains(cmd, "apt-cache policy containerd.io") {
			return []byte("  Installed: 1.6.9-1ubuntu1\n  Candidate: 1.7.0-0ubuntu1\n"), nil, nil
		}
		return nil, nil, fmt.Errorf("InstallContainerd.Check unexpected cmd call #%d: %s", callCount, cmd)
	}

	isDone, err := s.Check(ctx)
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if !isDone {
		t.Error("Check() = false, want true (correct version prefix installed)")
	}
	if callCount != 2 {
		t.Errorf("Expected 2 exec calls for Check, got %d. History: %v", callCount, mockConn.ExecHistory)
	}
}

func TestInstallContainerdStep_Check_NotInstalled(t *testing.T) {
	mockConn := step.NewMockStepConnector()
	ctx := newTestContextForContainerd(t, mockConn, "ubuntu", "amd64")
	s := InstallContainerdStep{Version: "1.6.9"}

	mockConn.ExecFunc = func(ctxGo context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.ExecHistory = append(mockConn.ExecHistory, cmd)
		if strings.Contains(cmd, "dpkg-query -W -f='${Status}' containerd.io") {
			// Simulate not installed (dpkg-query returns error)
			return nil, []byte("package 'containerd.io' is not installed"), &connector.CommandError{ExitCode: 1}
		}
		return nil, nil, fmt.Errorf("InstallContainerd.Check (not installed) unexpected cmd: %s", cmd)
	}

	isDone, err := s.Check(ctx)
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if isDone {
		t.Error("Check() = true, want false (package not installed)")
	}
}

func TestInstallContainerdStep_Check_Installed_WrongVersion(t *testing.T) {
	mockConn := step.NewMockStepConnector()
	ctx := newTestContextForContainerd(t, mockConn, "ubuntu", "amd64")
	s := InstallContainerdStep{Version: "1.7.0"} // Require 1.7.0

	callCount := 0
	mockConn.ExecFunc = func(ctxGo context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.ExecHistory = append(mockConn.ExecHistory, cmd)
		callCount++
		if callCount == 1 && strings.Contains(cmd, "dpkg-query -W -f='${Status}' containerd.io") {
			return []byte("install ok installed"), nil, nil // Simulate installed
		}
		if callCount == 2 && strings.Contains(cmd, "apt-cache policy containerd.io") {
			return []byte("  Installed: 1.6.9-1ubuntu1\n  Candidate: 1.7.0-0ubuntu1\n"), nil, nil // Installed 1.6.9
		}
		return nil, nil, fmt.Errorf("InstallContainerd.Check (wrong version) unexpected cmd: %s", cmd)
	}
	isDone, err := s.Check(ctx)
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if isDone {
		t.Error("Check() = true, want false (wrong version installed)")
	}
}
