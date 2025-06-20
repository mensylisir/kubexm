package runner

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"errors"
	"time"

	"github.com/mensylisir/kubexm/pkg/connector"
)

func TestNewRunner_Success(t *testing.T) {
	mockConn := NewMockConnector()
	mockConn.GetOSFunc = func(ctx context.Context) (*connector.OS, error) {
		return &connector.OS{ID: "linux", Arch: "amd64", Kernel: "5.4"}, nil
	}
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd // Update LastExecCmd, etc., inside the mock's Exec wrapper
		mockConn.LastExecOptions = options
		if mockConn.ExecHistory == nil {
			mockConn.ExecHistory = []string{}
		}
		mockConn.ExecHistory = append(mockConn.ExecHistory, cmd)

		if strings.Contains(cmd, "hostname") {
			return []byte("test-host"), nil, nil
		}
		if strings.Contains(cmd, "uname -r") {
			return []byte("5.4.0-test"), nil, nil
		}
		if strings.Contains(cmd, "nproc") {
			return []byte("4"), nil, nil
		}
		if strings.Contains(cmd, "grep MemTotal /proc/meminfo") {
			return []byte("8192000"), nil, nil // 8GB in KB
		}
		if strings.Contains(cmd, "ip -4 route get 8.8.8.8") {
			return []byte("8.8.8.8 dev eth0 src 192.168.1.100"), nil, nil
		}
		if strings.Contains(cmd, "ip -6 route get") { // no IPv6 for simplicity
			return nil, nil, fmt.Errorf("no ipv6")
		}
		return nil, nil, fmt.Errorf("unexpected command: %s", cmd)
	}

	r, err := NewRunner(context.Background(), mockConn)
	if err != nil {
		t.Fatalf("NewRunner() error = %v, wantErr nil", err)
	}
	if r == nil {
		t.Fatal("NewRunner() returned nil runner, want non-nil")
	}
	if r.Facts == nil {
		t.Fatal("Runner.Facts is nil, want non-nil")
	}
	if r.Facts.Hostname != "test-host" {
		t.Errorf("Facts.Hostname = %s, want test-host", r.Facts.Hostname)
	}
	if r.Facts.TotalCPU != 4 {
		t.Errorf("Facts.TotalCPU = %d, want 4", r.Facts.TotalCPU)
	}
	if r.Facts.TotalMemory != 8000 { // 8192000 KB / 1024 = 8000 MB
		t.Errorf("Facts.TotalMemory = %d, want 8000", r.Facts.TotalMemory)
	}
	if r.Facts.OS.ID != "linux" {
		t.Errorf("Facts.OS.ID = %s, want linux", r.Facts.OS.ID)
	}
	if !strings.Contains(r.Facts.IPv4Default, "192.168.1.100") {
         t.Errorf("Facts.IPv4Default = %s, want contains 192.168.1.100", r.Facts.IPv4Default)
    }
}

func TestNewRunner_GetOS_Fails(t *testing.T) {
	mockConn := NewMockConnector()
	expectedErr := errors.New("failed to get OS info")
	mockConn.GetOSFunc = func(ctx context.Context) (*connector.OS, error) {
		return nil, expectedErr
	}
	// Other ExecFuncs can be default or minimal, as GetOS is the one expected to fail.
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd
		mockConn.LastExecOptions = options
		// Default behavior for other commands so they don't fail before GetOS error is processed
		return []byte("dummy"), nil, nil
	}


	_, err := NewRunner(context.Background(), mockConn)
	if err == nil {
		t.Fatalf("NewRunner() with GetOS failing expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to get OS info") {
		t.Errorf("NewRunner() error = %v, want error containing 'failed to get OS info'", err)
	}
}

func TestNewRunner_Hostname_Fails(t *testing.T) {
	mockConn := NewMockConnector() // GetOS will use default mock (success)
	expectedErr := errors.New("hostname command failed")
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd
		mockConn.LastExecOptions = options
		if strings.Contains(cmd, "hostname") {
			return nil, nil, expectedErr
		}
		// Other commands succeed
		if strings.Contains(cmd, "uname -r") { return []byte("kernel"), nil, nil }
		if strings.Contains(cmd, "nproc") { return []byte("1"), nil, nil }
		if strings.Contains(cmd, "grep MemTotal") { return []byte("1024"), nil, nil }
		if strings.Contains(cmd, "ip -4 route") { return []byte("1.1.1.1"), nil, nil }
		if strings.Contains(cmd, "ip -6 route") { return nil, nil, fmt.Errorf("no ipv6") }
		return nil, nil, nil
	}

	_, err := NewRunner(context.Background(), mockConn)
	if err == nil {
		t.Fatalf("NewRunner() with hostname failing expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to gather some host facts") || !strings.Contains(err.Error(), "hostname command failed") {
		t.Errorf("NewRunner() error = %v, want error containing 'hostname command failed' and 'failed to gather'", err)
	}
}

func TestNewRunner_ConnectorNil(t *testing.T) {
	_, err := NewRunner(context.Background(), nil)
	if err == nil {
		t.Error("NewRunner() with nil connector expected error, got nil")
	}
	if !strings.Contains(err.Error(), "connector cannot be nil") {
		t.Errorf("Error message mismatch: got %v", err)
	}
}

func TestNewRunner_ConnectorNotConnected(t *testing.T) {
	mockConn := NewMockConnector()
	mockConn.IsConnectedFunc = func() bool { return false }
	_, err := NewRunner(context.Background(), mockConn)
	if err == nil {
		t.Error("NewRunner() with disconnected connector expected error, got nil")
	}
	if !strings.Contains(err.Error(), "connector is not connected") {
		t.Errorf("Error message mismatch: got %v", err)
	}
}
