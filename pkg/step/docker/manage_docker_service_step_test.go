package docker

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"text/template"
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
	containerdSteps "github.com/mensylisir/kubexm/pkg/step/containerd" // For ServiceAction
)

// Reusing mockRunnerForManageCriD as mockRunnerForManageDocker
type mockRunnerForManageDocker struct {
	runner.Runner
	IsServiceActiveFunc  func(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) (bool, error)
	StartServiceFunc     func(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error
	StopServiceFunc      func(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error
	EnableServiceFunc    func(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error
	DisableServiceFunc   func(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error
	DaemonReloadFunc     func(ctx context.Context, conn connector.Connector, facts *runner.Facts) error
	RunFunc              func(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (string, error)
	facts                *runner.Facts
}
func (m *mockRunnerForManageDocker) IsServiceActive(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) (bool, error) {
	if m.IsServiceActiveFunc != nil { return m.IsServiceActiveFunc(ctx, conn, facts, serviceName) }
	return false, fmt.Errorf("IsServiceActiveFunc not implemented")
}
func (m *mockRunnerForManageDocker) StartService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error {
	if m.StartServiceFunc != nil { return m.StartServiceFunc(ctx, conn, facts, serviceName) }
	return fmt.Errorf("StartServiceFunc not implemented")
}
// ... (implement other funcs similarly or copy from manage_cri_dockerd_service_step_test.go)


func TestManageDockerServiceStep_New(t *testing.T) {
	s := NewManageDockerServiceStep("TestManageDocker", containerdSteps.ServiceActionRestart, true)
	require.NotNil(t, s)
	meta := s.Meta()
	assert.Equal(t, "TestManageDocker", meta.Name)
	assert.Equal(t, fmt.Sprintf("Performs systemctl %s on %s service.", containerdSteps.ServiceActionRestart, DockerServiceName), meta.Description)

	mdss, ok := s.(*ManageDockerServiceStep)
	require.True(t, ok)
	assert.Equal(t, containerdSteps.ServiceActionRestart, mdss.Action)
	assert.Equal(t, DockerServiceName, mdss.ServiceName)
	assert.True(t, mdss.Sudo) // Constructor defaults Sudo to true

	sDefaults := NewManageDockerServiceStep("", containerdSteps.ServiceActionStop, false)
	mdssDefaults, _ := sDefaults.(*ManageDockerServiceStep)
	assert.Equal(t, fmt.Sprintf("ManageDockerService-%s", "Stop"), mdssDefaults.Meta().Name)
	assert.True(t, mdssDefaults.Sudo) // Constructor defaults Sudo to true
}

func TestManageDockerServiceStep_Run_Restart_Success(t *testing.T) {
	mockRunner := &mockRunnerForManageDocker{}
	mockCtx := mockStepContextForDockerCleanup(t, mockRunner, "host-restart-docker")
	s := NewManageDockerServiceStep("", containerdSteps.ServiceActionRestart, true).(*ManageDockerServiceStep)

	var restartCalled bool
	mockRunner.RestartServiceFunc = func(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error {
		assert.Equal(t, DockerServiceName, serviceName)
		restartCalled = true
		return nil
	}

	err := s.Run(mockCtx, mockCtx.GetHost())
	require.NoError(t, err)
	assert.True(t, restartCalled, "RestartService should be called")
}

func TestManageDockerServiceStep_Precheck_Enable_NotEnabled(t *testing.T) {
	mockRunner := &mockRunnerForManageDocker{}
	mockCtx := mockStepContextForDockerCleanup(t, mockRunner, "host-precheck-enable-docker")
	s := NewManageDockerServiceStep("", containerdSteps.ServiceActionEnable, true).(*ManageDockerServiceStep)

	mockRunner.RunFunc = func(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (string, error) {
		if strings.Contains(cmd, "systemctl is-enabled docker") {
			return "disabled", nil // Simulate service is not enabled
		}
		return "", fmt.Errorf("unexpected Run call in Precheck for Enable")
	}

	done, err := s.Precheck(mockCtx, mockCtx.GetHost())
	require.NoError(t, err)
	assert.False(t, done, "Precheck for Enable action should return false if service is not enabled")
}


// Ensure mockRunnerForManageDocker implements runner.Runner
var _ runner.Runner = (*mockRunnerForManageDocker)(nil)
// Ensure mockStepContextForDockerCleanup implements step.StepContext
var _ step.StepContext = (*mockStepContextForDockerCleanup)(t, nil, "")

// Add dummy implementations for other runner.Runner methods for mockRunnerForManageDocker
func (m *mockRunnerForManageDocker) GatherFacts(ctx context.Context, conn connector.Connector) (*runner.Facts, error) { return m.facts, nil }
func (m *mockRunnerForManageDocker) MustRun(ctx context.Context, conn connector.Connector, cmd string, sudo bool) string { return "" }
func (m *mockRunnerForManageDocker) Check(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (bool, error) { return false, nil }
func (m *mockRunnerForManageDocker) RunWithOptions(ctx context.Context, conn connector.Connector, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) { return nil,nil, nil }
func (m *mockRunnerForManageDocker) Download(ctx context.Context, conn connector.Connector, facts *runner.Facts, url, destPath string, sudo bool) error { return nil }
func (m *mockRunnerForManageDocker) Extract(ctx context.Context, conn connector.Connector, facts *runner.Facts, archivePath, destDir string, sudo bool) error { return nil }
func (m *mockRunnerForManageDocker) DownloadAndExtract(ctx context.Context, conn connector.Connector, facts *runner.Facts, url, destDir string, sudo bool) error { return nil }
func (m *mockRunnerForManageDocker) Exists(ctx context.Context, conn connector.Connector, path string) (bool, error) { return false, nil }
func (m *mockRunnerForManageDocker) IsDir(ctx context.Context, conn connector.Connector, path string) (bool, error) { return false, nil }
func (m *mockRunnerForManageDocker) ReadFile(ctx context.Context, conn connector.Connector, path string) ([]byte, error) { return nil, nil }
func (m *mockRunnerForManageDocker) WriteFile(ctx context.Context, conn connector.Connector, content []byte, destPath, permissions string, sudo bool) error { return nil }
func (m *mockRunnerForManageDocker) Mkdirp(ctx context.Context, conn connector.Connector, path, permissions string, sudo bool) error { return nil }
func (m *mockRunnerForManageDocker) Remove(ctx context.Context, conn connector.Connector, path string, sudo bool) error { return nil }
func (m *mockRunnerForManageDocker) Chmod(ctx context.Context, conn connector.Connector, path string, permissions string, sudo bool) error { return nil }
func (m *mockRunnerForManageDocker) Chown(ctx context.Context, conn connector.Connector, path, owner, group string, recursive bool) error { return nil }
func (m *mockRunnerForManageDocker) GetSHA256(ctx context.Context, conn connector.Connector, path string) (string, error) { return "", nil }
func (m *mockRunnerForManageDocker) LookPath(ctx context.Context, conn connector.Connector, file string) (string, error) { return "", nil }
func (m *mockRunnerForManageDocker) IsPortOpen(ctx context.Context, conn connector.Connector, facts *runner.Facts, port int) (bool, error) { return false, nil }
func (m *mockRunnerForManageDocker) WaitForPort(ctx context.Context, conn connector.Connector, facts *runner.Facts, port int, timeout time.Duration) error { return nil }
func (m *mockRunnerForManageDocker) SetHostname(ctx context.Context, conn connector.Connector, facts *runner.Facts, hostname string) error { return nil }
func (m *mockRunnerForManageDocker) AddHostEntry(ctx context.Context, conn connector.Connector, ip, fqdn string, hostnames ...string) error { return nil }
func (m *mockRunnerForManageDocker) InstallPackages(ctx context.Context, conn connector.Connector, facts *runner.Facts, packages ...string) error { return nil }
func (m *mockRunnerForManageDocker) RemovePackages(ctx context.Context, conn connector.Connector, facts *runner.Facts, packages ...string) error { return nil }
func (m *mockRunnerForManageDocker) UpdatePackageCache(ctx context.Context, conn connector.Connector, facts *runner.Facts) error { return nil }
func (m *mockRunnerForManageDocker) IsPackageInstalled(ctx context.Context, conn connector.Connector, facts *runner.Facts, packageName string) (bool, error) { return false, nil }
func (m *mockRunnerForManageDocker) AddRepository(ctx context.Context, conn connector.Connector, facts *runner.Facts, repoConfig string, isFilePath bool) error { return nil }
func (m *mockRunnerForManageDocker) StopService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error {
	if m.StopServiceFunc != nil { return m.StopServiceFunc(ctx, conn, facts, serviceName) }
	return fmt.Errorf("StopServiceFunc not implemented")
}
func (m *mockRunnerForManageDocker) RestartService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error {
	if m.RestartServiceFunc != nil { return m.RestartServiceFunc(ctx, conn, facts, serviceName) }
	return fmt.Errorf("RestartServiceFunc not implemented")
}
func (m *mockRunnerForManageDocker) DisableService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error {
	if m.DisableServiceFunc != nil { return m.DisableServiceFunc(ctx, conn, facts, serviceName) }
	return fmt.Errorf("DisableServiceFunc not implemented")
}
func (m *mockRunnerForManageDocker) DaemonReload(ctx context.Context, conn connector.Connector, facts *runner.Facts) error {
	if m.DaemonReloadFunc != nil { return m.DaemonReloadFunc(ctx, conn, facts) }
	return fmt.Errorf("DaemonReloadFunc not implemented")
}
func (m *mockRunnerForManageDocker) Render(ctx context.Context, conn connector.Connector, tmpl *template.Template, data interface{}, destPath, permissions string, sudo bool) error { return nil }
func (m *mockRunnerForManageDocker) UserExists(ctx context.Context, conn connector.Connector, username string) (bool, error) { return false, nil }
func (m *mockRunnerForManageDocker) GroupExists(ctx context.Context, conn connector.Connector, groupname string) (bool, error) { return false, nil }
func (m *mockRunnerForManageDocker) AddUser(ctx context.Context, conn connector.Connector, username, group, shell string, homeDir string, createHome bool, systemUser bool) error { return nil }
func (m *mockRunnerForManageDocker) AddGroup(ctx context.Context, conn connector.Connector, groupname string, systemGroup bool) error { return nil }
func (m *mockRunnerForManageDocker) GetPipelineCache() cache.PipelineCache { return nil }


func TestMockContextImplementation_DockerManageSvc(t *testing.T) {
	var _ step.StepContext = mockStepContextForDockerCleanup(t, &mockRunnerForManageDocker{}, "dummy")
}
