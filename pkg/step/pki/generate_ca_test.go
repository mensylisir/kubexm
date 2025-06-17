package pki

import (
	"context"
	// "errors"
	"fmt"
	"path/filepath" // For filepath.Base
	"strings"
	"testing"
	// "time" // Not directly used in these tests, but GenerateRootCAStep uses it

	"github.com/kubexms/kubexms/pkg/connector"
	"github.com/kubexms/kubexms/pkg/runner"
	"github.com/kubexms/kubexms/pkg/runtime"
	"github.com/kubexms/kubexms/pkg/step"
)

// Helper for PKI tests using the shared newTestContextForStep
func newTestContextForPKI(t *testing.T, mockConn *step.MockStepConnector) *runtime.Context {
	t.Helper()
	if mockConn == nil {
		mockConn = step.NewMockStepConnector()
	}
	// Default facts are usually sufficient for PKI steps unless they depend on specific OS details
	// not covered by the default mock connector's GetOS.
	facts := &runner.Facts{OS: &connector.OS{ID: "linux", Arch: "amd64"}, Hostname: "pki-test-host"}
	return step.newTestContextForStep(t, mockConn, facts)
}

func TestGenerateRootCAStep_Run_Success(t *testing.T) {
	mockConn := step.NewMockStepConnector()
	ctx := newTestContextForPKI(t, mockConn)

	s := GenerateRootCAStep{
		// Let CertPath and KeyPath use defaults, which depend on ctx.Host.Name
		CommonName: "test-ca",
		ValidityDays: 30,
		KeyBitSize: 2048,
	}
	// Call defaultPathsIfNeeded explicitly to know the paths for assertion
	// The Run method will call this again, which is fine.
	s.defaultPathsIfNeeded(ctx.Host.Name)

	var mkdirCertDirCalled, mkdirKeyDirCalled, genKeyCalled, chmodKeyCalled, genCertCalled, chmodCertCalled bool

	mockConn.LookPathFunc = func(ctxGo context.Context, file string) (string, error) {
		if file == "openssl" { return "/usr/bin/openssl", nil }
		return "", fmt.Errorf("unexpected lookpath: %s", file)
	}
	mockConn.ExecFunc = func(ctxGo context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.ExecHistory = append(mockConn.ExecHistory, cmd)
		// Check for mkdir commands
		if strings.HasPrefix(cmd, "mkdir -p") {
			if strings.Contains(cmd, filepath.Dir(s.CertPath)) { mkdirCertDirCalled = true }
			if strings.Contains(cmd, filepath.Dir(s.KeyPath)) && filepath.Dir(s.CertPath) != filepath.Dir(s.KeyPath) {
				mkdirKeyDirCalled = true
			} else if filepath.Dir(s.CertPath) == filepath.Dir(s.KeyPath) {
				// If paths are same, one mkdir call is enough
				mkdirKeyDirCalled = mkdirCertDirCalled
			}
			return nil, nil, nil
		}
		// Check for openssl commands
		if strings.HasPrefix(cmd, "openssl genpkey") && strings.Contains(cmd, s.KeyPath) && options.Sudo {
			genKeyCalled = true; return nil, nil, nil
		}
		if strings.HasPrefix(cmd, "chmod 0600") && strings.Contains(cmd, s.KeyPath) && options.Sudo {
			chmodKeyCalled = true; return nil, nil, nil
		}
		if strings.HasPrefix(cmd, "openssl req -x509") && strings.Contains(cmd, s.CertPath) && options.Sudo {
			genCertCalled = true; return nil, nil, nil
		}
		if strings.HasPrefix(cmd, "chmod 0644") && strings.Contains(cmd, s.CertPath) && options.Sudo {
			chmodCertCalled = true; return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("GenerateCA unexpected cmd: %s, sudo: %v", cmd, options.Sudo)
	}

	res := s.Run(ctx)
	if res.Status != "Succeeded" {
		t.Errorf("Run status = %s, want Succeeded. Msg: %s, Err: %v", res.Status, res.Message, res.Error)
	}
	if !mkdirCertDirCalled {t.Error("mkdir for CertPath directory not called or path mismatch")}
	// mkdirKeyDirCalled check is implicitly covered if CertDir == KeyDir, or explicitly if different
	if filepath.Dir(s.CertPath) != filepath.Dir(s.KeyPath) && !mkdirKeyDirCalled {
		t.Error("mkdir for KeyPath directory not called when different from CertDir")
	}
	if !genKeyCalled {t.Error("openssl genpkey not called")}
	if !chmodKeyCalled {t.Error("chmod for CA key not called")}
	if !genCertCalled {t.Error("openssl req -x509 not called")}
	if !chmodCertCalled {t.Error("chmod for CA cert not called")}
	if !strings.Contains(res.Message, "Root CA certificate and key generated successfully") {
		t.Errorf("Unexpected success message: %s", res.Message)
	}
	// Check if paths were stored in SharedData
	if val, ok := ctx.SharedData.Load(fmt.Sprintf("pki.ca.%s.certPath", ctx.Host.Name)); !ok || val != s.CertPath {
		t.Errorf("CertPath not stored correctly in SharedData. Got %v, expected %s", val, s.CertPath)
	}
	if val, ok := ctx.SharedData.Load(fmt.Sprintf("pki.ca.%s.keyPath", ctx.Host.Name)); !ok || val != s.KeyPath {
		t.Errorf("KeyPath not stored correctly in SharedData. Got %v, expected %s", val, s.KeyPath)
	}
}

func TestGenerateRootCAStep_Check_FilesExist(t *testing.T) {
	mockConn := step.NewMockStepConnector()
	ctx := newTestContextForPKI(t, mockConn)
	s := GenerateRootCAStep{}
	s.defaultPathsIfNeeded(ctx.Host.Name)

	mockConn.StatFunc = func(ctxGo context.Context, path string) (*connector.FileStat, error) {
		// Simulate both cert and key exist
		if path == s.CertPath || path == s.KeyPath {
			return &connector.FileStat{Name: filepath.Base(path), IsExist: true}, nil
		}
		// Should not be called for other paths in this test's context
		return &connector.FileStat{Name: filepath.Base(path), IsExist: false},
		    fmt.Errorf("StatFunc called for unexpected path in Check_FilesExist: %s", path)
	}
	isDone, err := s.Check(ctx)
	if err != nil {t.Fatalf("Check() error = %v", err)}
	if !isDone {t.Error("Check() = false, want true (files exist)")}
}

func TestGenerateRootCAStep_Check_CertMissing(t *testing.T) {
	mockConn := step.NewMockStepConnector()
	ctx := newTestContextForPKI(t, mockConn)
	s := GenerateRootCAStep{}
	s.defaultPathsIfNeeded(ctx.Host.Name)

	mockConn.StatFunc = func(ctxGo context.Context, path string) (*connector.FileStat, error) {
		if path == s.KeyPath { // Key exists
			return &connector.FileStat{Name: filepath.Base(path), IsExist: true}, nil
		}
		if path == s.CertPath { // Cert does not exist
			return &connector.FileStat{Name: filepath.Base(path), IsExist: false}, nil
		}
		return nil, fmt.Errorf("StatFunc called for unexpected path: %s", path)
	}
	isDone, err := s.Check(ctx)
	if err != nil {t.Fatalf("Check() error = %v", err)}
	if isDone {t.Error("Check() = true, want false (cert missing)")}
}
