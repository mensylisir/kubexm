package preflight

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/mensylisir/kubexm/pkg/config"  // For config.Cluster in test helper
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/logger"
	"github.com/mensylisir/kubexm/pkg/runner"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec" // For spec.StepSpec
	"github.com/mensylisir/kubexm/pkg/step" // For step.Result and mock helpers
)

// newTestContextForPreflight uses the shared helper from pkg/step.
func newTestContextForPreflight(t *testing.T, mockConn *step.MockStepConnector, facts *runner.Facts) *runtime.Context {
	t.Helper()
	// If facts are nil, newTestContextForStep will provide its own defaults.
	return step.newTestContextForStep(t, mockConn, facts)
}


func TestCheckCPUStepExecutor_Execute_MetFromFacts(t *testing.T) {
	mockConn := step.NewMockStepConnector()
	facts := &runner.Facts{TotalCPU: 4, OS: &connector.OS{ID: "linux-test", Arch: "amd64"}} // Ensure OS is not nil
	ctx := newTestContextForPreflight(t, mockConn, facts)

	cpuSpec := &CheckCPUStepSpec{MinCores: 2}
	executor := step.GetExecutor(step.GetSpecTypeName(cpuSpec))
	if executor == nil {t.Fatal("Executor not registered for CheckCPUStepSpec")}


	res := executor.Execute(cpuSpec, ctx)

	if res.Status != "Succeeded" {
		t.Errorf("Status = %s, want Succeeded. Msg: %s, Err: %v", res.Status, res.Message, res.Error)
	}
	if !strings.Contains(res.Message, "Host "+ctx.Host.Name+" has 4 CPU cores") {
		t.Errorf("Message = %q, expected to contain 'Host %s has 4 CPU cores'", res.Message, ctx.Host.Name)
	}
	if len(mockConn.ExecHistory) > 0 { // Should not call exec if facts are sufficient
		t.Errorf("Exec was called but CPU count should come from facts: %v", mockConn.ExecHistory)
	}
}

func TestCheckCPUStepExecutor_Execute_NotMetFromCommand(t *testing.T) {
	mockConn := step.NewMockStepConnector()
	// Simulate facts not having CPU info initially, or OS being nil to force command path
	facts := &runner.Facts{TotalCPU: 0, OS: &connector.OS{ID: "linux-test", Arch: "amd64"}}
	ctx := newTestContextForPreflight(t, mockConn, facts)

	cpuSpec := &CheckCPUStepSpec{MinCores: 4}
	executor := step.GetExecutor(step.GetSpecTypeName(cpuSpec))
	if executor == nil {t.Fatal("Executor not registered")}


	mockConn.ExecFunc = func(ctxGo context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		if cmd == "nproc" {
			return []byte("2"), nil, nil // Simulate 2 cores found
		}
		return nil, nil, fmt.Errorf("unexpected command: %s", cmd)
	}

	res := executor.Execute(cpuSpec, ctx)

	if res.Status != "Failed" {
		t.Errorf("Status = %s, want Failed. Msg: %s", res.Status, res.Message)
	}
	expectedMsg := fmt.Sprintf("host %s has 2 CPU cores, but minimum requirement is 4 cores", ctx.Host.Name)
	if !strings.Contains(res.Message, expectedMsg) {
		t.Errorf("Message = %q incorrect, expected to contain '%s'", res.Message, expectedMsg)
	}
}

func TestCheckCPUStepExecutor_Check_Met(t *testing.T) {
	mockConn := step.NewMockStepConnector()
	facts := &runner.Facts{TotalCPU: 2, OS: &connector.OS{ID: "linux-test", Arch: "amd64"}}
	ctx := newTestContextForPreflight(t, mockConn, facts)

	cpuSpec := &CheckCPUStepSpec{MinCores: 2}
	executor := step.GetExecutor(step.GetSpecTypeName(cpuSpec))
	if executor == nil {t.Fatal("Executor not registered")}

	isDone, err := executor.Check(cpuSpec, ctx)
	if err != nil {t.Fatalf("Check() error = %v", err)}
	if !isDone {t.Error("Check() = false, want true")}
}

func TestCheckCPUStepExecutor_Check_NotMet(t *testing.T) {
	mockConn := step.NewMockStepConnector()
	facts := &runner.Facts{TotalCPU: 1, OS: &connector.OS{ID: "linux-test", Arch: "amd64"}}
	ctx := newTestContextForPreflight(t, mockConn, facts)

	cpuSpec := &CheckCPUStepSpec{MinCores: 4}
	executor := step.GetExecutor(step.GetSpecTypeName(cpuSpec))
	if executor == nil {t.Fatal("Executor not registered")}

	isDone, err := executor.Check(cpuSpec, ctx)
	if err != nil {t.Fatalf("Check() error = %v", err)}
	if isDone {t.Error("Check() = true, want false")}
}

func TestCheckCPUStepExecutor_Execute_MetFromCommand_Darwin(t *testing.T) {
	mockConn := step.NewMockStepConnector()
	facts := &runner.Facts{TotalCPU: 0, OS: &connector.OS{ID: "darwin", Arch: "arm64"}}
	ctx := newTestContextForPreflight(t, mockConn, facts)

	cpuSpec := &CheckCPUStepSpec{MinCores: 8}
	executor := step.GetExecutor(step.GetSpecTypeName(cpuSpec))
	if executor == nil {t.Fatal("Executor not registered")}

	mockConn.ExecFunc = func(ctxGo context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		if cmd == "sysctl -n hw.ncpu" {
			return []byte("10"), nil, nil
		}
		return nil, nil, fmt.Errorf("unexpected command: %s", cmd)
	}

	res := executor.Execute(cpuSpec, ctx)

	if res.Status != "Succeeded" {
		t.Errorf("Status = %s, want Succeeded. Msg: %s, Err: %v", res.Status, res.Message, res.Error)
	}
	if !strings.Contains(res.Message, fmt.Sprintf("Host %s has 10 CPU cores", ctx.Host.Name)) {
		t.Errorf("Message = %q, expected 'Host %s has 10 CPU cores'", res.Message, ctx.Host.Name)
	}
}

func TestCheckCPUStepExecutor_Execute_CommandFails(t *testing.T) {
	mockConn := step.NewMockStepConnector()
	facts := &runner.Facts{TotalCPU: 0, OS: &connector.OS{ID: "linux-test", Arch: "amd64"}}
	ctx := newTestContextForPreflight(t, mockConn, facts)

	cpuSpec := &CheckCPUStepSpec{MinCores: 1}
	executor := step.GetExecutor(step.GetSpecTypeName(cpuSpec))
	if executor == nil {t.Fatal("Executor not registered")}

	expectedErr := errors.New("nproc deliberate failure")
	mockConn.ExecFunc = func(ctxGo context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		if cmd == "nproc" {
			return nil, []byte("some error from nproc"), expectedErr
		}
		return nil, nil, fmt.Errorf("unexpected command: %s", cmd)
	}

	res := executor.Execute(cpuSpec, ctx)

	if res.Status != "Failed" {t.Errorf("Status = %s, want Failed", res.Status)}
	// Check if the error from Execute wraps the expectedErr from the command
	if res.Error == nil || !strings.Contains(res.Error.Error(), expectedErr.Error()) {
		t.Errorf("Error = %v, want to contain original error %v", res.Error, expectedErr)
	}
}
