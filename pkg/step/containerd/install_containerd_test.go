package containerd

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/mensylisir/kubexm/pkg/config" // For config.Cluster in test helper
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/logger"
	"github.com/mensylisir/kubexm/pkg/runner"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec" // For spec.StepSpec
	"github.com/mensylisir/kubexm/pkg/step" // For step.Result and mock helpers
)

// newTestContextForContainerd is a helper for containerd step tests.
func newTestContextForContainerd(t *testing.T, mockConn *step.MockStepConnector, osID string) *runtime.Context {
	t.Helper()
	if mockConn == nil {
		mockConn = step.NewMockStepConnector()
	}
	currentOS := &connector.OS{ID: osID, Arch: "amd64", VersionID: "test-os-version"} // Provide a default Arch

	// Override GetOSFunc for package manager detection within the step's runner.
	mockConn.GetOSFunc = func(ctx context.Context) (*connector.OS, error) {
		return currentOS, nil
	}

	// Mock LookPath for package manager tool detection (apt-get, yum, dnf)
	// and version checking tools (apt-cache, rpm, dpkg-query).
	mockConn.LookPathFunc = func(ctx context.Context, file string) (string, error) {
		isUbuntu := (osID == "ubuntu" || osID == "debian")
		isCentOS := (osID == "centos" || osID == "rhel")
		isFedora := (osID == "fedora")

		if isUbuntu && (file == "apt-get" || file == "dpkg-query" || file == "apt-cache") {
			return "/usr/bin/" + file, nil
		}
		if isCentOS && (file == "yum" || file == "rpm") {
			return "/usr/bin/" + file, nil
		}
		if isFedora && (file == "dnf" || file == "rpm") {
			return "/usr/bin/" + file, nil
		}
		// For init system detection by EnableAndStartContainerdStep if tested through this context
		if file == "systemctl" { return "/usr/bin/systemctl", nil }

		return "", errors.New("LookPath mock: command '" + file + "' not configured for OS " + osID)
	}

	facts := &runner.Facts{OS: currentOS, Hostname: "containerd-test-host"}
	// Use the shared helper from pkg/step to create the context
	return step.newTestContextForStep(t, mockConn, facts)
}


func TestInstallContainerdStepExecutor_Execute_SuccessApt(t *testing.T) {
	mockConn := step.NewMockStepConnector()
	ctx := newTestContextForContainerd(t, mockConn, "ubuntu")

	installSpec := &InstallContainerdStepSpec{Version: "1.6.9-1"} // Specific APT version
	executor := step.GetExecutor(step.GetSpecTypeName(installSpec))
	if executor == nil {t.Fatal("Executor not registered for InstallContainerdStepSpec")}

	var updateCalled, installCalled bool
	mockConn.ExecFunc = func(ctxGo context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.ExecHistory = append(mockConn.ExecHistory, cmd)
		if cmd == "apt-get update -y" && options.Sudo {
			updateCalled = true; return nil, nil, nil
		}
		// Runner's InstallPackages will format this based on detected PM
		// For apt, it becomes containerd.io=1.6.9-1
		expectedInstallPkg := fmt.Sprintf("containerd.io=%s", installSpec.Version)
		if cmd == fmt.Sprintf("apt-get install -y %s", expectedInstallPkg) && options.Sudo {
			installCalled = true; return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("InstallContainerd apt unexpected cmd: %s", cmd)
	}

	res := executor.Execute(installSpec, ctx)
	if res.Status != "Succeeded" {
		t.Errorf("Execute status = %s, want Succeeded. Msg: %s, Err: %v", res.Status, res.Message, res.Error)
	}
	if !updateCalled {t.Error("apt-get update not called")}
	if !installCalled {t.Error("apt-get install for containerd.io not called correctly")}
}

func TestInstallContainerdStepExecutor_Check_InstalledCorrectVersion_Apt(t *testing.T) {
	mockConn := step.NewMockStepConnector()
	ctx := newTestContextForContainerd(t, mockConn, "ubuntu")

	// Version for check should match what apt-cache policy might return for the prefix.
	installSpec := &InstallContainerdStepSpec{Version: "1.6.9"}
	executor := step.GetExecutor(step.GetSpecTypeName(installSpec))
	if executor == nil {t.Fatal("Executor not registered")}

	callCount := 0
	mockConn.ExecFunc = func(ctxGo context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.ExecHistory = append(mockConn.ExecHistory, cmd)
		callCount++
		// Order of calls by IsPackageInstalled then version check by InstallContainerdStepExecutor.Check
		if callCount == 1 && strings.Contains(cmd, "dpkg-query -W -f='${Status}' containerd.io") {
			return []byte("install ok installed"), nil, nil // Simulate installed
		}
		if callCount == 2 && strings.Contains(cmd, "apt-cache policy containerd.io") {
			return []byte("  Installed: 1.6.9-1ubuntu1\n  Candidate: 1.7.0-0ubuntu1\n"), nil, nil // Matches "1.6.9" prefix
		}
		return nil, nil, fmt.Errorf("InstallContainerd.Check unexpected cmd call #%d: %s", callCount, cmd)
	}

	isDone, err := executor.Check(installSpec, ctx)
	if err != nil {t.Fatalf("Check() error = %v", err)}
	if !isDone {t.Error("Check() = false, want true (correct version prefix installed)")}
	if callCount != 2 { t.Errorf("Expected 2 exec calls for Check, got %d", callCount)}
}

func TestInstallContainerdStepExecutor_Check_NotInstalled_Apt(t *testing.T) {
	mockConn := step.NewMockStepConnector()
	ctx := newTestContextForContainerd(t, mockConn, "ubuntu")
	installSpec := &InstallContainerdStepSpec{Version: "1.6.9"}
	executor := step.GetExecutor(step.GetSpecTypeName(installSpec))
	if executor == nil {t.Fatal("Executor not registered")}

	mockConn.ExecFunc = func(ctxGo context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.ExecHistory = append(mockConn.ExecHistory, cmd)
		if strings.Contains(cmd, "dpkg-query -W -f='${Status}' containerd.io") {
			return nil, []byte("package 'containerd.io' is not installed"), &connector.CommandError{ExitCode: 1}
		}
		return nil, nil, fmt.Errorf("unexpected cmd: %s", cmd)
	}
	isDone, err := executor.Check(installSpec, ctx)
	if err != nil {t.Fatalf("Check() error = %v", err)}
	if isDone {t.Error("Check() = true, want false (not installed)")}
}
