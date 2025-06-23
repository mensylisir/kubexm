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

// Reusing mockStepContextForEtcd and mockRunnerForEtcd from backup_etcd_step_test.go

func TestCleanupEtcdConfigStep_New(t *testing.T) {
	s := NewCleanupEtcdConfigStep("TestCleanupEtcdCfg", "/opt/etcd/conf", "/opt/systemd/etcd.service", true)
	require.NotNil(t, s)
	meta := s.Meta()
	assert.Equal(t, "TestCleanupEtcdCfg", meta.Name)
	assert.Contains(t, meta.Description, "/opt/etcd/conf")
	assert.Contains(t, meta.Description, "/opt/systemd/etcd.service")

	cecs, ok := s.(*CleanupEtcdConfigStep)
	require.True(t, ok)
	assert.Equal(t, "/opt/etcd/conf", cecs.ConfigDir)
	assert.Equal(t, "/opt/systemd/etcd.service", cecs.ServiceFilePath)
	assert.True(t, cecs.Sudo)

	sDefaults := NewCleanupEtcdConfigStep("", "", "", false) // Sudo defaults to true in constructor
	cecsDefaults, _ := sDefaults.(*CleanupEtcdConfigStep)
	assert.Equal(t, "CleanupEtcdConfiguration", cecsDefaults.Meta().Name)
	assert.Equal(t, "/etc/etcd", cecsDefaults.ConfigDir)
	assert.Equal(t, EtcdServiceFileRemotePath, cecsDefaults.ServiceFilePath)
	assert.True(t, cecsDefaults.Sudo)
}

func TestCleanupEtcdConfigStep_Precheck_AllMissing(t *testing.T) {
	mockRunner := &mockRunnerForEtcd{}
	mockCtx := mockStepContextForEtcd(t, mockRunner, "host-cleanup-etcd-precheck-missing")
	s := NewCleanupEtcdConfigStep("", "/test/etcd_cfg", "/test/etcd.service", true).(*CleanupEtcdConfigStep)

	mockRunner.ExistsFunc = func(ctx context.Context, conn connector.Connector, path string) (bool, error) {
		return false, nil // All paths missing
	}

	done, err := s.Precheck(mockCtx, mockCtx.GetHost())
	require.NoError(t, err)
	assert.True(t, done, "Precheck should be done if all items are missing")
}

func TestCleanupEtcdConfigStep_Run_Success(t *testing.T) {
	mockRunner := &mockRunnerForEtcd{}
	mockCtx := mockStepContextForEtcd(t, mockRunner, "host-run-cleanup-etcd")

	configDir := "/data/etcd_config_to_clean"
	serviceFile := "/usr/lib/systemd/system/etcd-custom.service"
	s := NewCleanupEtcdConfigStep("", configDir, serviceFile, true).(*CleanupEtcdConfigStep)

	removedPaths := make(map[string]bool)
	mockRunner.RemoveFunc = func(ctx context.Context, conn connector.Connector, path string, sudo bool) error {
		assert.True(t, sudo, "Remove should be called with sudo for path: %s", path)
		removedPaths[path] = true
		return nil
	}

	err := s.Run(mockCtx, mockCtx.GetHost())
	require.NoError(t, err)

	assert.True(t, removedPaths[configDir], "ConfigDir should be removed")
	assert.True(t, removedPaths[serviceFile], "ServiceFilePath should be removed")
}

// Ensure mockRunnerForEtcd implements runner.Runner
var _ runner.Runner = (*mockRunnerForEtcd)(nil)
// Ensure mockStepContextForEtcd implements step.StepContext
var _ step.StepContext = (*mockStepContextForEtcd)(t, nil, "")

// Add dummy implementations for other runner.Runner methods for mockRunnerForEtcd
// (Many are already present in backup_etcd_step_test.go)
func (m *mockRunnerForEtcd) GatherFacts(ctx context.Context, conn connector.Connector) (*runner.Facts, error) { return nil, nil }
func (m *mockRunnerForEtcd) Run(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (string, error) { return "", nil }
func (m *mockRunnerForEtcd) MustRun(ctx context.Context, conn connector.Connector, cmd string, sudo bool) string { return "" }
func (m *mockRunnerForEtcd) Check(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (bool, error) { return false, nil }
func (m *mockRunnerForEtcd) Download(ctx context.Context, conn connector.Connector, facts *runner.Facts, url, destPath string, sudo bool) error { return nil }
func (m *mockRunnerForEtcd) Extract(ctx context.Context, conn connector.Connector, facts *runner.Facts, archivePath, destDir string, sudo bool) error { return nil }
func (m *mockRunnerForEtcd) DownloadAndExtract(ctx context.Context, conn connector.Connector, facts *runner.Facts, url, destDir string, sudo bool) error { return nil }
func (m *mockRunnerForEtcd) IsDir(ctx context.Context, conn connector.Connector, path string) (bool, error) { return false, nil }
func (m *mockRunnerForEtcd) ReadFile(ctx context.Context, conn connector.Connector, path string) ([]byte, error) { return nil, nil }
func (m *mockRunnerForEtcd) WriteFile(ctx context.Context, conn connector.Connector, content []byte, destPath, permissions string, sudo bool) error { return nil }
func (m *mockRunnerForEtcd) Chmod(ctx context.Context, conn connector.Connector, path string, permissions string, sudo bool) error { return nil }
func (m *mockRunnerForEtcd) Chown(ctx context.Context, conn connector.Connector, path, owner, group string, recursive bool) error { return nil }
func (m *mockRunnerForEtcd) GetSHA256(ctx context.Context, conn connector.Connector, path string) (string, error) { return "", nil }
func (m *mockRunnerForEtcd) IsPortOpen(ctx context.Context, conn connector.Connector, facts *runner.Facts, port int) (bool, error) { return false, nil }
func (m *mockRunnerForEtcd) WaitForPort(ctx context.Context, conn connector.Connector, facts *runner.Facts, port int, timeout time.Duration) error { return nil }
func (m *mockRunnerForEtcd) SetHostname(ctx context.Context, conn connector.Connector, facts *runner.Facts, hostname string) error { return nil }
func (m *mockRunnerForEtcd) AddHostEntry(ctx context.Context, conn connector.Connector, ip, fqdn string, hostnames ...string) error { return nil }
func (m *mockRunnerForEtcd) InstallPackages(ctx context.Context, conn connector.Connector, facts *runner.Facts, packages ...string) error { return nil }
func (m *mockRunnerForEtcd) RemovePackages(ctx context.Context, conn connector.Connector, facts *runner.Facts, packages ...string) error { return nil }
func (m *mockRunnerForEtcd) UpdatePackageCache(ctx context.Context, conn connector.Connector, facts *runner.Facts) error { return nil }
func (m *mockRunnerForEtcd) IsPackageInstalled(ctx context.Context, conn connector.Connector, facts *runner.Facts, packageName string) (bool, error) { return false, nil }
func (m *mockRunnerForEtcd) AddRepository(ctx context.Context, conn connector.Connector, facts *runner.Facts, repoConfig string, isFilePath bool) error { return nil }
func (m *mockRunnerForEtcd) StartService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForEtcd) StopService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForEtcd) RestartService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForEtcd) EnableService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForEtcd) DisableService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForEtcd) IsServiceActive(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) (bool, error) { return false, nil }
func (m *mockRunnerForEtcd) DaemonReload(ctx context.Context, conn connector.Connector, facts *runner.Facts) error { return nil }
func (m *mockRunnerForEtcd) Render(ctx context.Context, conn connector.Connector, tmpl *template.Template, data interface{}, destPath, permissions string, sudo bool) error { return nil }
func (m *mockRunnerForEtcd) UserExists(ctx context.Context, conn connector.Connector, username string) (bool, error) { return false, nil }
func (m *mockRunnerForEtcd) GroupExists(ctx context.Context, conn connector.Connector, groupname string) (bool, error) { return false, nil }
func (m *mockRunnerForEtcd) AddUser(ctx context.Context, conn connector.Connector, username, group, shell string, homeDir string, createHome bool, systemUser bool) error { return nil }
func (m *mockRunnerForEtcd) AddGroup(ctx context.Context, conn connector.Connector, groupname string, systemGroup bool) error { return nil }
func (m *mockRunnerForEtcd) GetPipelineCache() cache.PipelineCache { return nil }


func TestMockContextImplementation_EtcdCleanupConfig(t *testing.T) {
	var _ step.StepContext = mockStepContextForEtcd(t, &mockRunnerForEtcd{}, "dummy")
}
