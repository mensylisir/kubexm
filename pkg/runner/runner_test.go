package runner

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	// "time" // Not used in these specific refactored tests yet, but might be for others

	"github.com/mensylisir/kubexm/pkg/connector"
)

// TestNewRunner tests the NewRunner constructor.
func TestNewRunner(t *testing.T) {
	r := NewRunner()
	if r == nil {
		t.Fatal("NewRunner() returned nil")
	}
	if _, ok := r.(*defaultRunner); !ok {
		t.Errorf("NewRunner() did not return a *defaultRunner, got %T", r)
	}
}

// TestDefaultRunner_GatherFacts tests the GatherFacts method of defaultRunner.
func TestDefaultRunner_GatherFacts(t *testing.T) {
	ctx := context.Background()

	// Subtest for successful fact gathering
	t.Run("success", func(t *testing.T) {
		mockConn := NewMockConnector() // Assumes NewMockConnector() is defined and working
		mockConn.GetOSFunc = func(ctx context.Context) (*connector.OS, error) {
			return &connector.OS{ID: "linux", Arch: "amd64", Kernel: "5.4.0-generic"}, nil
		}
		mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
			mockConn.LastExecCmd = cmd
			mockConn.LastExecOptions = options
			if mockConn.ExecHistory == nil {
				mockConn.ExecHistory = []string{}
			}
			mockConn.ExecHistory = append(mockConn.ExecHistory, cmd)

			if strings.Contains(cmd, "hostname -f") {
				return []byte("test-host-fqdn"), nil, nil
			}
			if strings.Contains(cmd, "hostname") && !strings.Contains(cmd, "hostname -f") { // Fallback
				return []byte("test-host"), nil, nil
			}
			if strings.Contains(cmd, "nproc") {
				return []byte("4"), nil, nil
			}
			if strings.Contains(cmd, "grep MemTotal /proc/meminfo") {
				return []byte("8192000"), nil, nil // 8GB in KB
			}
			if strings.Contains(cmd, "ip -4 route get 8.8.8.8") {
				// Simulate output that includes the IP address in the 7th field
				return []byte("8.8.8.8 via 192.168.1.1 dev eth0 src 192.168.1.100 uid 0"), nil, nil
			}
			if strings.Contains(cmd, "ip -6 route get") {
				return nil, nil, fmt.Errorf("no ipv6 route") // Simulate no IPv6
			}
			// For package manager and init system detection
			if strings.Contains(cmd, "command -v apt-get") { return []byte("/usr/bin/apt-get"), nil, nil }
			if strings.Contains(cmd, "command -v systemctl") { return []byte("/usr/bin/systemctl"), nil, nil }

			return nil, nil, fmt.Errorf("GatherFacts.success: unhandled mock command: %s", cmd)
		}

		r := NewRunner()
		facts, err := r.GatherFacts(ctx, mockConn)
		if err != nil {
			t.Fatalf("GatherFacts() error = %v, wantErr nil", err)
		}
		if facts == nil {
			t.Fatal("GatherFacts() returned nil facts, want non-nil")
		}
		if facts.Hostname != "test-host-fqdn" {
			t.Errorf("Facts.Hostname = %s, want test-host-fqdn", facts.Hostname)
		}
		if facts.TotalCPU != 4 {
			t.Errorf("Facts.TotalCPU = %d, want 4", facts.TotalCPU)
		}
		if facts.TotalMemory != 8000 { // 8192000 KB / 1024 = 8000 MB
			t.Errorf("Facts.TotalMemory = %d, want 8000", facts.TotalMemory)
		}
		if facts.OS == nil || facts.OS.ID != "linux" {
			t.Errorf("Facts.OS.ID = %v, want linux", facts.OS)
		}
		if facts.Kernel != "5.4.0-generic" { // Assuming OS.Kernel is used
			t.Errorf("Facts.Kernel = %s, want 5.4.0-generic", facts.Kernel)
		}
		if facts.IPv4Default != "192.168.1.100" {
			t.Errorf("Facts.IPv4Default = %s, want 192.168.1.100", facts.IPv4Default)
		}
		if facts.PackageManager == nil || facts.PackageManager.Type != PackageManagerApt {
			t.Errorf("Facts.PackageManager.Type = %v, want %v", facts.PackageManager, PackageManagerApt)
		}
		if facts.InitSystem == nil || facts.InitSystem.Type != InitSystemSystemd {
			t.Errorf("Facts.InitSystem.Type = %v, want %v", facts.InitSystem, InitSystemSystemd)
		}
	})

	t.Run("success_centos_yum", func(t *testing.T) {
		mockConn := NewMockConnector()
		mockConn.GetOSFunc = func(ctx context.Context) (*connector.OS, error) {
			return &connector.OS{ID: "centos", Arch: "amd64", Kernel: "3.10.0-generic"}, nil
		}
		mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
			mockConn.LastExecCmd = cmd
			if strings.Contains(cmd, "hostname -f") { return []byte("centos-host.local"), nil, nil }
			if strings.Contains(cmd, "nproc") { return []byte("2"), nil, nil }
			if strings.Contains(cmd, "grep MemTotal /proc/meminfo") { return []byte("4096000"), nil, nil } // 4GB in KB
			if strings.Contains(cmd, "ip -4 route get 8.8.8.8") { return []byte("8.8.8.8 via 10.0.2.2 dev eth0 src 10.0.2.15"), nil, nil }
			if strings.Contains(cmd, "ip -6 route get") { return nil, nil, fmt.Errorf("no ipv6") }
			if strings.Contains(cmd, "command -v dnf") { return nil, nil, errors.New("dnf not found") } // Simulate dnf not found
			if strings.Contains(cmd, "command -v yum") { return []byte("/usr/bin/yum"), nil, nil }      // Simulate yum found
			if strings.Contains(cmd, "command -v systemctl") { return []byte("/usr/bin/systemctl"), nil, nil }
			return nil, nil, fmt.Errorf("GatherFacts.success_centos_yum: unhandled mock command: %s", cmd)
		}

		r := NewRunner()
		facts, err := r.GatherFacts(ctx, mockConn)
		if err != nil {
			t.Fatalf("GatherFacts() for centos/yum error = %v, wantErr nil", err)
		}
		if facts == nil {
			t.Fatal("GatherFacts() for centos/yum returned nil facts, want non-nil")
		}
		if facts.Hostname != "centos-host.local" {
			t.Errorf("Facts.Hostname = %s, want centos-host.local", facts.Hostname)
		}
		if facts.PackageManager == nil || facts.PackageManager.Type != PackageManagerYum {
			t.Errorf("Facts.PackageManager.Type = %v, want %v", facts.PackageManager.Type, PackageManagerYum)
		}
		if facts.InitSystem == nil || facts.InitSystem.Type != InitSystemSystemd {
			t.Errorf("Facts.InitSystem.Type = %v, want %v", facts.InitSystem.Type, InitSystemSystemd)
		}
		if facts.IPv4Default != "10.0.2.15" {
			t.Errorf("Facts.IPv4Default = %s, want 10.0.2.15", facts.IPv4Default)
		}
	})

	t.Run("success_hostname_fallback", func(t *testing.T) {
		mockConn := NewMockConnector()
		mockConn.GetOSFunc = func(ctx context.Context) (*connector.OS, error) {
			return &connector.OS{ID: "linux", Arch: "amd64", Kernel: "5.4.0-generic"}, nil
		}
		mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
			if strings.Contains(cmd, "hostname -f") {
				return nil, nil, errors.New("hostname -f failed") // Simulate hostname -f failure
			}
			if strings.Contains(cmd, "hostname") && !strings.Contains(cmd, "hostname -f") { // Fallback
				return []byte("fallback-host"), nil, nil
			}
			// Provide minimal successful responses for other commands
			if strings.Contains(cmd, "nproc") { return []byte("1"), nil, nil }
			if strings.Contains(cmd, "grep MemTotal") { return []byte("1024000"), nil, nil }
			if strings.Contains(cmd, "ip -4 route") { return []byte("default via 192.168.1.1 dev eth0 src 192.168.1.101 "), nil, nil }
			if strings.Contains(cmd, "ip -6 route") { return nil, nil, fmt.Errorf("no ipv6") }
			if strings.Contains(cmd, "command -v") { return []byte("/usr/bin/somecmd"), nil, nil }
			return nil, nil, fmt.Errorf("GatherFacts.success_hostname_fallback: unhandled mock command: %s", cmd)
		}

		r := NewRunner()
		facts, err := r.GatherFacts(ctx, mockConn)
		if err != nil {
			t.Fatalf("GatherFacts() with hostname fallback error = %v", err)
		}
		if facts.Hostname != "fallback-host" {
			t.Errorf("Facts.Hostname = %s, want fallback-host", facts.Hostname)
		}
	})

	t.Run("get_os_fails", func(t *testing.T) {
		mockConn := NewMockConnector()
		expectedErr := errors.New("mock GetOS failed")
		mockConn.GetOSFunc = func(ctx context.Context) (*connector.OS, error) {
			return nil, expectedErr
		}
		// No need to set ExecFunc as GetOS is the first call.

		r := NewRunner()
		_, err := r.GatherFacts(ctx, mockConn)
		if err == nil {
			t.Fatalf("GatherFacts() with GetOS failing expected error, got nil")
		}
		if !strings.Contains(err.Error(), "failed to get OS info") || !errors.Is(err, expectedErr) {
			t.Errorf("GatherFacts() error = %v, want error containing 'failed to get OS info' and wrapping mock error", err)
		}
	})

	t.Run("hostname_fails", func(t *testing.T) {
		mockConn := NewMockConnector() // GetOSFunc will use default mock (success: linux)
		expectedErr := errors.New("hostname command failed")
		mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
			if strings.Contains(cmd, "hostname") { // Both hostname -f and hostname
				return nil, nil, expectedErr
			}
			// Other commands for other facts succeed to isolate hostname failure
			if strings.Contains(cmd, "nproc") { return []byte("1"), nil, nil }
			if strings.Contains(cmd, "grep MemTotal") { return []byte("1024000"), nil, nil } // 1GB
			if strings.Contains(cmd, "ip -4 route") { return []byte("1.1.1.1 dev eth0 src 1.1.1.1"), nil, nil }
			if strings.Contains(cmd, "ip -6 route") { return nil, nil, fmt.Errorf("no ipv6") }
			if strings.Contains(cmd, "command -v") { return []byte("found"), nil, nil} // for package/init detection
			return nil, nil, fmt.Errorf("hostname_fails: unhandled mock command: %s", cmd)
		}

		r := NewRunner()
		_, err := r.GatherFacts(ctx, mockConn)
		if err == nil {
			t.Fatalf("GatherFacts() with hostname failing expected error, got nil")
		}
		// errgroup returns the first error, so we expect the hostname error.
		if !strings.Contains(err.Error(), "failed during concurrent fact gathering") || !errors.Is(err, expectedErr) {
			t.Errorf("GatherFacts() error = %v, want error containing 'failed during concurrent fact gathering' and wrapping mock hostname error", err)
		}
	})

	t.Run("connector_nil", func(t *testing.T) {
		r := NewRunner()
		_, err := r.GatherFacts(ctx, nil)
		if err == nil {
			t.Error("GatherFacts() with nil connector expected error, got nil")
		}
		if !strings.Contains(err.Error(), "connector cannot be nil") {
			t.Errorf("Error message mismatch, got %q, want to contain 'connector cannot be nil'", err.Error())
		}
	})

	t.Run("connector_not_connected", func(t *testing.T) {
		mockConn := NewMockConnector()
		mockConn.IsConnectedFunc = func() bool { return false } // Simulate disconnected

		r := NewRunner()
		_, err := r.GatherFacts(ctx, mockConn)
		if err == nil {
			t.Error("GatherFacts() with disconnected connector expected error, got nil")
		}
		if !strings.Contains(err.Error(), "connector is not connected") {
			t.Errorf("Error message mismatch, got %q, want to contain 'connector is not connected'", err.Error())
		}
	})
}
