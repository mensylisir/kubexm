package containerd

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
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
	"github.com/mensylisir/kubexm/pkg/step"
)

// mockStepContextForConfigureContainerd is a helper to create a StepContext for testing.
func mockStepContextForConfigureContainerd(t *testing.T, mockRunner runner.Runner, hostName string) step.StepContext {
	t.Helper()
	l, _ := logger.New(logger.DefaultOptions())
	mainCtx := &runtime.Context{
		GoCtx:         context.Background(),
		Logger:        l,
		ClusterConfig: &v1alpha1.Cluster{ObjectMeta: v1alpha1.ObjectMeta{Name: "test-cluster-cfg-containerd"}},
		Runner:        mockRunner,
		StepCache:     cache.NewStepCache(),
		GlobalWorkDir: "/tmp/kubexm_cfg_containerd_test",
		hostInfoMap:   make(map[string]*runtime.HostRuntimeInfo),
	}

	if hostName == "" {
		hostName = "test-host-cfg-containerd"
	}
	hostSpec := v1alpha1.HostSpec{Name: hostName, Address: "127.0.0.1", Type: "local"}
	currentHost := connector.NewHostFromSpec(hostSpec)
	mainCtx.GetHostInfoMap()[hostName] = &runtime.HostRuntimeInfo{
		Host:  currentHost,
		Conn:  &connector.LocalConnector{}, // Or a specific mock if step interacts directly with connector
		Facts: &runner.Facts{OS: &connector.OS{ID: "linux"}},
	}
	mainCtx.SetCurrentHost(currentHost)
	mainCtx.SetControlNode(currentHost) // Assuming this step could run on control or target

	return mainCtx
}

// mockRunnerForConfigureContainerd provides a mock implementation of runner.Runner.
type mockRunnerForConfigureContainerd struct {
	runner.Runner // Embed to satisfy the interface
	ExistsFunc    func(ctx context.Context, conn connector.Connector, path string) (bool, error)
	ReadFileFunc  func(ctx context.Context, conn connector.Connector, path string) ([]byte, error)
	WriteFileFunc func(ctx context.Context, conn connector.Connector, content []byte, destPath, permissions string, sudo bool) error
	MkdirpFunc    func(ctx context.Context, conn connector.Connector, path, permissions string, sudo bool) error
	RemoveFunc    func(ctx context.Context, conn connector.Connector, path string, sudo bool) error
}

func (m *mockRunnerForConfigureContainerd) Exists(ctx context.Context, conn connector.Connector, path string) (bool, error) {
	if m.ExistsFunc != nil {
		return m.ExistsFunc(ctx, conn, path)
	}
	return false, fmt.Errorf("ExistsFunc not implemented")
}
func (m *mockRunnerForConfigureContainerd) ReadFile(ctx context.Context, conn connector.Connector, path string) ([]byte, error) {
	if m.ReadFileFunc != nil {
		return m.ReadFileFunc(ctx, conn, path)
	}
	return nil, fmt.Errorf("ReadFileFunc not implemented")
}
func (m *mockRunnerForConfigureContainerd) WriteFile(ctx context.Context, conn connector.Connector, content []byte, destPath, permissions string, sudo bool) error {
	if m.WriteFileFunc != nil {
		return m.WriteFileFunc(ctx, conn, content, destPath, permissions, sudo)
	}
	return fmt.Errorf("WriteFileFunc not implemented")
}
func (m *mockRunnerForConfigureContainerd) Mkdirp(ctx context.Context, conn connector.Connector, path, permissions string, sudo bool) error {
	if m.MkdirpFunc != nil {
		return m.MkdirpFunc(ctx, conn, path, permissions, sudo)
	}
	return fmt.Errorf("MkdirpFunc not implemented")
}
func (m *mockRunnerForConfigureContainerd) Remove(ctx context.Context, conn connector.Connector, path string, sudo bool) error {
	if m.RemoveFunc != nil {
		return m.RemoveFunc(ctx, conn, path, sudo)
	}
	return fmt.Errorf("RemoveFunc not implemented")
}


func TestConfigureContainerdStep_NewConfigureContainerdStep(t *testing.T) {
	mirrors := map[string]MirrorConfiguration{"docker.io": {Endpoints: []string{"https://mirror.example.com"}}}
	insecure := []string{"my.insecure.reg:5000"}
	s := NewConfigureContainerdStep("TestConfig", "custom/pause:3.8", mirrors, insecure, "/opt/containerd/config.toml", "/opt/containerd/certs.d", "extra", true, true)
	require.NotNil(t, s)
	meta := s.Meta()
	assert.Equal(t, "TestConfig", meta.Name)
	assert.Contains(t, meta.Description, "/opt/containerd/config.toml")

	ccs, ok := s.(*ConfigureContainerdStep)
	require.True(t, ok)
	assert.Equal(t, "custom/pause:3.8", ccs.SandboxImage)
	assert.Equal(t, mirrors, ccs.RegistryMirrors)
	assert.True(t, ccs.UseSystemdCgroup)

	// Test defaults
	sDefaults := NewConfigureContainerdStep("", "", nil, nil, "", "", "", false, false)
	ccsDefaults, _ := sDefaults.(*ConfigureContainerdStep)
	assert.Equal(t, "ConfigureContainerdToml", ccsDefaults.Meta().Name)
	assert.Equal(t, DefaultContainerdConfigPath, ccsDefaults.ConfigFilePath)
	assert.Equal(t, "registry.k8s.io/pause:3.9", ccsDefaults.SandboxImage)
	assert.False(t, ccsDefaults.UseSystemdCgroup) // Default for the step constructor parameter
	assert.False(t, ccsDefaults.Sudo)           // Sudo passed as false
}

func TestConfigureContainerdStep_Run_Success(t *testing.T) {
	mockRunner := &mockRunnerForConfigureContainerd{}
	mockCtx := mockStepContextForConfigureContainerd(t, mockRunner, "host-run-success")

	configPath := "/test/config.toml"
	mirrors := map[string]MirrorConfiguration{"docker.io": {Endpoints: []string{"https://mirror.example.com"}}}
	s := NewConfigureContainerdStep("", "custom/pause", mirrors, nil, configPath, "", "", true, true).(*ConfigureContainerdStep)

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
	assert.Contains(t, writtenContent, "sandbox_image = \"custom/pause\"")
	assert.Contains(t, writtenContent, "SystemdCgroup = true")
	assert.Contains(t, writtenContent, "[plugins.\"io.containerd.grpc.v1.cri\".registry.mirrors.\"docker.io\"]")
	assert.Contains(t, writtenContent, "endpoint = [\"https://mirror.example.com\"]")
}

func TestConfigureContainerdStep_Precheck_ConfigMatches(t *testing.T) {
	mockRunner := &mockRunnerForConfigureContainerd{}
	mockCtx := mockStepContextForConfigureContainerd(t, mockRunner, "host-precheck-match")

	configPath := DefaultContainerdConfigPath
	s := NewConfigureContainerdStep("", "img", nil, nil, configPath, "", "", true, true).(*ConfigureContainerdStep)

	expectedRendered, _ := s.renderExpectedConfig()

	mockRunner.ExistsFunc = func(ctx context.Context, conn connector.Connector, path string) (bool, error) {
		if path == configPath { return true, nil }
		return false, nil
	}
	mockRunner.ReadFileFunc = func(ctx context.Context, conn connector.Connector, path string) ([]byte, error) {
		if path == configPath { return []byte(expectedRendered), nil }
		return nil, fmt.Errorf("unexpected ReadFile call")
	}

	done, err := s.Precheck(mockCtx, mockCtx.GetHost())
	require.NoError(t, err)
	assert.True(t, done, "Precheck should be done if config matches")
}

func TestConfigureContainerdStep_Precheck_ConfigMismatch(t *testing.T) {
	mockRunner := &mockRunnerForConfigureContainerd{}
	mockCtx := mockStepContextForConfigureContainerd(t, mockRunner, "host-precheck-mismatch")

	configPath := DefaultContainerdConfigPath
	s := NewConfigureContainerdStep("", "img", nil, nil, configPath, "", "", true, true).(*ConfigureContainerdStep)

	mockRunner.ExistsFunc = func(ctx context.Context, conn connector.Connector, path string) (bool, error) {
		if path == configPath { return true, nil }
		return false, nil
	}
	mockRunner.ReadFileFunc = func(ctx context.Context, conn connector.Connector, path string) ([]byte, error) {
		if path == configPath { return []byte("some other old content"), nil }
		return nil, fmt.Errorf("unexpected ReadFile call")
	}

	done, err := s.Precheck(mockCtx, mockCtx.GetHost())
	require.NoError(t, err)
	assert.False(t, done, "Precheck should not be done if config mismatches")
}

func TestConfigureContainerdStep_Precheck_FileNotExists(t *testing.T) {
	mockRunner := &mockRunnerForConfigureContainerd{}
	mockCtx := mockStepContextForConfigureContainerd(t, mockRunner, "host-precheck-noexist")
	configPath := "/tmp/nonexistent_config.toml"
	s := NewConfigureContainerdStep("", "img", nil, nil, configPath, "", "", true, true).(*ConfigureContainerdStep)

	mockRunner.ExistsFunc = func(ctx context.Context, conn connector.Connector, path string) (bool, error) {
		if path == configPath { return false, nil }
		return false, nil
	}

	done, err := s.Precheck(mockCtx, mockCtx.GetHost())
	require.NoError(t, err)
	assert.False(t, done, "Precheck should not be done if config file does not exist")
}

func TestConfigureContainerdStep_Rollback_Success(t *testing.T) {
	mockRunner := &mockRunnerForConfigureContainerd{}
	mockCtx := mockStepContextForConfigureContainerd(t, mockRunner, "host-rollback")

	configPath := "/to/be/removed.toml"
	s := NewConfigureContainerdStep("", "", nil, nil, configPath, "", "", true, true).(*ConfigureContainerdStep)

	var removeCalledWithPath string
	var removeCalledWithSudo bool
	mockRunner.RemoveFunc = func(ctx context.Context, conn connector.Connector, path string, sudo bool) error {
		removeCalledWithPath = path
		removeCalledWithSudo = sudo
		return nil
	}

	err := s.Rollback(mockCtx, mockCtx.GetHost())
	require.NoError(t, err)
	assert.Equal(t, configPath, removeCalledWithPath)
	assert.True(t, removeCalledWithSudo)
}

// Implement remaining runner.Runner methods for mockRunnerForConfigureContainerd
func (m *mockRunnerForConfigureContainerd) GatherFacts(ctx context.Context, conn connector.Connector) (*runner.Facts, error) { return nil, nil }
func (m *mockRunnerForConfigureContainerd) Run(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (string, error) { return "", nil }
func (m *mockRunnerForConfigureContainerd) MustRun(ctx context.Context, conn connector.Connector, cmd string, sudo bool) string { return "" }
func (m *mockRunnerForConfigureContainerd) Check(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (bool, error) { return false, nil }
func (m *mockRunnerForConfigureContainerd) RunWithOptions(ctx context.Context, conn connector.Connector, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) { return nil,nil, nil }
func (m *mockRunnerForConfigureContainerd) Download(ctx context.Context, conn connector.Connector, facts *runner.Facts, url, destPath string, sudo bool) error { return nil }
func (m *mockRunnerForConfigureContainerd) Extract(ctx context.Context, conn connector.Connector, facts *runner.Facts, archivePath, destDir string, sudo bool) error { return nil }
func (m *mockRunnerForConfigureContainerd) DownloadAndExtract(ctx context.Context, conn connector.Connector, facts *runner.Facts, url, destDir string, sudo bool) error { return nil }
func (m *mockRunnerForConfigureContainerd) IsDir(ctx context.Context, conn connector.Connector, path string) (bool, error) { return false, nil }
func (m *mockRunnerForConfigureContainerd) Chmod(ctx context.Context, conn connector.Connector, path string, permissions string, sudo bool) error { return nil }
func (m *mockRunnerForConfigureContainerd) Chown(ctx context.Context, conn connector.Connector, path, owner, group string, recursive bool) error { return nil }
func (m *mockRunnerForConfigureContainerd) GetSHA256(ctx context.Context, conn connector.Connector, path string) (string, error) { return "", nil }
func (m *mockRunnerForConfigureContainerd) LookPath(ctx context.Context, conn connector.Connector, file string) (string, error) { return "", nil }
func (m *mockRunnerForConfigureContainerd) IsPortOpen(ctx context.Context, conn connector.Connector, facts *runner.Facts, port int) (bool, error) { return false, nil }
func (m *mockRunnerForConfigureContainerd) WaitForPort(ctx context.Context, conn connector.Connector, facts *runner.Facts, port int, timeout time.Duration) error { return nil }
func (m *mockRunnerForConfigureContainerd) SetHostname(ctx context.Context, conn connector.Connector, facts *runner.Facts, hostname string) error { return nil }
func (m *mockRunnerForConfigureContainerd) AddHostEntry(ctx context.Context, conn connector.Connector, ip, fqdn string, hostnames ...string) error { return nil }
func (m *mockRunnerForConfigureContainerd) InstallPackages(ctx context.Context, conn connector.Connector, facts *runner.Facts, packages ...string) error { return nil }
func (m *mockRunnerForConfigureContainerd) RemovePackages(ctx context.Context, conn connector.Connector, facts *runner.Facts, packages ...string) error { return nil }
func (m *mockRunnerForConfigureContainerd) UpdatePackageCache(ctx context.Context, conn connector.Connector, facts *runner.Facts) error { return nil }
func (m *mockRunnerForConfigureContainerd) IsPackageInstalled(ctx context.Context, conn connector.Connector, facts *runner.Facts, packageName string) (bool, error) { return false, nil }
func (m *mockRunnerForConfigureContainerd) AddRepository(ctx context.Context, conn connector.Connector, facts *runner.Facts, repoConfig string, isFilePath bool) error { return nil }
func (m *mockRunnerForConfigureContainerd) StartService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForConfigureContainerd) StopService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForConfigureContainerd) RestartService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForConfigureContainerd) EnableService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForConfigureContainerd) DisableService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForConfigureContainerd) IsServiceActive(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) (bool, error) { return false, nil }
func (m *mockRunnerForConfigureContainerd) DaemonReload(ctx context.Context, conn connector.Connector, facts *runner.Facts) error { return nil }
func (m *mockRunnerForConfigureContainerd) Render(ctx context.Context, conn connector.Connector, tmpl *template.Template, data interface{}, destPath, permissions string, sudo bool) error { return nil }
func (m *mockRunnerForConfigureContainerd) UserExists(ctx context.Context, conn connector.Connector, username string) (bool, error) { return false, nil }
func (m *mockRunnerForConfigureContainerd) GroupExists(ctx context.Context, conn connector.Connector, groupname string) (bool, error) { return false, nil }
func (m *mockRunnerForConfigureContainerd) AddUser(ctx context.Context, conn connector.Connector, username, group, shell string, homeDir string, createHome bool, systemUser bool) error { return nil }
func (m *mockRunnerForConfigureContainerd) AddGroup(ctx context.Context, conn connector.Connector, groupname string, systemGroup bool) error { return nil }

var _ runner.Runner = (*mockRunnerForConfigureContainerd)(nil) // Ensure interface is satisfied

// Ensure mockStepContextForConfigureContainerd satisfies step.StepContext (it does via runtime.Context)
var _ step.StepContext = (*mockStepContextForConfigureContainerd)(t, nil, "")
