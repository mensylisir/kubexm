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

// mockRunnerForManageCriD provides a mock implementation of runner.Runner.
type mockRunnerForManageCriD struct {
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
func (m *mockRunnerForManageCriD) IsServiceActive(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) (bool, error) {
	if m.IsServiceActiveFunc != nil { return m.IsServiceActiveFunc(ctx, conn, facts, serviceName) }
	return false, fmt.Errorf("IsServiceActiveFunc not implemented")
}
func (m *mockRunnerForManageCriD) StartService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error {
	if m.StartServiceFunc != nil { return m.StartServiceFunc(ctx, conn, facts, serviceName) }
	return fmt.Errorf("StartServiceFunc not implemented")
}
func (m *mockRunnerForManageCriD) StopService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error {
	if m.StopServiceFunc != nil { return m.StopServiceFunc(ctx, conn, facts, serviceName) }
	return fmt.Errorf("StopServiceFunc not implemented")
}
func (m *mockRunnerForManageCriD) EnableService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error {
	if m.EnableServiceFunc != nil { return m.EnableServiceFunc(ctx, conn, facts, serviceName) }
	return fmt.Errorf("EnableServiceFunc not implemented")
}
func (m *mockRunnerForManageCriD) DisableService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error {
	if m.DisableServiceFunc != nil { return m.DisableServiceFunc(ctx, conn, facts, serviceName) }
	return fmt.Errorf("DisableServiceFunc not implemented")
}
func (m *mockRunnerForManageCriD) DaemonReload(ctx context.Context, conn connector.Connector, facts *runner.Facts) error {
	if m.DaemonReloadFunc != nil { return m.DaemonReloadFunc(ctx, conn, facts) }
	return fmt.Errorf("DaemonReloadFunc not implemented")
}
func (m *mockRunnerForManageCriD) Run(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (string, error) {
	if m.RunFunc != nil { return m.RunFunc(ctx, conn, cmd, sudo) }
	return "", fmt.Errorf("RunFunc not implemented")
}


func TestManageCriDockerdServiceStep_New(t *testing.T) {
	s := NewManageCriDockerdServiceStep("TestManageCriD", containerdSteps.ServiceActionStop, true)
	require.NotNil(t, s)
	meta := s.Meta()
	assert.Equal(t, "TestManageCriD", meta.Name)
	assert.Equal(t, fmt.Sprintf("Performs systemctl %s on %s service.", containerdSteps.ServiceActionStop, CriDockerdServiceName), meta.Description)

	mcdss, ok := s.(*ManageCriDockerdServiceStep)
	require.True(t, ok)
	assert.Equal(t, containerdSteps.ServiceActionStop, mcdss.Action)
	assert.Equal(t, CriDockerdServiceName, mcdss.ServiceName)
	assert.True(t, mcdss.Sudo) // Default Sudo is true in constructor

	sDefaults := NewManageCriDockerdServiceStep("", containerdSteps.ServiceActionRestart, false) // Pass false for sudo
	mcdssDefaults, _ := sDefaults.(*ManageCriDockerdServiceStep)
	assert.Equal(t, fmt.Sprintf("ManageCriDockerdService-%s", "Restart"), mcdssDefaults.Meta().Name)
	assert.True(t, mcdssDefaults.Sudo) // Constructor defaults Sudo to true
}

func TestManageCriDockerdServiceStep_Run_Enable_Success(t *testing.T) {
	mockRunner := &mockRunnerForManageCriD{}
	mockCtx := mockStepContextForDockerCleanup(t, mockRunner, "host-enable-crid")
	s := NewManageCriDockerdServiceStep("", containerdSteps.ServiceActionEnable, true).(*ManageCriDockerdServiceStep)

	var enableServiceCalled, enableSocketCalled bool
	mockRunner.EnableServiceFunc = func(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error {
		if serviceName == CriDockerdServiceName {
			enableServiceCalled = true
			return nil
		}
		if serviceName == strings.Replace(CriDockerdServiceName, ".service", ".socket", 1) || serviceName == "cri-dockerd.socket" {
			enableSocketCalled = true
			return nil
		}
		return fmt.Errorf("unexpected service name for Enable: %s", serviceName)
	}

	err := s.Run(mockCtx, mockCtx.GetHost())
	require.NoError(t, err)
	assert.True(t, enableServiceCalled, "EnableService for cri-dockerd.service should be called")
	assert.True(t, enableSocketCalled, "EnableService for cri-dockerd.socket should also be called")
}

func TestManageCriDockerdServiceStep_Precheck_Stop_NotActive(t *testing.T) {
	mockRunner := &mockRunnerForManageCriD{}
	mockCtx := mockStepContextForDockerCleanup(t, mockRunner, "host-precheck-stop-crid")
	s := NewManageCriDockerdServiceStep("", containerdSteps.ServiceActionStop, true).(*ManageCriDockerdServiceStep)

	mockRunner.IsServiceActiveFunc = func(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) (bool, error) {
		if serviceName == CriDockerdServiceName {
			return false, nil // Service is not active
		}
		return false, fmt.Errorf("unexpected service name for IsServiceActive: %s", serviceName)
	}

	done, err := s.Precheck(mockCtx, mockCtx.GetHost())
	require.NoError(t, err)
	assert.True(t, done, "Precheck should be done if service is already stopped for Stop action")
}

// Ensure mockRunnerForManageCriD implements runner.Runner
var _ runner.Runner = (*mockRunnerForManageCriD)(nil)
// Ensure mockStepContextForDockerCleanup implements step.StepContext
var _ step.StepContext = (*mockStepContextForDockerCleanup)(t, nil, "")

// Add dummy implementations for other runner.Runner methods for mockRunnerForManageCriD
func (m *mockRunnerForManageCriD) GatherFacts(ctx context.Context, conn connector.Connector) (*runner.Facts, error) { return m.facts, nil }
func (m *mockRunnerForManageCriD) MustRun(ctx context.Context, conn connector.Connector, cmd string, sudo bool) string { return "" }
func (m *mockRunnerForManageCriD) Check(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (bool, error) { return false, nil }
func (m *mockRunnerForManageCriD) RunWithOptions(ctx context.Context, conn connector.Connector, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) { return nil,nil, nil }
func (m *mockRunnerForManageCriD) Download(ctx context.Context, conn connector.Connector, facts *runner.Facts, url, destPath string, sudo bool) error { return nil }
func (m *mockRunnerForManageCriD) Extract(ctx context.Context, conn connector.Connector, facts *runner.Facts, archivePath, destDir string, sudo bool) error { return nil }
func (m *mockRunnerForManageCriD) DownloadAndExtract(ctx context.Context, conn connector.Connector, facts *runner.Facts, url, destDir string, sudo bool) error { return nil }
func (m *mockRunnerForManageCriD) Exists(ctx context.Context, conn connector.Connector, path string) (bool, error) { return false, nil }
func (m *mockRunnerForManageCriD) IsDir(ctx context.Context, conn connector.Connector, path string) (bool, error) { return false, nil }
func (m *mockRunnerForManageCriD) ReadFile(ctx context.Context, conn connector.Connector, path string) ([]byte, error) { return nil, nil }
func (m *mockRunnerForManageCriD) WriteFile(ctx context.Context, conn connector.Connector, content []byte, destPath, permissions string, sudo bool) error { return nil }
func (m *mockRunnerForManageCriD) Mkdirp(ctx context.Context, conn connector.Connector, path, permissions string, sudo bool) error { return nil }
func (m *mockRunnerForManageCriD) Remove(ctx context.Context, conn connector.Connector, path string, sudo bool) error { return nil }
func (m *mockRunnerForManageCriD) Chmod(ctx context.Context, conn connector.Connector, path string, permissions string, sudo bool) error { return nil }
func (m *mockRunnerForManageCriD) Chown(ctx context.Context, conn connector.Connector, path, owner, group string, recursive bool) error { return nil }
func (m *mockRunnerForManageCriD) GetSHA256(ctx context.Context, conn connector.Connector, path string) (string, error) { return "", nil }
func (m *mockRunnerForManageCriD) LookPath(ctx context.Context, conn connector.Connector, file string) (string, error) { return "", nil }
func (m *mockRunnerForManageCriD) IsPortOpen(ctx context.Context, conn connector.Connector, facts *runner.Facts, port int) (bool, error) { return false, nil }
func (m *mockRunnerForManageCriD) WaitForPort(ctx context.Context, conn connector.Connector, facts *runner.Facts, port int, timeout time.Duration) error { return nil }
func (m *mockRunnerForManageCriD) SetHostname(ctx context.Context, conn connector.Connector, facts *runner.Facts, hostname string) error { return nil }
func (m *mockRunnerForManageCriD) AddHostEntry(ctx context.Context, conn connector.Connector, ip, fqdn string, hostnames ...string) error { return nil }
func (m *mockRunnerForManageCriD) InstallPackages(ctx context.Context, conn connector.Connector, facts *runner.Facts, packages ...string) error { return nil }
func (m *mockRunnerForManageCriD) RemovePackages(ctx context.Context, conn connector.Connector, facts *runner.Facts, packages ...string) error { return nil }
func (m *mockRunnerForManageCriD) UpdatePackageCache(ctx context.Context, conn connector.Connector, facts *runner.Facts) error { return nil }
func (m *mockRunnerForManageCriD) IsPackageInstalled(ctx context.Context, conn connector.Connector, facts *runner.Facts, packageName string) (bool, error) { return false, nil }
func (m *mockRunnerForManageCriD) AddRepository(ctx context.Context, conn connector.Connector, facts *runner.Facts, repoConfig string, isFilePath bool) error { return nil }
func (m *mockRunnerForManageCriD) RestartService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForManageCriD) Render(ctx context.Context, conn connector.Connector, tmpl *template.Template, data interface{}, destPath, permissions string, sudo bool) error { return nil }
func (m *mockRunnerForManageCriD) UserExists(ctx context.Context, conn connector.Connector, username string) (bool, error) { return false, nil }
func (m *mockRunnerForManageCriD) GroupExists(ctx context.Context, conn connector.Connector, groupname string) (bool, error) { return false, nil }
func (m *mockRunnerForManageCriD) AddUser(ctx context.Context, conn connector.Connector, username, group, shell string, homeDir string, createHome bool, systemUser bool) error { return nil }
func (m *mockRunnerForManageCriD) AddGroup(ctx context.Context, conn connector.Connector, groupname string, systemGroup bool) error { return nil }
func (m *mockRunnerForManageCriD) GetPipelineCache() cache.PipelineCache { return nil }


func TestMockContextImplementation_DockerManageCriDSvc(t *testing.T) {
	var _ step.StepContext = mockStepContextForDockerCleanup(t, &mockRunnerForManageCriD{}, "dummy")
}
