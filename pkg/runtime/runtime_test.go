package runtime

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath" // Added
	"reflect"       // For reflect.DeepEqual
	"strings"
	"sync/atomic" // Added
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

	origRunnerNewRunner := runnerNewRunner
	defer func() { runnerNewRunner = origRunnerNewRunner }()
	var newRunnerCallCount int32 // Changed to int32 for atomic operations

	currentMockOsReadFile = func(name string) ([]byte, error) { // Mock ReadFile for master1's key
		if name == "/specific/path/master_key" {
			return []byte("test-key-content-master1"), nil
		}
		return nil, fmt.Errorf("os.ReadFile mock: unexpected path %s", name)
	}
	defer func() { currentMockOsReadFile = nil }()

	runnerNewRunner = func(ctx context.Context, conn connector.Connector) (*runner.Runner, error) {
		atomic.AddInt32(&newRunnerCallCount, 1) // Use atomic increment
		osInfo, _ := conn.GetOS(ctx)
		return &runner.Runner{Facts: &runner.Facts{OS: osInfo, Hostname: "mock-run-host"}}, nil
	}

	rt, err := NewRuntime(cfg, log)
	if err != nil {
		t.Fatalf("NewRuntime() with detailed config failed: %v", err)
	}
	if rt == nil {
		t.Fatal("NewRuntime() returned nil runtime")
	}
	if len(rt.GetAllHosts()) != 3 { // Use GetAllHosts()
		t.Errorf("len(rt.GetAllHosts()) = %d, want 3", len(rt.GetAllHosts()))
	}
	if atomic.LoadInt32(&newRunnerCallCount) != 3 { // Use atomic load
		t.Errorf("runner.NewRunner calls = %d, want 3", atomic.LoadInt32(&newRunnerCallCount))
	}

	// Global WorkDir Assertion
	if rt.GetWorkDir() != cfg.Spec.Global.WorkDir {
		t.Errorf("rt.GetWorkDir() = %s, want %s", rt.GetWorkDir(), cfg.Spec.Global.WorkDir)
	}

	master1 := rt.GetHost("master1")
	if master1 == nil {
		t.Fatal("master1 not found")
	}
	if master1.User != "masteruser" {
		t.Errorf("master1.User = %s", master1.User)
	}
	if master1.Port != 22 {
		t.Errorf("master1.Port = %d", master1.Port)
	}
	// Host WorkDir Assertions
	expectedM1WorkDir := filepath.Join(cfg.Spec.Global.WorkDir, "hosts", "master1")
	if actualM1WorkDir := rt.GetHostWorkDir("master1"); actualM1WorkDir != expectedM1WorkDir {
		t.Errorf("master1 workdir = %s, want %s", actualM1WorkDir, expectedM1WorkDir)
	}
	if string(master1.PrivateKey) != "test-key-content-master1" {
		t.Error("master1.PrivateKey content mismatch")
	}
	if _, ok := master1.Connector.(*connector.SSHConnector); !ok {
		t.Errorf("master1 connector type %T", master1.Connector)
	}

	worker1 := rt.GetHost("worker1")
	if worker1 == nil {
		t.Fatal("worker1 not found")
	}
	if worker1.User != "globaluser" {
		t.Errorf("worker1.User = %s", worker1.User)
	} // Inherited
	if worker1.Port != 22022 {
		t.Errorf("worker1.Port = %d", worker1.Port)
	} // Inherited
	// Host WorkDir Assertions
	expectedW1WorkDir := filepath.Join(cfg.Spec.Global.WorkDir, "hosts", "worker1")
	if actualW1WorkDir := rt.GetHostWorkDir("worker1"); actualW1WorkDir != expectedW1WorkDir {
		t.Errorf("worker1 workdir = %s, want %s", actualW1WorkDir, expectedW1WorkDir)
	}
	if string(worker1.PrivateKey) != "test-key-content-worker1" {
		t.Error("worker1.PrivateKey content mismatch")
	}

	localnode := rt.GetHost("localnode")
	if localnode == nil {
		t.Fatal("localnode not found")
	}
	if _, ok := localnode.Connector.(*connector.LocalConnector); !ok {
		t.Error("localnode not LocalConnector")
	}

	if rt.GlobalTimeout != 15*time.Second {
		t.Errorf("rt.GlobalTimeout = %v", rt.GlobalTimeout)
	}
}

func TestNewRuntime_PrivateKeyContentPrecedence(t *testing.T) {
	keyContent := "from-content-directly"
	base64KeyContent := base64.StdEncoding.EncodeToString([]byte(keyContent))
	cfg := &config.Cluster{Spec: config.ClusterSpec{
		Hosts:      []config.HostSpec{{Name: "h1", Address: "1.1.1.1", Port: 22, User: "u", PrivateKey: base64KeyContent, PrivateKeyPath: "/path/should/be/ignored"}},
		Kubernetes: config.KubernetesSpec{Version: "v1"}, Global: config.GlobalSpec{User: "u", Port: 22},
	}}
	config.SetDefaults(cfg)
	log := logger.Get()
	origRunnerNewRunner := runnerNewRunner
	defer func() { runnerNewRunner = origRunnerNewRunner }()
	runnerNewRunner = func(ctx context.Context, conn connector.Connector) (*runner.Runner, error) {
		return &runner.Runner{Facts: &runner.Facts{OS: &connector.OS{ID: "linux"}}}, nil
	}
	// osReadFile should not be called if PrivateKey content is provided.
	// The osReadFileMockWrapperForTest will error if currentMockOsReadFile is nil and it's called.
	currentMockOsReadFile = nil // Ensure no mock is set, so a call would error.

	rt, err := NewRuntime(cfg, log)
	if err != nil {
		t.Fatalf("NewRuntime failed: %v", err)
	}
	if len(rt.GetAllHosts()) != 1 { // Use GetAllHosts()
		t.Fatal("Expected 1 host")
	}
	if string(rt.GetAllHosts()[0].PrivateKey) != keyContent { // Use GetAllHosts()
		t.Errorf("Host.PrivateKey = %s, want %s", string(rt.GetAllHosts()[0].PrivateKey), keyContent)
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

func TestClusterRuntime_Copy(t *testing.T) {
	cfg := &config.Cluster{
		Metadata: config.Metadata{Name: "copy-test-cluster"},
		Spec: config.ClusterSpec{
			Global: config.GlobalSpec{User: "testuser", Port: 2222, WorkDir: "/tmp/copytest_unique_" + t.Name(), ConnectionTimeout: 5 * time.Second}, // Unique WorkDir
			Hosts: []config.HostSpec{
				{Name: "host1", Address: "1.1.1.1", Roles: []string{"roleA"}},
				{Name: "host2", Address: "2.2.2.2", Roles: []string{"roleB"}},
			},
			Kubernetes: config.KubernetesSpec{Version: "v1.2.3"},
		},
	}
	config.SetDefaults(cfg)
	log := logger.Get()

	// Ensure unique workdir is cleaned up
	t.Cleanup(func() { os.RemoveAll(cfg.Spec.Global.WorkDir) })

	origRunnerNewRunner := runnerNewRunner
	defer func() { runnerNewRunner = origRunnerNewRunner }()
	runnerNewRunner = func(ctx context.Context, conn connector.Connector) (*runner.Runner, error) {
		// Mock GetOS for runner facts
		mockConn, ok := conn.(*mockConnectorForRuntime)
		if ok && mockConn.GetOSFunc == nil { // If it's our mock and GetOS is not specifically set
			mockConn.GetOSFunc = func(ctx context.Context) (*connector.OS, error) {
				return &connector.OS{ID: "linux-mock", Arch: "amd64", Kernel: "mock-kernel", VersionID: "1.0"}, nil
			}
		}
		return &runner.Runner{Facts: &runner.Facts{OS: &connector.OS{ID: "linux-mock"}}}, nil
	}
	currentMockOsReadFile = nil // No key paths, so ReadFile shouldn't be called.

	originalRt, err := NewRuntime(cfg, log)
	if err != nil {
		t.Fatalf("NewRuntime failed: %v", err)
	}
	if originalRt == nil {
		t.Fatal("Original NewRuntime returned nil")
	}
	if originalRt.BaseRuntime == nil {
		t.Fatal("Original NewRuntime returned with nil BaseRuntime")
	}

	copiedRt := originalRt.Copy()
	if copiedRt == nil {
		t.Fatal("ClusterRuntime.Copy() returned nil")
	}
	if copiedRt.BaseRuntime == nil {
		t.Fatal("Copied ClusterRuntime has nil BaseRuntime")
	}

	// 1. Check basic properties
	if copiedRt.ObjName() != originalRt.ObjName() {
		t.Errorf("Copied ObjName mismatch: got %s, want %s", copiedRt.ObjName(), originalRt.ObjName())
	}
	if copiedRt.GlobalTimeout != originalRt.GlobalTimeout {
		t.Errorf("Copied GlobalTimeout mismatch: got %v, want %v", copiedRt.GlobalTimeout, originalRt.GlobalTimeout)
	}
	if copiedRt.ClusterConfig != originalRt.ClusterConfig { // Pointer should be same
		t.Error("Copied ClusterConfig is not the same pointer (it should be)")
	}

	// 2. Check BaseRuntime was copied (not same pointer for BaseRuntime itself)
	if copiedRt.BaseRuntime == originalRt.BaseRuntime {
		t.Error("BaseRuntime in copy is the same pointer as original's BaseRuntime")
	}
	if len(copiedRt.GetAllHosts()) != len(originalRt.GetAllHosts()) {
		t.Errorf("Host count mismatch in copy: got %d, want %d", len(copiedRt.GetAllHosts()), len(originalRt.GetAllHosts()))
	}
	// Ensure host pointers within the collections are the same
	if len(originalRt.GetAllHosts()) > 0 && len(copiedRt.GetAllHosts()) > 0 {
		if originalRt.GetAllHosts()[0] != copiedRt.GetAllHosts()[0] {
			t.Error("Host pointers in copied runtime's host list are not the same as original's")
		}
	}

	if copiedRt.GetWorkDir() != originalRt.GetWorkDir() {
		t.Errorf("Copied WorkDir mismatch: got %s, want %s", copiedRt.GetWorkDir(), originalRt.GetWorkDir())
	}

	// 3. Test independence of host list modifications
	originalHostCount := len(originalRt.GetAllHosts())
	if originalHostCount > 0 {
		hostToRemoveName := originalRt.GetAllHosts()[0].Name

		if copiedRt.GetHost(hostToRemoveName) == nil {
			t.Fatalf("Host %s expected in copiedRt before removal, but not found", hostToRemoveName)
		}

		err = copiedRt.RemoveHost(hostToRemoveName) // RemoveHost is a method on ClusterRuntime now
		if err != nil {
			t.Fatalf("Failed to remove host '%s' from copied runtime: %v", hostToRemoveName, err)
		}

		if len(copiedRt.GetAllHosts()) != originalHostCount-1 {
			t.Errorf("Host count in copiedRt after removal is %d, expected %d", len(copiedRt.GetAllHosts()), originalHostCount-1)
		}
		if len(originalRt.GetAllHosts()) != originalHostCount { // Original should be unchanged
			t.Errorf("Original runtime host count changed: got %d, expected %d", len(originalRt.GetAllHosts()), originalHostCount)
		}
		if originalRt.GetHost(hostToRemoveName) == nil { // Host should still be in original
			t.Errorf("Host %s was removed from originalRt or not found, but should exist", hostToRemoveName)
		}
		if copiedRt.GetHost(hostToRemoveName) != nil { // Host should be gone from copy
			t.Errorf("Host %s should have been removed from copiedRt, but still exists", hostToRemoveName)
		}
	} else {
		t.Log("Skipping host list independence test as no hosts were initialized by NewRuntime.")
	}

	// 4. Verify logger is shared (same pointer)
	if copiedRt.Logger() != originalRt.Logger() {
		t.Error("Logger instance is not shared between original and copied runtime")
	}

	// 5. Test that connector mock is correctly used by NewRuntime.
	// This is implicitly tested if NewRuntime succeeds without file access when PrivateKeyPath is not set.
	// And that runnerNewRunner mock is used.
}

func TestBaseRuntime_AddGetRemoveHost(t *testing.T) {
	log := logger.Get()
	workDir := t.TempDir() // Use t.TempDir for test-specific work directories
	br, err := NewBaseRuntime("test-br", workDir, false, false, log)
	if err != nil {
		t.Fatalf("NewBaseRuntime failed: %v", err)
	}

	host1 := &Host{Name: "h1", Roles: map[string]bool{"r1": true}}
	if err := br.AddHost(host1); err != nil {
		t.Fatalf("br.AddHost(h1) failed: %v", err)
	}
	if br.GetHost("h1") != host1 {
		t.Error("br.GetHost(h1) did not return host1")
	}
	if len(br.GetAllHosts()) != 1 {
		t.Errorf("len(br.GetAllHosts()) got %d, want 1", len(br.GetAllHosts()))
	}
	if len(br.GetHostsByRole("r1")) != 1 || br.GetHostsByRole("r1")[0] != host1 {
		t.Error("br.GetHostsByRole('r1') failed")
	}

	// Test adding duplicate
	if err := br.AddHost(host1); err == nil {
		t.Error("br.AddHost(h1) again should have failed (duplicate)")
	}

	host2 := &Host{Name: "h2", Roles: map[string]bool{"r1": true, "r2": true}}
	br.AddHost(host2)
	if len(br.GetHostsByRole("r1")) != 2 {
		t.Error("br.GetHostsByRole('r1') after adding h2 failed")
	}

	// Test removal
	if err := br.RemoveHost("h1"); err != nil {
		t.Fatalf("br.RemoveHost(h1) failed: %v", err)
	}
	if br.GetHost("h1") != nil {
		t.Error("br.GetHost(h1) after removal was not nil")
	}
	if len(br.GetAllHosts()) != 1 || br.GetAllHosts()[0] != host2 {
		t.Error("br.GetAllHosts() after removing h1 failed")
	}
	if len(br.GetHostsByRole("r1")) != 1 || br.GetHostsByRole("r1")[0] != host2 {
		t.Errorf("br.GetHostsByRole('r1') after removing h1 failed: got %v", br.GetHostsByRole("r1"))
	}
	if len(br.GetHostsByRole("r2")) != 1 || br.GetHostsByRole("r2")[0] != host2 {
		t.Error("br.GetHostsByRole('r2') after removing h1 failed")
	}
	if err := br.RemoveHost("hNonExistent"); err == nil {
		t.Error("br.RemoveHost for non-existent host should have failed")
	}
}

func TestBaseRuntime_WorkDirs(t *testing.T) {
	log := logger.Get()
	globalWorkDirParent := t.TempDir()
	globalWorkDir := filepath.Join(globalWorkDirParent, "test-br-work")
	// NewBaseRuntime will create globalWorkDir
	br, err := NewBaseRuntime("test-br-wd", globalWorkDir, false, false, log)
	if err != nil {
		t.Fatalf("NewBaseRuntime failed: %v", err)
	}
	if br.GetWorkDir() != globalWorkDir {
		t.Errorf("br.GetWorkDir() got %s, want %s", br.GetWorkDir(), globalWorkDir)
	}
	// Check if directory was created by NewBaseRuntime
	if _, errStat := os.Stat(globalWorkDir); os.IsNotExist(errStat) {
		t.Errorf("NewBaseRuntime did not create workDir: %s", globalWorkDir)
	}

	expectedHostDir := filepath.Join(globalWorkDir, "hosts", "myHost")
	if br.GetHostWorkDir("myHost") != expectedHostDir {
		t.Errorf("br.GetHostWorkDir('myHost') got %s, want %s", br.GetHostWorkDir("myHost"), expectedHostDir)
	}
}

func TestBaseRuntime_Copy(t *testing.T) {
	log := logger.Get()
	br, _ := NewBaseRuntime("test-br-copy", t.TempDir(), false, false, log)
	br.AddHost(&Host{Name: "h1", Roles: map[string]bool{"r1": true}})

	brCopy := br.Copy()
	if brCopy.ObjName() != "test-br-copy" {
		t.Error("Copy ObjName mismatch")
	}
	if len(brCopy.GetAllHosts()) != 1 {
		t.Error("Copy host count mismatch")
	}
	if brCopy.GetAllHosts()[0] != br.GetAllHosts()[0] { // Host pointers should be same
		t.Error("Host pointer in copy is different")
	}

	// Test independence
	brCopy.RemoveHost("h1")
	if len(brCopy.GetAllHosts()) != 0 {
		t.Error("Host not removed from copy")
	}
	if len(br.GetAllHosts()) != 1 {
		t.Error("Original affected by copy's RemoveHost")
	}
}
