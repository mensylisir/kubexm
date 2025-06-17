package preflight

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time" // Added for time.Now() in backup command name

	"github.com/kubexms/kubexms/pkg/connector"
	"github.com/kubexms/kubexms/pkg/runner"
	"github.com/kubexms/kubexms/pkg/runtime"
	"github.com/kubexms/kubexms/pkg/step"
)


func TestDisableSwapStep_Check_SwapOff(t *testing.T) {
	mockConn := step.NewMockStepConnector()
	// Ensure OS is Linux for /proc/swaps fallback or swapon behavior
	facts := &runner.Facts{OS: &connector.OS{ID: "linux"}}
	ctx := step.newTestContextForStep(t, mockConn, facts)
	s := DisableSwapStep{}

	// Simulate `swapon --summary` returning no active swap (e.g., only header or empty)
	mockConn.ExecFunc = func(ctxGo context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		if strings.Contains(cmd, "swapon --summary") {
			// Try --noheadings first
			if strings.Contains(cmd, "--noheadings") {
				return []byte(""), nil, nil // Empty output means no swap
			}
			return []byte("Filename				Type		Size	Used	Priority\n"), nil, nil
		}
		return nil, nil, fmt.Errorf("DisableSwapStep.Check unexpected cmd: %s", cmd)
	}

	isDone, err := s.Check(ctx)
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if !isDone { // isDone should be true if swap is off
		t.Error("Check() = false, want true (swap is off)")
	}
}

func TestDisableSwapStep_Check_SwapOn_ViaSwapon(t *testing.T) {
	mockConn := step.NewMockStepConnector()
	facts := &runner.Facts{OS: &connector.OS{ID: "linux"}}
	ctx := step.newTestContextForStep(t, mockConn, facts)
	s := DisableSwapStep{}

	mockConn.ExecFunc = func(ctxGo context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		if strings.Contains(cmd, "swapon --summary") {
			if strings.Contains(cmd, "--noheadings") { // Simulate noheadings not supported or returns data
				return []byte("/dev/sda2	partition	1024	0	-1\n"), nil, nil
			}
			return []byte("Filename	Type	Size	Used	Priority\n/dev/sda2	partition	1024	0	-1\n"), nil, nil
		}
		return nil, nil, fmt.Errorf("DisableSwapStep.Check unexpected cmd: %s", cmd)
	}

	isDone, err := s.Check(ctx)
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if isDone { // isDone should be false if swap is on
		t.Error("Check() = true, want false (swap is on)")
	}
}

func TestDisableSwapStep_Check_SwapOn_ViaProcSwaps(t *testing.T) {
	mockConn := step.NewMockStepConnector()
	facts := &runner.Facts{OS: &connector.OS{ID: "linux"}}
	ctx := step.newTestContextForStep(t, mockConn, facts)
	s := DisableSwapStep{}

	mockConn.ExecFunc = func(ctxGo context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		if strings.Contains(cmd, "swapon --summary") {
			// Simulate swapon command failure (e.g., not found)
			return nil, []byte("command not found"), &connector.CommandError{ExitCode: 127}
		}
		// ReadFile will be called by runner if swapon fails
		// This needs ReadFile to be a method on the mock connector or runner.
		// For now, assume ReadFile is part of runner and uses Exec.
		// The current DisableSwapStep.isSwapOn uses runner.ReadFile which uses connector.Exec("cat ...")
		if strings.HasPrefix(cmd, "cat /proc/swaps") {
			return []byte("Filename				Type		Size	Used	Priority\n/dev/dm-1                               partition	8388604	0	-2\n"), nil, nil
		}
		return nil, nil, fmt.Errorf("DisableSwapStep.Check (procfs) unexpected cmd: %s", cmd)
	}
	// Mock ReadFile if it was a direct connector method:
	// mockConn.ReadFileFunc = func...

	isDone, err := s.Check(ctx)
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if isDone { // isDone should be false if swap is on (via /proc/swaps)
		t.Error("Check() = true, want false (swap is on via /proc/swaps)")
	}
}


func TestDisableSwapStep_Run_Success(t *testing.T) {
	mockConn := step.NewMockStepConnector()
	facts := &runner.Facts{OS: &connector.OS{ID: "linux"}}
	ctx := step.newTestContextForStep(t, mockConn, facts)
	s := DisableSwapStep{}

	var swapoffCalled, backupCalled, sedCalled bool

	// Define a sequence of expected commands for ExecFunc
	cmdSequence := 0
	expectedCmds := []string{"swapoff -a", "cp /etc/fstab", "sed -E -i.prev_swap_state", "swapon --summary"}

	mockConn.ExecFunc = func(ctxGo context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.ExecHistory = append(mockConn.ExecHistory, cmd) // Record command

		currentExpectedPrefix := ""
		if cmdSequence < len(expectedCmds) {
			currentExpectedPrefix = strings.Fields(expectedCmds[cmdSequence])[0]
		}

		if strings.Contains(cmd, "swapoff -a") && options.Sudo {
			if !strings.HasPrefix(cmd, currentExpectedPrefix) {t.Errorf("Expected cmd prefix %s, got %s", currentExpectedPrefix, cmd)}
			cmdSequence++
			swapoffCalled = true
			return nil, nil, nil
		}
		if strings.HasPrefix(cmd, "cp /etc/fstab /etc/fstab.bak-kubexms-") && options.Sudo {
			if !strings.HasPrefix(cmd, currentExpectedPrefix) {t.Errorf("Expected cmd prefix %s, got %s", currentExpectedPrefix, cmd)}
			cmdSequence++
			backupCalled = true
			return nil, nil, nil
		}
		if strings.Contains(cmd, "sed -E -i.prev_swap_state") && strings.Contains(cmd, "/etc/fstab") && options.Sudo {
			if !strings.HasPrefix(cmd, currentExpectedPrefix) {t.Errorf("Expected cmd prefix %s, got %s", currentExpectedPrefix, cmd)}
			cmdSequence++
			sedCalled = true
			return nil, nil, nil
		}
		if strings.Contains(cmd, "swapon --summary") { // For final verification
			if !strings.HasPrefix(cmd, currentExpectedPrefix) && !strings.Contains(cmd, expectedCmds[len(expectedCmds)-1]) { // allow --noheadings
			    // This check is a bit loose because of --noheadings variant
			}
			cmdSequence++
			return []byte("Filename	Type	Size	Used	Priority\n"), nil, nil
		}
		return nil, nil, fmt.Errorf("DisableSwapStep.Run unexpected cmd: %s, sudo: %v", cmd, options.Sudo)
	}

	res := s.Run(ctx)
	if res.Status != "Succeeded" {
		t.Errorf("Run status = %s, want Succeeded. Msg: %s, Err: %v", res.Status, res.Message, res.Error)
	}
	if !swapoffCalled {t.Error("swapoff -a not called")}
	if !backupCalled {t.Error("fstab backup not called")}
	if !sedCalled {t.Error("sed to comment fstab not called")}
	if !strings.Contains(res.Message, "Swap is successfully disabled") {
		t.Errorf("Unexpected success message: %s", res.Message)
	}
	if cmdSequence != len(expectedCmds) {
		t.Errorf("Not all expected commands were called in sequence. Called %d, expected %d. History: %v", cmdSequence, len(expectedCmds), mockConn.ExecHistory)
	}
}

func TestDisableSwapStep_Run_SedFails(t *testing.T) {
	mockConn := step.NewMockStepConnector()
	facts := &runner.Facts{OS: &connector.OS{ID: "linux"}}
	ctx := step.newTestContextForStep(t, mockConn, facts)
	s := DisableSwapStep{}
	expectedErr := errors.New("sed command failed")

	mockConn.ExecFunc = func(ctxGo context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		if strings.Contains(cmd, "swapoff -a") { return nil, nil, nil }
		if strings.HasPrefix(cmd, "cp /etc/fstab") { return nil, nil, nil }
		if strings.Contains(cmd, "sed -E -i.prev_swap_state") {
			return nil, []byte("sed error output"), expectedErr
		}
		return nil, nil, fmt.Errorf("DisableSwapStep.Run (SedFails) unexpected cmd: %s", cmd)
	}
	res := s.Run(ctx)
	if res.Status != "Failed" {
		t.Errorf("Run status = %s, want Failed", res.Status)
	}
	// The error from runner.Run is wrapped by DisableSwapStep.Run
	if res.Error == nil || !strings.Contains(res.Error.Error(), expectedErr.Error()) {
		t.Errorf("Run error = %v, want to contain %v", res.Error, expectedErr)
	}
}
