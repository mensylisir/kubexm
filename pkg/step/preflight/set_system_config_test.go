package preflight

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time" // Required for step.NewResult if using time.Now() directly in test

	"github.com/mensylisir/kubexm/pkg/config" // For config.Cluster in test helper
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/logger"
	"github.com/mensylisir/kubexm/pkg/runner"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// Using step.newTestContextForStep from pkg/step/mock_objects_for_test.go

func TestSetSystemConfigStepExecutor_Execute_Success(t *testing.T) {
	mockConn := step.NewMockStepConnector()
	// OS info in facts can influence default reload command choice.
	facts := &runner.Facts{OS: &connector.OS{ID: "linux", Arch: "amd64"}}
	ctx := step.newTestContextForStep(t, mockConn, facts)

	reload := true
	sysctlSpec := &SetSystemConfigStepSpec{
		Params:         map[string]string{"net.ipv4.ip_forward": "1", "net.bridge.bridge-nf-call-iptables": "1"},
		Reload:         &reload, // Explicitly true
		ConfigFilePath: "/etc/sysctl.d/test-90-kubexms.conf", // Specific path in a .d directory
	}
	executor := step.GetExecutor(step.GetSpecTypeName(sysctlSpec))
	if executor == nil {t.Fatal("Executor not registered for SetSystemConfigStepSpec")}

	var writtenContent string
	var reloadCmdCalled string

	mockConn.CopyContentFunc = func(ctxGo context.Context, content []byte, dstPath string, options *connector.FileTransferOptions) error {
		if dstPath == sysctlSpec.ConfigFilePath && options.Sudo && options.Permissions == "0644" {
			writtenContent = string(content)
			return nil
		}
		return errors.New("unexpected CopyContent call")
	}
	mockConn.ExecFunc = func(ctxGo context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.ExecHistory = append(mockConn.ExecHistory, cmd)
		// For Check phase after execute, or initial Check if called.
		if strings.HasPrefix(cmd, "sysctl -n net.ipv4.ip_forward") { return []byte("1"), nil, nil }
		if strings.HasPrefix(cmd, "sysctl -n net.bridge.bridge-nf-call-iptables") { return []byte("1"), nil, nil }

		// For Reload: path is in /etc/sysctl.d/, so "sysctl --system" is expected
		if cmd == "sysctl --system" && options.Sudo {
			reloadCmdCalled = cmd
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("SetSystemConfig unexpected Exec cmd: %s", cmd)
	}

	res := executor.Execute(sysctlSpec, ctx)
	if res.Status != "Succeeded" {
		t.Errorf("Status = %s, want Succeeded. Msg: %s, Err: %v", res.Status, res.Message, res.Error)
	}
	if !strings.Contains(writtenContent, "net.ipv4.ip_forward = 1") {
		t.Errorf("Config content missing ip_forward. Got:\n%s", writtenContent)
	}
	if !strings.Contains(writtenContent, "net.bridge.bridge-nf-call-iptables = 1") {
		t.Errorf("Config content missing bridge-nf. Got:\n%s", writtenContent)
	}
	if sysctlSpec.shouldReload() && reloadCmdCalled == "" { // Use shouldReload() for clarity
		t.Error("sysctl reload command not called when Reload was true/default")
	}
}

func TestSetSystemConfigStepExecutor_Check_AllSet(t *testing.T) {
	mockConn := step.NewMockStepConnector()
	ctx := step.newTestContextForStep(t, mockConn, nil) // Default facts
	sysctlSpec := &SetSystemConfigStepSpec{
		Params: map[string]string{"vm.swappiness": "10"},
	}
	executor := step.GetExecutor(step.GetSpecTypeName(sysctlSpec))
	if executor == nil {t.Fatal("Executor not registered")}

	mockConn.ExecFunc = func(ctxGo context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.ExecHistory = append(mockConn.ExecHistory, cmd)
		if cmd == "sysctl -n vm.swappiness" { return []byte("10"), nil, nil}
		return nil, nil, errors.New("unexpected sysctl check command")
	}
	isDone, err := executor.Check(sysctlSpec, ctx)
	if err != nil {t.Fatalf("Check() error = %v", err)}
	if !isDone {t.Error("Check() = false, want true (params are set)")}
}

func TestSetSystemConfigStepExecutor_Check_OneNotSet(t *testing.T) {
	mockConn := step.NewMockStepConnector()
	ctx := step.newTestContextForStep(t, mockConn, nil)
	sysctlSpec := &SetSystemConfigStepSpec{
		Params: map[string]string{"vm.swappiness": "10", "fs.file-max": "100000"},
	}
	executor := step.GetExecutor(step.GetSpecTypeName(sysctlSpec))
	if executor == nil {t.Fatal("Executor not registered")}

	mockConn.ExecFunc = func(ctxGo context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.ExecHistory = append(mockConn.ExecHistory, cmd)
		if cmd == "sysctl -n vm.swappiness" { return []byte("10"), nil, nil} // This one is set
		if cmd == "sysctl -n fs.file-max" { return []byte("65536"), nil, nil} // This one is different
		return nil, nil, errors.New("unexpected sysctl check command")
	}
	isDone, err := executor.Check(sysctlSpec, ctx)
	if err != nil {t.Fatalf("Check() error = %v", err)}
	if isDone {t.Error("Check() = true, want false (one param not set correctly)")}
}

func TestSetSystemConfigStepExecutor_Execute_NoReload(t *testing.T) {
	mockConn := step.NewMockStepConnector()
	ctx := step.newTestContextForStep(t, mockConn, nil)

	noReload := false
	sysctlSpec := &SetSystemConfigStepSpec{
		Params: map[string]string{"net.ipv4.ip_forward": "1"},
		Reload: &noReload, // Explicitly false
		ConfigFilePath: "/etc/custom_sysctl.conf",
	}
	executor := step.GetExecutor(step.GetSpecTypeName(sysctlSpec))
	if executor == nil {t.Fatal("Executor not registered")}

	var reloadCmdCalled bool
	mockConn.CopyContentFunc = func(ctxGo context.Context, content []byte, dstPath string, options *connector.FileTransferOptions) error {
		return nil // Assume write succeeds
	}
	mockConn.ExecFunc = func(ctxGo context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.ExecHistory = append(mockConn.ExecHistory, cmd)
		if strings.HasPrefix(cmd, "sysctl -n net.ipv4.ip_forward") { return []byte("1"), nil, nil } // For verification
		if strings.HasPrefix(cmd, "sysctl -p") || cmd == "sysctl --system" {
			reloadCmdCalled = true
		}
		return nil, nil, nil
	}
	res := executor.Execute(sysctlSpec, ctx)
	if res.Status != "Succeeded" {t.Errorf("Status = %s, want Succeeded. Err: %v", res.Status, res.Error)}
	if reloadCmdCalled {t.Error("sysctl reload command was called when Reload was false")}
}
