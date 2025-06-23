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
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step"
)

// mockStepContextForGenerateService is a helper to create a StepContext for testing.
func mockStepContextForGenerateService(t *testing.T, mockRunner runner.Runner, hostName string) step.StepContext {
	t.Helper()
	l, _ := logger.New(logger.DefaultOptions())
	mainCtx := &runtime.Context{
		GoCtx:         context.Background(),
		Logger:        l,
		ClusterConfig: &v1alpha1.Cluster{ObjectMeta: v1alpha1.ObjectMeta{Name: "test-cluster-gen-svc"}},
		Runner:        mockRunner,
		StepCache:     cache.NewStepCache(),
		GlobalWorkDir: "/tmp/kubexm_gen_svc_test",
		hostInfoMap:   make(map[string]*runtime.HostRuntimeInfo),
	}

	if hostName == "" {
		hostName = "test-host-gen-svc"
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

// mockRunnerForGenerateService provides a mock implementation of runner.Runner.
type mockRunnerForGenerateService struct {
	runner.Runner
	ExistsFunc    func(ctx context.Context, conn connector.Connector, path string) (bool, error)
	ReadFileFunc  func(ctx context.Context, conn connector.Connector, path string) ([]byte, error)
	WriteFileFunc func(ctx context.Context, conn connector.Connector, content []byte, destPath, permissions string, sudo bool) error
	MkdirpFunc    func(ctx context.Context, conn connector.Connector, path, permissions string, sudo bool) error
	RemoveFunc    func(ctx context.Context, conn connector.Connector, path string, sudo bool) error
}

func (m *mockRunnerForGenerateService) Exists(ctx context.Context, conn connector.Connector, path string) (bool, error) {
	if m.ExistsFunc != nil {
		return m.ExistsFunc(ctx, conn, path)
	}
	return false, fmt.Errorf("ExistsFunc not implemented")
}
func (m *mockRunnerForGenerateService) ReadFile(ctx context.Context, conn connector.Connector, path string) ([]byte, error) {
	if m.ReadFileFunc != nil {
		return m.ReadFileFunc(ctx, conn, path)
	}
	return nil, fmt.Errorf("ReadFileFunc not implemented")
}
func (m *mockRunnerForGenerateService) WriteFile(ctx context.Context, conn connector.Connector, content []byte, destPath, permissions string, sudo bool) error {
	if m.WriteFileFunc != nil {
		return m.WriteFileFunc(ctx, conn, content, destPath, permissions, sudo)
	}
	return fmt.Errorf("WriteFileFunc not implemented")
}
func (m *mockRunnerForGenerateService) Mkdirp(ctx context.Context, conn connector.Connector, path, permissions string, sudo bool) error {
	if m.MkdirpFunc != nil {
		return m.MkdirpFunc(ctx, conn, path, permissions, sudo)
	}
	return fmt.Errorf("MkdirpFunc not implemented")
}
func (m *mockRunnerForGenerateService) Remove(ctx context.Context, conn connector.Connector, path string, sudo bool) error {
	if m.RemoveFunc != nil {
		return m.RemoveFunc(ctx, conn, path, sudo)
	}
	return fmt.Errorf("RemoveFunc not implemented")
}

func TestGenerateContainerdServiceStep_New(t *testing.T) {
	data := ContainerdServiceData{ExecStart: "/custom/bin/containerd", User: "ctd_user"}
	s := NewGenerateContainerdServiceStep("TestGenSvc", data, "/opt/systemd/containerd.service", true)
	require.NotNil(t, s)
	meta := s.Meta()
	assert.Equal(t, "TestGenSvc", meta.Name)
	assert.Contains(t, meta.Description, "/opt/systemd/containerd.service")

	gcss, ok := s.(*GenerateContainerdServiceStep)
	require.True(t, ok)
	assert.Equal(t, "/custom/bin/containerd", gcss.ServiceData.ExecStart)
	assert.Equal(t, "ctd_user", gcss.ServiceData.User)
	assert.True(t, gcss.Sudo)

	// Test defaults
	sDefaults := NewGenerateContainerdServiceStep("", ContainerdServiceData{}, "", false)
	gcssDefaults, _ := sDefaults.(*GenerateContainerdServiceStep)
	assert.Equal(t, "GenerateContainerdSystemdServiceFile", gcssDefaults.Meta().Name)
	assert.Equal(t, ContainerdServiceFileRemotePath, gcssDefaults.RemoteUnitPath)
	assert.Equal(t, "/usr/local/bin/containerd", gcssDefaults.ServiceData.ExecStart)
	assert.True(t, gcssDefaults.Sudo) // Default Sudo is true
}

func TestGenerateContainerdServiceStep_Run_Success(t *testing.T) {
	mockRunner := &mockRunnerForGenerateService{}
	mockCtx := mockStepContextForGenerateService(t, mockRunner, "host-run-gensvc")

	serviceData := ContainerdServiceData{
		ExecStart:    "/usr/bin/containerd_custom",
		ExecStartPre: []string{"-/sbin/modprobe custom_overlay"},
		Environment:  []string{"MY_VAR=value"},
	}
	configPath := "/test/systemd/containerd.service"
	s := NewGenerateContainerdServiceStep("", serviceData, configPath, true).(*GenerateContainerdServiceStep)

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
	assert.Contains(t, writtenContent, "ExecStart=/usr/bin/containerd_custom")
	assert.Contains(t, writtenContent, "ExecStartPre=-/sbin/modprobe custom_overlay")
	assert.Contains(t, writtenContent, `Environment="MY_VAR=value"`)
	assert.Contains(t, writtenContent, "TasksMax=infinity") // Check a default value
}

func TestGenerateContainerdServiceStep_Precheck_Matches(t *testing.T) {
	mockRunner := &mockRunnerForGenerateService{}
	mockCtx := mockStepContextForGenerateService(t, mockRunner, "host-precheck-match-gensvc")

	serviceData := ContainerdServiceData{Description: "Test Service"}
	configPath := ContainerdServiceFileRemotePath
	s := NewGenerateContainerdServiceStep("", serviceData, configPath, true).(*GenerateContainerdServiceStep)

	expectedRendered, _ := s.renderServiceFile(&s.ServiceData)

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

func TestGenerateContainerdServiceStep_Precheck_Mismatch(t *testing.T) {
	mockRunner := &mockRunnerForGenerateService{}
	mockCtx := mockStepContextForGenerateService(t, mockRunner, "host-precheck-mismatch-gensvc")
	s := NewGenerateContainerdServiceStep("", ContainerdServiceData{}, "", true).(*GenerateContainerdServiceStep)

	mockRunner.ExistsFunc = func(ctx context.Context, conn connector.Connector, path string) (bool, error) { return true, nil }
	mockRunner.ReadFileFunc = func(ctx context.Context, conn connector.Connector, path string) ([]byte, error) {
		return []byte("different content"), nil
	}

	done, err := s.Precheck(mockCtx, mockCtx.GetHost())
	require.NoError(t, err)
	assert.False(t, done, "Precheck should not be done if config mismatches")
}

func TestGenerateContainerdServiceStep_Rollback(t *testing.T) {
	mockRunner := &mockRunnerForGenerateService{}
	mockCtx := mockStepContextForGenerateService(t, mockRunner, "host-rollback-gensvc")
	configPath := "/test/rollback.service"
	s := NewGenerateContainerdServiceStep("", ContainerdServiceData{}, configPath, true).(*GenerateContainerdServiceStep)

	var removeCalledWithPath string
	mockRunner.RemoveFunc = func(ctx context.Context, conn connector.Connector, path string, sudo bool) error {
		removeCalledWithPath = path
		assert.True(t, sudo)
		return nil
	}

	err := s.Rollback(mockCtx, mockCtx.GetHost())
	require.NoError(t, err)
	assert.Equal(t, configPath, removeCalledWithPath)
}

var _ runner.Runner = (*mockRunnerForGenerateService)(nil)
var _ step.StepContext = (*mockStepContextForGenerateService)(t, nil, "")

// Add dummy implementations for other runner.Runner methods for mockRunnerForGenerateService
func (m *mockRunnerForGenerateService) GatherFacts(ctx context.Context, conn connector.Connector) (*runner.Facts, error) { return nil, nil }
func (m *mockRunnerForGenerateService) Run(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (string, error) { return "", nil }
func (m *mockRunnerForGenerateService) MustRun(ctx context.Context, conn connector.Connector, cmd string, sudo bool) string { return "" }
func (m *mockRunnerForGenerateService) Check(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (bool, error) { return false, nil }
func (m *mockRunnerForGenerateService) RunWithOptions(ctx context.Context, conn connector.Connector, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) { return nil,nil, nil }
func (m *mockRunnerForGenerateService) Download(ctx context.Context, conn connector.Connector, facts *runner.Facts, url, destPath string, sudo bool) error { return nil }
func (m *mockRunnerForGenerateService) Extract(ctx context.Context, conn connector.Connector, facts *runner.Facts, archivePath, destDir string, sudo bool) error { return nil }
func (m *mockRunnerForGenerateService) DownloadAndExtract(ctx context.Context, conn connector.Connector, facts *runner.Facts, url, destDir string, sudo bool) error { return nil }
func (m *mockRunnerForGenerateService) IsDir(ctx context.Context, conn connector.Connector, path string) (bool, error) { return false, nil }
func (m *mockRunnerForGenerateService) Chmod(ctx context.Context, conn connector.Connector, path string, permissions string, sudo bool) error { return nil }
func (m *mockRunnerForGenerateService) Chown(ctx context.Context, conn connector.Connector, path, owner, group string, recursive bool) error { return nil }
func (m *mockRunnerForGenerateService) GetSHA256(ctx context.Context, conn connector.Connector, path string) (string, error) { return "", nil }
func (m *mockRunnerForGenerateService) LookPath(ctx context.Context, conn connector.Connector, file string) (string, error) { return "", nil }
func (m *mockRunnerForGenerateService) IsPortOpen(ctx context.Context, conn connector.Connector, facts *runner.Facts, port int) (bool, error) { return false, nil }
func (m *mockRunnerForGenerateService) WaitForPort(ctx context.Context, conn connector.Connector, facts *runner.Facts, port int, timeout time.Duration) error { return nil }
func (m *mockRunnerForGenerateService) SetHostname(ctx context.Context, conn connector.Connector, facts *runner.Facts, hostname string) error { return nil }
func (m *mockRunnerForGenerateService) AddHostEntry(ctx context.Context, conn connector.Connector, ip, fqdn string, hostnames ...string) error { return nil }
func (m *mockRunnerForGenerateService) InstallPackages(ctx context.Context, conn connector.Connector, facts *runner.Facts, packages ...string) error { return nil }
func (m *mockRunnerForGenerateService) RemovePackages(ctx context.Context, conn connector.Connector, facts *runner.Facts, packages ...string) error { return nil }
func (m *mockRunnerForGenerateService) UpdatePackageCache(ctx context.Context, conn connector.Connector, facts *runner.Facts) error { return nil }
func (m *mockRunnerForGenerateService) IsPackageInstalled(ctx context.Context, conn connector.Connector, facts *runner.Facts, packageName string) (bool, error) { return false, nil }
func (m *mockRunnerForGenerateService) AddRepository(ctx context.Context, conn connector.Connector, facts *runner.Facts, repoConfig string, isFilePath bool) error { return nil }
func (m *mockRunnerForGenerateService) StartService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForGenerateService) StopService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForGenerateService) RestartService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForGenerateService) EnableService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForGenerateService) DisableService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForGenerateService) IsServiceActive(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) (bool, error) { return false, nil }
func (m *mockRunnerForGenerateService) DaemonReload(ctx context.Context, conn connector.Connector, facts *runner.Facts) error { return nil }
func (m *mockRunnerForGenerateService) Render(ctx context.Context, conn connector.Connector, tmpl *template.Template, data interface{}, destPath, permissions string, sudo bool) error { return nil }
func (m *mockRunnerForGenerateService) UserExists(ctx context.Context, conn connector.Connector, username string) (bool, error) { return false, nil }
func (m *mockRunnerForGenerateService) GroupExists(ctx context.Context, conn connector.Connector, groupname string) (bool, error) { return false, nil }
func (m *mockRunnerForGenerateService) AddUser(ctx context.Context, conn connector.Connector, username, group, shell string, homeDir string, createHome bool, systemUser bool) error { return nil }
func (m *mockRunnerForGenerateService) AddGroup(ctx context.Context, conn connector.Connector, groupname string, systemGroup bool) error { return nil }
func (m *mockRunnerForGenerateService) GetPipelineCache() cache.PipelineCache { return nil }

// Add dummy StepContext methods for mockStepContextForGenerateService (if needed beyond runtime.Context)
// The current mockStepContextForGenerateService returns a runtime.Context which implements step.StepContext.
func TestMockContextImplementation_GenerateSvc(t *testing.T) {
	var _ step.StepContext = mockStepContextForGenerateService(t, &mockRunnerForGenerateService{}, "dummy")
}
