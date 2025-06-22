package step

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/logger"
	"github.com/mensylisir/kubexm/pkg/runner"
	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1" // Added for ClusterConfig
	"github.com/mensylisir/kubexm/pkg/cache"      // Added for cache interfaces
	"path/filepath" // Added for path helpers
	// "github.com/mensylisir/kubexm/pkg/runtime" // REMOVE THIS IMPORT
	// "github.com/kubexms/kubexms/pkg/config" // Not directly needed by step tests if runtime.Context is constructed carefully
)

// MockStepConnector is a mock implementation of connector.Connector for step tests.
type MockStepConnector struct {
	ExecFunc        func(ctx context.Context, cmd string, options *connector.ExecOptions) (stdout, stderr []byte, err error)
	CopyContentFunc func(ctx context.Context, content []byte, dstPath string, options *connector.FileTransferOptions) error
	StatFunc        func(ctx context.Context, path string) (*connector.FileStat, error)
	LookPathFunc    func(ctx context.Context, file string) (string, error)
	GetOSFunc       func(ctx context.Context) (*connector.OS, error)
	// Add other connector methods if needed by steps being tested

	ExecHistory []string // Stores executed commands
	mu          sync.Mutex
}

func NewMockStepConnector() *MockStepConnector {
	return &MockStepConnector{
		ExecFunc: func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
			// Default ExecFunc behavior, can be overridden in tests.
			// fmt.Printf("MockStepConnector: Exec called with cmd: %s\n", cmd) // For debugging tests
			return []byte(""), []byte(""), nil
		},
		GetOSFunc: func(ctx context.Context) (*connector.OS, error) {
			return &connector.OS{ID: "linux", Arch: "amd64", VersionID: "test"}, nil
		},
		LookPathFunc: func(ctx context.Context, file string) (string, error) { return "/usr/bin/" + file, nil },
		StatFunc: func(ctx context.Context, path string) (*connector.FileStat, error) {
			if strings.Contains(path, "nonexistent") {
				return &connector.FileStat{Name: path, IsExist: false}, nil
			}
			isDir := strings.HasSuffix(path, "/") || path == "/tmp" || strings.Contains(path, "dir")
			return &connector.FileStat{Name: path, IsExist: true, IsDir: isDir, Mode: 0755, Size: 100, ModTime: time.Now()}, nil
		},
		CopyContentFunc: func(ctx context.Context, content []byte, dstPath string, options *connector.FileTransferOptions) error {return nil},
		ExecHistory: []string{},
	}
}
func (m *MockStepConnector) recordExec(cmd string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ExecHistory == nil {
		m.ExecHistory = []string{}
	}
	m.ExecHistory = append(m.ExecHistory, cmd)
}
func (m *MockStepConnector) Connect(ctx context.Context, cfg connector.ConnectionCfg) error { return nil }
func (m *MockStepConnector) Exec(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
	m.recordExec(cmd) // Record every command
	return m.ExecFunc(ctx, cmd, options)
}
func (m *MockStepConnector) Copy(ctx context.Context, srcPath, dstPath string, options *connector.FileTransferOptions) error { return fmt.Errorf("Copy not implemented in MockStepConnector") }
func (m *MockStepConnector) CopyContent(ctx context.Context, content []byte, dstPath string, options *connector.FileTransferOptions) error {
	return m.CopyContentFunc(ctx, content, dstPath, options)
}
func (m *MockStepConnector) Fetch(ctx context.Context, remotePath, localPath string) error { return fmt.Errorf("Fetch not implemented in MockStepConnector") }
func (m *MockStepConnector) Stat(ctx context.Context, path string) (*connector.FileStat, error) {
	return m.StatFunc(ctx, path)
}
func (m *MockStepConnector) LookPath(ctx context.Context, file string) (string, error) {
	return m.LookPathFunc(ctx, file)
}
func (m *MockStepConnector) GetOS(ctx context.Context) (*connector.OS, error) { return m.GetOSFunc(ctx) }
func (m *MockStepConnector) IsConnected() bool { return true }
func (m *MockStepConnector) Close() error      { return nil }

var _ connector.Connector = &MockStepConnector{}

// mockStepTestContext implements step.StepContext for testing steps.
type mockStepTestContext struct {
	goCtx         context.Context
	logger        *logger.Logger
	currentHost   connector.Host
	mockConnector connector.Connector // The connector for the current host
	runner        runner.Runner       // Runner configured with the mockConnector and facts
	clusterConfig *v1alpha1.Cluster
	facts         *runner.Facts // Facts for the currentHost

	// Caches
	internalStepCache   cache.Cache
	internalTaskCache   cache.Cache
	internalModuleCache cache.Cache

	// Global settings
	globalWorkDir           string
	verbose                 bool
	ignoreErr               bool
	globalConnectionTimeout time.Duration
	clusterArtifactsDir     string
}

func newTestContextForStep(t *testing.T, currentHostName string, mockConn connector.Connector, facts *runner.Facts, clusterCfg *v1alpha1.Cluster) StepContext {
	t.Helper()

	logInst, _ := logger.New(logger.DefaultConfig())
	logInst.SetLogLevel(logger.DebugLevel) // Or a configurable level for tests

	if currentHostName == "" {
		currentHostName = "test-step-host"
	}

	hostSpec := v1alpha1.HostSpec{Name: currentHostName, Address: "127.0.0.1", Type: "ssh", User: "test", Port: 22}
	currentHost := connector.NewHostFromSpec(hostSpec)

	if mockConn == nil {
		mockConn = NewMockStepConnector()
	}

	defaultFacts := &runner.Facts{
		OS:          &connector.OS{ID: "linux", Arch: "amd64", VersionID: "test-os"},
		Hostname:    currentHostName,
		TotalCPU:    2,
		TotalMemory: 2048,
		Kernel:      "test-kernel",
		IPv4Default: "127.0.0.1",
	}
	if facts != nil {
		defaultFacts = facts
	}

	// Create a runner instance configured with the provided mock connector and facts.
	// This is important because steps call ctx.GetRunner() and expect it to operate on the current host.
	testRunner := runner.Runner{
		Conn:  mockConn,
		Facts: defaultFacts,
	}


	if clusterCfg == nil {
		clusterCfg = &v1alpha1.Cluster{
			ObjectMeta: v1alpha1.ObjectMeta{Name: "test-step-cluster"},
			Spec: v1alpha1.ClusterSpec{
				Hosts: []v1alpha1.HostSpec{hostSpec},
				Global: &v1alpha1.GlobalSpec{WorkDir: "/tmp/_kubexm_step_test_work"},
			},
		}
		v1alpha1.SetDefaults_Cluster(clusterCfg)
	}

	globalWorkDir := "/tmp/_kubexm_step_test_work"
	if clusterCfg.Spec.Global != nil && clusterCfg.Spec.Global.WorkDir != "" {
		globalWorkDir = clusterCfg.Spec.Global.WorkDir
	}


	return &mockStepTestContext{
		goCtx:                   context.Background(),
		logger:                  logInst,
		currentHost:             currentHost,
		mockConnector:           mockConn,
		runner:                  testRunner,
		clusterConfig:           clusterCfg,
		facts:                   defaultFacts,
		internalStepCache:       cache.NewMemoryCache(),
		internalTaskCache:       cache.NewMemoryCache(),
		internalModuleCache:     cache.NewMemoryCache(),
		globalWorkDir:           globalWorkDir,
		clusterArtifactsDir:     filepath.Join(globalWorkDir, clusterCfg.Name),
		verbose:                 true, // Default for tests, can be configured
		globalConnectionTimeout: 30 * time.Second,
	}
}

// Implement step.StepContext for mockStepTestContext
func (m *mockStepTestContext) GoContext() context.Context          { return m.goCtx }
func (m *mockStepTestContext) GetLogger() *logger.Logger           { return m.logger }
func (m *mockStepTestContext) GetHost() connector.Host           { return m.currentHost }
func (m *mockStepTestContext) GetRunner() runner.Runner          { return m.runner }
func (m *mockStepTestContext) GetClusterConfig() *v1alpha1.Cluster { return m.clusterConfig }
func (m *mockStepTestContext) StepCache() cache.StepCache     { return m.internalStepCache }
func (m *mockStepTestContext) TaskCache() cache.TaskCache     { return m.internalTaskCache }
func (m *mockStepTestContext) ModuleCache() cache.ModuleCache   { return m.internalModuleCache }

func (m *mockStepTestContext) GetHostsByRole(role string) ([]connector.Host, error) {
	var hosts []connector.Host
	if m.clusterConfig != nil {
		for _, hSpec := range m.clusterConfig.Spec.Hosts {
			for _, r := range hSpec.Roles {
				if r == role {
					hosts = append(hosts, connector.NewHostFromSpec(hSpec))
					break
				}
			}
		}
	}
	// For step tests, often we only care about the current host or a control node.
	// This mock can be enhanced if tests need more complex role lookups.
	if len(hosts) == 0 {
		currentRoles := m.currentHost.GetRoles()
		if len(currentRoles) > 0 { // Check if currentHost has roles
			for _, r := range currentRoles { // Check all roles of currentHost
				if r == role {
					return []connector.Host{m.currentHost}, nil
				}
			}
		}
	}
	// If still no hosts found and role is ControlNodeRole, return a default control node
	if len(hosts) == 0 && role == common.ControlNodeRole {
		defaultCtrlNodeSpec := v1alpha1.HostSpec{Name: common.ControlNodeHostName, Address:"127.0.0.1", Type:"local", Roles: []string{common.ControlNodeRole}}
		return []connector.Host{connector.NewHostFromSpec(defaultCtrlNodeSpec)}, nil
	}
	return hosts, nil
}
func (m *mockStepTestContext) GetHostFacts(host connector.Host) (*runner.Facts, error) {
	if host.GetName() == m.currentHost.GetName() {
		return m.facts, nil
	}
	// For other hosts, return generic facts or error, depending on test needs
	return &runner.Facts{OS: &connector.OS{Arch: "amd64"}}, nil
}
func (m *mockStepTestContext) GetCurrentHostFacts() (*runner.Facts, error)             { return m.facts, nil }
func (m *mockStepTestContext) GetConnectorForHost(host connector.Host) (connector.Connector, error) {
	if host.GetName() == m.currentHost.GetName() {
		return m.mockConnector, nil
	}
	// For other hosts, return a new mock connector or error
	return NewMockStepConnector(), nil
}
func (m *mockStepTestContext) GetCurrentHostConnector() (connector.Connector, error) { return m.mockConnector, nil }
func (m *mockStepTestContext) GetControlNode() (connector.Host, error) {
	// Find or return a default control node for tests
	if m.clusterConfig != nil && m.clusterConfig.Spec.Hosts != nil {
		for _, hSpec := range m.clusterConfig.Spec.Hosts {
			for _, r := range hSpec.Roles {
				// This logic for finding control node might need to align with how runtime.Context does it
				if r == common.ControlNodeRole || r == common.MasterRole { // Use common constants
					return connector.NewHostFromSpec(hSpec), nil
				}
			}
		}
	}
	// Fallback to a default mock control node if not found in config or config is minimal
	// This ensures GetControlNode() always returns a valid host for tests not focused on multi-node setup.
	defaultCtrlNodeSpec := v1alpha1.HostSpec{Name: "mock-control-node", Address:"127.0.0.1", Type:"local", Roles: []string{common.ControlNodeRole}}
	return connector.NewHostFromSpec(defaultCtrlNodeSpec), nil
}
func (m *mockStepTestContext) GetGlobalWorkDir() string                      { return m.globalWorkDir }
func (m *mockStepTestContext) IsVerbose() bool                             { return m.verbose }
func (m *mockStepTestContext) ShouldIgnoreErr() bool                       { return m.ignoreErr }
func (m *mockStepTestContext) GetGlobalConnectionTimeout() time.Duration       { return m.globalConnectionTimeout }
func (m *mockStepTestContext) GetClusterArtifactsDir() string                { return m.clusterArtifactsDir }
func (m *mockStepTestContext) GetCertsDir() string                         { return filepath.Join(m.GetClusterArtifactsDir(), "certs") }
func (m *mockStepTestContext) GetEtcdCertsDir() string                     { return filepath.Join(m.GetCertsDir(), "etcd") }
func (m *mockStepTestContext) GetComponentArtifactsDir(name string) string   { return filepath.Join(m.GetClusterArtifactsDir(), name) }
func (m *mockStepTestContext) GetEtcdArtifactsDir() string                 { return m.GetComponentArtifactsDir("etcd") }
func (m *mockStepTestContext) GetContainerRuntimeArtifactsDir() string       { return m.GetComponentArtifactsDir("container_runtime") }
func (m *mockStepTestContext) GetKubernetesArtifactsDir() string             { return m.GetComponentArtifactsDir("kubernetes") }
func (m *mockStepTestContext) GetFileDownloadPath(c, v, a, f string) string      { return filepath.Join(m.GetComponentArtifactsDir(c), v, a, f) }
func (m *mockStepTestContext) GetHostDir(hostname string) string               { return filepath.Join(m.GetGlobalWorkDir(), hostname) }
func (m *mockStepTestContext) WithGoContext(gCtx context.Context) StepContext {
	newCtx := *m
	newCtx.goCtx = gCtx
	return &newCtx
}

var _ StepContext = (*mockStepTestContext)(nil) // Ensure interface is implemented
