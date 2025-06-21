package etcd

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/connector"
	// "github.com/mensylisir/kubexm/pkg/mock" // Removed
	"github.com/mensylisir/kubexm/pkg/logger" // Added for local mock context
	"github.com/mensylisir/kubexm/pkg/runner" // Added for local mock context and runner interface
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/stretchr/testify/assert"
	testmock "github.com/stretchr/testify/mock" // Alias for testify's mock
)

// MockTestRunner is a mock for runner.Runner using testify/mock
type MockTestRunner struct {
	testmock.Mock
	// Store a mock connector if needed, or assume runner calls are self-contained for these tests
	Connector connector.Connector
}

// Implement runner.Runner methods that are used by CopyEtcdBinariesToPathStep
func (m *MockTestRunner) Exists(ctx context.Context, conn connector.Connector, path string) (bool, error) {
	args := m.Called(ctx, conn, path)
	return args.Bool(0), args.Error(1)
}

func (m *MockTestRunner) Mkdirp(ctx context.Context, conn connector.Connector, path, permissions string, sudo bool) error {
	args := m.Called(ctx, conn, path, permissions, sudo)
	return args.Error(0)
}

func (m *MockTestRunner) RunWithOptions(ctx context.Context, conn connector.Connector, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) {
	args := m.Called(ctx, conn, cmd, opts)
	// Handle nil for byte slices if that's how your mock is set up
	var b1 []byte
	if args.Get(0) != nil {
		b1 = args.Get(0).([]byte)
	}
	var b2 []byte
	if args.Get(1) != nil {
		b2 = args.Get(1).([]byte)
	}
	return b1, b2, args.Error(2)
}

func (m *MockTestRunner) Chmod(ctx context.Context, conn connector.Connector, path, permissions string, sudo bool) error {
	args := m.Called(ctx, conn, path, permissions, sudo)
	return args.Error(0)
}

func (m *MockTestRunner) Remove(ctx context.Context, conn connector.Connector, path string, sudo bool) error {
	args := m.Called(ctx, conn, path, sudo)
	return args.Error(0)
}
// Add other runner.Runner methods if they get called, returning nil or default values.
func (m *MockTestRunner) GatherFacts(ctx context.Context, conn connector.Connector) (*runner.Facts, error) { return nil, nil }
func (m *MockTestRunner) Run(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (string, error) { return "", nil }
func (m *MockTestRunner) MustRun(ctx context.Context, conn connector.Connector, cmd string, sudo bool) string { return "" }
func (m *MockTestRunner) Check(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (bool, error) { return false, nil }
func (m *MockTestRunner) Download(ctx context.Context, conn connector.Connector, facts *runner.Facts, url, destPath string, sudo bool) error { return nil }
func (m *MockTestRunner) Extract(ctx context.Context, conn connector.Connector, facts *runner.Facts, archivePath, destDir string, sudo bool) error { return nil }
func (m *MockTestRunner) DownloadAndExtract(ctx context.Context, conn connector.Connector, facts *runner.Facts, url, destDir string, sudo bool) error { return nil }
func (m *MockTestRunner) IsDir(ctx context.Context, conn connector.Connector, path string) (bool, error) { return false, nil }
func (m *MockTestRunner) ReadFile(ctx context.Context, conn connector.Connector, path string) ([]byte, error) { return nil, nil }
func (m *MockTestRunner) WriteFile(ctx context.Context, conn connector.Connector, content []byte, destPath, permissions string, sudo bool) error { return nil }
func (m *MockTestRunner) Chown(ctx context.Context, conn connector.Connector, path, owner, group string, recursive bool) error { return nil }
func (m *MockTestRunner) GetSHA256(ctx context.Context, conn connector.Connector, path string) (string, error) { return "", nil }
func (m *MockTestRunner) LookPath(ctx context.Context, conn connector.Connector, file string) (string, error) { return "", nil }
func (m *MockTestRunner) IsPortOpen(ctx context.Context, conn connector.Connector, facts *runner.Facts, port int) (bool, error) { return false, nil }
func (m *MockTestRunner) WaitForPort(ctx context.Context, conn connector.Connector, facts *runner.Facts, port int, timeout time.Duration) error { return nil }
func (m *MockTestRunner) SetHostname(ctx context.Context, conn connector.Connector, facts *runner.Facts, hostname string) error { return nil }
func (m *MockTestRunner) AddHostEntry(ctx context.Context, conn connector.Connector, ip, fqdn string, hostnames ...string) error { return nil }
func (m *MockTestRunner) InstallPackages(ctx context.Context, conn connector.Connector, facts *runner.Facts, packages ...string) error { return nil }
func (m *MockTestRunner) RemovePackages(ctx context.Context, conn connector.Connector, facts *runner.Facts, packages ...string) error { return nil }
func (m *MockTestRunner) UpdatePackageCache(ctx context.Context, conn connector.Connector, facts *runner.Facts) error { return nil }
func (m *MockTestRunner) IsPackageInstalled(ctx context.Context, conn connector.Connector, facts *runner.Facts, packageName string) (bool, error) { return false, nil }
func (m *MockTestRunner) AddRepository(ctx context.Context, conn connector.Connector, facts *runner.Facts, repoConfig string, isFilePath bool) error { return nil }
func (m *MockTestRunner) StartService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *MockTestRunner) StopService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *MockTestRunner) RestartService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *MockTestRunner) EnableService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *MockTestRunner) DisableService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *MockTestRunner) IsServiceActive(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) (bool, error) { return false, nil }
func (m *MockTestRunner) DaemonReload(ctx context.Context, conn connector.Connector, facts *runner.Facts) error { return nil }
func (m *MockTestRunner) Render(ctx context.Context, conn connector.Connector, tmpl *text.template.Template, data interface{}, destPath, permissions string, sudo bool) error { return nil }
func (m *MockTestRunner) UserExists(ctx context.Context, conn connector.Connector, username string) (bool, error) { return false, nil }
func (m *MockTestRunner) GroupExists(ctx context.Context, conn connector.Connector, groupname string) (bool, error) { return false, nil }
func (m *MockTestRunner) AddUser(ctx context.Context, conn connector.Connector, username, group, shell string, homeDir string, createHome bool, systemUser bool) error { return nil }
func (m *MockTestRunner) AddGroup(ctx context.Context, conn connector.Connector, groupname string, systemGroup bool) error { return nil }


// newTestStepEtcdContext creates a runtime.StepContext for etcd step tests.
func newTestStepEtcdContext(t *testing.T, mockRunner *MockTestRunner, cacheValues map[string]interface{}) runtime.StepContext {
	l, _ := logger.New(logger.DefaultConfig())

	mockConn := &step.MockStepConnector{} // Basic mock connector for the host runtime
	if mockRunner != nil {
		mockRunner.Connector = mockConn // Assign a connector to the runner if runner needs it
	}

	mockHost := connector.NewHostFromSpec(v1alpha1.Host{Name: "test-etcd-host", Address: "1.2.3.4"})

	rtCtx := &runtime.Context{
		GoCtx:  context.Background(),
		Logger: l,
		Runner: mockRunner, // Use the testify mock runner
		HostRuntimes: map[string]*runtime.HostRuntime{
			"test-etcd-host": {
				Host:  mockHost,
				Conn:  mockConn,
				Facts: &runner.Facts{OS: &connector.OS{ID: "linux", Arch: "amd64"}},
			},
		},
		CurrentHost: mockHost,
		TaskCache:   runtime.NewTaskCache(),
	}

	if cacheValues != nil {
		for k, v := range cacheValues {
			rtCtx.TaskCache.Set(k, v)
		}
	}
	return rtCtx // *runtime.Context itself implements runtime.StepContext
}

func TestCopyEtcdBinariesToPathStep_Run_Success(t *testing.T) {
	mockRunner := new(MockTestRunner) // Using testify mock
	mockConn := &step.MockStepConnector{}
	mockRunner.Connector = mockConn


	cache := make(map[string]interface{})
	cache[ExtractedEtcdDirCacheKey] = "/tmp/etcd-extracted"
	stepCtx := newTestStepEtcdContext(t, mockRunner, cache)


	s := NewCopyEtcdBinariesToPathStep(
		"CopyEtcdBinaries",
		ExtractedEtcdDirCacheKey,
		"/usr/local/bin",
		"3.5.9", // Expected version
		true,    // Sudo
		true,    // Remove source
	)

	mockRunner.On("Exists", context.TODO(), mockConn, "/tmp/etcd-extracted/etcd").Return(true, nil)
	mockRunner.On("Exists", context.TODO(), mockConn, "/tmp/etcd-extracted/etcdctl").Return(true, nil)
	mockRunner.On("Mkdirp", context.TODO(), mockConn, "/usr/local/bin", "0755", true).Return(nil)
	// For RunWithOptions used for `cp`, we need to match the ExecOptions carefully.
	// Using testmock.Anything for ExecOptions if exact match is complex or not critical for this test focus.
	mockRunner.On("RunWithOptions", context.TODO(), mockConn, "cp -fp /tmp/etcd-extracted/etcd /usr/local/bin/etcd", testmock.AnythingOfType("*connector.ExecOptions")).Return([]byte("copied etcd"), []byte(""), nil)
	mockRunner.On("Chmod", context.TODO(), mockConn, "/usr/local/bin/etcd", "0755", true).Return(nil)
	mockRunner.On("RunWithOptions", context.TODO(), mockConn, "cp -fp /tmp/etcd-extracted/etcdctl /usr/local/bin/etcdctl", testmock.AnythingOfType("*connector.ExecOptions")).Return([]byte("copied etcdctl"), []byte(""), nil)
	mockRunner.On("Chmod", context.TODO(), mockConn, "/usr/local/bin/etcdctl", "0755", true).Return(nil)
	mockRunner.On("Remove", context.TODO(), mockConn, "/tmp/etcd-extracted", true).Return(nil)


	err := s.Run(stepCtx, stepCtx.GetHost())
	assert.NoError(t, err)
	mockRunner.AssertExpectations(t)
}

func TestCopyEtcdBinariesToPathStep_Precheck_AlreadyInstalled(t *testing.T) {
	mockRunner := new(MockTestRunner)
	mockConn := &step.MockStepConnector{}
	mockRunner.Connector = mockConn
	stepCtx := newTestStepEtcdContext(t, mockRunner, nil)


	s := NewCopyEtcdBinariesToPathStep(
		"CopyEtcdBinaries",
		"", // Not used in precheck directly
		"/usr/local/bin",
		"3.5.9", // Expected version
		true,
		false,
	)

	mockRunner.On("Exists", context.TODO(), mockConn, "/usr/local/bin/etcd").Return(true, nil)
	mockRunner.On("Exists", context.TODO(), mockConn, "/usr/local/bin/etcdctl").Return(true, nil)

	mockRunner.On("RunWithOptions", context.TODO(), mockConn, "/usr/local/bin/etcd --version", testmock.MatchedBy(func(opts *connector.ExecOptions) bool { return !opts.Sudo && opts.Check })).Return([]byte("etcd Version: 3.5.9"), []byte(""), nil)
	mockRunner.On("RunWithOptions", context.TODO(), mockConn, "/usr/local/bin/etcdctl version", testmock.MatchedBy(func(opts *connector.ExecOptions) bool { return !opts.Sudo && opts.Check })).Return([]byte("etcdctl version: 3.5.9"), []byte(""), nil)


	done, err := s.Precheck(stepCtx, stepCtx.GetHost())
	assert.NoError(t, err)
	assert.True(t, done)
	mockRunner.AssertExpectations(t)
}

func TestCopyEtcdBinariesToPathStep_Precheck_NotInstalled(t *testing.T) {
	mockRunner := new(MockTestRunner)
	mockConn := &step.MockStepConnector{}
	mockRunner.Connector = mockConn
	stepCtx := newTestStepEtcdContext(t, mockRunner, nil)


	s := NewCopyEtcdBinariesToPathStep("CopyEtcdBinaries", "", "/usr/local/bin", "3.5.9", true, false)

	mockRunner.On("Exists", context.TODO(), mockConn, "/usr/local/bin/etcd").Return(false, nil)

	done, err := s.Precheck(stepCtx, stepCtx.GetHost())
	assert.NoError(t, err)
	assert.False(t, done)
	mockRunner.AssertCalled(t, "Exists", context.TODO(), mockConn, "/usr/local/bin/etcd")
	mockRunner.AssertNotCalled(t, "Exists", context.TODO(), mockConn, "/usr/local/bin/etcdctl")
}


func TestCopyEtcdBinariesToPathStep_Rollback(t *testing.T) {
	mockRunner := new(MockTestRunner)
	mockConn := &step.MockStepConnector{}
	mockRunner.Connector = mockConn
	stepCtx := newTestStepEtcdContext(t, mockRunner, nil)


	s := NewCopyEtcdBinariesToPathStep("CopyEtcdBinaries", "", "/usr/local/bin", "", true, false)

	mockRunner.On("Remove", context.TODO(), mockConn, "/usr/local/bin/etcd", true).Return(nil)
	mockRunner.On("Remove", context.TODO(), mockConn, "/usr/local/bin/etcdctl", true).Return(nil)

	err := s.Rollback(stepCtx, stepCtx.GetHost())
	assert.NoError(t, err)
	mockRunner.AssertExpectations(t)
}

// Helper to create a basic runtime.Context for step testing (simplified)
// You might need a more sophisticated mock or test helper package for this.
// For now, this is a placeholder.
// A real test setup for runtime.StepContext would involve mocking the parts of runtime.Context
// that StepContext facade exposes (Logger, Runner, ConnectorForHost, HostFacts, GoContext, TaskCache).

// MockHost is a simple mock for connector.Host
type MockHost struct {
	name    string
	address string
	roles   []string
	arch    string
}

func (m *MockHost) GetName() string                 { return m.name }
func (m *MockHost) GetAddress() string              { return m.address }
func (m *MockHost) GetPort() int                    { return 22 }
func (m *MockHost) GetUser() string                 { return "testuser" }
func (m *MockHost) GetRoles() []string              { return m.roles }
func (m *MockHost) GetHostSpec() v1alpha1.HostSpec { return v1alpha1.HostSpec{} } // Adjust as needed
func (m *MockHost) GetArch() string                 { return m.arch } // Added for tests needing arch


// MockStepContext is a simple mock for runtime.StepContext
type MockStepContext struct {
	CtxLogger    *logger.Logger
	CtxRunner    runner.Runner
	HostInfo     connector.Host
	HostFactsVal *runtime.Facts
	GoCtx        context.Context
	Cache        runtime.TaskCache // Using runtime.TaskCache for simplicity
}

func (msc *MockStepContext) GetLogger() *logger.Logger                                     { return msc.CtxLogger }
func (msc *MockStepContext) GetRunner() runner.Runner                                      { return msc.CtxRunner }
func (msc *MockStepContext) GetConnectorForHost(h connector.Host) (connector.Connector, error) {
	if msc.CtxRunner != nil {
		if mr, ok := msc.CtxRunner.(*connector.MockConnector); ok { // Assuming Runner can be a MockConnector for tests
			return mr, nil
		}
	}
	return nil, fmt.Errorf("mock connector not set up in StepContext")
}
func (msc *MockStepContext) GetHostFacts(h connector.Host) (*runtime.Facts, error)         { return msc.HostFactsVal, nil }
func (msc *MockStepContext) GoContext() context.Context                                    { return msc.GoCtx }
func (msc *MockStepContext) TaskCache() runtime.TaskCache                                  { return msc.Cache }
func (msc *MockStepContext) Host() connector.Host                                          { return msc.HostInfo } // Added to satisfy runtime.StepContext if it needs it.
func (msc *MockStepContext) GetClusterConfig() *v1alpha1.Cluster                           { return &v1alpha1.Cluster{} } // Basic mock
func (msc *MockStepContext) GetHostsByRole(role string) ([]connector.Host, error)          { return nil, nil } // Basic mock
func (msc *MockStepContext) GetGlobalWorkDir() string                                      { return "/tmp/kubexm_work" } // Basic mock

func NewMockStepContext(t *testing.T, mockRunner runner.Runner, host connector.Host) *MockStepContext {
	cache := runtime.NewTaskCache()
	cache.Set(ExtractedEtcdDirCacheKey, "/tmp/fake-extracted-etcd") // Default for some tests

	return &MockStepContext{
		CtxLogger: logger.Get().With("test", t.Name()),
		CtxRunner: mockRunner,
		HostInfo:  host,
		HostFactsVal: &runtime.Facts{
			OS:       &connector.OS{ID: "linux", Arch: "amd64", PrettyName: "TestOS"},
			Hostname: host.GetName(),
		},
		GoCtx: context.Background(),
		Cache: cache,
	}
}
