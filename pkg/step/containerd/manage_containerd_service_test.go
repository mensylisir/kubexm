package containerd

import (
	"context"
	"fmt"
	"strings"
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
	"github.com/mensylisir/kubexm/pkg/step"
)

// mockStepContextForManageService is a helper to create a StepContext for testing.
func mockStepContextForManageService(t *testing.T, mockRunner runner.Runner, hostName string) step.StepContext {
	t.Helper()
	l, _ := logger.New(logger.DefaultOptions())
	mainCtx := &runtime.Context{
		GoCtx:         context.Background(),
		Logger:        l,
		ClusterConfig: &v1alpha1.Cluster{ObjectMeta: v1alpha1.ObjectMeta{Name: "test-cluster-manage-svc"}},
		Runner:        mockRunner,
		StepCache:     cache.NewStepCache(),
		GlobalWorkDir: "/tmp/kubexm_manage_svc_test",
		hostInfoMap:   make(map[string]*runtime.HostRuntimeInfo),
	}

	if hostName == "" {
		hostName = "test-host-manage-svc"
	}
	hostSpec := v1alpha1.HostSpec{Name: hostName, Address: "127.0.0.1", Type: "local"}
	currentHost := connector.NewHostFromSpec(hostSpec)
	mainCtx.GetHostInfoMap()[hostName] = &runtime.HostRuntimeInfo{
		Host:  currentHost,
		Conn:  &connector.LocalConnector{},
		Facts: &runner.Facts{OS: &connector.OS{ID: "linux"}, InitSystem: &runner.ServiceInfo{Type: runner.InitSystemSystemd}}, // Assume systemd
	}
	mainCtx.SetCurrentHost(currentHost)
	mainCtx.SetControlNode(currentHost)
	return mainCtx
}

// mockRunnerForManageService provides a mock implementation of runner.Runner.
type mockRunnerForManageService struct {
	runner.Runner
	IsServiceActiveFunc  func(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) (bool, error)
	StartServiceFunc     func(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error
	StopServiceFunc      func(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error
	RestartServiceFunc   func(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error
	EnableServiceFunc    func(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error
	DisableServiceFunc   func(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error
	DaemonReloadFunc     func(ctx context.Context, conn connector.Connector, facts *runner.Facts) error
	RunFunc              func(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (string, error) // For is-enabled and reload
}

func (m *mockRunnerForManageService) IsServiceActive(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) (bool, error) {
	if m.IsServiceActiveFunc != nil {
		return m.IsServiceActiveFunc(ctx, conn, facts, serviceName)
	}
	return false, fmt.Errorf("IsServiceActiveFunc not implemented")
}
func (m *mockRunnerForManageService) StartService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error {
	if m.StartServiceFunc != nil {
		return m.StartServiceFunc(ctx, conn, facts, serviceName)
	}
	return fmt.Errorf("StartServiceFunc not implemented")
}
func (m *mockRunnerForManageService) StopService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error {
	if m.StopServiceFunc != nil {
		return m.StopServiceFunc(ctx, conn, facts, serviceName)
	}
	return fmt.Errorf("StopServiceFunc not implemented")
}
func (m *mockRunnerForManageService) RestartService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error {
	if m.RestartServiceFunc != nil {
		return m.RestartServiceFunc(ctx, conn, facts, serviceName)
	}
	return fmt.Errorf("RestartServiceFunc not implemented")
}
func (m *mockRunnerForManageService) EnableService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error {
	if m.EnableServiceFunc != nil {
		return m.EnableServiceFunc(ctx, conn, facts, serviceName)
	}
	return fmt.Errorf("EnableServiceFunc not implemented")
}
func (m *mockRunnerForManageService) DisableService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error {
	if m.DisableServiceFunc != nil {
		return m.DisableServiceFunc(ctx, conn, facts, serviceName)
	}
	return fmt.Errorf("DisableServiceFunc not implemented")
}
func (m *mockRunnerForManageService) DaemonReload(ctx context.Context, conn connector.Connector, facts *runner.Facts) error {
	if m.DaemonReloadFunc != nil {
		return m.DaemonReloadFunc(ctx, conn, facts)
	}
	return fmt.Errorf("DaemonReloadFunc not implemented")
}
func (m *mockRunnerForManageService) Run(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (string, error) {
	if m.RunFunc != nil {
		return m.RunFunc(ctx, conn, cmd, sudo)
	}
	return "", fmt.Errorf("RunFunc not implemented")
}


func TestManageContainerdServiceStep_New(t *testing.T) {
	s := NewManageContainerdServiceStep("TestManage", ServiceActionStart, true)
	require.NotNil(t, s)
	meta := s.Meta()
	assert.Equal(t, "TestManage", meta.Name)
	assert.Equal(t, fmt.Sprintf("Performs action '%s' on service '%s'.", ServiceActionStart, containerdServiceName), meta.Description)

	mcss, ok := s.(*ManageContainerdServiceStep)
	require.True(t, ok)
	assert.Equal(t, ServiceActionStart, mcss.Action)
	assert.Equal(t, containerdServiceName, mcss.ServiceName)
	assert.True(t, mcss.Sudo)

	sDefaults := NewManageContainerdServiceStep("", ServiceActionEnable, false)
	mcssDefaults, _ := sDefaults.(*ManageContainerdServiceStep)
	assert.Equal(t, "Enable containerd service", mcssDefaults.Meta().Name) // Default name format
	assert.False(t, mcssDefaults.Sudo)
}

func TestManageContainerdServiceStep_Run_Start_Success(t *testing.T) {
	mockRunner := &mockRunnerForManageService{}
	mockCtx := mockStepContextForManageService(t, mockRunner, "host-start-svc")
	s := NewManageContainerdServiceStep("", ServiceActionStart, true).(*ManageContainerdServiceStep)

	var startCalled, isActiveCalledCount int
	mockRunner.StartServiceFunc = func(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error {
		assert.Equal(t, containerdServiceName, serviceName)
		startCalled++
		return nil
	}
	mockRunner.IsServiceActiveFunc = func(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) (bool, error) {
		assert.Equal(t, containerdServiceName, serviceName)
		isActiveCalledCount++
		return true, nil // Simulate active after start
	}

	err := s.Run(mockCtx, mockCtx.GetHost())
	require.NoError(t, err)
	assert.Equal(t, 1, startCalled, "StartService should be called once")
	assert.Equal(t, 1, isActiveCalledCount, "IsServiceActive should be called once after start")
}

func TestManageContainerdServiceStep_Run_Enable_Success(t *testing.T) {
	mockRunner := &mockRunnerForManageService{}
	mockCtx := mockStepContextForManageService(t, mockRunner, "host-enable-svc")
	s := NewManageContainerdServiceStep("", ServiceActionEnable, true).(*ManageContainerdServiceStep)

	var enableCalled bool
	mockRunner.EnableServiceFunc = func(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error {
		assert.Equal(t, containerdServiceName, serviceName)
		enableCalled = true
		return nil
	}

	err := s.Run(mockCtx, mockCtx.GetHost())
	require.NoError(t, err)
	assert.True(t, enableCalled, "EnableService should be called")
}

func TestManageContainerdServiceStep_Precheck_Start_AlreadyActive(t *testing.T) {
	mockRunner := &mockRunnerForManageService{}
	mockCtx := mockStepContextForManageService(t, mockRunner, "host-precheck-active")
	s := NewManageContainerdServiceStep("", ServiceActionStart, true).(*ManageContainerdServiceStep)

	mockRunner.IsServiceActiveFunc = func(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) (bool, error) {
		return true, nil // Service is already active
	}

	done, err := s.Precheck(mockCtx, mockCtx.GetHost())
	require.NoError(t, err)
	assert.True(t, done, "Precheck should be done if service is already active for Start action")
}

func TestManageContainerdServiceStep_Precheck_Enable_RunsAlways(t *testing.T) {
	mockRunner := &mockRunnerForManageService{}
	mockCtx := mockStepContextForManageService(t, mockRunner, "host-precheck-enable")
	s := NewManageContainerdServiceStep("", ServiceActionEnable, true).(*ManageContainerdServiceStep)

	// Mock IsServiceEnabled (via Run) to return "disabled"
	mockRunner.RunFunc = func(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (string, error) {
		if strings.Contains(cmd, "systemctl is-enabled containerd") {
			return "disabled", nil
		}
		return "", fmt.Errorf("unexpected Run call in Precheck for Enable")
	}

	done, err := s.Precheck(mockCtx, mockCtx.GetHost())
	require.NoError(t, err)
	// Precheck for Enable/Disable currently always returns false to let Run handle it.
	assert.False(t, done, "Precheck for Enable action should return false to let Run proceed")
}

func TestManageContainerdServiceStep_Rollback_ForStartAction(t *testing.T) {
	mockRunner := &mockRunnerForManageService{}
	mockCtx := mockStepContextForManageService(t, mockRunner, "host-rollback-start")
	s := NewManageContainerdServiceStep("", ServiceActionStart, true).(*ManageContainerdServiceStep)

	var stopCalled bool
	mockRunner.StopServiceFunc = func(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error {
		assert.Equal(t, containerdServiceName, serviceName)
		stopCalled = true
		return nil
	}

	err := s.Rollback(mockCtx, mockCtx.GetHost())
	require.NoError(t, err)
	assert.True(t, stopCalled, "StopService should be called for rollback of Start action")
}

// Ensure mockRunnerForManageService implements runner.Runner
var _ runner.Runner = (*mockRunnerForManageService)(nil)

// Add dummy implementations for other runner.Runner methods for mockRunnerForManageService
func (m *mockRunnerForManageService) GatherFacts(ctx context.Context, conn connector.Connector) (*runner.Facts, error) { return m.facts, nil } // Provide facts
func (m *mockRunnerForManageService) MustRun(ctx context.Context, conn connector.Connector, cmd string, sudo bool) string { return "" }
func (m *mockRunnerForManageService) Check(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (bool, error) { return false, nil }
func (m *mockRunnerForManageService) RunWithOptions(ctx context.Context, conn connector.Connector, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) { return nil,nil, nil }
func (m *mockRunnerForManageService) Download(ctx context.Context, conn connector.Connector, facts *runner.Facts, url, destPath string, sudo bool) error { return nil }
func (m *mockRunnerForManageService) Extract(ctx context.Context, conn connector.Connector, facts *runner.Facts, archivePath, destDir string, sudo bool) error { return nil }
func (m *mockRunnerForManageService) DownloadAndExtract(ctx context.Context, conn connector.Connector, facts *runner.Facts, url, destDir string, sudo bool) error { return nil }
func (m *mockRunnerForManageService) Exists(ctx context.Context, conn connector.Connector, path string) (bool, error) { return false, nil }
func (m *mockRunnerForManageService) IsDir(ctx context.Context, conn connector.Connector, path string) (bool, error) { return false, nil }
func (m *mockRunnerForManageService) ReadFile(ctx context.Context, conn connector.Connector, path string) ([]byte, error) { return nil, nil }
func (m *mockRunnerForManageService) WriteFile(ctx context.Context, conn connector.Connector, content []byte, destPath, permissions string, sudo bool) error { return nil }
func (m *mockRunnerForManageService) Mkdirp(ctx context.Context, conn connector.Connector, path, permissions string, sudo bool) error { return nil }
func (m *mockRunnerForManageService) Remove(ctx context.Context, conn connector.Connector, path string, sudo bool) error { return nil }
func (m *mockRunnerForManageService) Chmod(ctx context.Context, conn connector.Connector, path string, permissions string, sudo bool) error { return nil }
func (m *mockRunnerForManageService) Chown(ctx context.Context, conn connector.Connector, path, owner, group string, recursive bool) error { return nil }
func (m *mockRunnerForManageService) GetSHA256(ctx context.Context, conn connector.Connector, path string) (string, error) { return "", nil }
func (m *mockRunnerForManageService) LookPath(ctx context.Context, conn connector.Connector, file string) (string, error) { return "", nil }
func (m *mockRunnerForManageService) IsPortOpen(ctx context.Context, conn connector.Connector, facts *runner.Facts, port int) (bool, error) { return false, nil }
func (m *mockRunnerForManageService) WaitForPort(ctx context.Context, conn connector.Connector, facts *runner.Facts, port int, timeout time.Duration) error { return nil }
func (m *mockRunnerForManageService) SetHostname(ctx context.Context, conn connector.Connector, facts *runner.Facts, hostname string) error { return nil }
func (m *mockRunnerForManageService) AddHostEntry(ctx context.Context, conn connector.Connector, ip, fqdn string, hostnames ...string) error { return nil }
func (m *mockRunnerForManageService) InstallPackages(ctx context.Context, conn connector.Connector, facts *runner.Facts, packages ...string) error { return nil }
func (m *mockRunnerForManageService) RemovePackages(ctx context.Context, conn connector.Connector, facts *runner.Facts, packages ...string) error { return nil }
func (m *mockRunnerForManageService) UpdatePackageCache(ctx context.Context, conn connector.Connector, facts *runner.Facts) error { return nil }
func (m *mockRunnerForManageService) IsPackageInstalled(ctx context.Context, conn connector.Connector, facts *runner.Facts, packageName string) (bool, error) { return false, nil }
func (m *mockRunnerForManageService) AddRepository(ctx context.Context, conn connector.Connector, facts *runner.Facts, repoConfig string, isFilePath bool) error { return nil }
func (m *mockRunnerForManageService) Render(ctx context.Context, conn connector.Connector, tmpl *template.Template, data interface{}, destPath, permissions string, sudo bool) error { return nil }
func (m *mockRunnerForManageService) UserExists(ctx context.Context, conn connector.Connector, username string) (bool, error) { return false, nil }
func (m *mockRunnerForManageService) GroupExists(ctx context.Context, conn connector.Connector, groupname string) (bool, error) { return false, nil }
func (m *mockRunnerForManageService) AddUser(ctx context.Context, conn connector.Connector, username, group, shell string, homeDir string, createHome bool, systemUser bool) error { return nil }
func (m *mockRunnerForManageService) AddGroup(ctx context.Context, conn connector.Connector, groupname string, systemGroup bool) error { return nil }
func (m *mockRunnerForManageService) GetPipelineCache() cache.PipelineCache { return nil }

// Add dummy StepContext methods for mockStepContextForManageService
// Many are covered by runtime.Context embedding.
func TestMockContextImplementation_ManageSvc(t *testing.T) {
	var _ step.StepContext = mockStepContextForManageService(t, &mockRunnerForManageService{}, "dummy")
}
