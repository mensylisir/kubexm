package etcd

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/mock" // Assuming a mock package for runtime context
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/stretchr/testify/assert"
)

func newTestStepContext(t *testing.T, mockRunner *mock.MockRunnerHost, cacheValues map[string]interface{}) runtime.StepContext {
	ctx := mock.NewMockRuntimeContext(t) // This needs to be a more complete mock or test setup
	if mockRunner == nil {
		mockRunner = mock.NewMockRunnerHost(t) // Default mock runner
	}

	// Setup a mock host
	mockHost := connector.NewHostFromSpec(v1alpha1.Host{Name: "test-etcd-host", Address: "1.2.3.4"})
	ctx.HostRuntimes = map[string]*runtime.HostRuntime{
		"test-etcd-host": {
			Host:  mockHost,
			Conn:  mockRunner.Connector, // Use connector from mock runner
			Facts: &runtime.Facts{OS: &connector.OS{ID: "linux", Arch: "amd64"}},
		},
	}

	// Populate task cache
	if cacheValues != nil {
		for k, v := range cacheValues {
			ctx.TaskCache().Set(k, v)
		}
	}
    ctx.SetRunner(mockRunner) // Ensure the context uses our mock runner
	return ctx.Step(mockHost) // Return a StepContext for the mockHost
}

func TestCopyEtcdBinariesToPathStep_Run_Success(t *testing.T) {
	mockRunner := mock.NewMockRunnerHost(t)

	cache := make(map[string]interface{})
	cache[ExtractedEtcdDirCacheKey] = "/tmp/etcd-extracted"

	stepCtx := newTestStepContext(t, mockRunner, cache)

	s := NewCopyEtcdBinariesToPathStep(
		"CopyEtcdBinaries",
		ExtractedEtcdDirCacheKey,
		"/usr/local/bin",
		"3.5.9", // Expected version
		true,    // Sudo
		true,    // Remove source
	)

	mockRunner.On("Exists", context.TODO(), mockRunner.Connector, "/tmp/etcd-extracted/etcd").Return(true, nil)
	mockRunner.On("Exists", context.TODO(), mockRunner.Connector, "/tmp/etcd-extracted/etcdctl").Return(true, nil)
	mockRunner.On("Mkdirp", context.TODO(), mockRunner.Connector, "/usr/local/bin", "0755", true).Return(nil)
	mockRunner.On("Run", context.TODO(), mockRunner.Connector, "cp -fp /tmp/etcd-extracted/etcd /usr/local/bin/etcd", true).Return("copied etcd", nil)
	mockRunner.On("Chmod", context.TODO(), mockRunner.Connector, "/usr/local/bin/etcd", "0755", true).Return(nil)
	mockRunner.On("Run", context.TODO(), mockRunner.Connector, "cp -fp /tmp/etcd-extracted/etcdctl /usr/local/bin/etcdctl", true).Return("copied etcdctl", nil)
	mockRunner.On("Chmod", context.TODO(), mockRunner.Connector, "/usr/local/bin/etcdctl", "0755", true).Return(nil)
	mockRunner.On("Remove", context.TODO(), mockRunner.Connector, "/tmp/etcd-extracted", true).Return(nil)

	err := s.Run(stepCtx, stepCtx.Host())
	assert.NoError(t, err)
	mockRunner.AssertExpectations(t)
}

func TestCopyEtcdBinariesToPathStep_Precheck_AlreadyInstalled(t *testing.T) {
	mockRunner := mock.NewMockRunnerHost(t)
	stepCtx := newTestStepContext(t, mockRunner, nil)

	s := NewCopyEtcdBinariesToPathStep(
		"CopyEtcdBinaries",
		"", // Not used in precheck directly
		"/usr/local/bin",
		"3.5.9", // Expected version
		true,
		false,
	).(*CopyEtcdBinariesToPathStep) // Cast to access struct fields if needed, or pass through constructor

	mockRunner.On("Exists", context.TODO(), mockRunner.Connector, "/usr/local/bin/etcd").Return(true, nil)
	mockRunner.On("Exists", context.TODO(), mockRunner.Connector, "/usr/local/bin/etcdctl").Return(true, nil)

	// Mock version checks
	mockRunner.On("RunWithOptions", context.TODO(), mockRunner.Connector, "/usr/local/bin/etcd --version", &connector.ExecOptions{Sudo: false, Check: true}).Return([]byte("etcd Version: 3.5.9"), []byte(""), nil)
	mockRunner.On("RunWithOptions", context.TODO(), mockRunner.Connector, "/usr/local/bin/etcdctl version", &connector.ExecOptions{Sudo: false, Check: true}).Return([]byte("etcdctl version: 3.5.9"), []byte(""), nil)


	done, err := s.Precheck(stepCtx, stepCtx.Host())
	assert.NoError(t, err)
	assert.True(t, done)
	mockRunner.AssertExpectations(t)
}

func TestCopyEtcdBinariesToPathStep_Precheck_NotInstalled(t *testing.T) {
	mockRunner := mock.NewMockRunnerHost(t)
	stepCtx := newTestStepContext(t, mockRunner, nil)

	s := NewCopyEtcdBinariesToPathStep("CopyEtcdBinaries", "", "/usr/local/bin", "3.5.9", true, false)

	mockRunner.On("Exists", context.TODO(), mockRunner.Connector, "/usr/local/bin/etcd").Return(false, nil)
	// No need to mock Exists for etcdctl if the first one returns false

	done, err := s.Precheck(stepCtx, stepCtx.Host())
	assert.NoError(t, err)
	assert.False(t, done)
	mockRunner.AssertExpectations(t) // Ensures Exists was called for etcd
}


func TestCopyEtcdBinariesToPathStep_Rollback(t *testing.T) {
	mockRunner := mock.NewMockRunnerHost(t)
	stepCtx := newTestStepContext(t, mockRunner, nil)

	s := NewCopyEtcdBinariesToPathStep("CopyEtcdBinaries", "", "/usr/local/bin", "", true, false)

	mockRunner.On("Remove", context.TODO(), mockRunner.Connector, "/usr/local/bin/etcd", true).Return(nil)
	mockRunner.On("Remove", context.TODO(), mockRunner.Connector, "/usr/local/bin/etcdctl", true).Return(nil)

	err := s.Rollback(stepCtx, stepCtx.Host())
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
