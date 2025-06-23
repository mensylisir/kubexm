package docker

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

// Reusing mockStepContextForDockerCleanup and mockRunnerForInstallDocker (adjusting as needed)
// from other docker step tests.

func TestRemoveDockerEngineStep_New(t *testing.T) {
	customPkgs := []string{"custom-docker-pkg", "another-pkg"}
	s := NewRemoveDockerEngineStep("TestRemoveDockerPkg", customPkgs, true, true) // purge = true
	require.NotNil(t, s)
	meta := s.Meta()
	assert.Equal(t, "TestRemoveDockerPkg", meta.Name)
	assert.Contains(t, meta.Description, "custom-docker-pkg")
	assert.Contains(t, meta.Description, "Purge: true")

	rdes, ok := s.(*RemoveDockerEngineStep)
	require.True(t, ok)
	assert.Equal(t, customPkgs, rdes.Packages)
	assert.True(t, rdes.Purge)
	assert.True(t, rdes.Sudo)

	sDefaults := NewRemoveDockerEngineStep("", nil, false, false) // Sudo defaults to true
	rdesDefaults, _ := sDefaults.(*RemoveDockerEngineStep)
	assert.Equal(t, "RemoveDockerEngine", rdesDefaults.Meta().Name)
	assert.NotEmpty(t, rdesDefaults.Packages) // Default packages
	assert.False(t, rdesDefaults.Purge)
	assert.True(t, rdesDefaults.Sudo)
}

func TestRemoveDockerEngineStep_Precheck_NotInstalled(t *testing.T) {
	mockRunner := &mockRunnerForInstallDocker{} // Reusing from install_docker_engine_step_test
	mockCtx := mockStepContextForDockerCleanup(t, mockRunner, "host-docker-precheck-notinstalled")
	pkgs := []string{"docker-ce"}
	s := NewRemoveDockerEngineStep("", pkgs, false, true).(*RemoveDockerEngineStep)

	mockRunner.IsPackageInstalledFunc = func(ctx context.Context, conn connector.Connector, facts *runner.Facts, pkgName string) (bool, error) {
		if pkgName == "docker-ce" {
			return false, nil // Package not installed
		}
		return false, fmt.Errorf("unexpected IsPackageInstalled call for %s", pkgName)
	}

	done, err := s.Precheck(mockCtx, mockCtx.GetHost())
	require.NoError(t, err)
	assert.True(t, done, "Precheck should be done if key package is not installed")
}

func TestRemoveDockerEngineStep_Run_Success(t *testing.T) {
	mockRunner := &mockRunnerForInstallDocker{}
	mockCtx := mockStepContextForDockerCleanup(t, mockRunner, "host-run-remove-docker")

	pkgsToRemove := []string{"docker-ce", "docker-ce-cli"}
	s := NewRemoveDockerEngineStep("", pkgsToRemove, true, true).(*RemoveDockerEngineStep) // Purge = true

	var removePkgsCalledWith []string
	var purgeOptionUsed bool // This would require RemovePackages to accept a purge flag

	mockRunner.RemovePackagesFunc = func(ctx context.Context, conn connector.Connector, facts *runner.Facts, packages ...string) error {
		removePkgsCalledWith = packages
		// If RemovePackages supported purge:
		// if options.Purge { purgeOptionUsed = true }
		// For now, we just check the packages. The step's Purge field is informational for the runner.
		return nil
	}

	err := s.Run(mockCtx, mockCtx.GetHost())
	require.NoError(t, err)
	assert.Equal(t, pkgsToRemove, removePkgsCalledWith)
	// If testing purge: assert.True(t, purgeOptionUsed, "Purge option should have been used by runner")
}

// Ensure mockRunnerForInstallDocker implements runner.Runner
var _ runner.Runner = (*mockRunnerForInstallDocker)(nil)
// Ensure mockStepContextForDockerCleanup implements step.StepContext
var _ step.StepContext = (*mockStepContextForDockerCleanup)(t, nil, "")

// Add dummy implementations for other runner.Runner methods for mockRunnerForInstallDocker
// (Many are already present in install_docker_engine_step_test.go)
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


func TestMockContextImplementation_DockerRemoveEngine(t *testing.T) {
	var _ step.StepContext = mockStepContextForDockerCleanup(t, &mockRunnerForInstallDocker{}, "dummy")
}
