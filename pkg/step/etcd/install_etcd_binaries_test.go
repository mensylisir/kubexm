package etcd

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kubexms/kubexms/pkg/config" // For config.Cluster in test helper
	"github.com/kubexms/kubexms/pkg/connector"
	"github.com/kubexms/kubexms/pkg/logger"
	"github.com/kubexms/kubexms/pkg/runner"
	"github.com/kubexms/kubexms/pkg/runtime"
	"github.com/kubexms/kubexms/pkg/spec"
	"github.com/kubexms/kubexms/pkg/step"
)

// newTestContextForEtcd is a helper, uses the shared step.newTestContextForStep
func newTestContextForEtcd(t *testing.T, mockConn *step.MockStepConnector, osArch string) *runtime.Context {
	t.Helper()
	if mockConn == nil {
		mockConn = step.NewMockStepConnector()
	}
	arch := "amd64" // Default if not specified
	if osArch != "" { arch = osArch }

	// OS and Arch are important for this step's applyDefaults and download URL construction.
	facts := &runner.Facts{OS: &connector.OS{ID: "linux", Arch: arch}, Hostname: "etcd-test-host"}

	// Mock LookPath for DownloadAndExtract's internal checks (curl/wget)
	// and for any commands the step itself might try to locate (though it usually execs full paths).
	mockConn.LookPathFunc = func(ctx context.Context, file string) (string, error) {
		switch file {
		case "curl", "wget", "tar", "mkdir", "cp", "chmod", "rm":
			return "/usr/bin/" + file, nil
		// For fact-gathering commands that might be called by newTestContextForStep's runner init
		case "hostname", "uname", "nproc", "grep", "awk", "ip", "cat":
			return "/usr/bin/" + file, nil
		default:
			return "", errors.New("LookPath mock: command '" + file + "' not configured for etcd test context")
		}
	}

	return step.newTestContextForStep(t, mockConn, facts)
}


func TestInstallEtcdBinariesStepExecutor_Execute_Success(t *testing.T) {
	mockConn := step.NewMockStepConnector()
	ctx := newTestContextForEtcd(t, mockConn, "amd64")

	etcdSpec := &InstallEtcdBinariesStepSpec{
		Version:   "v3.5.9",
		TargetDir: "/usr/test/bin",
		Arch:      "amd64", // Explicitly set for test predictability
	}
	// Note: etcdSpec.applyDefaults(ctx) will be called by the executor.

	executor := step.GetExecutor(step.GetSpecTypeName(etcdSpec))
	if executor == nil {t.Fatal("Executor not registered for InstallEtcdBinariesStepSpec")}

	var curlCalled, tarCalled, cpEtcdCalled, cpEtcdctlCalled, chmodEtcdCalled, chmodEtcdctlCalled bool

	mockConn.ExecFunc = func(ctxGo context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.ExecHistory = append(mockConn.ExecHistory, cmd)

		// Simulate DownloadAndExtract's underlying commands
		if strings.Contains(cmd, "curl -sSL -o") && strings.Contains(cmd, "etcd-v3.5.9-linux-amd64.tar.gz") {
			curlCalled = true; return nil, nil, nil
		}
		if strings.HasPrefix(cmd, "mkdir -p /tmp/etcd-extract-v3.5.9-") { return nil, nil, nil } // mkdir for extractDir
		if strings.HasPrefix(cmd, "tar -xzf") && strings.Contains(cmd, "/tmp/etcd-v3.5.9-linux-amd64.tar.gz") {
			tarCalled = true; return nil, nil, nil
		}
		// rm -rf for downloaded archive (part of DownloadAndExtract)
		if strings.HasPrefix(cmd, "rm -rf") && strings.Contains(cmd, "/tmp/etcd-v3.5.9-linux-amd64.tar.gz") {
			return nil, nil, nil
		}

		// Commands directly run by InstallEtcdBinariesStepExecutor.Execute
		if cmd == fmt.Sprintf("mkdir -p %s", etcdSpec.TargetDir) && options.Sudo { return nil, nil, nil }

		if strings.HasPrefix(cmd, "cp /tmp/etcd-extract-v3.5.9-") && strings.HasSuffix(cmd, "/etcd "+filepath.Join(etcdSpec.TargetDir, "etcd")) && options.Sudo {
			cpEtcdCalled = true; return nil, nil, nil
		}
		if strings.HasPrefix(cmd, "cp /tmp/etcd-extract-v3.5.9-") && strings.HasSuffix(cmd, "/etcdctl "+filepath.Join(etcdSpec.TargetDir, "etcdctl")) && options.Sudo {
			cpEtcdctlCalled = true; return nil, nil, nil
		}

		if cmd == fmt.Sprintf("chmod +x %s", filepath.Join(etcdSpec.TargetDir, "etcd")) && options.Sudo {
			chmodEtcdCalled = true; return nil, nil, nil
		}
		if cmd == fmt.Sprintf("chmod +x %s", filepath.Join(etcdSpec.TargetDir, "etcdctl")) && options.Sudo {
			chmodEtcdctlCalled = true; return nil, nil, nil
		}

		if strings.HasPrefix(cmd, "rm -rf /tmp/etcd-extract-v3.5.9-") && options.Sudo { // Cleanup of extract dir
			return nil, nil, nil
		}

		// For final Check (version verification)
		if cmd == filepath.Join(etcdSpec.TargetDir, "etcd")+" --version" { return []byte("etcd Version: 3.5.9"), nil, nil}
		if cmd == filepath.Join(etcdSpec.TargetDir, "etcdctl")+" version" { return []byte("etcdctl version: 3.5.9"), nil, nil}

		// Allow fact-gathering commands from newTestContextForStep's runner setup
		if strings.Contains(cmd, "hostname") || strings.Contains(cmd, "uname -r") || strings.Contains(cmd, "nproc") || strings.Contains(cmd, "grep MemTotal") || strings.Contains(cmd, "ip -4 route") || strings.Contains(cmd, "ip -6 route") || strings.HasPrefix(cmd, "cat /proc/swaps") {
			return []byte("mock_fact_output"), nil, nil
		}
		return nil, nil, fmt.Errorf("InstallEtcd.Execute unexpected cmd: '%s', sudo: %v", cmd, options.Sudo)
	}

	res := executor.Execute(etcdSpec, ctx)
	if res.Status != "Succeeded" {
		t.Fatalf("Execute status = %s, want Succeeded. Msg: %s, Err: %v. History: %v", res.Status, res.Message, res.Error, mockConn.ExecHistory)
	}
	if !curlCalled { t.Error("curl command for download not detected in history.") }
	if !tarCalled { t.Error("tar command for extraction not detected in history.") }
	if !cpEtcdCalled || !cpEtcdctlCalled { t.Error("cp commands for etcd/etcdctl not detected.")}
	if !chmodEtcdCalled || !chmodEtcdctlCalled { t.Error("chmod commands for etcd/etcdctl not detected.")}

	if !strings.Contains(res.Message, "Etcd v3.5.9 binaries installed successfully") {
		t.Errorf("Unexpected success message: %s", res.Message)
	}
}

func TestInstallEtcdBinariesStepExecutor_Check_AlreadyInstalled(t *testing.T) {
	mockConn := step.NewMockStepConnector()
	ctx := newTestContextForEtcd(t, mockConn, "amd64")

	etcdSpec := &InstallEtcdBinariesStepSpec{Version: "v3.5.9", TargetDir: "/usr/local/bin"}
	executor := step.GetExecutor(step.GetSpecTypeName(etcdSpec))
	if executor == nil {t.Fatal("Executor not registered")}

	mockConn.StatFunc = func(ctxGo context.Context, path string) (*connector.FileStat, error) {
		if path == "/usr/local/bin/etcd" || path == "/usr/local/bin/etcdctl" {
			return &connector.FileStat{Name: filepath.Base(path), IsExist: true}, nil
		}
		return &connector.FileStat{Name: filepath.Base(path), IsExist: false}, nil
	}
	mockConn.ExecFunc = func(ctxGo context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.ExecHistory = append(mockConn.ExecHistory, cmd)
		if cmd == "/usr/local/bin/etcd --version" { return []byte("etcd Version: 3.5.9"), nil, nil }
		if cmd == "/usr/local/bin/etcdctl version" { return []byte("etcdctl version: 3.5.9"), nil, nil }
		// Allow fact-gathering commands
		if strings.Contains(cmd, "hostname") || strings.Contains(cmd, "uname -r") { return []byte("mock_fact_output"), nil, nil }
		return nil, nil, fmt.Errorf("unexpected version check command: %s", cmd)
	}

	isDone, err := executor.Check(etcdSpec, ctx)
	if err != nil {t.Fatalf("Check() error = %v", err)}
	if !isDone {t.Error("Check() = false, want true (etcd already installed with correct version)")}
}
