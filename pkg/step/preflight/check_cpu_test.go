package preflight

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/kubexms/kubexms/pkg/connector"
	"github.com/kubexms/kubexms/pkg/runner"
	"github.com/kubexms/kubexms/pkg/runtime"
	"github.com/kubexms/kubexms/pkg/step" // For the test helper and step.Step
)

// Using the mock_objects_for_test.go from the step package directly via step.newTestContextForStep

func TestCheckCPUStep_Run_MetFromFacts(t *testing.T) {
	mockConn := step.NewMockStepConnector()
	facts := &runner.Facts{TotalCPU: 4, OS: &connector.OS{ID: "linux", Arch: "amd64"}}
	ctx := step.newTestContextForStep(t, mockConn, facts)

	s := CheckCPUStep{MinCores: 2}
	res := s.Run(ctx)

	if res.Status != "Succeeded" {
		t.Errorf("Status = %s, want Succeeded. Msg: %s, Err: %v", res.Status, res.Message, res.Error)
	}
	if !strings.Contains(res.Message, "Host test-host-step has 4 CPU cores") {
		t.Errorf("Message = %q, expected to contain 'Host test-host-step has 4 CPU cores'", res.Message)
	}
	if len(mockConn.ExecHistory) > 0 {
		t.Errorf("Exec was called but CPU count should come from facts: %v", mockConn.ExecHistory)
	}
}

func TestCheckCPUStep_Run_NotMetFromFacts(t *testing.T) {
	mockConn := step.NewMockStepConnector()
	facts := &runner.Facts{TotalCPU: 1, OS: &connector.OS{ID: "linux", Arch: "amd64"}}
	ctx := step.newTestContextForStep(t, mockConn, facts)

	s := CheckCPUStep{MinCores: 2}
	res := s.Run(ctx)

	if res.Status != "Failed" {
		t.Errorf("Status = %s, want Failed", res.Status)
	}
	if !strings.Contains(res.Message, "host test-host-step has 1 CPU cores, but minimum requirement is 2") {
		t.Errorf("Message = %q incorrect, expected 'host test-host-step has 1 CPU cores, but minimum requirement is 2'", res.Message)
	}
}

func TestCheckCPUStep_Run_MetFromCommand_Linux(t *testing.T) {
	mockConn := step.NewMockStepConnector()
	// Simulate facts not having CPU info initially
	facts := &runner.Facts{TotalCPU: 0, OS: &connector.OS{ID: "linux", Arch: "amd64"}}
	ctx := step.newTestContextForStep(t, mockConn, facts)

	mockConn.ExecFunc = func(ctxGo context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		if cmd == "nproc" {
			return []byte("8"), nil, nil
		}
		return nil, nil, fmt.Errorf("unexpected command: %s", cmd)
	}

	s := CheckCPUStep{MinCores: 4}
	res := s.Run(ctx)

	if res.Status != "Succeeded" {
		t.Errorf("Status = %s, want Succeeded. Msg: %s, Err: %v", res.Status, res.Message, res.Error)
	}
	if !strings.Contains(res.Message, "Host test-host-step has 8 CPU cores") {
		t.Errorf("Message = %q, expected to contain 'Host test-host-step has 8 CPU cores'", res.Message)
	}
	if len(mockConn.ExecHistory) != 1 || mockConn.ExecHistory[0] != "nproc" {
		t.Errorf("Expected 'nproc' to be called, got: %v", mockConn.ExecHistory)
	}
}

func TestCheckCPUStep_Run_MetFromCommand_Darwin(t *testing.T) {
	mockConn := step.NewMockStepConnector()
	facts := &runner.Facts{TotalCPU: 0, OS: &connector.OS{ID: "darwin", Arch: "arm64"}}
	ctx := step.newTestContextForStep(t, mockConn, facts)

	mockConn.ExecFunc = func(ctxGo context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		if cmd == "sysctl -n hw.ncpu" {
			return []byte("10"), nil, nil
		}
		return nil, nil, fmt.Errorf("unexpected command: %s", cmd)
	}
	s := CheckCPUStep{MinCores: 8}
	res := s.Run(ctx)

	if res.Status != "Succeeded" {
		t.Errorf("Status = %s, want Succeeded. Msg: %s, Err: %v", res.Status, res.Message, res.Error)
	}
	if !strings.Contains(res.Message, "Host test-host-step has 10 CPU cores") {
		t.Errorf("Message = %q, expected 'Host test-host-step has 10 CPU cores'", res.Message)
	}
}


func TestCheckCPUStep_Run_CommandFails(t *testing.T) {
	mockConn := step.NewMockStepConnector()
	facts := &runner.Facts{TotalCPU: 0, OS: &connector.OS{ID: "linux", Arch: "amd64"}}
	ctx := step.newTestContextForStep(t, mockConn, facts)

	expectedErr := errors.New("nproc failed")
	mockConn.ExecFunc = func(ctxGo context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		if cmd == "nproc" {
			return nil, []byte("some error"), expectedErr
		}
		return nil, nil, fmt.Errorf("unexpected command: %s", cmd)
	}
	s := CheckCPUStep{MinCores: 1}
	res := s.Run(ctx)

	if res.Status != "Failed" {t.Errorf("Status = %s, want Failed", res.Status)}
	if !errors.Is(res.Error, expectedErr) {
		t.Errorf("Error = %v, want to wrap %v", res.Error, expectedErr)
	}
}


func TestCheckCPUStep_Check_Met(t *testing.T) {
	mockConn := step.NewMockStepConnector()
	facts := &runner.Facts{TotalCPU: 2, OS: &connector.OS{ID: "linux", Arch: "amd64"}}
	ctx := step.newTestContextForStep(t, mockConn, facts)
	s := CheckCPUStep{MinCores: 2}

	isDone, err := s.Check(ctx)
	if err != nil {t.Fatalf("Check() error = %v", err)}
	if !isDone {t.Error("Check() = false, want true")}
}

func TestCheckCPUStep_Check_NotMet_CommandFallback(t *testing.T) {
	mockConn := step.NewMockStepConnector()
	facts := &runner.Facts{TotalCPU: 0, OS: &connector.OS{ID: "linux", Arch: "amd64"}} // Force command
	ctx := step.newTestContextForStep(t, mockConn, facts)

	mockConn.ExecFunc = func(ctxGo context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		if cmd == "nproc" { return []byte("1"), nil, nil } // Has 1 core
		return nil, nil, fmt.Errorf("unexpected command: %s", cmd)
	}

	s := CheckCPUStep{MinCores: 2} // Needs 2 cores
	isDone, err := s.Check(ctx)
	if err != nil {t.Fatalf("Check() error = %v", err)}
	if isDone {t.Error("Check() = true (1 core < 2 cores), want false")}
}
