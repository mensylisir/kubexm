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
	"sync/atomic" // Added
	"testing"
	"time"

	// "github.com/kubexms/kubexms/pkg/config" // Removed
	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"github.com/mensylisir/kubexm/pkg/connector" // For connector types and OS struct
	"github.com/mensylisir/kubexm/pkg/logger"
	"github.com/mensylisir/kubexm/pkg/runner"
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
	cfg := &v1alpha1.Cluster{
		TypeMeta:   metav1.TypeMeta{APIVersion: v1alpha1.SchemeGroupVersion.String(), Kind: "Cluster"},
		ObjectMeta: metav1.ObjectMeta{Name: "detailed-cluster"},
		Spec: v1alpha1.ClusterSpec{
			Global: &v1alpha1.GlobalSpec{
				User:              "globaluser",
				Port:              22022,
				ConnectionTimeout: 15 * time.Second,
				// WorkDir is managed by t.TempDir() below
				// PrivateKeyPath can be set if needed for the test logic
			},
			Hosts: []v1alpha1.HostSpec{
				{Name: "master1", Address: "10.0.0.1", Port: 22, User: "masteruser", PrivateKeyPath: "/specific/path/master_key", Roles: []string{"master", "etcd"}, Type: "ssh"},
				{Name: "worker1", Address: "10.0.0.2", PrivateKey: base64.StdEncoding.EncodeToString([]byte("test-key-content-worker1")), Roles: []string{"worker"}, Type: "ssh"},
				{Name: "localnode", Type: "local", Roles: []string{"local"}},
			},
			Kubernetes: &v1alpha1.KubernetesConfig{Version: "v1.23.0"}, // Kubernetes is a pointer
		},
	}
	testGlobalWorkDir := t.TempDir()
	cfg.Spec.Global.WorkDir = testGlobalWorkDir
	t.Cleanup(func() { os.RemoveAll(testGlobalWorkDir) })

	v1alpha1.SetDefaults_Cluster(cfg)
	log := logger.Get()

	origRunnerNewRunner := runnerNewRunner
	defer func() { runnerNewRunner = origRunnerNewRunner }()
	var newRunnerCallCount int32 // Changed to int32

	currentMockOsReadFile = func(name string) ([]byte, error) {
		if name == "/specific/path/master_key" {
			return []byte("test-key-content-master1"), nil
		}
		return nil, fmt.Errorf("os.ReadFile mock: unexpected path %s", name)
	}
	defer func() { currentMockOsReadFile = nil }()

	runnerNewRunner = func(ctx context.Context, conn connector.Connector) (*runner.Runner, error) {
		atomic.AddInt32(&newRunnerCallCount, 1) // Use atomic
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
	if len(rt.GetAllHosts()) != 3 { // Use GetAllHosts
		t.Errorf("len(rt.GetAllHosts()) = %d, want 3", len(rt.GetAllHosts()))
	}
	if atomic.LoadInt32(&newRunnerCallCount) != 3 { // Use atomic
		t.Errorf("runner.NewRunner calls = %d, want 3", atomic.LoadInt32(&newRunnerCallCount))
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
	// Assert host work dir using GetHostWorkDir
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
	if rt.ClusterConfig.ObjectMeta.Name != "detailed-cluster" {
		t.Errorf("rt.ClusterConfig.ObjectMeta.Name = %s", rt.ClusterConfig.ObjectMeta.Name)
	}
}

func TestNewRuntime_PrivateKeyContentPrecedence(t *testing.T) {
	keyContent := "from-content-directly"
	base64KeyContent := base64.StdEncoding.EncodeToString([]byte(keyContent))
	cfg := &v1alpha1.Cluster{
		TypeMeta:   metav1.TypeMeta{APIVersion: v1alpha1.SchemeGroupVersion.String(), Kind: "Cluster"},
		ObjectMeta: metav1.ObjectMeta{Name: "pk-test"},
		Spec: v1alpha1.ClusterSpec{
			Hosts: []v1alpha1.HostSpec{{Name: "h1", Address: "1.1.1.1", Port: 22, User: "u", PrivateKey: base64KeyContent, PrivateKeyPath: "/path/should/be/ignored"}},
			Kubernetes: &v1alpha1.KubernetesConfig{Version: "v1.23.0"}, // Required
			Global:     &v1alpha1.GlobalSpec{User: "u", Port: 22},       // Required for host defaults
		},
	}
	v1alpha1.SetDefaults_Cluster(cfg)
	log := logger.Get()

	origRunnerNewRunner := runnerNewRunner
	defer func() { runnerNewRunner = origRunnerNewRunner }()
	runnerNewRunner = func(ctx context.Context, conn connector.Connector) (*runner.Runner, error) {
		return &runner.Runner{Facts: &runner.Facts{OS: &connector.OS{ID: "linux"}}}, nil
	}
	currentMockOsReadFile = nil // Ensure no mock is set, so a call would error.

	rt, err := NewRuntime(cfg, log)
	if err != nil {
		t.Fatalf("NewRuntime failed: %v", err)
	}
	if len(rt.GetAllHosts()) != 1 { // Use GetAllHosts
		t.Fatal("Expected 1 host")
	}
	if string(rt.GetAllHosts()[0].PrivateKey) != keyContent { // Use GetAllHosts
		t.Errorf("Host.PrivateKey = %s, want %s", string(rt.GetAllHosts()[0].PrivateKey), keyContent)
	}
}

func TestNewRuntime_ConnectionFailure_KeyReadError(t *testing.T) {
	currentMockOsReadFile = func(name string) ([]byte, error) {
		if name == "/path/to/host2/key" {
			return nil, errors.New("simulated read key error for host2")
		}
		if name == "/path/to/host1/key" {
			return []byte("host1_key_data"), nil
		} // host1 key is fine
		return nil, fmt.Errorf("unexpected ReadFile call to %s", name)
	}
	defer func() { currentMockOsReadFile = nil }()

	cfg := &v1alpha1.Cluster{
		TypeMeta:   metav1.TypeMeta{APIVersion: v1alpha1.SchemeGroupVersion.String(), Kind: "Cluster"},
		ObjectMeta: metav1.ObjectMeta{Name: "key-read-error-test"},
		Spec: v1alpha1.ClusterSpec{
			Hosts: []v1alpha1.HostSpec{
				{Name: "host1", Address: "1.1.1.1", Type: "ssh", User: "u1", Port: 22, PrivateKeyPath: "/path/to/host1/key"},
				{Name: "host2", Address: "2.2.2.2", Type: "ssh", User: "u2", Port: 22, PrivateKeyPath: "/path/to/host2/key"},
			},
			Global:     &v1alpha1.GlobalSpec{ConnectionTimeout: 100 * time.Millisecond, User: "u", Port: 22}, // Required for host defaults
			Kubernetes: &v1alpha1.KubernetesConfig{Version: "v1.23.0"},                                     // Required
		},
	}
	v1alpha1.SetDefaults_Cluster(cfg)
	log := logger.Get()

	origRunner := runnerNewRunner
	defer func() { runnerNewRunner = origRunner }()
	var runnerCallsForHost1 int32
	runnerNewRunner = func(ctx context.Context, conn connector.Connector) (*runner.Runner, error) {
		atomic.AddInt32(&runnerCallsForHost1, 1)
		return &runner.Runner{Facts: &runner.Facts{OS: &connector.OS{ID: "linux"}}}, nil
	}

	rt, err := NewRuntime(cfg, log)
	if err == nil {
		t.Fatal("NewRuntime with a failing host key read expected error, got nil")
	}
	if rt != nil {
		t.Errorf("NewRuntime returned non-nil rt on error: %+v", rt)
	}

	initErr, ok := err.(*InitializationError)
	if !ok {
		t.Fatalf("Expected InitializationError, got %T: %v", err, err)
	}
	if len(initErr.SubErrors()) == 0 { // Assuming SubErrors is a method returning a slice
		t.Fatal("Expected sub-errors, got none")
	}

	foundHost2KeyError := false
	for _, subErr := range initErr.SubErrors() { // Assuming SubErrors is a method
		if strings.Contains(subErr.Error(), "host2") && strings.Contains(subErr.Error(), "failed to read private key") {
			foundHost2KeyError = true
			break
		}
	}
	if !foundHost2KeyError {
		t.Errorf("Expected error related to host2 key read, got: %v", initErr.Error())
	}
	if atomic.LoadInt32(&runnerCallsForHost1) != 1 {
		t.Errorf("runnerNewRunner was called %d times, expected 1 (only for host1)", runnerCallsForHost1)
	}
}

func TestNewRuntime_RunnerInitializationFailure(t *testing.T) {
	cfg := &v1alpha1.Cluster{
		TypeMeta:   metav1.TypeMeta{APIVersion: v1alpha1.SchemeGroupVersion.String(), Kind: "Cluster"},
		ObjectMeta: metav1.ObjectMeta{Name: "runner-fail-test"},
		Spec: v1alpha1.ClusterSpec{
			Hosts:      []v1alpha1.HostSpec{{Name: "host1", Address: "1.1.1.1", Type: "ssh", User: "u", Port: 22, PrivateKeyPath: "/pk"}},
			Global:     &v1alpha1.GlobalSpec{ConnectionTimeout: 1 * time.Second, User: "u", Port: 22}, // Required for host defaults
			Kubernetes: &v1alpha1.KubernetesConfig{Version: "v1.23.0"},                               // Required
		},
	}
	v1alpha1.SetDefaults_Cluster(cfg)
	log := logger.Get()

	currentMockOsReadFile = func(name string) ([]byte, error) { return []byte("keydata"), nil }
	defer func() { currentMockOsReadFile = nil }()

	origRunner := runnerNewRunner
	defer func() { runnerNewRunner = origRunner }()
	expectedRunnerErr := errors.New("runner init deliberately failed")
	runnerNewRunner = func(ctx context.Context, conn connector.Connector) (*runner.Runner, error) {
		return nil, expectedRunnerErr
	}

	_, err := NewRuntime(cfg, log)
	if err == nil {
		t.Fatal("NewRuntime with failing runner init expected error, got nil")
	}
	initErr, ok := err.(*InitializationError)
	if !ok {
		t.Fatalf("Expected InitializationError, got %T: %v", err, err)
	}
	subErrs := initErr.SubErrors() // Assuming SubErrors is a method
	if len(subErrs) != 1 {
		t.Errorf("Expected 1 sub-error for runner failure, got %d", len(subErrs))
	} else {
		if !strings.Contains(subErrs[0].Error(), "host1") ||
			!strings.Contains(subErrs[0].Error(), "runner init failed") ||
			!errors.Is(subErrs[0], expectedRunnerErr) {
			t.Errorf("Runner init sub-error message/type mismatch: %v", subErrs[0])
		}
	}
}

// TODO: Add TestClusterRuntime_Copy if it was defined previously, ensuring it uses v1alpha1.Cluster
// and calls v1alpha1.SetDefaults_Cluster(cfg).

func TestNewRuntimeFromYAML(t *testing.T) {
	// Mock runner.NewRunner to prevent actual host connections during these YAML tests
	// unless specific host interactions are being tested.
	origRunnerNewRunner := runnerNewRunner
	defer func() { runnerNewRunner = origRunnerNewRunner }()
	runnerNewRunner = func(ctx context.Context, conn connector.Connector) (*runner.Runner, error) {
		// Provide a minimal mock runner
		osInfo, _ := conn.GetOS(ctx) // Use the mock connector's GetOS
		return &runner.Runner{Facts: &runner.Facts{OS: osInfo, Hostname: "mock-runner-host"}}, nil
	}

	// Mock os.ReadFile for private key loading if any test YAML uses PrivateKeyPath
	// For these tests, we'll mostly use inline keys or no keys.
	currentMockOsReadFile = func(name string) ([]byte, error) {
		return nil, fmt.Errorf("os.ReadFile mock: unexpected call to %s in NewRuntimeFromYAML tests", name)
	}
	defer func() { currentMockOsReadFile = nil }()


	validYAML := `
apiVersion: kubexms.io/v1alpha1
kind: Cluster
metadata:
  name: yaml-test-cluster
spec:
  global:
    user: yamluser
  hosts:
    - name: node-yaml
      address: 192.168.1.100
      roles: ["controlplane", "worker"]
  kubernetes:
    version: "1.27.1"
  network:
    plugin: "calico"
`
	// This YAML is structurally valid for parsing but might fail deeper validation
	// if NewRuntime expects certain fields that are defaulted by v1alpha1.SetDefaults_Cluster
	// but not present here (e.g. Kubernetes version if it became mandatory post-defaulting).
	// The conversion function itself is simple, so this mainly tests parser + conversion + NewRuntime call.

	invalidYAMLBadSyntax := `
apiVersion: kubexms.io/v1alpha1
kind: Cluster
metadata:
  name: bad-yaml
spec:
  kubernetes: version: "noquote
`

	minimalYAML := `
apiVersion: kubexms.io/v1alpha1
kind: Cluster
metadata:
  name: minimal-cluster
spec:
  hosts:
  - name: m1
    address: 1.2.3.4
  # Crucial: NewRuntime expects certain fields to be non-nil after defaulting.
  # For example, if cfg.Spec.Kubernetes is nil, NewRuntime might fail if it accesses it.
  # v1alpha1.SetDefaults_Cluster would typically initialize these.
  # The conversion function now just copies these pointers. If they are nil in config.Cluster.Spec
  # (because they were nil in YAML and config.Cluster.Spec uses v1alpha1 types directly),
  // they will be nil in the v1alpha1.Cluster passed to NewRuntime.
  # So, for a successful NewRuntime call, the YAML needs to provide enough structure
  # or the SetDefaults_Cluster (called within NewRuntime) needs to be robust.
  # Let's add a minimal Kubernetes block as NewRuntime likely expects it.
  kubernetes:
    version: "v0.0.0-defaulted" # This would be set by SetDefaults_KubernetesConfig typically
  global: # Global is also often defaulted and expected
    user: "default"
    port: 22
`


	testCases := []struct {
		name          string
		yamlData      []byte
		expectError   bool
		expectedName  string
		expectedK8sVer string
		expectedNumHosts int
	}{
		{
			name:          "valid full YAML",
			yamlData:      []byte(validYAML),
			expectError:   false,
			expectedName:  "yaml-test-cluster",
			expectedK8sVer: "1.27.1",
			expectedNumHosts: 1,
		},
		{
			name:          "minimal valid YAML for runtime",
			yamlData:      []byte(minimalYAML),
			expectError:   false,
			expectedName:  "minimal-cluster",
			expectedK8sVer: "v0.0.0-defaulted",
			expectedNumHosts: 1,
		},
		{
			name:        "invalid YAML syntax",
			yamlData:    []byte(invalidYAMLBadSyntax),
			expectError: true,
		},
		{
			name:        "empty YAML data",
			yamlData:    []byte(""),
			expectError: true, // Parser should error
		},
		{
			name: "YAML with omitted fields for defaulting",
			yamlData: []byte(`
apiVersion: kubexms.io/v1alpha1
kind: Cluster
metadata:
  name: test-defaults
spec:
  hosts:
  - name: node1
    address: 1.1.1.1
  # kubernetes, etcd, network, registry, etc. are omitted to test defaulting
`),
			expectError:   false, // Expecting successful creation with defaults
			expectedName:  "test-defaults",
			// We will check specific default values below, not just k8s version
		},
	}

	log := logger.Get() // Use a common logger

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rt, err := NewRuntimeFromYAML(tc.yamlData, log)

			if tc.expectError {
				if err == nil {
					t.Errorf("Expected an error, but got nil")
				}
			} else {
				if err != nil {
					t.Fatalf("Expected no error, but got: %v", err)
				}
				if rt == nil {
					t.Fatalf("Expected a non-nil ClusterRuntime, but got nil")
				}
				if rt.ClusterConfig == nil {
					t.Fatalf("ClusterRuntime.ClusterConfig is nil")
				}
				if rt.ClusterConfig.Name != tc.expectedName {
					t.Errorf("Expected cluster name %q, got %q", tc.expectedName, rt.ClusterConfig.Name)
				}
				if rt.ClusterConfig.Spec.Kubernetes == nil && tc.expectedK8sVer != "" {
					t.Errorf("Expected Kubernetes version %q, but Kubernetes spec is nil", tc.expectedK8sVer)
				} else if rt.ClusterConfig.Spec.Kubernetes != nil && rt.ClusterConfig.Spec.Kubernetes.Version != tc.expectedK8sVer {
					t.Errorf("Expected Kubernetes version %q, got %q", tc.expectedK8sVer, rt.ClusterConfig.Spec.Kubernetes.Version)
				}
				if len(rt.GetAllHosts()) != tc.expectedNumHosts {
					t.Errorf("Expected %d hosts, got %d", tc.expectedNumHosts, len(rt.GetAllHosts()))
				}

				// Specific checks for the defaulting test case
				if tc.name == "YAML with omitted fields for defaulting" {
					// Kubernetes defaults
					if rt.ClusterConfig.Spec.Kubernetes == nil {
						t.Fatalf("Expected rt.ClusterConfig.Spec.Kubernetes to be defaulted, but it was nil")
					}
					if rt.ClusterConfig.Spec.Kubernetes.ContainerManager != "docker" {
						t.Errorf("Expected Kubernetes.ContainerManager default 'docker', got '%s'", rt.ClusterConfig.Spec.Kubernetes.ContainerManager)
					}
					if rt.ClusterConfig.Spec.Kubernetes.MaxPods == nil || *rt.ClusterConfig.Spec.Kubernetes.MaxPods != 110 {
						t.Errorf("Expected Kubernetes.MaxPods default 110, got %v", rt.ClusterConfig.Spec.Kubernetes.MaxPods)
					}
					// Etcd defaults
					if rt.ClusterConfig.Spec.Etcd == nil {
						t.Fatalf("Expected rt.ClusterConfig.Spec.Etcd to be defaulted, but it was nil")
					}
					if rt.ClusterConfig.Spec.Etcd.Type != "kubexm" {
						t.Errorf("Expected Etcd.Type default 'kubexm', got '%s'", rt.ClusterConfig.Spec.Etcd.Type)
					}
					if rt.ClusterConfig.Spec.Etcd.HeartbeatIntervalMillis == nil || *rt.ClusterConfig.Spec.Etcd.HeartbeatIntervalMillis != 250 {
						val := "nil"
						if rt.ClusterConfig.Spec.Etcd.HeartbeatIntervalMillis != nil {
							val = fmt.Sprintf("%d", *rt.ClusterConfig.Spec.Etcd.HeartbeatIntervalMillis)
						}
						t.Errorf("Expected Etcd.HeartbeatIntervalMillis default 250, got %s", val)
					}
					if rt.ClusterConfig.Spec.Etcd.ElectionTimeoutMillis == nil || *rt.ClusterConfig.Spec.Etcd.ElectionTimeoutMillis != 5000 {
						val := "nil"
						if rt.ClusterConfig.Spec.Etcd.ElectionTimeoutMillis != nil {
							val = fmt.Sprintf("%d", *rt.ClusterConfig.Spec.Etcd.ElectionTimeoutMillis)
						}
						t.Errorf("Expected Etcd.ElectionTimeoutMillis default 5000, got %s", val)
					}
					// Network defaults
					if rt.ClusterConfig.Spec.Network == nil {
						t.Fatalf("Expected rt.ClusterConfig.Spec.Network to be defaulted, but it was nil")
					}
					if rt.ClusterConfig.Spec.Network.Plugin != "calico" {
						t.Errorf("Expected Network.Plugin default 'calico', got '%s'", rt.ClusterConfig.Spec.Network.Plugin)
					}
					if rt.ClusterConfig.Spec.Network.Calico == nil {
						t.Fatalf("Expected Network.Calico to be defaulted when plugin is 'calico', but it was nil")
					}
					if rt.ClusterConfig.Spec.Network.Calico.IPIPMode != "Always" {
						t.Errorf("Expected Calico.IPIPMode default 'Always', got '%s'", rt.ClusterConfig.Spec.Network.Calico.IPIPMode)
					}
					if rt.ClusterConfig.Spec.Network.Calico.VXLANMode != "Never" {
						t.Errorf("Expected Calico.VXLANMode default 'Never', got '%s'", rt.ClusterConfig.Spec.Network.Calico.VXLANMode)
					}
					// Registry defaults
					if rt.ClusterConfig.Spec.Registry == nil {
						t.Fatalf("Expected rt.ClusterConfig.Spec.Registry to be defaulted, but it was nil")
					}
					if rt.ClusterConfig.Spec.Registry.PrivateRegistry != "dockerhub.kubexm.local" {
						t.Errorf("Expected Registry.PrivateRegistry default 'dockerhub.kubexm.local', got '%s'", rt.ClusterConfig.Spec.Registry.PrivateRegistry)
					}
				}
			}
		})
	}
}
