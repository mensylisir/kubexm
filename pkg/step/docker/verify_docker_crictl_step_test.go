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
)

// mockRunnerForVerifyDocker provides a mock implementation of runner.Runner for verify steps.
type mockRunnerForVerifyDocker struct {
	runner.Runner
	LookPathFunc       func(ctx context.Context, conn connector.Connector, file string) (string, error)
	RunWithOptionsFunc func(ctx context.Context, conn connector.Connector, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error)
}

func (m *mockRunnerForVerifyDocker) LookPath(ctx context.Context, conn connector.Connector, file string) (string, error) {
	if m.LookPathFunc != nil {
		return m.LookPathFunc(ctx, conn, file)
	}
	return "", fmt.Errorf("LookPathFunc not implemented")
}
func (m *mockRunnerForVerifyDocker) RunWithOptions(ctx context.Context, conn connector.Connector, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) {
	if m.RunWithOptionsFunc != nil {
		return m.RunWithOptionsFunc(ctx, conn, cmd, opts)
	}
	return nil, nil, fmt.Errorf("RunWithOptionsFunc not implemented")
}


func TestVerifyDockerCrictlStep_New(t *testing.T) {
	s := NewVerifyDockerCrictlStep("TestVerifyDocker", "/opt/bin/crictl", "unix:///var/run/custom-crid.sock", true)
	require.NotNil(t, s)
	meta := s.Meta()
	assert.Equal(t, "TestVerifyDocker", meta.Name)
	assert.Contains(t, meta.Description, "/opt/bin/crictl")
	assert.Contains(t, meta.Description, "unix:///var/run/custom-crid.sock")

	vdcs, ok := s.(*VerifyDockerCrictlStep)
	require.True(t, ok)
	assert.Equal(t, "/opt/bin/crictl", vdcs.CrictlPath)
	assert.Equal(t, "unix:///var/run/custom-crid.sock", vdcs.RuntimeEndpoint)
	assert.True(t, vdcs.Sudo)

	sDefaults := NewVerifyDockerCrictlStep("", "", "", false)
	vdcsDefaults, _ := sDefaults.(*VerifyDockerCrictlStep)
	assert.Equal(t, "VerifyDockerWithCrictl", vdcsDefaults.Meta().Name)
	assert.Equal(t, "crictl", vdcsDefaults.CrictlPath)
	assert.Equal(t, "unix:///run/cri-dockerd.sock", vdcsDefaults.RuntimeEndpoint)
	assert.False(t, vdcsDefaults.Sudo)
}

func TestVerifyDockerCrictlStep_Run_Success(t *testing.T) {
	mockRunner := &mockRunnerForVerifyDocker{}
	mockCtx := mockStepContextForDockerCleanup(t, mockRunner, "host-verify-docker-ok")
	s := NewVerifyDockerCrictlStep("", "crictl", "unix:///run/cri-dockerd.sock", false).(*VerifyDockerCrictlStep)

	expectedCmd := "crictl --runtime-endpoint unix:///run/cri-dockerd.sock info"
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
			return []byte("runtimeName: docker\nServerVersion: 20.10.7\nSomething else..."), nil, nil
		}
		return nil, nil, fmt.Errorf("unexpected RunWithOptions call: %s", cmd)
	}

	err := s.Run(mockCtx, mockCtx.GetHost())
	require.NoError(t, err)
	assert.True(t, lookPathCalled, "LookPath for crictl should have been called")
	assert.True(t, runWithOptionsCalled, "RunWithOptions for crictl info should have been called")
}

// Ensure mockRunnerForVerifyDocker implements runner.Runner
var _ runner.Runner = (*mockRunnerForVerifyDocker)(nil)
// Ensure mockStepContextForDockerCleanup implements step.StepContext
var _ step.StepContext = (*mockStepContextForDockerCleanup)(t, nil, "")

// Add dummy implementations for other runner.Runner methods for mockRunnerForVerifyDocker
func (m *mockRunnerForVerifyDocker) GatherFacts(ctx context.Context, conn connector.Connector) (*runner.Facts, error) { return nil, nil }
func (m *mockRunnerForVerifyDocker) Run(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (string, error) { return "", nil }
func (m *mockRunnerForVerifyDocker) MustRun(ctx context.Context, conn connector.Connector, cmd string, sudo bool) string { return "" }
func (m *mockRunnerForVerifyDocker) Check(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (bool, error) { return false, nil }
func (m *mockRunnerForVerifyDocker) Download(ctx context.Context, conn connector.Connector, facts *runner.Facts, url, destPath string, sudo bool) error { return nil }
func (m *mockRunnerForVerifyDocker) Extract(ctx context.Context, conn connector.Connector, facts *runner.Facts, archivePath, destDir string, sudo bool) error { return nil }
func (m *mockRunnerForVerifyDocker) DownloadAndExtract(ctx context.Context, conn connector.Connector, facts *runner.Facts, url, destDir string, sudo bool) error { return nil }
func (m *mockRunnerForVerifyDocker) Exists(ctx context.Context, conn connector.Connector, path string) (bool, error) { return false, nil }
func (m *mockRunnerForVerifyDocker) IsDir(ctx context.Context, conn connector.Connector, path string) (bool, error) { return false, nil }
func (m *mockRunnerForVerifyDocker) ReadFile(ctx context.Context, conn connector.Connector, path string) ([]byte, error) { return nil, nil }
func (m *mockRunnerForVerifyDocker) WriteFile(ctx context.Context, conn connector.Connector, content []byte, destPath, permissions string, sudo bool) error { return nil }
func (m *mockRunnerForVerifyDocker) Mkdirp(ctx context.Context, conn connector.Connector, path, permissions string, sudo bool) error { return nil }
func (m *mockRunnerForVerifyDocker) Remove(ctx context.Context, conn connector.Connector, path string, sudo bool) error { return nil }
func (m *mockRunnerForVerifyDocker) Chmod(ctx context.Context, conn connector.Connector, path string, permissions string, sudo bool) error { return nil }
func (m *mockRunnerForVerifyDocker) Chown(ctx context.Context, conn connector.Connector, path, owner, group string, recursive bool) error { return nil }
func (m *mockRunnerForVerifyDocker) GetSHA256(ctx context.Context, conn connector.Connector, path string) (string, error) { return "", nil }
func (m *mockRunnerForVerifyDocker) IsPortOpen(ctx context.Context, conn connector.Connector, facts *runner.Facts, port int) (bool, error) { return false, nil }
func (m *mockRunnerForVerifyDocker) WaitForPort(ctx context.Context, conn connector.Connector, facts *runner.Facts, port int, timeout time.Duration) error { return nil }
func (m *mockRunnerForVerifyDocker) SetHostname(ctx context.Context, conn connector.Connector, facts *runner.Facts, hostname string) error { return nil }
func (m *mockRunnerForVerifyDocker) AddHostEntry(ctx context.Context, conn connector.Connector, ip, fqdn string, hostnames ...string) error { return nil }
func (m *mockRunnerForVerifyDocker) InstallPackages(ctx context.Context, conn connector.Connector, facts *runner.Facts, packages ...string) error { return nil }
func (m *mockRunnerForVerifyDocker) RemovePackages(ctx context.Context, conn connector.Connector, facts *runner.Facts, packages ...string) error { return nil }
func (m *mockRunnerForVerifyDocker) UpdatePackageCache(ctx context.Context, conn connector.Connector, facts *runner.Facts) error { return nil }
func (m *mockRunnerForVerifyDocker) IsPackageInstalled(ctx context.Context, conn connector.Connector, facts *runner.Facts, packageName string) (bool, error) { return false, nil }
func (m *mockRunnerForVerifyDocker) AddRepository(ctx context.Context, conn connector.Connector, facts *runner.Facts, repoConfig string, isFilePath bool) error { return nil }
func (m *mockRunnerForVerifyDocker) StartService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForVerifyDocker) StopService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForVerifyDocker) RestartService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForVerifyDocker) EnableService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForVerifyDocker) DisableService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForVerifyDocker) IsServiceActive(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) (bool, error) { return false, nil }
func (m *mockRunnerForVerifyDocker) DaemonReload(ctx context.Context, conn connector.Connector, facts *runner.Facts) error { return nil }
func (m *mockRunnerForVerifyDocker) Render(ctx context.Context, conn connector.Connector, tmpl *template.Template, data interface{}, destPath, permissions string, sudo bool) error { return nil }
func (m *mockRunnerForVerifyDocker) UserExists(ctx context.Context, conn connector.Connector, username string) (bool, error) { return false, nil }
func (m *mockRunnerForVerifyDocker) GroupExists(ctx context.Context, conn connector.Connector, groupname string) (bool, error) { return false, nil }
func (m *mockRunnerForVerifyDocker) AddUser(ctx context.Context, conn connector.Connector, username, group, shell string, homeDir string, createHome bool, systemUser bool) error { return nil }
func (m *mockRunnerForVerifyDocker) AddGroup(ctx context.Context, conn connector.Connector, groupname string, systemGroup bool) error { return nil }
func (m *mockRunnerForVerifyDocker) GetPipelineCache() cache.PipelineCache { return nil }


func TestMockContextImplementation_DockerVerifyCriD(t *testing.T) {
	var _ step.StepContext = mockStepContextForDockerCleanup(t, &mockRunnerForVerifyDocker{}, "dummy")
}
