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
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// newTestContextForContainerd is defined in install_containerd_test.go
// and assumed accessible as these test files are in the same package.

func TestEnableAndStartContainerdStepExecutor_Execute_SuccessSystemd(t *testing.T) {
	mockConn := step.NewMockStepConnector()
	ctx := newTestContextForContainerd(t, mockConn, "ubuntu") // Ubuntu uses systemd

	// Ensure LookPath for systemctl succeeds (this is usually mocked in newTestContextForContainerd)
	// If not, override it here for clarity for this specific test:
	mockConn.LookPathFunc = func(ctxGo context.Context, file string) (string, error) {
		if file == "systemctl" { return "/usr/bin/systemctl", nil }
		// Handle other lookups if DetectPackageManager is indirectly called via runner setup in helper
		if file == "apt-get" || file == "dpkg-query" || file == "apt-cache" { return "/usr/bin/"+file, nil}
		return "", errors.New("LookPath: " + file + " not configured for mock in this test")
	}

	manageSpec := &EnableAndStartContainerdStepSpec{}
	executor := step.GetExecutor(step.GetSpecTypeName(manageSpec))
	if executor == nil {t.Fatal("Executor not registered for EnableAndStartContainerdStepSpec")}

	var daemonReloadCalled, enableCalled, startCalled, isActiveCalled bool
	mockConn.ExecFunc = func(ctxGo context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.ExecHistory = append(mockConn.ExecHistory, cmd)
		// Runner's service methods will call these underlying commands based on detected init system (systemd here)
		if cmd == "systemctl daemon-reload" && options.Sudo { daemonReloadCalled = true; return nil, nil, nil }
		if cmd == "systemctl enable containerd" && options.Sudo { enableCalled = true; return nil, nil, nil }
		if cmd == "systemctl start containerd" && options.Sudo { startCalled = true; return nil, nil, nil }
		// Runner's IsServiceActive for systemd uses `systemctl is-active --quiet containerd`
		if cmd == "systemctl is-active --quiet containerd" && (options == nil || !options.Sudo) {
			isActiveCalled = true; return nil, nil, nil // Exit 0 for active
		}
		return nil, nil, fmt.Errorf("EnableAndStartContainerd unexpected Exec: %s, sudo: %v", cmd, options.Sudo)
	}

	res := executor.Execute(manageSpec, ctx)
	if res.Status != "Succeeded" {
		t.Errorf("Execute status = %s, want Succeeded. Msg: %s, Err: %v", res.Status, res.Message, res.Error)
	}
	if !daemonReloadCalled {t.Error("systemctl daemon-reload not called")}
	if !enableCalled {t.Error("systemctl enable containerd not called")}
	if !startCalled {t.Error("systemctl start containerd not called")}
	if !isActiveCalled {t.Error("systemctl is-active containerd not called for verification")}
}

func TestEnableAndStartContainerdStepExecutor_Check_IsActive(t *testing.T) {
	mockConn := step.NewMockStepConnector()
	ctx := newTestContextForContainerd(t, mockConn, "ubuntu") // systemd
	// Ensure LookPath for systemctl is mocked to succeed (usually by newTestContextForContainerd)
	mockConn.LookPathFunc = func(ctxGo context.Context, file string) (string, error) {
		if file == "systemctl" { return "/usr/bin/systemctl", nil }
		if file == "apt-get" || file == "dpkg-query" || file == "apt-cache" { return "/usr/bin/"+file, nil}
		return "", errors.New("LookPath: " + file + " not found for mock")
	}


	manageSpec := &EnableAndStartContainerdStepSpec{}
	executor := step.GetExecutor(step.GetSpecTypeName(manageSpec))
	if executor == nil {t.Fatal("Executor not registered")}

	mockConn.ExecFunc = func(ctxGo context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.ExecHistory = append(mockConn.ExecHistory, cmd)
		// Runner's IsServiceActive for systemd calls this
		if cmd == "systemctl is-active --quiet containerd" { return nil, nil, nil } // Active
		return nil, nil, errors.New("unexpected check command")
	}
	isDone, err := executor.Check(manageSpec, ctx)
	if err != nil {t.Fatalf("Check() error = %v", err)}
	if !isDone {t.Error("Check() = false, want true (service active)")}
}

func TestEnableAndStartContainerdStepExecutor_Check_NotActive(t *testing.T) {
	mockConn := step.NewMockStepConnector()
	ctx := newTestContextForContainerd(t, mockConn, "ubuntu") // systemd
	mockConn.LookPathFunc = func(ctxGo context.Context, file string) (string, error) { // Ensure systemctl is found
		if file == "systemctl" { return "/usr/bin/systemctl", nil }
		if file == "apt-get" || file == "dpkg-query" || file == "apt-cache" { return "/usr/bin/"+file, nil}
		return "", errors.New("LookPath: " + file + " not found for mock")
	}

	manageSpec := &EnableAndStartContainerdStepSpec{}
	executor := step.GetExecutor(step.GetSpecTypeName(manageSpec))
	if executor == nil {t.Fatal("Executor not registered")}

	mockConn.ExecFunc = func(ctxGo context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.ExecHistory = append(mockConn.ExecHistory, cmd)
		if cmd == "systemctl is-active --quiet containerd" {
			return nil, nil, &connector.CommandError{ExitCode: 3} // Inactive
		}
		return nil, nil, errors.New("unexpected check command")
	}
	isDone, err := executor.Check(manageSpec, ctx)
	if err != nil {t.Fatalf("Check() error = %v", err)}
	if isDone {t.Error("Check() = true, want false (service not active)")}
}
