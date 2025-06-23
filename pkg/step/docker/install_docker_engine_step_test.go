package docker

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

// mockRunnerForInstallDocker provides a mock implementation of runner.Runner.
type mockRunnerForInstallDocker struct {
	runner.Runner
	IsPackageInstalledFunc func(ctx context.Context, conn connector.Connector, facts *runner.Facts, packageName string) (bool, error)
	InstallPackagesFunc    func(ctx context.Context, conn connector.Connector, facts *runner.Facts, packages ...string) error
	RemovePackagesFunc     func(ctx context.Context, conn connector.Connector, facts *runner.Facts, packages ...string) error
	UpdatePackageCacheFunc func(ctx context.Context, conn connector.Connector, facts *runner.Facts) error
	RunFunc                func(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (string, error)
}

func (m *mockRunnerForInstallDocker) IsPackageInstalled(ctx context.Context, conn connector.Connector, facts *runner.Facts, packageName string) (bool, error) {
	if m.IsPackageInstalledFunc != nil {
		return m.IsPackageInstalledFunc(ctx, conn, facts, packageName)
	}
	return false, fmt.Errorf("IsPackageInstalledFunc not implemented")
}
func (m *mockRunnerForInstallDocker) InstallPackages(ctx context.Context, conn connector.Connector, facts *runner.Facts, packages ...string) error {
	if m.InstallPackagesFunc != nil {
		return m.InstallPackagesFunc(ctx, conn, facts, packages...)
	}
	return fmt.Errorf("InstallPackagesFunc not implemented")
}
func (m *mockRunnerForInstallDocker) RemovePackages(ctx context.Context, conn connector.Connector, facts *runner.Facts, packages ...string) error {
	if m.RemovePackagesFunc != nil {
		return m.RemovePackagesFunc(ctx, conn, facts, packages...)
	}
	return fmt.Errorf("RemovePackagesFunc not implemented")
}
func (m *mockRunnerForInstallDocker) UpdatePackageCache(ctx context.Context, conn connector.Connector, facts *runner.Facts) error {
	if m.UpdatePackageCacheFunc != nil {
		return m.UpdatePackageCacheFunc(ctx, conn, facts)
	}
	return fmt.Errorf("UpdatePackageCacheFunc not implemented")
}
func (m *mockRunnerForInstallDocker) Run(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (string, error) {
	if m.RunFunc != nil {
		return m.RunFunc(ctx, conn, cmd, sudo)
	}
	return "", fmt.Errorf("RunFunc not implemented")
}


func TestInstallDockerEngineStep_New(t *testing.T) {
	customPkgs := []string{"custom-docker", "custom-cli"}
	extraCmds := []string{"echo 'repo setup'"}
	s := NewInstallDockerEngineStep("TestInstallDocker", customPkgs, extraCmds, true)
	require.NotNil(t, s)
	meta := s.Meta()
	assert.Equal(t, "TestInstallDocker", meta.Name)
	assert.Contains(t, meta.Description, "custom-docker")

	ides, ok := s.(*InstallDockerEngineStep)
	require.True(t, ok)
	assert.Equal(t, customPkgs, ides.Packages)
	assert.Equal(t, extraCmds, ides.ExtraRepoSetupCmds)
	assert.True(t, ides.Sudo)

	sDefaults := NewInstallDockerEngineStep("", nil, nil, false)
	idesDefaults, _ := sDefaults.(*InstallDockerEngineStep)
	assert.Equal(t, "InstallDockerEngine", idesDefaults.Meta().Name)
	assert.NotEmpty(t, idesDefaults.Packages) // Default packages should be set
	assert.Nil(t, idesDefaults.ExtraRepoSetupCmds)
	assert.True(t, idesDefaults.Sudo) // Default Sudo is true
}

func TestInstallDockerEngineStep_Precheck_Installed(t *testing.T) {
	mockRunner := &mockRunnerForInstallDocker{}
	mockCtx := mockStepContextForDockerCleanup(t, mockRunner, "host-docker-precheck-installed")
	s := NewInstallDockerEngineStep("", []string{"docker-ce"}, nil, true).(*InstallDockerEngineStep)

	mockRunner.IsPackageInstalledFunc = func(ctx context.Context, conn connector.Connector, facts *runner.Facts, pkgName string) (bool, error) {
		if pkgName == "docker-ce" {
			return true, nil
		}
		return false, nil
	}

	done, err := s.Precheck(mockCtx, mockCtx.GetHost())
	require.NoError(t, err)
	assert.True(t, done, "Precheck should be done if key package is installed")
}

func TestInstallDockerEngineStep_Run_Success(t *testing.T) {
	mockRunner := &mockRunnerForInstallDocker{}
	mockCtx := mockStepContextForDockerCleanup(t, mockRunner, "host-run-install-docker")

	pkgs := []string{"docker-ce", "docker-ce-cli"}
	repoCmds := []string{"curl -fsSL get.docker.com -o get-docker.sh", "sh get-docker.sh"}
	s := NewInstallDockerEngineStep("", pkgs, repoCmds, true).(*InstallDockerEngineStep)

	var repoCmdsExecuted []string
	var updateCacheCalled, installPkgsCalled bool

	mockRunner.RunFunc = func(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (string, error) {
		repoCmdsExecuted = append(repoCmdsExecuted, cmd)
		assert.True(t, sudo)
		return "", nil
	}
	mockRunner.UpdatePackageCacheFunc = func(ctx context.Context, conn connector.Connector, facts *runner.Facts) error {
		updateCacheCalled = true
		return nil
	}
	mockRunner.InstallPackagesFunc = func(ctx context.Context, conn connector.Connector, facts *runner.Facts, packages ...string) error {
		installPkgsCalled = true
		assert.Equal(t, pkgs, packages)
		return nil
	}

	err := s.Run(mockCtx, mockCtx.GetHost())
	require.NoError(t, err)
	assert.Equal(t, repoCmds, repoCmdsExecuted, "Extra repo setup commands mismatch")
	assert.True(t, updateCacheCalled, "UpdatePackageCache should have been called")
	assert.True(t, installPkgsCalled, "InstallPackages should have been called")
}

// Ensure mockRunnerForInstallDocker implements runner.Runner
var _ runner.Runner = (*mockRunnerForInstallDocker)(nil)
// Ensure mockStepContextForDockerCleanup implements step.StepContext
var _ step.StepContext = (*mockStepContextForDockerCleanup)(t, nil, "")

// Add dummy implementations for other runner.Runner methods for mockRunnerForInstallDocker
func (m *mockRunnerForInstallDocker) GatherFacts(ctx context.Context, conn connector.Connector) (*runner.Facts, error) { return nil, nil }
func (m *mockRunnerForInstallDocker) MustRun(ctx context.Context, conn connector.Connector, cmd string, sudo bool) string { return "" }
func (m *mockRunnerForInstallDocker) Check(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (bool, error) { return false, nil }
func (m *mockRunnerForInstallDocker) RunWithOptions(ctx context.Context, conn connector.Connector, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) { return nil,nil, nil }
func (m *mockRunnerForInstallDocker) Download(ctx context.Context, conn connector.Connector, facts *runner.Facts, url, destPath string, sudo bool) error { return nil }
func (m *mockRunnerForInstallDocker) Extract(ctx context.Context, conn connector.Connector, facts *runner.Facts, archivePath, destDir string, sudo bool) error { return nil }
func (m *mockRunnerForInstallDocker) DownloadAndExtract(ctx context.Context, conn connector.Connector, facts *runner.Facts, url, destDir string, sudo bool) error { return nil }
func (m *mockRunnerForInstallDocker) Exists(ctx context.Context, conn connector.Connector, path string) (bool, error) { return false, nil }
func (m *mockRunnerForInstallDocker) IsDir(ctx context.Context, conn connector.Connector, path string) (bool, error) { return false, nil }
func (m *mockRunnerForInstallDocker) ReadFile(ctx context.Context, conn connector.Connector, path string) ([]byte, error) { return nil, nil }
func (m *mockRunnerForInstallDocker) WriteFile(ctx context.Context, conn connector.Connector, content []byte, destPath, permissions string, sudo bool) error { return nil }
func (m *mockRunnerForInstallDocker) Mkdirp(ctx context.Context, conn connector.Connector, path, permissions string, sudo bool) error { return nil }
func (m *mockRunnerForInstallDocker) Chmod(ctx context.Context, conn connector.Connector, path string, permissions string, sudo bool) error { return nil }
func (m *mockRunnerForInstallDocker) Chown(ctx context.Context, conn connector.Connector, path, owner, group string, recursive bool) error { return nil }
func (m *mockRunnerForInstallDocker) GetSHA256(ctx context.Context, conn connector.Connector, path string) (string, error) { return "", nil }
func (m *mockRunnerForInstallDocker) LookPath(ctx context.Context, conn connector.Connector, file string) (string, error) { return "", nil }
func (m *mockRunnerForInstallDocker) IsPortOpen(ctx context.Context, conn connector.Connector, facts *runner.Facts, port int) (bool, error) { return false, nil }
func (m *mockRunnerForInstallDocker) WaitForPort(ctx context.Context, conn connector.Connector, facts *runner.Facts, port int, timeout time.Duration) error { return nil }
func (m *mockRunnerForInstallDocker) SetHostname(ctx context.Context, conn connector.Connector, facts *runner.Facts, hostname string) error { return nil }
func (m *mockRunnerForInstallDocker) AddHostEntry(ctx context.Context, conn connector.Connector, ip, fqdn string, hostnames ...string) error { return nil }
func (m *mockRunnerForInstallDocker) AddRepository(ctx context.Context, conn connector.Connector, facts *runner.Facts, repoConfig string, isFilePath bool) error { return nil }
func (m *mockRunnerForInstallDocker) StartService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForInstallDocker) StopService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForInstallDocker) RestartService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForInstallDocker) EnableService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForInstallDocker) DisableService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForInstallDocker) IsServiceActive(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) (bool, error) { return false, nil }
func (m *mockRunnerForInstallDocker) DaemonReload(ctx context.Context, conn connector.Connector, facts *runner.Facts) error { return nil }
func (m *mockRunnerForInstallDocker) Render(ctx context.Context, conn connector.Connector, tmpl *template.Template, data interface{}, destPath, permissions string, sudo bool) error { return nil }
func (m *mockRunnerForInstallDocker) UserExists(ctx context.Context, conn connector.Connector, username string) (bool, error) { return false, nil }
func (m *mockRunnerForInstallDocker) GroupExists(ctx context.Context, conn connector.Connector, groupname string) (bool, error) { return false, nil }
func (m *mockRunnerForInstallDocker) AddUser(ctx context.Context, conn connector.Connector, username, group, shell string, homeDir string, createHome bool, systemUser bool) error { return nil }
func (m *mockRunnerForInstallDocker) AddGroup(ctx context.Context, conn connector.Connector, groupname string, systemGroup bool) error { return nil }
func (m *mockRunnerForInstallDocker) GetPipelineCache() cache.PipelineCache { return nil }

func TestMockContextImplementation_DockerInstallEngine(t *testing.T) {
	var _ step.StepContext = mockStepContextForDockerCleanup(t, &mockRunnerForInstallDocker{}, "dummy")
}
