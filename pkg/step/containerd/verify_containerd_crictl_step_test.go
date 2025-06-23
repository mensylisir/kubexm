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

// mockStepContextForVerify is a helper to create a StepContext for testing.
func mockStepContextForVerify(t *testing.T, mockRunner runner.Runner, hostName string) step.StepContext {
	t.Helper()
	l, _ := logger.New(logger.DefaultOptions())
	mainCtx := &runtime.Context{
		GoCtx:         context.Background(),
		Logger:        l,
		ClusterConfig: &v1alpha1.Cluster{ObjectMeta: v1alpha1.ObjectMeta{Name: "test-cluster-verify-ctd"}},
		Runner:        mockRunner,
		StepCache:     cache.NewStepCache(),
		GlobalWorkDir: "/tmp/kubexm_verify_ctd_test",
		hostInfoMap:   make(map[string]*runtime.HostRuntimeInfo),
	}

	if hostName == "" {
		hostName = "test-host-verify-ctd"
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

// mockRunnerForVerify provides a mock implementation of runner.Runner.
type mockRunnerForVerify struct {
	runner.Runner
	LookPathFunc       func(ctx context.Context, conn connector.Connector, file string) (string, error)
	RunWithOptionsFunc func(ctx context.Context, conn connector.Connector, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error)
}

func (m *mockRunnerForVerify) LookPath(ctx context.Context, conn connector.Connector, file string) (string, error) {
	if m.LookPathFunc != nil {
		return m.LookPathFunc(ctx, conn, file)
	}
	return "", fmt.Errorf("LookPathFunc not implemented")
}
func (m *mockRunnerForVerify) RunWithOptions(ctx context.Context, conn connector.Connector, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) {
	if m.RunWithOptionsFunc != nil {
		return m.RunWithOptionsFunc(ctx, conn, cmd, opts)
	}
	return nil, nil, fmt.Errorf("RunWithOptionsFunc not implemented")
}


func TestVerifyContainerdCrictlStep_New(t *testing.T) {
	s := NewVerifyContainerdCrictlStep("TestVerify", "/usr/bin/crictl", "unix:///custom.sock", true)
	require.NotNil(t, s)
	meta := s.Meta()
	assert.Equal(t, "TestVerify", meta.Name)
	assert.Contains(t, meta.Description, "/usr/bin/crictl")
	assert.Contains(t, meta.Description, "unix:///custom.sock")

	vccs, ok := s.(*VerifyContainerdCrictlStep)
	require.True(t, ok)
	assert.Equal(t, "/usr/bin/crictl", vccs.CrictlPath)
	assert.Equal(t, "unix:///custom.sock", vccs.RuntimeEndpoint)
	assert.True(t, vccs.Sudo)

	// Test defaults
	sDefaults := NewVerifyContainerdCrictlStep("", "", "", false)
	vccsDefaults, _ := sDefaults.(*VerifyContainerdCrictlStep)
	assert.Equal(t, "VerifyContainerdWithCrictl", vccsDefaults.Meta().Name)
	assert.Equal(t, "crictl", vccsDefaults.CrictlPath)
	assert.Equal(t, "unix:///run/containerd/containerd.sock", vccsDefaults.RuntimeEndpoint)
	assert.False(t, vccsDefaults.Sudo)
}

func TestVerifyContainerdCrictlStep_Precheck(t *testing.T) {
	mockRunner := &mockRunnerForVerify{}
	mockCtx := mockStepContextForVerify(t, mockRunner, "host1")
	s := NewVerifyContainerdCrictlStep("", "", "", false).(*VerifyContainerdCrictlStep)

	done, err := s.Precheck(mockCtx, mockCtx.GetHost())
	require.NoError(t, err)
	assert.False(t, done, "Precheck should always return false for verification steps")
}

func TestVerifyContainerdCrictlStep_Run_Success(t *testing.T) {
	mockRunner := &mockRunnerForVerify{}
	mockCtx := mockStepContextForVerify(t, mockRunner, "host-run-verify-ok")
	s := NewVerifyContainerdCrictlStep("", "crictl", "unix:///test.sock", false).(*VerifyContainerdCrictlStep)

	expectedCmd := "crictl --runtime-endpoint unix:///test.sock info"
	var lookPathCalled, runWithOptionsCalled bool

	mockRunner.LookPathFunc = func(ctx context.Context, conn connector.Connector, file string) (string, error) {
		if file == "crictl" {
			lookPathCalled = true
			return "/usr/local/bin/crictl", nil
		}
		return "", fmt.Errorf("unexpected LookPath call: %s", file)
	}
	mockRunner.RunWithOptionsFunc = func(ctx context.Context, conn connector.Connector, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) {
		if cmd == expectedCmd {
			runWithOptionsCalled = true
			assert.False(t, opts.Sudo)
			// Simulate successful output containing expected fields
			return []byte("runtimeName: containerd\nruntimeVersion: 1.7.0\nSome other info..."), nil, nil
		}
		return nil, nil, fmt.Errorf("unexpected RunWithOptions call: %s", cmd)
	}

	err := s.Run(mockCtx, mockCtx.GetHost())
	require.NoError(t, err)
	assert.True(t, lookPathCalled, "LookPath for crictl should have been called")
	assert.True(t, runWithOptionsCalled, "RunWithOptions for crictl info should have been called")
}

func TestVerifyContainerdCrictlStep_Run_CrictlNotFound(t *testing.T) {
	mockRunner := &mockRunnerForVerify{}
	mockCtx := mockStepContextForVerify(t, mockRunner, "host-run-crictl-notfound")
	s := NewVerifyContainerdCrictlStep("", "mycrictl", "", false).(*VerifyContainerdCrictlStep)

	mockRunner.LookPathFunc = func(ctx context.Context, conn connector.Connector, file string) (string, error) {
		if file == "mycrictl" {
			return "", fmt.Errorf("mycrictl not found in PATH")
		}
		return "", fmt.Errorf("unexpected LookPath call: %s", file)
	}

	err := s.Run(mockCtx, mockCtx.GetHost())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "crictl command 'mycrictl' not found")
}

func TestVerifyContainerdCrictlStep_Run_CrictlInfoFails(t *testing.T) {
	mockRunner := &mockRunnerForVerify{}
	mockCtx := mockStepContextForVerify(t, mockRunner, "host-run-info-fails")
	s := NewVerifyContainerdCrictlStep("", "crictl", "", false).(*VerifyContainerdCrictlStep)

	expectedErr := fmt.Errorf("crictl info exited with error")
	mockRunner.LookPathFunc = func(ctx context.Context, conn connector.Connector, file string) (string, error) { return "/bin/crictl", nil }
	mockRunner.RunWithOptionsFunc = func(ctx context.Context, conn connector.Connector, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) {
		if strings.Contains(cmd, "crictl info") {
			return []byte("some output"), []byte("error details"), expectedErr
		}
		return nil, nil, fmt.Errorf("unexpected RunWithOptions call")
	}

	err := s.Run(mockCtx, mockCtx.GetHost())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "crictl info command")
	assert.Contains(t, err.Error(), "failed")
	assert.Contains(t, err.Error(), "error details") // Ensure stderr is part of the error message
	assert.ErrorIs(t, err, expectedErr)
}

func TestVerifyContainerdCrictlStep_Run_CrictlInfoOutputMissingFields(t *testing.T) {
	mockRunner := &mockRunnerForVerify{}
	mockCtx := mockStepContextForVerify(t, mockRunner, "host-run-info-missing-fields")
	s := NewVerifyContainerdCrictlStep("", "crictl", "", false).(*VerifyContainerdCrictlStep)

	mockRunner.LookPathFunc = func(ctx context.Context, conn connector.Connector, file string) (string, error) { return "/bin/crictl", nil }
	mockRunner.RunWithOptionsFunc = func(ctx context.Context, conn connector.Connector, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) {
		if strings.Contains(cmd, "crictl info") {
			return []byte("Some output but not the droids you are looking for."), nil, nil // Missing runtimeName/Version
		}
		return nil, nil, fmt.Errorf("unexpected RunWithOptions call")
	}

	err := s.Run(mockCtx, mockCtx.GetHost())
	// Current step logs a warning but doesn't error out if fields are missing, as long as command succeeds.
	require.NoError(t, err, "Run should not error if crictl info succeeds but output is unexpected, only log a warning")
}


func TestVerifyContainerdCrictlStep_Rollback(t *testing.T) {
	mockRunner := &mockRunnerForVerify{}
	mockCtx := mockStepContextForVerify(t, mockRunner, "host-rollback-verify")
	s := NewVerifyContainerdCrictlStep("", "", "", false).(*VerifyContainerdCrictlStep)

	err := s.Rollback(mockCtx, mockCtx.GetHost())
	assert.NoError(t, err, "Rollback should be a no-op")
}

// Ensure mockRunnerForVerify implements runner.Runner
var _ runner.Runner = (*mockRunnerForVerify)(nil)
// Ensure mockStepContextForVerify implements step.StepContext
var _ step.StepContext = (*mockStepContextForVerify)(t, nil, "")

// Add dummy implementations for other runner.Runner methods for mockRunnerForVerify
func (m *mockRunnerForVerify) GatherFacts(ctx context.Context, conn connector.Connector) (*runner.Facts, error) { return nil, nil }
func (m *mockRunnerForVerify) Run(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (string, error) { return "", nil }
func (m *mockRunnerForVerify) MustRun(ctx context.Context, conn connector.Connector, cmd string, sudo bool) string { return "" }
func (m *mockRunnerForVerify) Check(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (bool, error) { return false, nil }
func (m *mockRunnerForVerify) Download(ctx context.Context, conn connector.Connector, facts *runner.Facts, url, destPath string, sudo bool) error { return nil }
func (m *mockRunnerForVerify) Extract(ctx context.Context, conn connector.Connector, facts *runner.Facts, archivePath, destDir string, sudo bool) error { return nil }
func (m *mockRunnerForVerify) DownloadAndExtract(ctx context.Context, conn connector.Connector, facts *runner.Facts, url, destDir string, sudo bool) error { return nil }
func (m *mockRunnerForVerify) Exists(ctx context.Context, conn connector.Connector, path string) (bool, error) { return false, nil }
func (m *mockRunnerForVerify) IsDir(ctx context.Context, conn connector.Connector, path string) (bool, error) { return false, nil }
func (m *mockRunnerForVerify) ReadFile(ctx context.Context, conn connector.Connector, path string) ([]byte, error) { return nil, nil }
func (m *mockRunnerForVerify) WriteFile(ctx context.Context, conn connector.Connector, content []byte, destPath, permissions string, sudo bool) error { return nil }
func (m *mockRunnerForVerify) Mkdirp(ctx context.Context, conn connector.Connector, path, permissions string, sudo bool) error { return nil }
func (m *mockRunnerForVerify) Remove(ctx context.Context, conn connector.Connector, path string, sudo bool) error { return nil }
func (m *mockRunnerForVerify) Chmod(ctx context.Context, conn connector.Connector, path string, permissions string, sudo bool) error { return nil }
func (m *mockRunnerForVerify) Chown(ctx context.Context, conn connector.Connector, path, owner, group string, recursive bool) error { return nil }
func (m *mockRunnerForVerify) GetSHA256(ctx context.Context, conn connector.Connector, path string) (string, error) { return "", nil }
func (m *mockRunnerForVerify) IsPortOpen(ctx context.Context, conn connector.Connector, facts *runner.Facts, port int) (bool, error) { return false, nil }
func (m *mockRunnerForVerify) WaitForPort(ctx context.Context, conn connector.Connector, facts *runner.Facts, port int, timeout time.Duration) error { return nil }
func (m *mockRunnerForVerify) SetHostname(ctx context.Context, conn connector.Connector, facts *runner.Facts, hostname string) error { return nil }
func (m *mockRunnerForVerify) AddHostEntry(ctx context.Context, conn connector.Connector, ip, fqdn string, hostnames ...string) error { return nil }
func (m *mockRunnerForVerify) InstallPackages(ctx context.Context, conn connector.Connector, facts *runner.Facts, packages ...string) error { return nil }
func (m *mockRunnerForVerify) RemovePackages(ctx context.Context, conn connector.Connector, facts *runner.Facts, packages ...string) error { return nil }
func (m *mockRunnerForVerify) UpdatePackageCache(ctx context.Context, conn connector.Connector, facts *runner.Facts) error { return nil }
func (m *mockRunnerForVerify) IsPackageInstalled(ctx context.Context, conn connector.Connector, facts *runner.Facts, packageName string) (bool, error) { return false, nil }
func (m *mockRunnerForVerify) AddRepository(ctx context.Context, conn connector.Connector, facts *runner.Facts, repoConfig string, isFilePath bool) error { return nil }
func (m *mockRunnerForVerify) StartService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForVerify) StopService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForVerify) RestartService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForVerify) EnableService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForVerify) DisableService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForVerify) DaemonReload(ctx context.Context, conn connector.Connector, facts *runner.Facts) error { return nil }
func (m *mockRunnerForVerify) Render(ctx context.Context, conn connector.Connector, tmpl *template.Template, data interface{}, destPath, permissions string, sudo bool) error { return nil }
func (m *mockRunnerForVerify) UserExists(ctx context.Context, conn connector.Connector, username string) (bool, error) { return false, nil }
func (m *mockRunnerForVerify) GroupExists(ctx context.Context, conn connector.Connector, groupname string) (bool, error) { return false, nil }
func (m *mockRunnerForVerify) AddUser(ctx context.Context, conn connector.Connector, username, group, shell string, homeDir string, createHome bool, systemUser bool) error { return nil }
func (m *mockRunnerForVerify) AddGroup(ctx context.Context, conn connector.Connector, groupname string, systemGroup bool) error { return nil }
func (m *mockRunnerForVerify) GetPipelineCache() cache.PipelineCache { return nil }

func TestMockContextImplementation_VerifyCtd(t *testing.T) {
	var _ step.StepContext = mockStepContextForVerify(t, &mockRunnerForVerify{}, "dummy")
}
