package preflight

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/kubexms/kubexms/pkg/config" // For config.Cluster in test helper
	"github.com/kubexms/kubexms/pkg/connector"
	"github.com/kubexms/kubexms/pkg/logger"
	"github.com/kubexms/kubexms/pkg/runner"
	"github.com/kubexms/kubexms/pkg/runtime"
	"github.com/kubexms/kubexms/pkg/spec"
	"github.com/kubexms/kubexms/pkg/step"
)

// newTestContextForPreflight is defined in check_cpu_test.go,
// assuming these test files are in the same package `preflight_test` (though they are in `preflight`).
// For Go tests, helpers in `*_test.go` files are package-scoped.
// If `newTestContextForStep` is in `pkg/step` and exported (or test is in `package step_test`), it can be used.
// Using step.newTestContextForStep which is defined in pkg/step/mock_objects_for_test.go

func TestCheckMemoryStepExecutor_Execute_MetFromFacts(t *testing.T) {
	mockConn := step.NewMockStepConnector()
	facts := &runner.Facts{TotalMemory: 4096, OS: &connector.OS{ID: "linux-test", Arch: "amd64"}} // 4GB
	ctx := step.newTestContextForStep(t, mockConn, facts)

	memSpec := &CheckMemoryStepSpec{MinMemoryMB: 2048}
	executor := step.GetExecutor(step.GetSpecTypeName(memSpec))
	if executor == nil {t.Fatal("Executor not registered for CheckMemoryStepSpec")}

	res := executor.Execute(memSpec, ctx)
	if res.Status != "Succeeded" {
		t.Errorf("Status = %s, want Succeeded. Msg: %s, Err: %v", res.Status, res.Message, res.Error)
	}
	expectedMsgPart := fmt.Sprintf("Host %s has 4096 MB memory", ctx.Host.Name)
	if !strings.Contains(res.Message, expectedMsgPart) {
		t.Errorf("Message = %q incorrect, expected to contain '%s'", res.Message, expectedMsgPart)
	}
}

func TestCheckMemoryStepExecutor_Execute_NotMetFromCommand_Linux(t *testing.T) {
	mockConn := step.NewMockStepConnector()
	facts := &runner.Facts{TotalMemory: 0, OS: &connector.OS{ID: "linux", Arch: "amd64"}}
	ctx := step.newTestContextForStep(t, mockConn, facts)

	memSpec := &CheckMemoryStepSpec{MinMemoryMB: 4096}
	executor := step.GetExecutor(step.GetSpecTypeName(memSpec))
	if executor == nil {t.Fatal("Executor not registered")}

	mockConn.ExecFunc = func(ctxGo context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		if strings.Contains(cmd, "grep MemTotal /proc/meminfo") {
			return []byte("2097152  kB"), nil, nil // 2GB in kB
		}
		return nil, nil, fmt.Errorf("unexpected command: %s", cmd)
	}

	res := executor.Execute(memSpec, ctx)
	if res.Status != "Failed" {
		t.Errorf("Status = %s, want Failed. Msg: %s", res.Status, res.Message)
	}
	expectedMsgPart := fmt.Sprintf("host %s has 2048 MB memory, but minimum requirement is 4096 MB", ctx.Host.Name)
	if !strings.Contains(res.Message, expectedMsgPart) {
		t.Errorf("Message = %q incorrect, expected to contain '%s'", res.Message, expectedMsgPart)
	}
}

func TestCheckMemoryStepExecutor_Execute_MetFromCommand_Darwin(t *testing.T) {
	mockConn := step.NewMockStepConnector()
	facts := &runner.Facts{TotalMemory: 0, OS: &connector.OS{ID: "darwin", Arch: "arm64"}}
	ctx := step.newTestContextForStep(t, mockConn, facts)

	memSpec := &CheckMemoryStepSpec{MinMemoryMB: 1000} // 1GB
	executor := step.GetExecutor(step.GetSpecTypeName(memSpec))
	if executor == nil {t.Fatal("Executor not registered")}

	mockConn.ExecFunc = func(ctxGo context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		if cmd == "sysctl -n hw.memsize" {
			return []byte(fmt.Sprintf("%d", 2*1024*1024*1024)), nil, nil // 2GB in Bytes
		}
		return nil, nil, fmt.Errorf("unexpected command: %s", cmd)
	}

	res := executor.Execute(memSpec, ctx)
	if res.Status != "Succeeded" {
		t.Errorf("Status = %s, want Succeeded. Msg: %s, Err: %v", res.Status, res.Message, res.Error)
	}
	expectedMsgPart := fmt.Sprintf("Host %s has 2048 MB memory", ctx.Host.Name)
	if !strings.Contains(res.Message, expectedMsgPart) {
		t.Errorf("Message = %q incorrect, expected to contain '%s'", res.Message, expectedMsgPart)
	}
}


func TestCheckMemoryStepExecutor_Check_Met(t *testing.T) {
	mockConn := step.NewMockStepConnector()
	facts := &runner.Facts{TotalMemory: 8192, OS: &connector.OS{ID: "linux-test", Arch: "amd64"}}
	ctx := step.newTestContextForStep(t, mockConn, facts)

	memSpec := &CheckMemoryStepSpec{MinMemoryMB: 4096}
	executor := step.GetExecutor(step.GetSpecTypeName(memSpec))
	if executor == nil {t.Fatal("Executor not registered")}

	isDone, err := executor.Check(memSpec, ctx)
	if err != nil {t.Fatalf("Check() error = %v", err)}
	if !isDone {t.Error("Check() = false, want true")}
}
