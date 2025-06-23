package etcd

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

// Reusing mockStepContextForEtcd and mockRunnerForEtcd
// from backup_etcd_step_test.go, adding service management funcs to runner.

type mockRunnerForBootstrapEtcd struct {
	runner.Runner
	IsServiceActiveFunc func(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) (bool, error)
	DaemonReloadFunc    func(ctx context.Context, conn connector.Connector, facts *runner.Facts) error
	EnableServiceFunc   func(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error
	StartServiceFunc    func(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error
	StopServiceFunc     func(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error
	DisableServiceFunc  func(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error
}

func (m *mockRunnerForBootstrapEtcd) IsServiceActive(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) (bool, error) {
	if m.IsServiceActiveFunc != nil { return m.IsServiceActiveFunc(ctx, conn, facts, serviceName) }
	return false, fmt.Errorf("IsServiceActiveFunc not implemented")
}
func (m *mockRunnerForBootstrapEtcd) DaemonReload(ctx context.Context, conn connector.Connector, facts *runner.Facts) error {
	if m.DaemonReloadFunc != nil { return m.DaemonReloadFunc(ctx, conn, facts) }
	return fmt.Errorf("DaemonReloadFunc not implemented")
}
func (m *mockRunnerForBootstrapEtcd) EnableService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error {
	if m.EnableServiceFunc != nil { return m.EnableServiceFunc(ctx, conn, facts, serviceName) }
	return fmt.Errorf("EnableServiceFunc not implemented")
}
func (m *mockRunnerForBootstrapEtcd) StartService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error {
	if m.StartServiceFunc != nil { return m.StartServiceFunc(ctx, conn, facts, serviceName) }
	return fmt.Errorf("StartServiceFunc not implemented")
}
func (m *mockRunnerForBootstrapEtcd) StopService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error {
	if m.StopServiceFunc != nil { return m.StopServiceFunc(ctx, conn, facts, serviceName) }
	return fmt.Errorf("StopServiceFunc not implemented")
}
func (m *mockRunnerForBootstrapEtcd) DisableService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error {
	if m.DisableServiceFunc != nil { return m.DisableServiceFunc(ctx, conn, facts, serviceName) }
	return fmt.Errorf("DisableServiceFunc not implemented")
}


func TestBootstrapFirstEtcdNodeStep_New(t *testing.T) {
	s := NewBootstrapFirstEtcdNodeStep("TestBootstrap", "etcd-custom", true)
	require.NotNil(t, s)
	meta := s.Meta()
	assert.Equal(t, "TestBootstrap", meta.Name)
	assert.Contains(t, meta.Description, "etcd-custom service")

	bfens, ok := s.(*BootstrapFirstEtcdNodeStep)
	require.True(t, ok)
	assert.Equal(t, "etcd-custom", bfens.ServiceName)
	assert.True(t, bfens.Sudo)

	sDefaults := NewBootstrapFirstEtcdNodeStep("", "", false)
	bfensDefaults, _ := sDefaults.(*BootstrapFirstEtcdNodeStep)
	assert.Equal(t, "BootstrapFirstEtcdNode-etcd", bfensDefaults.Meta().Name)
	assert.Equal(t, "etcd", bfensDefaults.ServiceName)
	assert.True(t, bfensDefaults.Sudo) // Default Sudo is true
}

func TestBootstrapFirstEtcdNodeStep_Precheck_ServiceActive(t *testing.T) {
	mockRunner := &mockRunnerForBootstrapEtcd{}
	mockCtx := mockStepContextForEtcd(t, mockRunner, "host-bootstrap-precheck-active")
	s := NewBootstrapFirstEtcdNodeStep("", "etcd", true).(*BootstrapFirstEtcdNodeStep)

	mockRunner.IsServiceActiveFunc = func(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) (bool, error) {
		if serviceName == "etcd" {
			return true, nil // Service is active
		}
		return false, nil
	}

	done, err := s.Precheck(mockCtx, mockCtx.GetHost())
	require.NoError(t, err)
	assert.True(t, done, "Precheck should be done if service is already active")
}

func TestBootstrapFirstEtcdNodeStep_Run_Success(t *testing.T) {
	mockRunner := &mockRunnerForBootstrapEtcd{}
	mockCtx := mockStepContextForEtcd(t, mockRunner, "host-run-bootstrap")
	s := NewBootstrapFirstEtcdNodeStep("", "etcd", true).(*BootstrapFirstEtcdNodeStep)

	var daemonReloadCalled, enableCalled, startCalled bool
	var finalIsActiveCheckCount int

	mockRunner.DaemonReloadFunc = func(ctx context.Context, conn connector.Connector, facts *runner.Facts) error {
		daemonReloadCalled = true
		return nil
	}
	mockRunner.EnableServiceFunc = func(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error {
		if serviceName == "etcd" {
			enableCalled = true
			return nil
		}
		return fmt.Errorf("unexpected service for EnableService: %s", serviceName)
	}
	mockRunner.StartServiceFunc = func(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error {
		if serviceName == "etcd" {
			startCalled = true
			return nil
		}
		return fmt.Errorf("unexpected service for StartService: %s", serviceName)
	}
	// Mock IsServiceActive for the verification call within StartServiceFunc (inside ManageEtcdServiceStep's Run)
	mockRunner.IsServiceActiveFunc = func(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) (bool, error) {
		if serviceName == "etcd" {
			finalIsActiveCheckCount++
			return true, nil // Simulate service becomes active
		}
		return false, nil
	}


	err := s.Run(mockCtx, mockCtx.GetHost())
	require.NoError(t, err)
	assert.True(t, daemonReloadCalled, "DaemonReload should be called")
	assert.True(t, enableCalled, "EnableService should be called")
	assert.True(t, startCalled, "StartService should be called")
	assert.Equal(t, 1, finalIsActiveCheckCount, "IsServiceActive should be called once by the ManageEtcdServiceStep for start action's verification")
}

// Ensure mockRunnerForBootstrapEtcd implements runner.Runner
var _ runner.Runner = (*mockRunnerForBootstrapEtcd)(nil)
// Ensure mockStepContextForEtcd implements step.StepContext
var _ step.StepContext = (*mockStepContextForEtcd)(t, nil, "")

// Add dummy implementations for other runner.Runner methods for mockRunnerForBootstrapEtcd
func (m *mockRunnerForBootstrapEtcd) GatherFacts(ctx context.Context, conn connector.Connector) (*runner.Facts, error) { return m.facts, nil }
func (m *mockRunnerForBootstrapEtcd) Run(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (string, error) { return "", nil }
func (m *mockRunnerForBootstrapEtcd) MustRun(ctx context.Context, conn connector.Connector, cmd string, sudo bool) string { return "" }
func (m *mockRunnerForBootstrapEtcd) Check(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (bool, error) { return false, nil }
func (m *mockRunnerForBootstrapEtcd) RunWithOptions(ctx context.Context, conn connector.Connector, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) { return nil,nil, nil }
func (m *mockRunnerForBootstrapEtcd) Download(ctx context.Context, conn connector.Connector, facts *runner.Facts, url, destPath string, sudo bool) error { return nil }
func (m *mockRunnerForBootstrapEtcd) Extract(ctx context.Context, conn connector.Connector, facts *runner.Facts, archivePath, destDir string, sudo bool) error { return nil }
func (m *mockRunnerForBootstrapEtcd) DownloadAndExtract(ctx context.Context, conn connector.Connector, facts *runner.Facts, url, destDir string, sudo bool) error { return nil }
func (m *mockRunnerForBootstrapEtcd) Exists(ctx context.Context, conn connector.Connector, path string) (bool, error) { return false, nil }
func (m *mockRunnerForBootstrapEtcd) IsDir(ctx context.Context, conn connector.Connector, path string) (bool, error) { return false, nil }
func (m *mockRunnerForBootstrapEtcd) ReadFile(ctx context.Context, conn connector.Connector, path string) ([]byte, error) { return nil, nil }
func (m *mockRunnerForBootstrapEtcd) WriteFile(ctx context.Context, conn connector.Connector, content []byte, destPath, permissions string, sudo bool) error { return nil }
func (m *mockRunnerForBootstrapEtcd) Mkdirp(ctx context.Context, conn connector.Connector, path, permissions string, sudo bool) error { return nil }
func (m *mockRunnerForBootstrapEtcd) Remove(ctx context.Context, conn connector.Connector, path string, sudo bool) error { return nil }
func (m *mockRunnerForBootstrapEtcd) Chmod(ctx context.Context, conn connector.Connector, path string, permissions string, sudo bool) error { return nil }
func (m *mockRunnerForBootstrapEtcd) Chown(ctx context.Context, conn connector.Connector, path, owner, group string, recursive bool) error { return nil }
func (m *mockRunnerForBootstrapEtcd) GetSHA256(ctx context.Context, conn connector.Connector, path string) (string, error) { return "", nil }
func (m *mockRunnerForBootstrapEtcd) LookPath(ctx context.Context, conn connector.Connector, file string) (string, error) { return "", nil }
func (m *mockRunnerForBootstrapEtcd) IsPortOpen(ctx context.Context, conn connector.Connector, facts *runner.Facts, port int) (bool, error) { return false, nil }
func (m *mockRunnerForBootstrapEtcd) WaitForPort(ctx context.Context, conn connector.Connector, facts *runner.Facts, port int, timeout time.Duration) error { return nil }
func (m *mockRunnerForBootstrapEtcd) SetHostname(ctx context.Context, conn connector.Connector, facts *runner.Facts, hostname string) error { return nil }
func (m *mockRunnerForBootstrapEtcd) AddHostEntry(ctx context.Context, conn connector.Connector, ip, fqdn string, hostnames ...string) error { return nil }
func (m *mockRunnerForBootstrapEtcd) InstallPackages(ctx context.Context, conn connector.Connector, facts *runner.Facts, packages ...string) error { return nil }
func (m *mockRunnerForBootstrapEtcd) RemovePackages(ctx context.Context, conn connector.Connector, facts *runner.Facts, packages ...string) error { return nil }
func (m *mockRunnerForBootstrapEtcd) UpdatePackageCache(ctx context.Context, conn connector.Connector, facts *runner.Facts) error { return nil }
func (m *mockRunnerForBootstrapEtcd) IsPackageInstalled(ctx context.Context, conn connector.Connector, facts *runner.Facts, packageName string) (bool, error) { return false, nil }
func (m *mockRunnerForBootstrapEtcd) AddRepository(ctx context.Context, conn connector.Connector, facts *runner.Facts, repoConfig string, isFilePath bool) error { return nil }
func (m *mockRunnerForBootstrapEtcd) RestartService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForBootstrapEtcd) Render(ctx context.Context, conn connector.Connector, tmpl *template.Template, data interface{}, destPath, permissions string, sudo bool) error { return nil }
func (m *mockRunnerForBootstrapEtcd) UserExists(ctx context.Context, conn connector.Connector, username string) (bool, error) { return false, nil }
func (m *mockRunnerForBootstrapEtcd) GroupExists(ctx context.Context, conn connector.Connector, groupname string) (bool, error) { return false, nil }
func (m *mockRunnerForBootstrapEtcd) AddUser(ctx context.Context, conn connector.Connector, username, group, shell string, homeDir string, createHome bool, systemUser bool) error { return nil }
func (m *mockRunnerForBootstrapEtcd) AddGroup(ctx context.Context, conn connector.Connector, groupname string, systemGroup bool) error { return nil }
func (m *mockRunnerForBootstrapEtcd) GetPipelineCache() cache.PipelineCache { return nil }


func TestMockContextImplementation_EtcdBootstrap(t *testing.T) {
	var _ step.StepContext = mockStepContextForEtcd(t, &mockRunnerForBootstrapEtcd{}, "dummy")
}
