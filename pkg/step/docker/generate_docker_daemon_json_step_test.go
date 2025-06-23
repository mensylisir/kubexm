package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
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
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step"
)

// Reusing mockStepContextForDockerCleanup and mockRunnerForDockerCleanup (adjusting as needed)
// from cleanup_docker_config_step_test.go.

func TestGenerateDockerDaemonJSONStep_New(t *testing.T) {
	cfg := DockerDaemonConfig{RegistryMirrors: []string{"https://mirror.docker.com"}, StorageDriver: "btrfs"}
	s := NewGenerateDockerDaemonJSONStep("TestGenDaemon", cfg, "/custom/daemon.json", true)
	require.NotNil(t, s)
	meta := s.Meta()
	assert.Equal(t, "TestGenDaemon", meta.Name)
	assert.Contains(t, meta.Description, "/custom/daemon.json")

	gddjs, ok := s.(*GenerateDockerDaemonJSONStep)
	require.True(t, ok)
	assert.Equal(t, "/custom/daemon.json", gddjs.ConfigFilePath)
	assert.Equal(t, []string{"https://mirror.docker.com"}, gddjs.Config.RegistryMirrors)
	assert.Equal(t, "btrfs", gddjs.Config.StorageDriver) // User-provided storage driver
	assert.Contains(t, gddjs.Config.ExecOpts, "native.cgroupdriver=systemd", "Default systemd cgroup driver should be added if not present")
	assert.True(t, gddjs.Sudo)

	// Test defaults
	sDefaults := NewGenerateDockerDaemonJSONStep("", DockerDaemonConfig{}, "", false)
	gddjsDefaults, _ := sDefaults.(*GenerateDockerDaemonJSONStep)
	assert.Equal(t, "GenerateDockerDaemonJSON", gddjsDefaults.Meta().Name)
	assert.Equal(t, DefaultDockerDaemonJSONPath, gddjsDefaults.ConfigFilePath)
	assert.Equal(t, "json-file", gddjsDefaults.Config.LogDriver)
	assert.Equal(t, map[string]string{"max-size": "100m"}, gddjsDefaults.Config.LogOpts)
	assert.Contains(t, gddjsDefaults.Config.ExecOpts, "native.cgroupdriver=systemd")
	assert.True(t, gddjsDefaults.Sudo) // Default Sudo is true
}

func TestGenerateDockerDaemonJSONStep_Run_Success(t *testing.T) {
	mockRunner := &mockRunnerForDockerCleanup{} // Reusing from cleanup_docker_config_step_test
	mockCtx := mockStepContextForDockerCleanup(t, mockRunner, "host-run-daemonjson")

	cfgToSet := DockerDaemonConfig{
		RegistryMirrors: []string{"https://my.mirror.one"},
		ExecOpts:        []string{"native.cgroupdriver=systemd", "some-other-opt"},
	}
	configPath := "/etc/docker/test-daemon.json"
	s := NewGenerateDockerDaemonJSONStep("", cfgToSet, configPath, true).(*GenerateDockerDaemonJSONStep)

	var writtenContent string
	var mkdirPath string

	mockRunner.MkdirpFunc = func(ctx context.Context, conn connector.Connector, path, permissions string, sudo bool) error {
		mkdirPath = path
		assert.Equal(t, filepath.Dir(configPath), path)
		assert.True(t, sudo)
		return nil
	}
	mockRunner.WriteFileFunc = func(ctx context.Context, conn connector.Connector, content []byte, destPath, permissions string, sudo bool) error {
		writtenContent = string(content)
		assert.Equal(t, configPath, destPath)
		assert.Equal(t, "0644", permissions)
		assert.True(t, sudo)
		return nil
	}

	err := s.Run(mockCtx, mockCtx.GetHost())
	require.NoError(t, err)
	assert.Equal(t, filepath.Dir(configPath), mkdirPath)

	// Verify written content by unmarshalling and comparing relevant fields
	var writtenCfg DockerDaemonConfig
	errUnmarshal := json.Unmarshal([]byte(writtenContent), &writtenCfg)
	require.NoError(t, errUnmarshal, "Failed to unmarshal written content: %s", writtenContent)

	assert.Equal(t, cfgToSet.RegistryMirrors, writtenCfg.RegistryMirrors)
	assert.ElementsMatch(t, cfgToSet.ExecOpts, writtenCfg.ExecOpts) // Use ElementsMatch for slices where order might not matter
	assert.Equal(t, "json-file", writtenCfg.LogDriver) // Check default
}

func TestGenerateDockerDaemonJSONStep_Precheck_Matches(t *testing.T) {
	mockRunner := &mockRunnerForDockerCleanup{}
	mockCtx := mockStepContextForDockerCleanup(t, mockRunner, "host-precheck-match-daemon")

	cfgToSet := DockerDaemonConfig{StorageDriver: "overlay2"} // ExecOpts with systemd will be added by New
	s := NewGenerateDockerDaemonJSONStep("", cfgToSet, DefaultDockerDaemonJSONPath, true).(*GenerateDockerDaemonJSONStep)

	expectedRendered, _ := s.renderExpectedConfig()

	mockRunner.ExistsFunc = func(ctx context.Context, conn connector.Connector, path string) (bool, error) {
		return true, nil
	}
	mockRunner.ReadFileFunc = func(ctx context.Context, conn connector.Connector, path string) ([]byte, error) {
		return []byte(expectedRendered), nil
	}

	done, err := s.Precheck(mockCtx, mockCtx.GetHost())
	require.NoError(t, err)
	assert.True(t, done, "Precheck should be done if config matches")
}

// Ensure mockRunnerForDockerCleanup implements runner.Runner (if not already covered)
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
func (m *mockRunnerForDockerCleanup) Mkdirp(ctx context.Context, conn connector.Connector, path, permissions string, sudo bool) error {
	if m.MkdirpFunc != nil {
		return m.MkdirpFunc(ctx, conn, path, permissions, sudo)
	}
	return nil
}
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

func TestMockContextImplementation_DockerGenDaemon(t *testing.T) {
	var _ step.StepContext = mockStepContextForDockerCleanup(t, &mockRunnerForDockerCleanup{}, "dummy")
}
