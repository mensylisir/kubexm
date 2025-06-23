package containerd

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/cache"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/logger"
	"github.com/mensylisir/kubexm/pkg/runner"
	"github.com/mensylisir/kubexm/pkg/runtime" // For runtime.Context full
	"github.com/mensylisir/kubexm/pkg/step"
)

// Reusing mockStepContextForContainerdCleanup and mockRunnerForCleanup from cleanup_containerd_config_step_test.go
// as they are in the same package. If they were different, new mocks would be defined here.

func TestCleanupContainerdDataStep_NewCleanupContainerdDataStep(t *testing.T) {
	s := NewCleanupContainerdDataStep("TestCleanupData", "/mnt/containerd_data", true)
	require.NotNil(t, s)
	meta := s.Meta()
	assert.Equal(t, "TestCleanupData", meta.Name)
	assert.Contains(t, meta.Description, "/mnt/containerd_data")

	ccs, ok := s.(*CleanupContainerdDataStep)
	require.True(t, ok)
	assert.Equal(t, "/mnt/containerd_data", ccs.DataDir)
	assert.True(t, ccs.Sudo)

	// Test defaults
	sDefaults := NewCleanupContainerdDataStep("", "", false) // Sudo will be true by default in constructor
	ccsDefaults, _ := sDefaults.(*CleanupContainerdDataStep)
	assert.Equal(t, "CleanupContainerdDataDirectory", ccsDefaults.meta.Name)
	assert.Equal(t, "/var/lib/containerd", ccsDefaults.DataDir)
	assert.True(t, ccsDefaults.Sudo)
}

func TestCleanupContainerdDataStep_Precheck_DirMissing(t *testing.T) {
	mockRunner := &mockRunnerForCleanup{}
	mockCtx := mockStepContextForContainerdCleanup(t, mockRunner, "host1")

	dataDir := "/var/lib/containerd-test-missing"
	s := NewCleanupContainerdDataStep("", dataDir, true).(*CleanupContainerdDataStep)

	mockRunner.ExistsFunc = func(ctx context.Context, conn connector.Connector, path string) (bool, error) {
		if path == dataDir {
			return false, nil // Simulate data dir is missing
		}
		return false, fmt.Errorf("unexpected Exists call for path: %s", path)
	}

	done, err := s.Precheck(mockCtx, mockCtx.GetHost())
	require.NoError(t, err)
	assert.True(t, done, "Precheck should be done if data dir is missing")
}

func TestCleanupContainerdDataStep_Precheck_DirExists(t *testing.T) {
	mockRunner := &mockRunnerForCleanup{}
	mockCtx := mockStepContextForContainerdCleanup(t, mockRunner, "host1")

	dataDir := "/var/lib/containerd-test-exists"
	s := NewCleanupContainerdDataStep("", dataDir, true).(*CleanupContainerdDataStep)

	mockRunner.ExistsFunc = func(ctx context.Context, conn connector.Connector, path string) (bool, error) {
		if path == dataDir {
			return true, nil // Simulate data dir exists
		}
		return false, fmt.Errorf("unexpected Exists call for path: %s", path)
	}

	done, err := s.Precheck(mockCtx, mockCtx.GetHost())
	require.NoError(t, err)
	assert.False(t, done, "Precheck should not be done if data dir exists")
}

func TestCleanupContainerdDataStep_Run_Success_ServiceNotActive(t *testing.T) {
	mockRunner := &mockRunnerForCleanup{}
	mockCtx := mockStepContextForContainerdCleanup(t, mockRunner, "host1")

	dataDir := "/var/lib/containerd-test-run"
	s := NewCleanupContainerdDataStep("", dataDir, true).(*CleanupContainerdDataStep)

	var removedPath string
	mockRunner.IsServiceActiveFunc = func(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) (bool, error) {
		if serviceName == "containerd" {
			return false, nil // Service is not active
		}
		return false, fmt.Errorf("unexpected IsServiceActive call for service: %s", serviceName)
	}
	mockRunner.RemoveFunc = func(ctx context.Context, conn connector.Connector, path string, sudo bool) error {
		assert.True(t, sudo, "Remove should be called with sudo")
		if path == dataDir {
			removedPath = path
			return nil
		}
		return fmt.Errorf("unexpected Remove call for path: %s", path)
	}

	err := s.Run(mockCtx, mockCtx.GetHost())
	require.NoError(t, err)
	assert.Equal(t, dataDir, removedPath, "DataDir should be removed")
}

func TestCleanupContainerdDataStep_Run_Success_ServiceActiveAndStopped(t *testing.T) {
	mockRunner := &mockRunnerForCleanup{}
	mockCtx := mockStepContextForContainerdCleanup(t, mockRunner, "host1")

	dataDir := "/var/lib/containerd-test-run-stop"
	s := NewCleanupContainerdDataStep("", dataDir, true).(*CleanupContainerdDataStep)

	var serviceStopped, pathRemoved bool
	mockRunner.IsServiceActiveFunc = func(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) (bool, error) {
		if serviceName == "containerd" {
			if !serviceStopped { // First call, service is active
				return true, nil
			}
			return false, nil // Subsequent calls, service is stopped
		}
		return false, fmt.Errorf("unexpected IsServiceActive call for service: %s", serviceName)
	}
	mockRunner.RunFunc = func(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (string, error) {
		if cmd == "systemctl stop containerd" && sudo {
			serviceStopped = true
			return "", nil
		}
		return "", fmt.Errorf("unexpected Run call with cmd: %s", cmd)
	}
	mockRunner.RemoveFunc = func(ctx context.Context, conn connector.Connector, path string, sudo bool) error {
		if path == dataDir && sudo {
			pathRemoved = true
			return nil
		}
		return fmt.Errorf("unexpected Remove call for path: %s", path)
	}

	err := s.Run(mockCtx, mockCtx.GetHost())
	require.NoError(t, err)
	assert.True(t, serviceStopped, "Containerd service should have been stopped")
	assert.True(t, pathRemoved, "DataDir should have been removed")
}

func TestCleanupContainerdDataStep_Run_StopServiceFails(t *testing.T) {
	mockRunner := &mockRunnerForCleanup{}
	mockCtx := mockStepContextForContainerdCleanup(t, mockRunner, "host1")

	s := NewCleanupContainerdDataStep("", "/var/lib/containerd", true).(*CleanupContainerdDataStep)
	expectedErr := fmt.Errorf("failed to stop containerd")

	mockRunner.IsServiceActiveFunc = func(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) (bool, error) {
		return true, nil // Service is active
	}
	mockRunner.RunFunc = func(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (string, error) {
		if cmd == "systemctl stop containerd" {
			return "", expectedErr
		}
		return "", fmt.Errorf("unexpected Run call")
	}
	// RemoveFunc should not be called if stop fails
	var removeCalled bool
	mockRunner.RemoveFunc = func(ctx context.Context, conn connector.Connector, path string, sudo bool) error {
		removeCalled = true
		return nil
	}

	err := s.Run(mockCtx, mockCtx.GetHost())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to stop containerd before data dir removal")
	assert.False(t, removeCalled, "Remove should not be called if stopping service fails")
}

func TestCleanupContainerdDataStep_Run_RemoveFails(t *testing.T) {
	mockRunner := &mockRunnerForCleanup{}
	mockCtx := mockStepContextForContainerdCleanup(t, mockRunner, "host1")

	dataDir := "/var/lib/containerd-remove-fail"
	s := NewCleanupContainerdDataStep("", dataDir, true).(*CleanupContainerdDataStep)
	expectedErr := fmt.Errorf("disk full")

	mockRunner.IsServiceActiveFunc = func(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) (bool, error) {
		return false, nil // Service not active
	}
	mockRunner.RemoveFunc = func(ctx context.Context, conn connector.Connector, path string, sudo bool) error {
		if path == dataDir {
			return expectedErr
		}
		return nil
	}

	err := s.Run(mockCtx, mockCtx.GetHost())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to remove containerd data directory")
	assert.Contains(t, err.Error(), expectedErr.Error())
}

func TestCleanupContainerdDataStep_Run_CriticalDirProtection(t *testing.T) {
	mockRunner := &mockRunnerForCleanup{} // Not actually used as it should error before runner calls
	mockCtx := mockStepContextForContainerdCleanup(t, mockRunner, "host1")

	criticalDirs := []string{"/", "/var", "/var/lib", "/etc"}
	for _, criticalDir := range criticalDirs {
		t.Run(fmt.Sprintf("Protect_%s", criticalDir), func(t *testing.T) {
			s := NewCleanupContainerdDataStep("", criticalDir, true).(*CleanupContainerdDataStep)
			err := s.Run(mockCtx, mockCtx.GetHost())
			require.Error(t, err)
			assert.Contains(t, err.Error(), "refusing to remove potentially critical directory")
		})
	}
}

func TestCleanupContainerdDataStep_Rollback(t *testing.T) {
	mockRunner := &mockRunnerForCleanup{}
	mockCtx := mockStepContextForContainerdCleanup(t, mockRunner, "host1")
	s := NewCleanupContainerdDataStep("", "", true).(*CleanupContainerdDataStep)

	err := s.Rollback(mockCtx, mockCtx.GetHost())
	assert.NoError(t, err, "Rollback should be a no-op and not return an error")
}

// Add dummy methods to mockRunnerForCleanup to satisfy runner.Runner
func (m *mockRunnerForCleanup) GatherFacts(ctx context.Context, conn connector.Connector) (*runner.Facts, error) { return nil, nil }
func (m *mockRunnerForCleanup) Run(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (string, error) {
	if m.RunFunc != nil { // Add RunFunc to mockRunnerForCleanup if needed for other tests
		return m.RunFunc(ctx, conn, cmd, sudo)
	}
	return "", nil
}
func (m *mockRunnerForCleanup) MustRun(ctx context.Context, conn connector.Connector, cmd string, sudo bool) string { return "" }
func (m *mockRunnerForCleanup) Check(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (bool, error) { return false, nil }
func (m *mockRunnerForCleanup) RunWithOptions(ctx context.Context, conn connector.Connector, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) { return nil,nil, nil }
func (m *mockRunnerForCleanup) Download(ctx context.Context, conn connector.Connector, facts *runner.Facts, url, destPath string, sudo bool) error { return nil }
func (m *mockRunnerForCleanup) Extract(ctx context.Context, conn connector.Connector, facts *runner.Facts, archivePath, destDir string, sudo bool) error { return nil }
func (m *mockRunnerForCleanup) DownloadAndExtract(ctx context.Context, conn connector.Connector, facts *runner.Facts, url, destDir string, sudo bool) error { return nil }
func (m *mockRunnerForCleanup) IsDir(ctx context.Context, conn connector.Connector, path string) (bool, error) { return false, nil }
func (m *mockRunnerForCleanup) ReadFile(ctx context.Context, conn connector.Connector, path string) ([]byte, error) { return nil, nil }
func (m *mockRunnerForCleanup) WriteFile(ctx context.Context, conn connector.Connector, content []byte, destPath, permissions string, sudo bool) error { return nil }
func (m *mockRunnerForCleanup) Mkdirp(ctx context.Context, conn connector.Connector, path, permissions string, sudo bool) error { return nil }
func (m *mockRunnerForCleanup) Chmod(ctx context.Context, conn connector.Connector, path string, permissions string, sudo bool) error { return nil }
func (m *mockRunnerForCleanup) Chown(ctx context.Context, conn connector.Connector, path, owner, group string, recursive bool) error { return nil }
func (m *mockRunnerForCleanup) GetSHA256(ctx context.Context, conn connector.Connector, path string) (string, error) { return "", nil }
func (m *mockRunnerForCleanup) LookPath(ctx context.Context, conn connector.Connector, file string) (string, error) { return "", nil }
func (m *mockRunnerForCleanup) IsPortOpen(ctx context.Context, conn connector.Connector, facts *runner.Facts, port int) (bool, error) { return false, nil }
func (m *mockRunnerForCleanup) WaitForPort(ctx context.Context, conn connector.Connector, facts *runner.Facts, port int, timeout time.Duration) error { return nil }
func (m *mockRunnerForCleanup) SetHostname(ctx context.Context, conn connector.Connector, facts *runner.Facts, hostname string) error { return nil }
func (m *mockRunnerForCleanup) AddHostEntry(ctx context.Context, conn connector.Connector, ip, fqdn string, hostnames ...string) error { return nil }
func (m *mockRunnerForCleanup) InstallPackages(ctx context.Context, conn connector.Connector, facts *runner.Facts, packages ...string) error { return nil }
func (m *mockRunnerForCleanup) RemovePackages(ctx context.Context, conn connector.Connector, facts *runner.Facts, packages ...string) error { return nil }
func (m *mockRunnerForCleanup) UpdatePackageCache(ctx context.Context, conn connector.Connector, facts *runner.Facts) error { return nil }
func (m *mockRunnerForCleanup) IsPackageInstalled(ctx context.Context, conn connector.Connector, facts *runner.Facts, packageName string) (bool, error) { return false, nil }
func (m *mockRunnerForCleanup) AddRepository(ctx context.Context, conn connector.Connector, facts *runner.Facts, repoConfig string, isFilePath bool) error { return nil }
func (m *mockRunnerForCleanup) StartService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForCleanup) StopService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error {
	// Required for IsServiceActiveFunc interaction in Run
	if m.StopServiceFunc != nil { // Add StopServiceFunc to mockRunnerForCleanup if needed
		return m.StopServiceFunc(ctx, conn, facts, serviceName)
	}
	return nil
}
func (m *mockRunnerForCleanup) RestartService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForCleanup) EnableService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForCleanup) DisableService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForCleanup) IsServiceActive(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) (bool, error) {
	if m.IsServiceActiveFunc != nil {
		return m.IsServiceActiveFunc(ctx, conn, facts, serviceName)
	}
	return false, fmt.Errorf("IsServiceActiveFunc not implemented")
}
func (m *mockRunnerForCleanup) DaemonReload(ctx context.Context, conn connector.Connector, facts *runner.Facts) error { return nil }
func (m *mockRunnerForCleanup) Render(ctx context.Context, conn connector.Connector, tmpl *template.Template, data interface{}, destPath, permissions string, sudo bool) error { return nil }
func (m *mockRunnerForCleanup) UserExists(ctx context.Context, conn connector.Connector, username string) (bool, error) { return false, nil }
func (m *mockRunnerForCleanup) GroupExists(ctx context.Context, conn connector.Connector, groupname string) (bool, error) { return false, nil }
func (m *mockRunnerForCleanup) AddUser(ctx context.Context, conn connector.Connector, username, group, shell string, homeDir string, createHome bool, systemUser bool) error { return nil }
func (m *mockRunnerForCleanup) AddGroup(ctx context.Context, conn connector.Connector, groupname string, systemGroup bool) error { return nil }

// Add missing fields/methods to mockRunnerForCleanup if tests require them
type mockRunnerForCleanup struct {
	runner.Runner
	ExistsFunc          func(ctx context.Context, conn connector.Connector, path string) (bool, error)
	RemoveFunc          func(ctx context.Context, conn connector.Connector, path string, sudo bool) error
	IsServiceActiveFunc func(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) (bool, error)
	RunFunc             func(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (string, error) // For stopping service
	StopServiceFunc     func(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error
}


var _ runner.Runner = (*mockRunnerForCleanup)(nil)

// Ensure mockStepContextForContainerdCleanup implements step.StepContext
var _ step.StepContext = (*mockStepContextForContainerdCleanup)(nil)

// Dummy implementations for the rest of step.StepContext for mockStepContextForContainerdCleanup
// Many are inherited from the embedded runtime.Context.
// Those that are specific to step.StepContext or need overriding are here.
func (m *mockStepContextForContainerdCleanup) GetLogger() *logger.Logger          { return m.mainCtx.GetLogger() }
func (m *mockStepContextForContainerdCleanup) GoContext() context.Context            { return m.mainCtx.GoContext() }
func (m *mockStepContextForContainerdCleanup) GetHost() connector.Host                                      { return m.mainCtx.GetHost() }
func (m *mockStepContextForContainerdCleanup) GetRunner() runner.Runner                                   { return m.mainCtx.GetRunner() }
func (m *mockStepContextForContainerdCleanup) GetClusterConfig() *v1alpha1.Cluster                      { return m.mainCtx.GetClusterConfig() }
func (m *mockStepContextForContainerdCleanup) StepCache() cache.StepCache                               { return m.mainCtx.StepCache() }
func (m *mockStepContextForContainerdCleanup) TaskCache() cache.TaskCache                               { return m.mainCtx.TaskCache() }
func (m *mockStepContextForContainerdCleanup) ModuleCache() cache.ModuleCache                             { return m.mainCtx.ModuleCache() }
func (m *mockStepContextForContainerdCleanup) GetHostsByRole(role string) ([]connector.Host, error)    { return m.mainCtx.GetHostsByRole(role) }
func (m *mockStepContextForContainerdCleanup) GetHostFacts(host connector.Host) (*runner.Facts, error)           { return m.mainCtx.GetHostFacts(host) }
func (m *mockStepContextForContainerdCleanup) GetCurrentHostFacts() (*runner.Facts, error)                  { return m.mainCtx.GetCurrentHostFacts() }
func (m *mockStepContextForContainerdCleanup) GetConnectorForHost(h connector.Host) (connector.Connector, error) { return m.mainCtx.GetConnectorForHost(h) }
func (m *mockStepContextForContainerdCleanup) GetCurrentHostConnector() (connector.Connector, error)        { return m.mainCtx.GetCurrentHostConnector() }
func (m *mockStepContextForContainerdCleanup) GetControlNode() (connector.Host, error)                  { return m.mainCtx.GetControlNode() }
func (m *mockStepContextForContainerdCleanup) GetGlobalWorkDir() string                                   { return m.mainCtx.GetGlobalWorkDir() }
func (m *mockStepContextForContainerdCleanup) IsVerbose() bool                                        { return m.mainCtx.IsVerbose() }
func (m *mockStepContextForContainerdCleanup) ShouldIgnoreErr() bool                                  { return m.mainCtx.ShouldIgnoreErr() }
func (m *mockStepContextForContainerdCleanup) GetGlobalConnectionTimeout() time.Duration                { return m.mainCtx.GetGlobalConnectionTimeout() }
func (m *mockStepContextForContainerdCleanup) GetClusterArtifactsDir() string                         { return m.mainCtx.GetClusterArtifactsDir() }
func (m *mockStepContextForContainerdCleanup) GetCertsDir() string                                    { return m.mainCtx.GetCertsDir() }
func (m *mockStepContextForContainerdCleanup) GetEtcdCertsDir() string                                { return m.mainCtx.GetEtcdCertsDir() }
func (m *mockStepContextForContainerdCleanup) GetComponentArtifactsDir(componentName string) string     { return m.mainCtx.GetComponentArtifactsDir(componentName) }
func (m *mockStepContextForContainerdCleanup) GetEtcdArtifactsDir() string                            { return m.mainCtx.GetEtcdArtifactsDir() }
func (m *mockStepContextForContainerdCleanup) GetContainerRuntimeArtifactsDir() string                { return m.mainCtx.GetContainerRuntimeArtifactsDir() }
func (m *mockStepContextForContainerdCleanup) GetKubernetesArtifactsDir() string                      { return m.mainCtx.GetKubernetesArtifactsDir() }
func (m *mockStepContextForContainerdCleanup) GetFileDownloadPath(c, v, a, f string) string             { return m.mainCtx.GetFileDownloadPath(c,v,a,f) }
func (m *mockStepContextForContainerdCleanup) GetHostDir(hostname string) string                      { return m.mainCtx.GetHostDir(hostname) }
func (m *mockStepContextForContainerdCleanup) WithGoContext(gCtx context.Context) step.StepContext      { return m.mainCtx.WithGoContext(gCtx) }

// This is the actual mock context struct used in these tests.
type mockStepContextForContainerdCleanup struct {
	// mainCtx holds the actual runtime.Context or a mock that implements step.StepContext
	mainCtx step.StepContext // Changed to step.StepContext for direct use
}
