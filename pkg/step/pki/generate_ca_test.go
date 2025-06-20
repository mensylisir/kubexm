package pki

import (
	"context"
	"errors" // For errors.New in LookPath mock
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time" // For step.NewResult

	"github.com/mensylisir/kubexm/pkg/config" // For config.Cluster in test helper
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/logger"
	"github.com/mensylisir/kubexm/pkg/runner"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// newTestContextForPKI is a helper, can be local or from a shared test util in pkg/step.
func newTestContextForPKI(t *testing.T, mockConn *step.MockStepConnector) *runtime.Context {
	t.Helper()
	if mockConn == nil {
		mockConn = step.NewMockStepConnector()
	}
	// PKI steps often use openssl, so mock LookPath for it.
	mockConn.LookPathFunc = func(ctx context.Context, file string) (string, error) {
		if file == "openssl" {
			return "/usr/bin/openssl", nil
		}
		// Allow other lookups that might happen during default fact gathering in newTestContextForStep's runner
		if file == "hostname" || file == "uname" || file == "nproc" || file == "grep" || file == "awk" || file == "ip" || file == "cat" {
			return "/usr/bin/" + file, nil
		}
		return "", errors.New("LookPath: " + file + " not found for mock in PKI test context")
	}

	facts := &runner.Facts{OS: &connector.OS{ID: "linux-test", Arch: "amd64"}, Hostname: "pki-host"}
	// Use the shared helper, it will set up runner and host with these facts and mockConn
	return step.newTestContextForStep(t, mockConn, facts)
}


func TestGenerateRootCAStepExecutor_Execute_Success(t *testing.T) {
	mockConn := step.NewMockStepConnector()
	ctx := newTestContextForPKI(t, mockConn)

	pkiSpec := &GenerateRootCAStepSpec{
		CertPath:     "/testpki/ca.crt", // Explicit paths for test predictability
		KeyPath:      "/testpki/ca.key",
		CommonName:   "my-test-ca",
		ValidityDays: 30,
		KeyBitSize:   2048,
	}
	// Note: pkiSpec.applyDefaults(ctx.Host.Name) will be called by the executor.

	executor := step.GetExecutor(step.GetSpecTypeName(pkiSpec))
	if executor == nil {t.Fatal("Executor not registered for GenerateRootCAStepSpec")}

	var mkdirCalls, genKeyCalled, chmodKeyCalled, genCertCalled, chmodCertCalled int

	mockConn.ExecFunc = func(ctxGo context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.ExecHistory = append(mockConn.ExecHistory, cmd)
		if strings.HasPrefix(cmd, "mkdir -p") && options.Sudo {
			// Check if the path being created is one of the expected parent directories
			certDir := filepath.Dir(pkiSpec.CertPath)
			keyDir := filepath.Dir(pkiSpec.KeyPath)
			if strings.Contains(cmd, certDir) || strings.Contains(cmd, keyDir) {
				mkdirCalls++
				return nil, nil, nil
			}
		}
		if strings.HasPrefix(cmd, "openssl genpkey") && strings.Contains(cmd, pkiSpec.KeyPath) && options.Sudo {
			genKeyCalled++; return nil, nil, nil
		}
		if strings.HasPrefix(cmd, "chmod 0600") && strings.Contains(cmd, pkiSpec.KeyPath) && options.Sudo {
			chmodKeyCalled++; return nil, nil, nil
		}
		if strings.HasPrefix(cmd, "openssl req -x509") && strings.Contains(cmd, pkiSpec.CertPath) && options.Sudo {
			genCertCalled++; return nil, nil, nil
		}
		if strings.HasPrefix(cmd, "chmod 0644") && strings.Contains(cmd, pkiSpec.CertPath) && options.Sudo {
			chmodCertCalled++; return nil, nil, nil
		}
		// Allow fact-gathering commands from newTestContextForStep's runner setup to pass through
		if strings.Contains(cmd, "hostname") || strings.Contains(cmd, "uname -r") || strings.Contains(cmd, "nproc") || strings.Contains(cmd, "grep MemTotal") || strings.Contains(cmd, "ip -4 route") || strings.Contains(cmd, "ip -6 route") || strings.HasPrefix(cmd, "cat /proc/swaps") {
			return []byte("mock_fact_output"), nil, nil
		}
		return nil, nil, fmt.Errorf("GenerateCA unexpected cmd: %s, sudo: %v", cmd, options.Sudo)
	}

	res := executor.Execute(pkiSpec, ctx)
	if res.Status != "Succeeded" {
		t.Fatalf("Execute status = %s, want Succeeded. Msg: %s, Err: %v. History: %v", res.Status, res.Message, res.Error, mockConn.ExecHistory)
	}
	if mkdirCalls == 0 {t.Error("mkdir for PKI path not called or path mismatch")}
	if genKeyCalled != 1 {t.Errorf("openssl genpkey call count = %d, want 1", genKeyCalled)}
	if chmodKeyCalled != 1 {t.Errorf("chmod for CA key call count = %d, want 1", chmodKeyCalled)}
	if genCertCalled != 1 {t.Errorf("openssl req -x509 call count = %d, want 1", genCertCalled)}
	if chmodCertCalled != 1 {t.Errorf("chmod for CA cert call count = %d, want 1", chmodCertCalled)}
}

func TestGenerateRootCAStepExecutor_Check_FilesDoNotExist(t *testing.T) {
	mockConn := step.NewMockStepConnector()
	ctx := newTestContextForPKI(t, mockConn)
	// Explicitly set paths for test as applyDefaults is called by executor.
	pkiSpec := &GenerateRootCAStepSpec{CertPath: "/testpki/ca.crt", KeyPath: "/testpki/ca.key"}

	executor := step.GetExecutor(step.GetSpecTypeName(pkiSpec))
	if executor == nil {t.Fatal("Executor not registered")}

	mockConn.StatFunc = func(ctxGo context.Context, path string) (*connector.FileStat, error) {
		// Simulate files do not exist
		return &connector.FileStat{Name: filepath.Base(path), IsExist: false}, nil
	}
	isDone, err := executor.Check(pkiSpec, ctx)
	if err != nil {t.Fatalf("Check() error = %v", err)}
	if isDone {t.Error("Check() = true, want false (files do not exist)")}
}

func TestGenerateRootCAStepExecutor_Check_FilesExist(t *testing.T) {
	mockConn := step.NewMockStepConnector()
	ctx := newTestContextForPKI(t, mockConn)
	pkiSpec := &GenerateRootCAStepSpec{CertPath: "/testpki/ca.crt", KeyPath: "/testpki/ca.key"}
	executor := step.GetExecutor(step.GetSpecTypeName(pkiSpec))
	if executor == nil {t.Fatal("Executor not registered")}

	mockConn.StatFunc = func(ctxGo context.Context, path string) (*connector.FileStat, error) {
		// Need to ensure applyDefaults in Check uses the same logic for path generation
		// or that paths are absolute and predictable. Test uses absolute paths.
		if path == pkiSpec.CertPath || path == pkiSpec.KeyPath {
			return &connector.FileStat{Name: filepath.Base(path), IsExist: true}, nil
		}
		return &connector.FileStat{Name: filepath.Base(path), IsExist: false}, nil
	}
	isDone, err := executor.Check(pkiSpec, ctx)
	if err != nil {t.Fatalf("Check() error = %v", err)}
	if !isDone {t.Error("Check() = false, want true (files exist)")}
}
