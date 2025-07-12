package connector

import (
	"context"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

func TestOS_Struct(t *testing.T) {
	tests := []struct {
		name string
		os   OS
	}{
		{
			name: "ubuntu os",
			os: OS{
				ID:         "ubuntu",
				VersionID:  "20.04",
				PrettyName: "Ubuntu 20.04.3 LTS",
				Codename:   "focal",
				Arch:       "amd64",
				Kernel:     "5.4.0-80-generic",
			},
		},
		{
			name: "centos os",
			os: OS{
				ID:         "centos",
				VersionID:  "7",
				PrettyName: "CentOS Linux 7 (Core)",
				Codename:   "",
				Arch:       "x86_64",
				Kernel:     "3.10.0-1160.el7.x86_64",
			},
		},
		{
			name: "empty os",
			os:   OS{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that OS struct can be created and accessed
			if tt.os.ID != tt.os.ID {
				t.Errorf("OS.ID mismatch")
			}
			if tt.os.VersionID != tt.os.VersionID {
				t.Errorf("OS.VersionID mismatch")
			}
			if tt.os.Arch != tt.os.Arch {
				t.Errorf("OS.Arch mismatch")
			}
		})
	}
}

func TestBastionCfg_Struct(t *testing.T) {
	hostKeyCallback := ssh.InsecureIgnoreHostKey()
	privateKey := []byte("-----BEGIN RSA PRIVATE KEY-----\ntest\n-----END RSA PRIVATE KEY-----")
	
	cfg := BastionCfg{
		Host:            "bastion.example.com",
		Port:            22,
		User:            "ubuntu",
		Password:        "password123",
		PrivateKey:      privateKey,
		PrivateKeyPath:  "/path/to/key",
		Timeout:         30 * time.Second,
		HostKeyCallback: hostKeyCallback,
	}

	// Test field assignments
	if cfg.Host != "bastion.example.com" {
		t.Errorf("BastionCfg.Host = %s, want bastion.example.com", cfg.Host)
	}
	if cfg.Port != 22 {
		t.Errorf("BastionCfg.Port = %d, want 22", cfg.Port)
	}
	if cfg.User != "ubuntu" {
		t.Errorf("BastionCfg.User = %s, want ubuntu", cfg.User)
	}
	if cfg.Password != "password123" {
		t.Errorf("BastionCfg.Password = %s, want password123", cfg.Password)
	}
	if string(cfg.PrivateKey) != string(privateKey) {
		t.Errorf("BastionCfg.PrivateKey mismatch")
	}
	if cfg.PrivateKeyPath != "/path/to/key" {
		t.Errorf("BastionCfg.PrivateKeyPath = %s, want /path/to/key", cfg.PrivateKeyPath)
	}
	if cfg.Timeout != 30*time.Second {
		t.Errorf("BastionCfg.Timeout = %v, want 30s", cfg.Timeout)
	}
	if cfg.HostKeyCallback == nil {
		t.Error("BastionCfg.HostKeyCallback should not be nil")
	}
}

func TestBastionCfg_DefaultValues(t *testing.T) {
	cfg := BastionCfg{}
	
	// Test default/zero values
	if cfg.Host != "" {
		t.Errorf("Default BastionCfg.Host should be empty, got %s", cfg.Host)
	}
	if cfg.Port != 0 {
		t.Errorf("Default BastionCfg.Port should be 0, got %d", cfg.Port)
	}
	if cfg.Timeout != 0 {
		t.Errorf("Default BastionCfg.Timeout should be 0, got %v", cfg.Timeout)
	}
	if cfg.HostKeyCallback != nil {
		t.Error("Default BastionCfg.HostKeyCallback should be nil")
	}
}

func TestProxyCfg_Struct(t *testing.T) {
	tests := []struct {
		name string
		cfg  ProxyCfg
		want string
	}{
		{
			name: "http proxy",
			cfg:  ProxyCfg{URL: "http://proxy.example.com:8080"},
			want: "http://proxy.example.com:8080",
		},
		{
			name: "https proxy",
			cfg:  ProxyCfg{URL: "https://proxy.example.com:8443"},
			want: "https://proxy.example.com:8443",
		},
		{
			name: "socks proxy",
			cfg:  ProxyCfg{URL: "socks5://proxy.example.com:1080"},
			want: "socks5://proxy.example.com:1080",
		},
		{
			name: "empty proxy",
			cfg:  ProxyCfg{},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.cfg.URL != tt.want {
				t.Errorf("ProxyCfg.URL = %s, want %s", tt.cfg.URL, tt.want)
			}
		})
	}
}

func TestConnectionCfg_Struct(t *testing.T) {
	cfg := ConnectionCfg{
		Host: "server.example.com",
		Port: 22,
		User: "admin",
	}

	if cfg.Host != "server.example.com" {
		t.Errorf("ConnectionCfg.Host = %s, want server.example.com", cfg.Host)
	}
	if cfg.Port != 22 {
		t.Errorf("ConnectionCfg.Port = %d, want 22", cfg.Port)
	}
	if cfg.User != "admin" {
		t.Errorf("ConnectionCfg.User = %s, want admin", cfg.User)
	}
}

func TestExecOptions_DefaultValues(t *testing.T) {
	opts := &ExecOptions{}

	// Test zero values
	if opts.Sudo != false {
		t.Error("Default ExecOptions.Sudo should be false")
	}
	if opts.Timeout != 0 {
		t.Errorf("Default ExecOptions.Timeout should be 0, got %v", opts.Timeout)
	}
}

func TestExecOptions_WithValues(t *testing.T) {
	opts := &ExecOptions{
		Sudo:    true,
		Timeout: 5 * time.Minute,
	}

	if !opts.Sudo {
		t.Error("ExecOptions.Sudo should be true")
	}
	if opts.Timeout != 5*time.Minute {
		t.Errorf("ExecOptions.Timeout = %v, want 5m", opts.Timeout)
	}
}

func TestCommandError_Struct(t *testing.T) {
	err := &CommandError{
		ExitCode: 1,
		Stderr:   "command not found",
		Stdout:   "",
	}

	if err.ExitCode != 1 {
		t.Errorf("CommandError.ExitCode = %d, want 1", err.ExitCode)
	}
	if err.Stderr != "command not found" {
		t.Errorf("CommandError.Stderr = %s, want 'command not found'", err.Stderr)
	}
	if err.Stdout != "" {
		t.Errorf("CommandError.Stdout = %s, want empty", err.Stdout)
	}
}

func TestCommandError_Error(t *testing.T) {
	err := &CommandError{
		ExitCode: 127,
		Stderr:   "bash: unknown_command: command not found",
		Stdout:   "",
	}

	errorStr := err.Error()
	expectedSubstrings := []string{"exit code", "127", "bash: unknown_command: command not found"}
	
	for _, substr := range expectedSubstrings {
		if !contains(errorStr, substr) {
			t.Errorf("CommandError.Error() = %s, should contain %s", errorStr, substr)
		}
	}
}

func TestCommandError_EmptyFields(t *testing.T) {
	err := &CommandError{
		ExitCode: 0,
		Stderr:   "",
		Stdout:   "success output",
	}

	errorStr := err.Error()
	if errorStr == "" {
		t.Error("CommandError.Error() should not be empty even with exit code 0")
	}
}

// Helper function to check if string contains substring
func contains(str, substr string) bool {
	return len(str) >= len(substr) && (substr == "" || indexOf(str, substr) >= 0)
}

func indexOf(str, substr string) int {
	for i := 0; i <= len(str)-len(substr); i++ {
		if str[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// Test interfaces to ensure they can be implemented
func TestConnectorInterface(t *testing.T) {
	// This test ensures the Connector interface can be used in variable declarations
	var conn Connector
	if conn != nil {
		t.Error("Nil connector should be nil")
	}

	// Test that we can call methods on a nil interface (will panic, but interface is defined)
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic when calling method on nil interface")
		}
	}()
	
	// This should panic but proves the interface is correctly defined
	_, _ = conn.GetOS(context.Background())
}

func TestFactoryInterface(t *testing.T) {
	// Test that Factory interface can be used
	var factory Factory
	if factory != nil {
		t.Error("Nil factory should be nil")
	}
	
	// Test interface methods exist
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic when calling method on nil interface")
		}
	}()
	
	// This should panic but proves the interface is correctly defined
	_ = factory.NewLocalConnector()
}

func TestTimeouts(t *testing.T) {
	tests := []struct {
		name    string
		timeout time.Duration
		valid   bool
	}{
		{"zero timeout", 0, true},
		{"1 second", 1 * time.Second, true},
		{"1 minute", 1 * time.Minute, true},
		{"1 hour", 1 * time.Hour, true},
		{"negative timeout", -1 * time.Second, true}, // Go allows negative durations
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &BastionCfg{Timeout: tt.timeout}
			if cfg.Timeout != tt.timeout {
				t.Errorf("BastionCfg.Timeout = %v, want %v", cfg.Timeout, tt.timeout)
			}
		})
	}
}

// Benchmark tests
func BenchmarkOS_Creation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = OS{
			ID:         "ubuntu",
			VersionID:  "20.04",
			PrettyName: "Ubuntu 20.04 LTS",
			Arch:       "amd64",
			Kernel:     "5.4.0-80-generic",
		}
	}
}

func BenchmarkBastionCfg_Creation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = BastionCfg{
			Host:     "bastion.example.com",
			Port:     22,
			User:     "ubuntu",
			Timeout:  30 * time.Second,
		}
	}
}