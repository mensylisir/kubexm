package docker

import (
	"context"
	"fmt"
	"path/filepath"
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

// Reusing mockStepContextForDockerCleanup and mockRunnerForDockerCleanup
// from earlier docker step tests.

func TestRemoveCriDockerdStep_New(t *testing.T) {
	s := NewRemoveCriDockerdStep("TestRemoveCriD", "/opt/bin", "/opt/systemd", true)
	require.NotNil(t, s)
	meta := s.Meta()
	assert.Equal(t, "TestRemoveCriD", meta.Name)
	assert.Contains(t, meta.Description, "/opt/bin")
	assert.Contains(t, meta.Description, "/opt/systemd")

	rcds, ok := s.(*RemoveCriDockerdStep)
	require.True(t, ok)
	assert.Equal(t, "/opt/bin", rcds.TargetBinaryDir)
	assert.Equal(t, "/opt/systemd", rcds.TargetSystemdDir)
	assert.True(t, rcds.Sudo)

	sDefaults := NewRemoveCriDockerdStep("", "", "", false) // Sudo defaults to true in constructor
	rcdsDefaults, _ := sDefaults.(*RemoveCriDockerdStep)
	assert.Equal(t, "RemoveCriDockerd", rcdsDefaults.Meta().Name)
	assert.Equal(t, "/usr/local/bin", rcdsDefaults.TargetBinaryDir)
	assert.Equal(t, "/etc/systemd/system", rcdsDefaults.TargetSystemdDir)
	assert.True(t, rcdsDefaults.Sudo)
}

func TestRemoveCriDockerdStep_Precheck_AllMissing(t *testing.T) {
	mockRunner := &mockRunnerForDockerCleanup{}
	mockCtx := mockStepContextForDockerCleanup(t, mockRunner, "host-removecrid-precheck-missing")
	s := NewRemoveCriDockerdStep("", "/test/bin", "/test/systemd", true).(*RemoveCriDockerdStep)

	mockRunner.ExistsFunc = func(ctx context.Context, conn connector.Connector, path string) (bool, error) {
		return false, nil // All items missing
	}

	done, err := s.Precheck(mockCtx, mockCtx.GetHost())
	require.NoError(t, err)
	assert.True(t, done, "Precheck should be done if all items are missing")
}

func TestRemoveCriDockerdStep_Run_Success(t *testing.T) {
	mockRunner := &mockRunnerForDockerCleanup{}
	mockCtx := mockStepContextForDockerCleanup(t, mockRunner, "host-run-removecrid")

	binDir := "/test/bin"
	sysdDir := "/test/systemd"
	s := NewRemoveCriDockerdStep("", binDir, sysdDir, true).(*RemoveCriDockerdStep)

	expectedItemsToRemove := []string{
		filepath.Join(binDir, "cri-dockerd"),
		filepath.Join(sysdDir, "cri-dockerd.service"),
		filepath.Join(sysdDir, "cri-dockerd.socket"),
	}
	removedItems := make(map[string]bool)

	mockRunner.RemoveFunc = func(ctx context.Context, conn connector.Connector, path string, sudo bool) error {
		assert.True(t, sudo)
		removedItems[path] = true
		return nil
	}

	err := s.Run(mockCtx, mockCtx.GetHost())
	require.NoError(t, err)

	for _, item := range expectedItemsToRemove {
		assert.True(t, removedItems[item], "Expected item %s to be removed", item)
	}
}

// Ensure mockRunnerForDockerCleanup implements runner.Runner
var _ runner.Runner = (*mockRunnerForDockerCleanup)(nil)
// Ensure mockStepContextForDockerCleanup implements step.StepContext
var _ step.StepContext = (*mockStepContextForDockerCleanup)(t, nil, "")

// Add dummy implementations for other runner.Runner methods for mockRunnerForDockerCleanup
// (Many are already present in cleanup_docker_config_step_test.go)
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


func TestMockContextImplementation_DockerRemoveCriD(t *testing.T) {
	var _ step.StepContext = mockStepContextForDockerCleanup(t, &mockRunnerForDockerCleanup{}, "dummy")
}
