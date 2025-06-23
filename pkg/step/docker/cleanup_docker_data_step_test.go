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

// Reusing mockStepContextForDockerCleanup and mockRunnerForDockerCleanup
// from cleanup_docker_config_step_test.go as they are in the same package.

func TestCleanupDockerDataStep_New(t *testing.T) {
	s := NewCleanupDockerDataStep("TestCleanupDockerData", "/mnt/docker_data_custom", true)
	require.NotNil(t, s)
	meta := s.Meta()
	assert.Equal(t, "TestCleanupDockerData", meta.Name)
	assert.Contains(t, meta.Description, "/mnt/docker_data_custom")

	cds, ok := s.(*CleanupDockerDataStep)
	require.True(t, ok)
	assert.Equal(t, "/mnt/docker_data_custom", cds.DataDir)
	assert.True(t, cds.Sudo)

	sDefaults := NewCleanupDockerDataStep("", "", false) // Sudo defaults to true in constructor
	cdsDefaults, _ := sDefaults.(*CleanupDockerDataStep)
	assert.Equal(t, "CleanupDockerDataDirectory", cdsDefaults.meta.Name)
	assert.Equal(t, "/var/lib/docker", cdsDefaults.DataDir)
	assert.True(t, cdsDefaults.Sudo)
}

func TestCleanupDockerDataStep_Precheck_DirMissing(t *testing.T) {
	mockRunner := &mockRunnerForDockerCleanup{}
	mockCtx := mockStepContextForDockerCleanup(t, mockRunner, "host1")
	dataDir := "/var/lib/docker-test-missing"
	s := NewCleanupDockerDataStep("", dataDir, true).(*CleanupDockerDataStep)

	mockRunner.ExistsFunc = func(ctx context.Context, conn connector.Connector, path string) (bool, error) {
		if path == dataDir {
			return false, nil
		}
		return false, fmt.Errorf("unexpected Exists call for path: %s", path)
	}

	done, err := s.Precheck(mockCtx, mockCtx.GetHost())
	require.NoError(t, err)
	assert.True(t, done)
}

func TestCleanupDockerDataStep_Run_Success_ServicesNotActive(t *testing.T) {
	mockRunner := &mockRunnerForDockerCleanup{}
	mockCtx := mockStepContextForDockerCleanup(t, mockRunner, "host1")
	dataDir := "/data/docker_to_clean"
	s := NewCleanupDockerDataStep("", dataDir, true).(*CleanupDockerDataStep)

	var removedPath string
	mockRunner.IsServiceActiveFunc = func(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) (bool, error) {
		if serviceName == "docker" || serviceName == "cri-dockerd" {
			return false, nil // Services are not active
		}
		return false, fmt.Errorf("unexpected IsServiceActive call: %s", serviceName)
	}
	mockRunner.RemoveFunc = func(ctx context.Context, conn connector.Connector, path string, sudo bool) error {
		assert.True(t, sudo)
		if path == dataDir {
			removedPath = path
			return nil
		}
		return fmt.Errorf("unexpected Remove call: %s", path)
	}

	err := s.Run(mockCtx, mockCtx.GetHost())
	require.NoError(t, err)
	assert.Equal(t, dataDir, removedPath)
}

func TestCleanupDockerDataStep_Run_Success_ServicesActiveAndStopped(t *testing.T) {
	mockRunner := &mockRunnerForDockerCleanup{}
	mockCtx := mockStepContextForDockerCleanup(t, mockRunner, "host1")
	dataDir := "/data/docker_to_clean_active"
	s := NewCleanupDockerDataStep("", dataDir, true).(*CleanupDockerDataStep)

	servicesState := map[string]bool{"docker": true, "cri-dockerd": true}
	stoppedServices := make(map[string]bool)
	var pathRemoved bool

	mockRunner.IsServiceActiveFunc = func(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) (bool, error) {
		return servicesState[serviceName], nil
	}
	mockRunner.RunFunc = func(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (string, error) {
		if cmd == "systemctl stop docker" && sudo {
			servicesState["docker"] = false
			stoppedServices["docker"] = true
			return "", nil
		}
		if cmd == "systemctl stop cri-dockerd" && sudo {
			servicesState["cri-dockerd"] = false
			stoppedServices["cri-dockerd"] = true
			return "", nil
		}
		return "", fmt.Errorf("unexpected Run call: %s", cmd)
	}
	mockRunner.RemoveFunc = func(ctx context.Context, conn connector.Connector, path string, sudo bool) error {
		if path == dataDir && sudo {
			pathRemoved = true
			return nil
		}
		return fmt.Errorf("unexpected Remove call")
	}

	err := s.Run(mockCtx, mockCtx.GetHost())
	require.NoError(t, err)
	assert.True(t, stoppedServices["docker"], "Docker service should have been stopped")
	assert.True(t, stoppedServices["cri-dockerd"], "cri-dockerd service should have been stopped")
	assert.True(t, pathRemoved, "DataDir should have been removed")
}

func TestCleanupDockerDataStep_Run_CriticalDirProtection(t *testing.T) {
	mockRunner := &mockRunnerForDockerCleanup{}
	mockCtx := mockStepContextForDockerCleanup(t, mockRunner, "host1")
	criticalDirs := []string{"/", "/var", "/var/lib", "/etc"}

	for _, criticalDir := range criticalDirs {
		t.Run(fmt.Sprintf("Protect_%s", criticalDir), func(t *testing.T) {
			s := NewCleanupDockerDataStep("", criticalDir, true).(*CleanupDockerDataStep)
			err := s.Run(mockCtx, mockCtx.GetHost())
			require.Error(t, err)
			assert.Contains(t, err.Error(), "refusing to remove potentially critical directory")
		})
	}
}

// Ensure mockRunnerForDockerCleanup implements runner.Runner
var _ runner.Runner = (*mockRunnerForDockerCleanup)(nil)
// Ensure mockStepContextForDockerCleanup implements step.StepContext
var _ step.StepContext = (*mockStepContextForDockerCleanup)(t, nil, "")

// Add dummy implementations for other runner.Runner methods for mockRunnerForDockerCleanup
func (m *mockRunnerForDockerCleanup) GatherFacts(ctx context.Context, conn connector.Connector) (*runner.Facts, error) { return nil, nil }
// Run is implemented above for service stop
// func (m *mockRunnerForDockerCleanup) Run(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (string, error) { return "", nil }
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
// IsServiceActive is implemented for tests
// func (m *mockRunnerForDockerCleanup) IsServiceActive(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) (bool, error) { return false, nil }
func (m *mockRunnerForDockerCleanup) DaemonReload(ctx context.Context, conn connector.Connector, facts *runner.Facts) error { return nil }
func (m *mockRunnerForDockerCleanup) Render(ctx context.Context, conn connector.Connector, tmpl *template.Template, data interface{}, destPath, permissions string, sudo bool) error { return nil }
func (m *mockRunnerForDockerCleanup) UserExists(ctx context.Context, conn connector.Connector, username string) (bool, error) { return false, nil }
func (m *mockRunnerForDockerCleanup) GroupExists(ctx context.Context, conn connector.Connector, groupname string) (bool, error) { return false, nil }
func (m *mockRunnerForDockerCleanup) AddUser(ctx context.Context, conn connector.Connector, username, group, shell string, homeDir string, createHome bool, systemUser bool) error { return nil }
func (m *mockRunnerForDockerCleanup) AddGroup(ctx context.Context, conn connector.Connector, groupname string, systemGroup bool) error { return nil }
func (m *mockRunnerForDockerCleanup) GetPipelineCache() cache.PipelineCache { return nil }

func TestMockContextImplementation_DockerCleanupData(t *testing.T) {
	var _ step.StepContext = mockStepContextForDockerCleanup(t, &mockRunnerForDockerCleanup{}, "dummy")
}

// Add RunFunc to mockRunnerForDockerCleanup if not already present from other tests in package
type mockRunnerForDockerCleanup struct {
	runner.Runner
	ExistsFunc func(ctx context.Context, conn connector.Connector, path string) (bool, error)
	RemoveFunc func(ctx context.Context, conn connector.Connector, path string, sudo bool) error
	IsServiceActiveFunc func(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) (bool, error)
	RunFunc func(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (string, error)
}
