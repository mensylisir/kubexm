package preflight

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

// Using step.newTestContextForStep from pkg/step/mock_objects_for_test.go

func TestLoadKernelModulesStepExecutor_Execute_LoadAndVerify(t *testing.T) {
	mockConn := step.NewMockStepConnector()
	// Default facts from newTestContextForStep are fine for this test.
	ctx := step.newTestContextForStep(t, mockConn, nil)

	modSpec := &LoadKernelModulesStepSpec{Modules: []string{"br_netfilter", "overlay"}}
	executor := step.GetExecutor(step.GetSpecTypeName(modSpec))
	if executor == nil {t.Fatal("Executor not registered for LoadKernelModulesStepSpec")}

	loadedModules := make(map[string]bool) // Simulate which modules get "loaded"

	mockConn.ExecFunc = func(ctxGo context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.ExecHistory = append(mockConn.ExecHistory, cmd)
		if strings.HasPrefix(cmd, "lsmod | awk '{print $1}' | grep -xq") {
			moduleName := strings.Fields(cmd)[5]
			if loadedModules[moduleName] {
				return []byte(moduleName), nil, nil // Found by grep (exit 0)
			}
			return nil, nil, &connector.CommandError{ExitCode: 1} // Not found by grep -q (exit 1)
		}
		if strings.HasPrefix(cmd, "modprobe") && options.Sudo {
			moduleName := strings.Fields(cmd)[1]
			// Simulate successful modprobe: module will now be "loaded" for subsequent lsmod checks.
			loadedModules[moduleName] = true
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("LoadKernelModules unexpected cmd: %s", cmd)
	}

	res := executor.Execute(modSpec, ctx)
	if res.Status != "Succeeded" {
		t.Errorf("Status = %s, want Succeeded. Msg: %s, Err: %v", res.Status, res.Message, res.Error)
	}
	if !loadedModules["br_netfilter"] || !loadedModules["overlay"] {
		t.Error("Not all modules were marked as loaded by mock logic.")
	}
	// Check exec history for modprobe calls
	modprobeBr := false
	modprobeOverlay := false
	for _, historyCmd := range mockConn.ExecHistory {
		if historyCmd == "modprobe br_netfilter" { modprobeBr = true}
		if historyCmd == "modprobe overlay" { modprobeOverlay = true}
	}
	if !modprobeBr || !modprobeOverlay {
		t.Errorf("Expected modprobe commands not found in history. br_netfilter: %v, overlay: %v. History: %v",
			modprobeBr, modprobeOverlay, mockConn.ExecHistory)
	}
}

func TestLoadKernelModulesStepExecutor_Check_AllLoaded(t *testing.T) {
	mockConn := step.NewMockStepConnector()
	ctx := step.newTestContextForStep(t, mockConn, nil)
	modSpec := &LoadKernelModulesStepSpec{Modules: []string{"test_mod1"}}
	executor := step.GetExecutor(step.GetSpecTypeName(modSpec))
	if executor == nil {t.Fatal("Executor not registered")}

	mockConn.ExecFunc = func(ctxGo context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.ExecHistory = append(mockConn.ExecHistory, cmd)
		if strings.Contains(cmd, "lsmod | awk '{print $1}' | grep -xq test_mod1") {
			return []byte("test_mod1"), nil, nil // Module is found (grep exit 0)
		}
		return nil, nil, errors.New("unexpected check command in Check_AllLoaded")
	}
	isDone, err := executor.Check(modSpec, ctx)
	if err != nil {t.Fatalf("Check() error = %v", err)}
	if !isDone {t.Error("Check() = false, want true (module loaded)")}
}

func TestLoadKernelModulesStepExecutor_Check_OneNotLoaded(t *testing.T) {
	mockConn := step.NewMockStepConnector()
	ctx := step.newTestContextForStep(t, mockConn, nil)
	modSpec := &LoadKernelModulesStepSpec{Modules: []string{"mod_exists", "mod_missing"}}
	executor := step.GetExecutor(step.GetSpecTypeName(modSpec))
	if executor == nil {t.Fatal("Executor not registered")}

	mockConn.ExecFunc = func(ctxGo context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.ExecHistory = append(mockConn.ExecHistory, cmd)
		if strings.Contains(cmd, "lsmod | awk '{print $1}' | grep -xq mod_exists") {
			return []byte("mod_exists"), nil, nil // mod_exists is found
		}
		if strings.Contains(cmd, "lsmod | awk '{print $1}' | grep -xq mod_missing") {
			return nil, nil, &connector.CommandError{ExitCode: 1} // mod_missing not found
		}
		return nil, nil, errors.New("unexpected check command in Check_OneNotLoaded")
	}
	isDone, err := executor.Check(modSpec, ctx)
	if err != nil {t.Fatalf("Check() error = %v", err)}
	if isDone {t.Error("Check() = true, want false (one module missing)")}
}

func TestLoadKernelModulesStepExecutor_Execute_OneFailsToLoad(t *testing.T) {
	mockConn := step.NewMockStepConnector()
	ctx := step.newTestContextForStep(t, mockConn, nil)
	modSpec := &LoadKernelModulesStepSpec{Modules: []string{"good_mod", "bad_mod"}}
	executor := step.GetExecutor(step.GetSpecTypeName(modSpec))
	if executor == nil {t.Fatal("Executor not registered")}

	loadedModules := make(map[string]bool)
	modprobeError := errors.New("modprobe failed for bad_mod")

	mockConn.ExecFunc = func(ctxGo context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.ExecHistory = append(mockConn.ExecHistory, cmd)
		if strings.HasPrefix(cmd, "lsmod | awk '{print $1}' | grep -xq") {
			moduleName := strings.Fields(cmd)[5]
			if loadedModules[moduleName] { return []byte(moduleName), nil, nil }
			return nil, nil, &connector.CommandError{ExitCode: 1}
		}
		if strings.HasPrefix(cmd, "modprobe") {
			moduleName := strings.Fields(cmd)[1]
			if moduleName == "good_mod" {
				loadedModules[moduleName] = true
				return nil, nil, nil
			}
			if moduleName == "bad_mod" {
				return nil, []byte("error loading bad_mod"), modprobeError
			}
		}
		return nil, nil, fmt.Errorf("unexpected cmd: %s", cmd)
	}
	res := executor.Execute(modSpec, ctx)
	if res.Status != "Failed" {t.Errorf("Status = %s, want Failed", res.Status)}
	if res.Error == nil || !strings.Contains(res.Error.Error(), "failed to load module bad_mod") {
		t.Errorf("Expected error about bad_mod, got: %v", res.Error)
	}
	if !strings.Contains(res.Message, "Successfully loaded: [good_mod]") || !strings.Contains(res.Message, "Failed or could not verify: host preflight-host: failed to load module bad_mod") {
		t.Errorf("Unexpected message: %s", res.Message)
	}
}
