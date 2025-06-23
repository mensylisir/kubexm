package docker

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
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step"
)

// mockStepContextForDockerCleanup is a helper to create a StepContext for testing.
func mockStepContextForDockerCleanup(t *testing.T, mockRunner runner.Runner, hostName string) step.StepContext {
	t.Helper()
	l, _ := logger.New(logger.DefaultOptions())
	mainCtx := &runtime.Context{
		GoCtx:         context.Background(),
		Logger:        l,
		ClusterConfig: &v1alpha1.Cluster{ObjectMeta: v1alpha1.ObjectMeta{Name: "test-cluster-d-cleanup"}},
		Runner:        mockRunner,
		StepCache:     cache.NewStepCache(),
		GlobalWorkDir: "/tmp/kubexm_d_cleanup_test",
		hostInfoMap:   make(map[string]*runtime.HostRuntimeInfo),
	}

	if hostName == "" {
		hostName = "test-host-d-cleanup"
	}
	hostSpec := v1alpha1.HostSpec{Name: hostName, Address: "127.0.0.1", Type: "local"}
	currentHost := connector.NewHostFromSpec(hostSpec)
	mainCtx.GetHostInfoMap()[hostName] = &runtime.HostRuntimeInfo{
		Host:  currentHost,
		Conn:  &connector.LocalConnector{},
		Facts: &runner.Facts{OS: &connector.OS{ID: "linux"}},
	}
	mainCtx.SetCurrentHost(currentHost)
	mainCtx.SetControlNode(currentHost)
	return mainCtx
}

// mockRunnerForDockerCleanup provides a mock implementation of runner.Runner.
// (Can reuse mockRunnerForCleanup from containerd tests if they are in the same package for testing,
// or define a similar one here if in different test packages).
// For this example, assuming a similar mock structure is needed.
type mockRunnerForDockerCleanup struct {
	runner.Runner
	ExistsFunc func(ctx context.Context, conn connector.Connector, path string) (bool, error)
	RemoveFunc func(ctx context.Context, conn connector.Connector, path string, sudo bool) error
}

func (m *mockRunnerForDockerCleanup) Exists(ctx context.Context, conn connector.Connector, path string) (bool, error) {
	if m.ExistsFunc != nil {
		return m.ExistsFunc(ctx, conn, path)
	}
	return false, fmt.Errorf("ExistsFunc not implemented")
}
func (m *mockRunnerForDockerCleanup) Remove(ctx context.Context, conn connector.Connector, path string, sudo bool) error {
	if m.RemoveFunc != nil {
		return m.RemoveFunc(ctx, conn, path, sudo)
	}
	return fmt.Errorf("RemoveFunc not implemented")
}

func TestCleanupDockerConfigStep_New(t *testing.T) {
	s := NewCleanupDockerConfigStep("TestCleanupDocker", "/etc/custom/daemon.json", "/etc/custom_docker_root", "/lib/svc/docker.service", "/lib/sock/docker.socket", true)
	require.NotNil(t, s)
	meta := s.Meta()
	assert.Equal(t, "TestCleanupDocker", meta.Name)
	assert.Contains(t, meta.Description, "/etc/custom/daemon.json")
	assert.Contains(t, meta.Description, "/etc/custom_docker_root")

	ccs, ok := s.(*CleanupDockerConfigStep)
	require.True(t, ok)
	assert.Equal(t, "/etc/custom/daemon.json", ccs.DaemonJSONPath)
	assert.Equal(t, "/etc/custom_docker_root", ccs.DockerRootDir)
	assert.True(t, ccs.Sudo)

	sDefaults := NewCleanupDockerConfigStep("", "", "", "", "", false) // Sudo true is default in constructor
	ccsDefaults, _ := sDefaults.(*CleanupDockerConfigStep)
	assert.Equal(t, "CleanupDockerConfiguration", ccsDefaults.meta.Name)
	assert.Equal(t, DefaultDockerDaemonJSONPath, ccsDefaults.DaemonJSONPath)
	assert.Equal(t, "", ccsDefaults.DockerRootDir) // Default is empty
	assert.True(t, ccsDefaults.Sudo)
}

func TestCleanupDockerConfigStep_Precheck_AllMissing(t *testing.T) {
	mockRunner := &mockRunnerForDockerCleanup{}
	mockCtx := mockStepContextForDockerCleanup(t, mockRunner, "host1")
	s := NewCleanupDockerConfigStep("", "/d.json", "/d_root", "/svc.service", "/sock.socket", true).(*CleanupDockerConfigStep)

	mockRunner.ExistsFunc = func(ctx context.Context, conn connector.Connector, path string) (bool, error) {
		return false, nil // All paths missing
	}
	done, err := s.Precheck(mockCtx, mockCtx.GetHost())
	require.NoError(t, err)
	assert.True(t, done)
}

func TestCleanupDockerConfigStep_Run_Success(t *testing.T) {
	mockRunner := &mockRunnerForDockerCleanup{}
	mockCtx := mockStepContextForDockerCleanup(t, mockRunner, "host1")

	daemonPath := "/etc/docker/daemon.json"
	rootDir := "/etc/docker"
	servicePath := "/lib/systemd/system/docker.service"
	socketPath := "/run/docker.sock" // Example, might be different

	s := NewCleanupDockerConfigStep("", daemonPath, rootDir, servicePath, socketPath, true).(*CleanupDockerConfigStep)

	removedPaths := make(map[string]bool)
	mockRunner.RemoveFunc = func(ctx context.Context, conn connector.Connector, path string, sudo bool) error {
		assert.True(t, sudo)
		removedPaths[path] = true
		return nil
	}

	err := s.Run(mockCtx, mockCtx.GetHost())
	require.NoError(t, err)

	assert.True(t, removedPaths[daemonPath])
	assert.True(t, removedPaths[rootDir])
	assert.True(t, removedPaths[servicePath])
	assert.True(t, removedPaths[socketPath])
}

func TestCleanupDockerConfigStep_Run_SkipOptionalEmptyPaths(t *testing.T) {
	mockRunner := &mockRunnerForDockerCleanup{}
	mockCtx := mockStepContextForDockerCleanup(t, mockRunner, "host1")

	daemonPath := "/etc/docker/daemon.json"
	// DockerRootDir, ServiceFilePath, SocketFilePath are empty
	s := NewCleanupDockerConfigStep("", daemonPath, "", "", "", true).(*CleanupDockerConfigStep)

	removedPaths := make(map[string]bool)
	mockRunner.RemoveFunc = func(ctx context.Context, conn connector.Connector, path string, sudo bool) error {
		removedPaths[path] = true
		return nil
	}

	err := s.Run(mockCtx, mockCtx.GetHost())
	require.NoError(t, err)
	assert.True(t, removedPaths[daemonPath])
	assert.Len(t, removedPaths, 1, "Only daemon.json should be attempted for removal")
}


// Ensure mockRunnerForDockerCleanup implements runner.Runner
var _ runner.Runner = (*mockRunnerForDockerCleanup)(nil)
// Ensure mockStepContextForDockerCleanup implements step.StepContext
var _ step.StepContext = (*mockStepContextForDockerCleanup)(t, nil, "")

// Add dummy implementations for other runner.Runner methods for mockRunnerForDockerCleanup
func (m *mockRunnerForDockerCleanup) GatherFacts(ctx context.Context, conn connector.Connector) (*runner.Facts, error) { return nil, nil }
func (m *mockRunnerForDockerCleanup) Run(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (string, error) { return "", nil }
func (m *mockRunnerForDockerCleanup) MustRun(ctx context.Context, conn connector.Connector, cmd string, sudo bool) string { return "" }
func (m *mockRunnerForDockerCleanup) Check(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (bool, error) { return false, nil }
func (m *mockRunnerForDockerCleanup) RunWithOptions(ctx context.Context, conn connector.Connector, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) { return nil,nil, nil }
func (m *mockRunnerForDockerCleanup) Download(ctx context.Context, conn connector.Connector, facts *runner.Facts, url, destPath string, sudo bool) error { return nil }
func (m *mockRunnerForDockerCleanup) Extract(ctx context.Context, conn connector.Connector, facts *runner.Facts, archivePath, destDir string, sudo bool) error { return nil }
func (m *mockRunnerForDockerCleanup) DownloadAndExtract(ctx context.Context, conn connector.Connector, facts *runner.Facts, url, destDir string, sudo bool) error { return nil }
func (m *mockRunnerForDockerCleanup) IsDir(ctx context.Context, conn connector.Connector, path string) (bool, error) { return false, nil }
func (m *mockRunnerForDockerCleanup) ReadFile(ctx context.Context, conn connector.Connector, path string) ([]byte, error) { return nil, nil }
func (m *mockRunnerForDockerCleanup) WriteFile(ctx context.Context, conn connector.Connector, content []byte, destPath, permissions string, sudo bool) error { return nil }
func (m *mockRunnerForDockerCleanup) Mkdirp(ctx context.Context, conn connector.Connector, path, permissions string, sudo bool) error { return nil }
func (m *mockRunnerForDockerCleanup) Chmod(ctx context.Context, conn connector.Connector, path string, permissions string, sudo bool) error { return nil }
func (m *mockRunnerForDockerCleanup) Chown(ctx context.Context, conn connector.Connector, path, owner, group string, recursive bool) error { return nil }
func (m *mockRunnerForDockerCleanup) GetSHA256(ctx context.Context, conn connector.Connector, path string) (string, error) { return "", nil }
func (m *mockRunnerForDockerCleanup) LookPath(ctx context.Context, conn connector.Connector, file string) (string, error) { return "", nil }
func (m *mockRunnerForDockerCleanup) IsPortOpen(ctx context.Context, conn connector.Connector, facts *runner.Facts, port int) (bool, error) { return false, nil }
func (m *mockRunnerForDockerCleanup) WaitForPort(ctx context.Context, conn connector.Connector, facts *runner.Facts, port int, timeout time.Duration) error { return nil }
func (m *mockRunnerForDockerCleanup) SetHostname(ctx context.Context, conn connector.Connector, facts *runner.Facts, hostname string) error { return nil }
func (m *mockRunnerForDockerCleanup) AddHostEntry(ctx context.Context, conn connector.Connector, ip, fqdn string, hostnames ...string) error { return nil }
func (m *mockRunnerForDockerCleanup) InstallPackages(ctx context.Context, conn connector.Connector, facts *runner.Facts, packages ...string) error { return nil }
func (m *mockRunnerForDockerCleanup) RemovePackages(ctx context.Context, conn connector.Connector, facts *runner.Facts, packages ...string) error { return nil }
func (m *mockRunnerForDockerCleanup) UpdatePackageCache(ctx context.Context, conn connector.Connector, facts *runner.Facts) error { return nil }
func (m *mockRunnerForDockerCleanup) IsPackageInstalled(ctx context.Context, conn connector.Connector, facts *runner.Facts, packageName string) (bool, error) { return false, nil }
func (m *mockRunnerForDockerCleanup) AddRepository(ctx context.Context, conn connector.Connector, facts *runner.Facts, repoConfig string, isFilePath bool) error { return nil }
func (m *mockRunnerForDockerCleanup) StartService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForDockerCleanup) StopService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForDockerCleanup) RestartService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForDockerCleanup) EnableService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForDockerCleanup) DisableService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForDockerCleanup) IsServiceActive(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) (bool, error) { return false, nil }
func (m *mockRunnerForDockerCleanup) DaemonReload(ctx context.Context, conn connector.Connector, facts *runner.Facts) error { return nil }
func (m *mockRunnerForDockerCleanup) Render(ctx context.Context, conn connector.Connector, tmpl *template.Template, data interface{}, destPath, permissions string, sudo bool) error { return nil }
func (m *mockRunnerForDockerCleanup) UserExists(ctx context.Context, conn connector.Connector, username string) (bool, error) { return false, nil }
func (m *mockRunnerForDockerCleanup) GroupExists(ctx context.Context, conn connector.Connector, groupname string) (bool, error) { return false, nil }
func (m *mockRunnerForDockerCleanup) AddUser(ctx context.Context, conn connector.Connector, username, group, shell string, homeDir string, createHome bool, systemUser bool) error { return nil }
func (m *mockRunnerForDockerCleanup) AddGroup(ctx context.Context, conn connector.Connector, groupname string, systemGroup bool) error { return nil }
func (m *mockRunnerForDockerCleanup) GetPipelineCache() cache.PipelineCache { return nil }

func TestMockContextImplementation_DockerCleanupConfig(t *testing.T) {
	var _ step.StepContext = mockStepContextForDockerCleanup(t, &mockRunnerForDockerCleanup{}, "dummy")
}
