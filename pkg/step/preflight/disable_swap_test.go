package preflight

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/mensylisir/kubexm/pkg/config" // For config.Cluster in test helper
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/logger"
	"github.com/mensylisir/kubexm/pkg/runner"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// Using step.newTestContextForStep from pkg/step/mock_objects_for_test.go

func TestDisableSwapStepExecutor_Check_SwapOff(t *testing.T) {
	mockConn := step.NewMockStepConnector()
	facts := &runner.Facts{OS: &connector.OS{ID: "linux", Arch: "amd64"}}
	ctx := step.newTestContextForStep(t, mockConn, facts)

	swapSpec := &DisableSwapStepSpec{}
	executor := step.GetExecutor(step.GetSpecTypeName(swapSpec))
	if executor == nil {t.Fatal("Executor not registered for DisableSwapStepSpec")}

	mockConn.ExecFunc = func(ctxGo context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		if strings.Contains(cmd, "swapon --summary") {
			if strings.Contains(cmd, "--noheadings") { // First attempt by isSwapOn
				return []byte(""), nil, nil // Empty output means no swap
			}
			// Fallback attempt if --noheadings failed (not tested here, assuming first call works)
			return []byte("Filename				Type		Size	Used	Priority\n"), nil, nil
		}
		// Mock `cat /proc/swaps` if it were to be called by ReadFile
		if strings.HasPrefix(cmd, "cat /proc/swaps") {
			return []byte("Filename				Type		Size	Used	Priority\n"), nil, nil // Only header
		}
		return nil, nil, fmt.Errorf("DisableSwap.Check unexpected cmd: %s", cmd)
	}

	isDone, err := executor.Check(swapSpec, ctx)
	if err != nil { t.Fatalf("Check() error = %v", err) }
	if !isDone { t.Error("Check() = false, want true (swap is off)") }
}

func TestDisableSwapStepExecutor_Check_SwapOn_ViaSwapon(t *testing.T) {
	mockConn := step.NewMockStepConnector()
	facts := &runner.Facts{OS: &connector.OS{ID: "linux", Arch: "amd64"}}
	ctx := step.newTestContextForStep(t, mockConn, facts)
	swapSpec := &DisableSwapStepSpec{}
	executor := step.GetExecutor(step.GetSpecTypeName(swapSpec))
	if executor == nil {t.Fatal("Executor not registered")}

	mockConn.ExecFunc = func(ctxGo context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		if strings.Contains(cmd, "swapon --summary") {
			// Simulating output that includes a swap entry
			return []byte("Filename	Type	Size	Used	Priority\n/dev/sda2	partition	1024	0	-1\n"), nil, nil
		}
		return nil, nil, fmt.Errorf("DisableSwap.Check (SwapOn) unexpected cmd: %s", cmd)
	}
	isDone, err := executor.Check(swapSpec, ctx)
	if err != nil {t.Fatalf("Check() error = %v", err)}
	if isDone {t.Error("Check() = true, want false (swap is on)")}
}


func TestDisableSwapStepExecutor_Run_Success(t *testing.T) {
	mockConn := step.NewMockStepConnector()
	facts := &runner.Facts{OS: &connector.OS{ID: "linux", Arch: "amd64"}}
	ctx := step.newTestContextForStep(t, mockConn, facts)

	swapSpec := &DisableSwapStepSpec{}
	executor := step.GetExecutor(step.GetSpecTypeName(swapSpec))
	if executor == nil {t.Fatal("Executor not registered")}

	var swapoffCalled, backupCalled, sedCalled, finalCheckCalled bool

	mockConn.ExecFunc = func(ctxGo context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.ExecHistory = append(mockConn.ExecHistory, cmd)
		if strings.Contains(cmd, "swapoff -a") && options.Sudo {
			swapoffCalled = true; return nil, nil, nil
		}
		if strings.HasPrefix(cmd, "cp /etc/fstab /etc/fstab.bak-kubexms-") && options.Sudo {
			backupCalled = true; return nil, nil, nil
		}
		if strings.Contains(cmd, "sed -E -i.prev_swap_state") && strings.Contains(cmd, "/etc/fstab") && options.Sudo {
			sedCalled = true; return nil, nil, nil
		}
		if strings.Contains(cmd, "swapon --summary") { // For final verification by isSwapOn
			finalCheckCalled = true
			return []byte(""), nil, nil // No active swap after operations
		}
		return nil, nil, fmt.Errorf("DisableSwapStep.Run unexpected cmd: %s, sudo: %v", cmd, options.Sudo)
	}

	res := executor.Execute(swapSpec, ctx)
	if res.Status != "Succeeded" {
		t.Errorf("Run status = %s, want Succeeded. Msg: %s, Err: %v", res.Status, res.Message, res.Error)
	}
	if !swapoffCalled {t.Error("swapoff -a not called")}
	if !backupCalled {t.Error("fstab backup not called")}
	if !sedCalled {t.Error("sed to comment fstab not called")}
	if !finalCheckCalled {t.Error("final swapon --summary check not called")}
	if !strings.Contains(res.Message, "Swap is successfully disabled") {
		t.Errorf("Unexpected success message: %s", res.Message)
	}
}

func TestDisableSwapStepExecutor_Run_SedFails(t *testing.T) {
	mockConn := step.NewMockStepConnector()
	facts := &runner.Facts{OS: &connector.OS{ID: "linux", Arch: "amd64"}}
	ctx := step.newTestContextForStep(t, mockConn, facts)

	swapSpec := &DisableSwapStepSpec{}
	executor := step.GetExecutor(step.GetSpecTypeName(swapSpec))
	if executor == nil {t.Fatal("Executor not registered")}

	expectedErr := errors.New("sed command failed")

	mockConn.ExecFunc = func(ctxGo context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.ExecHistory = append(mockConn.ExecHistory, cmd)
		if strings.Contains(cmd, "swapoff -a") { return nil, nil, nil }
		if strings.HasPrefix(cmd, "cp /etc/fstab") { return nil, nil, nil }
		if strings.Contains(cmd, "sed -E -i.prev_swap_state") {
			return nil, []byte("sed error output"), expectedErr
		}
		return nil, nil, fmt.Errorf("DisableSwapStep.Run (SedFails) unexpected cmd: %s", cmd)
	}
	res := executor.Execute(swapSpec, ctx)
	if res.Status != "Failed" {
		t.Errorf("Run status = %s, want Failed", res.Status)
	}
	if res.Error == nil || !strings.Contains(res.Error.Error(), expectedErr.Error()) {
		t.Errorf("Run error = %v, want to contain original error %v", res.Error, expectedErr)
	}
}
