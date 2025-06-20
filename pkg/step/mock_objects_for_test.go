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
	"github.com/mensylisir/kubexm/pkg/runtime"
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


// newTestContextForStep creates a runtime.Context with a mock runner and connector for testing steps.
// It initializes a runner.Runner with pre-filled Facts to avoid calling runner.NewRunner's
// internal fact-gathering logic during step test setup.
func newTestContextForStep(t *testing.T, mockConn *MockStepConnector, facts *runner.Facts) *runtime.Context {
	t.Helper()
	if mockConn == nil {
		mockConn = NewMockStepConnector()
	}
	if facts == nil { // Provide default facts if none are given
		osInfo, _ := mockConn.GetOS(context.Background()) // Use GetOS from mockConn for consistency
		facts = &runner.Facts{
			OS:          osInfo,
			Hostname:    "test-host-step",
			TotalCPU:    2,
			TotalMemory: 2048, // MiB
			Kernel: "test-kernel",
			IPv4Default: "127.0.0.1",
		}
	}

	// Create a runner.Runner instance directly with the mock connector and pre-defined facts.
	// This bypasses the need to mock the complex runner.NewRunner function for step tests.
	testRunner := &runner.Runner{
		Conn:  mockConn,
		Facts: facts,
	}

	// Setup a minimal runtime.Host and runtime.ClusterRuntime for the context.
	host := &runtime.Host{
		Name:      "test-host-step",
		Address:   "127.0.0.1",
		Connector: mockConn,   // The Host's connector is our mock connector.
		Runner:    testRunner, // The Host's runner uses the mock connector and pre-set facts.
		WorkDir:   "/tmp/kubexms_test_workdir",
	}

	// Ensure global logger is initialized for tests, or use a specific test logger.
	// logger.Init(logger.DefaultOptions()) // Make sure a logger is available.
	// For test isolation, it might be better to pass a specific logger instance.
	testLoggerOpts := logger.DefaultOptions()
	testLoggerOpts.ConsoleOutput = false // Suppress log output during normal test runs
	testLoggerOpts.FileOutput = false
	logInst, _ := logger.NewLogger(testLoggerOpts)


	clusterRt := &runtime.ClusterRuntime{
		Logger: logInst, // Use a controlled logger instance.
		Hosts:  []*runtime.Host{host},
		Inventory: map[string]*runtime.Host{host.Name: host},
		RoleInventory: make(map[string][]*runtime.Host), // Initialize to avoid nil panic
		WorkDir: "/tmp/kubexms_global_work",
	}
	if len(host.Roles) > 0 { // Populate RoleInventory if host has roles
	    for roleName := range host.Roles {
	        clusterRt.RoleInventory[roleName] = append(clusterRt.RoleInventory[roleName], host)
	    }
	}


	return runtime.NewHostContext(context.Background(), host, clusterRt)
}
