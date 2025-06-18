package runtime

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath" // Not strictly needed in this version of tests, but often useful
	"strings"
	// "sync" // Not directly needed by these specific test functions
	"reflect" // For reflect.DeepEqual
	"testing"
	"time"

	"github.com/kubexms/kubexms/pkg/config"
	"github.com/kubexms/kubexms/pkg/connector" // For connector types and OS struct
	"github.com/kubexms/kubexms/pkg/logger"
	"github.com/kubexms/kubexms/pkg/runner"
	// "github.com/kubexms/kubexms/pkg/spec" // Not directly needed here
)

// mockConnectorForRuntime (as previously defined for runtime tests, may need slight adjustments)
type mockConnectorForRuntime struct {
	ConnectFunc     func(ctx context.Context, cfg connector.ConnectionCfg) error
	GetOSFunc       func(ctx context.Context) (*connector.OS, error)
	ExecFunc        func(ctx context.Context, cmd string, options *connector.ExecOptions) (stdout []byte, stderr []byte, err error)
	IsConnectedFunc func() bool
	CloseFunc       func() error
	LastConnectionCfg *connector.ConnectionCfg // Store last Cfg for inspection
}
func (m *mockConnectorForRuntime) Connect(ctx context.Context, cfg connector.ConnectionCfg) error {
	m.LastConnectionCfg = &cfg
	if m.ConnectFunc != nil { return m.ConnectFunc(ctx, cfg) }
	return nil
}
func (m *mockConnectorForRuntime) Exec(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
	if m.ExecFunc != nil { return m.ExecFunc(ctx, cmd, options) }
	if strings.Contains(cmd, "hostname") { return []byte("mockhost"), nil, nil }
	if strings.Contains(cmd, "uname -r") { return []byte("mock-kernel"), nil, nil }
	if strings.Contains(cmd, "nproc") { return []byte("2"), nil, nil }
	if strings.Contains(cmd, "grep MemTotal") { return []byte("2048000"), nil, nil } // 2GB
	if strings.Contains(cmd, "ip -4 route") { return []byte("1.2.3.4"), nil, nil }
	if strings.Contains(cmd, "ip -6 route") { return []byte("::1"), nil, nil }
	return []byte("default mock exec output"), []byte(""), nil
}
func (m *mockConnectorForRuntime) Copy(ctx context.Context, srcPath, dstPath string, options *connector.FileTransferOptions) error { return fmt.Errorf("not impl") }
func (m *mockConnectorForRuntime) CopyContent(ctx context.Context, content []byte, dstPath string, options *connector.FileTransferOptions) error { return fmt.Errorf("not impl") }
func (m *mockConnectorForRuntime) Fetch(ctx context.Context, remotePath, localPath string) error { return fmt.Errorf("not impl") }
func (m *mockConnectorForRuntime) Stat(ctx context.Context, path string) (*connector.FileStat, error) { return &connector.FileStat{Name:path, IsExist:true}, nil } // Default to exists for runner facts
func (m *mockConnectorForRuntime) LookPath(ctx context.Context, file string) (string, error) { return "/" + file, nil }
func (m *mockConnectorForRuntime) GetOS(ctx context.Context) (*connector.OS, error) {
	if m.GetOSFunc != nil { return m.GetOSFunc(ctx) }
	return &connector.OS{ID: "linux-mock", Arch: "amd64", Kernel: "mock-kernel", VersionID: "1.0"}, nil
}
func (m *mockConnectorForRuntime) IsConnected() bool { if m.IsConnectedFunc != nil { return m.IsConnectedFunc() }; return true }
func (m *mockConnectorForRuntime) Close() error { if m.CloseFunc != nil { return m.CloseFunc() }; return nil }
var _ connector.Connector = &mockConnectorForRuntime{}


// Store original os.ReadFile and allow per-test mocking
var originalOsReadFileRef = osReadFile // Keep a reference to the original package var value (which is os.ReadFile)
var currentMockOsReadFile func(name string) ([]byte, error)

func osReadFileMockWrapperForTest(name string) ([]byte, error) {
	if currentMockOsReadFile != nil {
		return currentMockOsReadFile(name)
	}
	// If called when currentMockOsReadFile is nil, it means a test didn't set it up as expected.
	// Fallback to original for safety, or panic/error for stricter test setup.
	// For tests, it's better to be explicit. If a test expects ReadFile to be called, it should mock it.
	// However, some tests might not care about ReadFile if keys are provided directly.
	// Let's make it error if a test calls ReadFile without mocking it, to catch unexpected file access.
	// This requires tests that *do* expect ReadFile to set currentMockOsReadFile.
	// For tests not involving PrivateKeyPath, this won't be hit.
	// For tests that DO use PrivateKeyPath, they MUST set currentMockOsReadFile.
	return nil, fmt.Errorf("osReadFile called in test for path '%s' but no mock was set via currentMockOsReadFile", name)
}

// TestMain can be used to set up package-level test utilities if needed.
// For now, we set the mock wrapper once.
func TestMain(m *testing.M) {
	// Replace the package global osReadFile with our wrapper for the duration of tests in this package.
	osReadFile = osReadFileMockWrapperForTest
	// Run tests
	code := m.Run()
	// Restore original osReadFile after all tests in this package have run.
	osReadFile = originalOsReadFileRef
	os.Exit(code)
}


func TestNewRuntime_Success_DetailedConfig(t *testing.T) {
	cfg := &config.Cluster{
		APIVersion: config.DefaultAPIVersion, Kind: config.ClusterKind,
		Metadata:   config.Metadata{Name: "detailed-cluster"},
		Spec: config.ClusterSpec{
			Global: config.GlobalSpec{
				User: "globaluser", Port: 22022, ConnectionTimeout: 15 * time.Second,
				WorkDir: "/mnt/global_work", PrivateKeyPath: "/global/ssh/key_ignored_if_host_specific",
			},
			Hosts: []config.HostSpec{
				{ Name: "master1", Address: "10.0.0.1", Port: 22, User: "masteruser", PrivateKeyPath: "/specific/path/master_key", Roles: []string{"master", "etcd"}, Type: "ssh", WorkDir: "/hostwork/m1"},
				{ Name: "worker1", Address: "10.0.0.2", PrivateKey: base64.StdEncoding.EncodeToString([]byte("test-key-content-worker1")), Roles: []string{"worker"}, Type: "ssh"},
				{ Name: "localnode", Type: "local", Roles: []string{"local"}},
			},
			Kubernetes: config.KubernetesSpec{Version: "v1.23.0"}, // Required by validation
		},
	}
	config.SetDefaults(cfg)
	log := logger.Get()

	origRunnerNewRunner := runnerNewRunner; defer func() { runnerNewRunner = origRunnerNewRunner }()
	var newRunnerCallCount int

	currentMockOsReadFile = func(name string) ([]byte, error) { // Mock ReadFile for master1's key
		if name == "/specific/path/master_key" { return []byte("test-key-content-master1"), nil }
		return nil, fmt.Errorf("os.ReadFile mock: unexpected path %s", name)
	}
	defer func() { currentMockOsReadFile = nil }()

	runnerNewRunner = func(ctx context.Context, conn connector.Connector) (*runner.Runner, error) {
		newRunnerCallCount++
		osInfo, _ := conn.GetOS(ctx)
		return &runner.Runner{Facts: &runner.Facts{OS: osInfo, Hostname: "mock-run-host"}}, nil
	}

	rt, err := NewRuntime(cfg, log)
	if err != nil { t.Fatalf("NewRuntime() with detailed config failed: %v", err) }
	if rt == nil { t.Fatal("NewRuntime() returned nil runtime") }
	if len(rt.Hosts) != 3 { t.Errorf("len(rt.Hosts) = %d, want 3", len(rt.Hosts)) }
	if newRunnerCallCount != 3 { t.Errorf("runner.NewRunner calls = %d, want 3", newRunnerCallCount) }

	master1 := rt.GetHost("master1")
	if master1 == nil { t.Fatal("master1 not found") }
	if master1.User != "masteruser" { t.Errorf("master1.User = %s", master1.User) }
	if master1.Port != 22 { t.Errorf("master1.Port = %d", master1.Port) }
	if master1.WorkDir != "/hostwork/m1" { t.Errorf("master1.WorkDir = %s", master1.WorkDir)}
	if string(master1.PrivateKey) != "test-key-content-master1" { t.Error("master1.PrivateKey content mismatch") }
	if _, ok := master1.Connector.(*connector.SSHConnector); !ok { t.Errorf("master1 connector type %T", master1.Connector)}


	worker1 := rt.GetHost("worker1")
	if worker1 == nil { t.Fatal("worker1 not found") }
	if worker1.User != "globaluser" { t.Errorf("worker1.User = %s", worker1.User) } // Inherited
	if worker1.Port != 22022 { t.Errorf("worker1.Port = %d", worker1.Port) } // Inherited
	if worker1.WorkDir != "/mnt/global_work" {t.Errorf("worker1.WorkDir = %s", worker1.WorkDir)} // Inherited
	if string(worker1.PrivateKey) != "test-key-content-worker1" { t.Error("worker1.PrivateKey content mismatch") }

	localnode := rt.GetHost("localnode")
	if localnode == nil {t.Fatal("localnode not found")}
	if _, ok := localnode.Connector.(*connector.LocalConnector); !ok { t.Error("localnode not LocalConnector")}

	if rt.GlobalTimeout != 15*time.Second { t.Errorf("rt.GlobalTimeout = %v", rt.GlobalTimeout)}
}

func TestNewRuntime_PrivateKeyContentPrecedence(t *testing.T) {
	keyContent := "from-content-directly"; base64KeyContent := base64.StdEncoding.EncodeToString([]byte(keyContent))
	cfg := &config.Cluster{ Spec: config.ClusterSpec{
		Hosts: []config.HostSpec{ {Name: "h1", Address: "1.1.1.1", Port: 22, User: "u", PrivateKey: base64KeyContent, PrivateKeyPath: "/path/should/be/ignored"}},
		Kubernetes: config.KubernetesSpec{Version: "v1"}, Global: config.GlobalSpec{User:"u", Port:22},
	}}
	config.SetDefaults(cfg); log := logger.Get()
	origRunnerNewRunner := runnerNewRunner; defer func() { runnerNewRunner = origRunnerNewRunner }()
	runnerNewRunner = func(ctx context.Context, conn connector.Connector) (*runner.Runner, error) {
		return &runner.Runner{Facts: &runner.Facts{OS: &connector.OS{ID:"linux"}}}, nil
	}
	// osReadFile should not be called if PrivateKey content is provided.
	// The osReadFileMockWrapperForTest will error if currentMockOsReadFile is nil and it's called.
	currentMockOsReadFile = nil // Ensure no mock is set, so a call would error.

	rt, err := NewRuntime(cfg, log)
	if err != nil {t.Fatalf("NewRuntime failed: %v", err)}
	if len(rt.Hosts) != 1 {t.Fatal("Expected 1 host")}
	if string(rt.Hosts[0].PrivateKey) != keyContent {
		t.Errorf("Host.PrivateKey = %s, want %s", string(rt.Hosts[0].PrivateKey), keyContent)
	}
}

func TestNewRuntime_ConnectionFailure_KeyReadError(t *testing.T) {
	currentMockOsReadFile = func(name string) ([]byte, error) {
		if name == "/path/to/host2/key" { return nil, errors.New("simulated read key error for host2") }
		if name == "/path/to/host1/key" { return []byte("host1_key_data"), nil } // host1 key is fine
		return nil, fmt.Errorf("unexpected ReadFile call to %s", name)
	}
	defer func() { currentMockOsReadFile = nil }()

	cfg := &config.Cluster{ Spec: config.ClusterSpec{
		Hosts: []config.HostSpec{
			{Name: "host1", Address: "1.1.1.1", Type: "ssh", User:"u1", Port:22, PrivateKeyPath:"/path/to/host1/key"},
			{Name: "host2", Address: "2.2.2.2", Type: "ssh", User:"u2", Port:22, PrivateKeyPath:"/path/to/host2/key"},
		},
		Global: config.GlobalSpec{ConnectionTimeout: 100 * time.Millisecond}, // Short timeout
		Kubernetes: config.KubernetesSpec{Version:"v1"},
	}}
	config.SetDefaults(cfg); log := logger.Get()

	origRunner := runnerNewRunner; defer func() { runnerNewRunner = origRunner }()
	var runnerCallsForHost1 int32
	runnerNewRunner = func(ctx context.Context, conn connector.Connector) (*runner.Runner, error) {
		// This should only be called for host1 successfully.
		// We need to get the host address from the connector's config if possible,
		// or rely on the test setup ensuring only host1 proceeds this far.
		// For simplicity, assume it's host1 if called.
		atomic.AddInt32(&runnerCallsForHost1, 1)
		return &runner.Runner{Facts: &runner.Facts{OS: &connector.OS{ID:"linux"}}}, nil
	}

	rt, err := NewRuntime(cfg, log)
	if err == nil { t.Fatal("NewRuntime with a failing host key read expected error, got nil") }
	if rt != nil { t.Errorf("NewRuntime returned non-nil rt on error: %+v", rt) }

	initErr, ok := err.(*InitializationError)
	if !ok { t.Fatalf("Expected InitializationError, got %T: %v", err, err) }
	if len(initErr.SubErrors) == 0 { t.Fatal("Expected sub-errors, got none") }

	foundHost2KeyError := false
	for _, subErr := range initErr.SubErrors {
		if strings.Contains(subErr.Error(), "host2") && strings.Contains(subErr.Error(), "failed to read private key") {
			foundHost2KeyError = true; break
		}
	}
	if !foundHost2KeyError { t.Errorf("Expected error related to host2 key read, got: %v", initErr.Error()) }
	if atomic.LoadInt32(&runnerCallsForHost1) != 1 {
		t.Errorf("runnerNewRunner was called %d times, expected 1 (only for host1)", runnerCallsForHost1)
	}
}

// TestNewRuntime_RunnerInitializationFailure, TestNewHostContext, TestInitializationError_Methods
// can remain similar to previous versions, ensuring they use config.SetDefaults on their test cfgs.
// For brevity, only one more is fully fleshed out here.

func TestNewRuntime_RunnerInitializationFailure(t *testing.T) {
	cfg := &config.Cluster{ Spec: config.ClusterSpec{
		Hosts: []config.HostSpec{{Name: "host1", Address: "1.1.1.1", Type: "ssh", User:"u", Port:22, PrivateKeyPath:"/pk"}},
		Global: config.GlobalSpec{ConnectionTimeout: 1 * time.Second},
		Kubernetes: config.KubernetesSpec{Version:"v1"},
	}}
	config.SetDefaults(cfg); log := logger.Get()

	currentMockOsReadFile = func(name string) ([]byte, error) { return []byte("keydata"), nil } // Mock key reading
	defer func() { currentMockOsReadFile = nil }()

	origRunner := runnerNewRunner; defer func() { runnerNewRunner = origRunner }()
	expectedRunnerErr := errors.New("runner init deliberately failed")
	runnerNewRunner = func(ctx context.Context, conn connector.Connector) (*runner.Runner, error) {
		return nil, expectedRunnerErr // Fail runner creation for any host
	}

	_, err := NewRuntime(cfg, log)
	if err == nil { t.Fatal("NewRuntime with failing runner init expected error, got nil") }
	initErr, ok := err.(*InitializationError)
	if !ok { t.Fatalf("Expected InitializationError, got %T: %v", err, err) }
	if len(initErr.SubErrors) != 1 { t.Errorf("Expected 1 sub-error for runner failure, got %d", len(initErr.SubErrors)) } else {
		if !strings.Contains(initErr.SubErrors[0].Error(), "host1") ||
		!strings.Contains(initErr.SubErrors[0].Error(), "runner init failed") ||
		!errors.Is(initErr.SubErrors[0], expectedRunnerErr) {
			t.Errorf("Runner init sub-error message/type mismatch: %v", initErr.SubErrors[0])
		}
	}
}
