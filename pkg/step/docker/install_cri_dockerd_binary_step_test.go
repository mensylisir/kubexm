package docker

import (
	"context"
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

// mockStepContextForInstallCriD is a helper to create a StepContext for testing.
func mockStepContextForInstallCriD(t *testing.T, mockRunner runner.Runner, hostName string, taskCacheValues map[string]interface{}) step.StepContext {
	t.Helper()
	l, _ := logger.New(logger.DefaultOptions())
	mainCtx := &runtime.Context{
		GoCtx:         context.Background(),
		Logger:        l,
		ClusterConfig: &v1alpha1.Cluster{ObjectMeta: v1alpha1.ObjectMeta{Name: "test-cluster-install-crid"}},
		Runner:        mockRunner,
		StepCache:     cache.NewStepCache(),
		TaskCache:     cache.NewTaskCache(),
		GlobalWorkDir: "/tmp/kubexm_install_crid_test",
		hostInfoMap:   make(map[string]*runtime.HostRuntimeInfo),
	}

	if hostName == "" {
		hostName = "test-host-install-crid"
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

	if taskCacheValues != nil {
		for k, v := range taskCacheValues {
			mainCtx.TaskCache().Set(k, v)
		}
	}
	return mainCtx
}

// mockRunnerForInstallCriD provides a mock implementation of runner.Runner.
type mockRunnerForInstallCriD struct {
	runner.Runner
	ExistsFunc func(ctx context.Context, conn connector.Connector, path string) (bool, error)
	MkdirpFunc func(ctx context.Context, conn connector.Connector, path, permissions string, sudo bool) error
	RunFunc    func(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (string, error) // For cp
	ChmodFunc  func(ctx context.Context, conn connector.Connector, path, permissions string, sudo bool) error
	RemoveFunc func(ctx context.Context, conn connector.Connector, path string, sudo bool) error
}
func (m *mockRunnerForInstallCriD) Exists(ctx context.Context, conn connector.Connector, path string) (bool, error) {
	if m.ExistsFunc != nil { return m.ExistsFunc(ctx, conn, path) }
	return false, fmt.Errorf("ExistsFunc not implemented")
}
func (m *mockRunnerForInstallCriD) Mkdirp(ctx context.Context, conn connector.Connector, path, perm string, sudo bool) error {
	if m.MkdirpFunc != nil { return m.MkdirpFunc(ctx, conn, path, perm, sudo) }
	return fmt.Errorf("MkdirpFunc not implemented")
}
func (m *mockRunnerForInstallCriD) Run(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (string, error) {
	if m.RunFunc != nil { return m.RunFunc(ctx, conn, cmd, sudo) }
	return "", fmt.Errorf("RunFunc not implemented")
}
func (m *mockRunnerForInstallCriD) Chmod(ctx context.Context, conn connector.Connector, path, perm string, sudo bool) error {
	if m.ChmodFunc != nil { return m.ChmodFunc(ctx, conn, path, perm, sudo) }
	return fmt.Errorf("ChmodFunc not implemented")
}
func (m *mockRunnerForInstallCriD) Remove(ctx context.Context, conn connector.Connector, path string, sudo bool) error {
	if m.RemoveFunc != nil { return m.RemoveFunc(ctx, conn, path, sudo) }
	return fmt.Errorf("RemoveFunc not implemented")
}


func TestInstallCriDockerdBinaryStep_New(t *testing.T) {
	s := NewInstallCriDockerdBinaryStep("TestInstallCriDUnits", "extKey", "/opt/bin", "/opt/systemd", true)
	require.NotNil(t, s)
	meta := s.Meta()
	assert.Equal(t, "TestInstallCriDUnits", meta.Name)
	assert.Contains(t, meta.Description, "/opt/bin")
	assert.Contains(t, meta.Description, "/opt/systemd")

	icbs, ok := s.(*InstallCriDockerdBinaryStep)
	require.True(t, ok)
	assert.Equal(t, "extKey", icbs.ExtractedDirCacheKey)
	assert.Equal(t, "/opt/bin", icbs.TargetBinaryDir)
	assert.Equal(t, "/opt/systemd", icbs.TargetSystemdDir)
	assert.True(t, icbs.Sudo)

	sDefaults := NewInstallCriDockerdBinaryStep("", "", "", "", false)
	icbsDefaults, _ := sDefaults.(*InstallCriDockerdBinaryStep)
	assert.Equal(t, "InstallCriDockerdBinaryAndUnits", icbsDefaults.Meta().Name)
	assert.Equal(t, CriDockerdExtractedDirCacheKey, icbsDefaults.ExtractedDirCacheKey)
	assert.Equal(t, "/usr/local/bin", icbsDefaults.TargetBinaryDir)
	assert.Equal(t, "/etc/systemd/system", icbsDefaults.TargetSystemdDir)
	assert.False(t, icbsDefaults.Sudo)
}

func TestInstallCriDockerdBinaryStep_Precheck_AllExist(t *testing.T) {
	mockRunner := &mockRunnerForInstallCriD{}
	mockCtx := mockStepContextForInstallCriD(t, mockRunner, "host-crid-precheck-exists", nil)

	targetBinDir := "/usr/test/bin"
	targetSysdDir := "/usr/test/systemd"
	s := NewInstallCriDockerdBinaryStep("", "", targetBinDir, targetSysdDir, true).(*InstallCriDockerdBinaryStep)

	mockRunner.ExistsFunc = func(ctx context.Context, conn connector.Connector, path string) (bool, error) {
		switch path {
		case filepath.Join(targetBinDir, "cri-dockerd"),
			filepath.Join(targetSysdDir, "cri-dockerd.service"),
			filepath.Join(targetSysdDir, "cri-dockerd.socket"):
			return true, nil
		default:
			return false, nil
		}
	}

	done, err := s.Precheck(mockCtx, mockCtx.GetHost())
	require.NoError(t, err)
	assert.True(t, done, "Precheck should be done if all items exist")
}

func TestInstallCriDockerdBinaryStep_Run_Success(t *testing.T) {
	mockRunner := &mockRunnerForInstallCriD{}
	extractedPath := "/tmp/extracted_crid_content"
	taskCache := map[string]interface{}{CriDockerdExtractedDirCacheKey: extractedPath}
	mockCtx := mockStepContextForInstallCriD(t, mockRunner, "host-run-install-crid", taskCache)

	targetBinDir := "/opt/custom/bin"
	targetSysdDir := "/opt/custom/systemd"
	s := NewInstallCriDockerdBinaryStep("", CriDockerdExtractedDirCacheKey, targetBinDir, targetSysdDir, true).(*InstallCriDockerdBinaryStep)

	var mkdirPaths, cpCmds, chmodPaths []string
	mockRunner.MkdirpFunc = func(ctx context.Context, conn connector.Connector, path, permissions string, sudo bool) error {
		mkdirPaths = append(mkdirPaths, path)
		assert.True(t, sudo)
		return nil
	}
	mockRunner.ExistsFunc = func(ctx context.Context, conn connector.Connector, path string) (bool, error) {
		// Assume source files exist in the extracted directory
		if strings.HasPrefix(path, extractedPath) {
			return true, nil
		}
		return false, nil
	}
	mockRunner.RunFunc = func(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (string, error) {
		if strings.HasPrefix(cmd, "cp -fp") || strings.HasPrefix(cmd, "cp -f") {
			cpCmds = append(cpCmds, cmd)
			assert.True(t, sudo)
			return "", nil
		}
		return "", fmt.Errorf("unexpected Run call: %s", cmd)
	}
	mockRunner.ChmodFunc = func(ctx context.Context, conn connector.Connector, path, permissions string, sudo bool) error {
		chmodPaths = append(chmodPaths, path+":"+permissions)
		assert.True(t, sudo)
		return nil
	}

	err := s.Run(mockCtx, mockCtx.GetHost())
	require.NoError(t, err)

	assert.Contains(t, mkdirPaths, targetBinDir)
	assert.Contains(t, mkdirPaths, targetSysdDir)

	expectedBinSource := filepath.Join(extractedPath, "cri-dockerd")
	expectedBinTarget := filepath.Join(targetBinDir, "cri-dockerd")
	assert.Contains(t, cpCmds, fmt.Sprintf("cp -fp %s %s", expectedBinSource, expectedBinTarget))
	assert.Contains(t, chmodPaths, expectedBinTarget+":0755")

	expectedServiceSource := filepath.Join(extractedPath, "packaging/systemd/cri-dockerd.service")
	expectedServiceTarget := filepath.Join(targetSysdDir, "cri-dockerd.service")
	assert.Contains(t, cpCmds, fmt.Sprintf("cp -f %s %s", expectedServiceSource, expectedServiceTarget))
	assert.Contains(t, chmodPaths, expectedServiceTarget+":0644")

	expectedSocketSource := filepath.Join(extractedPath, "packaging/systemd/cri-dockerd.socket")
	expectedSocketTarget := filepath.Join(targetSysdDir, "cri-dockerd.socket")
	assert.Contains(t, cpCmds, fmt.Sprintf("cp -f %s %s", expectedSocketSource, expectedSocketTarget))
	assert.Contains(t, chmodPaths, expectedSocketTarget+":0644")
}

// Ensure mockRunnerForInstallCriD implements runner.Runner
var _ runner.Runner = (*mockRunnerForInstallCriD)(nil)
// Ensure mockStepContextForInstallCriD implements step.StepContext
var _ step.StepContext = (*mockStepContextForInstallCriD)(t, nil, "", nil)

// Add dummy implementations for other runner.Runner methods for mockRunnerForInstallCriD
func (m *mockRunnerForInstallCriD) GatherFacts(ctx context.Context, conn connector.Connector) (*runner.Facts, error) { return nil, nil }
func (m *mockRunnerForInstallCriD) MustRun(ctx context.Context, conn connector.Connector, cmd string, sudo bool) string { return "" }
func (m *mockRunnerForInstallCriD) Check(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (bool, error) { return false, nil }
func (m *mockRunnerForInstallCriD) RunWithOptions(ctx context.Context, conn connector.Connector, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) { return nil,nil, nil }
func (m *mockRunnerForInstallCriD) Download(ctx context.Context, conn connector.Connector, facts *runner.Facts, url, destPath string, sudo bool) error { return nil }
func (m *mockRunnerForInstallCriD) Extract(ctx context.Context, conn connector.Connector, facts *runner.Facts, archivePath, destDir string, sudo bool) error { return nil }
func (m *mockRunnerForInstallCriD) DownloadAndExtract(ctx context.Context, conn connector.Connector, facts *runner.Facts, url, destDir string, sudo bool) error { return nil }
func (m *mockRunnerForInstallCriD) IsDir(ctx context.Context, conn connector.Connector, path string) (bool, error) { return false, nil }
func (m *mockRunnerForInstallCriD) ReadFile(ctx context.Context, conn connector.Connector, path string) ([]byte, error) { return nil, nil }
func (m *mockRunnerForInstallCriD) WriteFile(ctx context.Context, conn connector.Connector, content []byte, destPath, permissions string, sudo bool) error { return nil }
func (m *mockRunnerForInstallCriD) Chown(ctx context.Context, conn connector.Connector, path, owner, group string, recursive bool) error { return nil }
func (m *mockRunnerForInstallCriD) GetSHA256(ctx context.Context, conn connector.Connector, path string) (string, error) { return "", nil }
func (m *mockRunnerForInstallCriD) LookPath(ctx context.Context, conn connector.Connector, file string) (string, error) { return "", nil }
func (m *mockRunnerForInstallCriD) IsPortOpen(ctx context.Context, conn connector.Connector, facts *runner.Facts, port int) (bool, error) { return false, nil }
func (m *mockRunnerForInstallCriD) WaitForPort(ctx context.Context, conn connector.Connector, facts *runner.Facts, port int, timeout time.Duration) error { return nil }
func (m *mockRunnerForInstallCriD) SetHostname(ctx context.Context, conn connector.Connector, facts *runner.Facts, hostname string) error { return nil }
func (m *mockRunnerForInstallCriD) AddHostEntry(ctx context.Context, conn connector.Connector, ip, fqdn string, hostnames ...string) error { return nil }
func (m *mockRunnerForInstallCriD) InstallPackages(ctx context.Context, conn connector.Connector, facts *runner.Facts, packages ...string) error { return nil }
func (m *mockRunnerForInstallCriD) RemovePackages(ctx context.Context, conn connector.Connector, facts *runner.Facts, packages ...string) error { return nil }
func (m *mockRunnerForInstallCriD) UpdatePackageCache(ctx context.Context, conn connector.Connector, facts *runner.Facts) error { return nil }
func (m *mockRunnerForInstallCriD) IsPackageInstalled(ctx context.Context, conn connector.Connector, facts *runner.Facts, packageName string) (bool, error) { return false, nil }
func (m *mockRunnerForInstallCriD) AddRepository(ctx context.Context, conn connector.Connector, facts *runner.Facts, repoConfig string, isFilePath bool) error { return nil }
func (m *mockRunnerForInstallCriD) StartService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForInstallCriD) StopService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForInstallCriD) RestartService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForInstallCriD) EnableService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForInstallCriD) DisableService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForInstallCriD) IsServiceActive(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) (bool, error) { return false, nil }
func (m *mockRunnerForInstallCriD) DaemonReload(ctx context.Context, conn connector.Connector, facts *runner.Facts) error { return nil }
func (m *mockRunnerForInstallCriD) Render(ctx context.Context, conn connector.Connector, tmpl *template.Template, data interface{}, destPath, permissions string, sudo bool) error { return nil }
func (m *mockRunnerForInstallCriD) UserExists(ctx context.Context, conn connector.Connector, username string) (bool, error) { return false, nil }
func (m *mockRunnerForInstallCriD) GroupExists(ctx context.Context, conn connector.Connector, groupname string) (bool, error) { return false, nil }
func (m *mockRunnerForInstallCriD) AddUser(ctx context.Context, conn connector.Connector, username, group, shell string, homeDir string, createHome bool, systemUser bool) error { return nil }
func (m *mockRunnerForInstallCriD) AddGroup(ctx context.Context, conn connector.Connector, groupname string, systemGroup bool) error { return nil }
func (m *mockRunnerForInstallCriD) GetPipelineCache() cache.PipelineCache { return nil }

func TestMockContextImplementation_DockerInstallCriD(t *testing.T) {
	var _ step.StepContext = mockStepContextForInstallCriD(t, &mockRunnerForInstallCriD{}, "dummy", nil)
}
