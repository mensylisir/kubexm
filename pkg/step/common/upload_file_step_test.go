package common

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
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
	"github.com/mensylisir/kubexm/pkg/runtime" // For runtime.Context full
	"github.com/mensylisir/kubexm/pkg/step"
)

// mockStepContextForUpload is a helper to create a StepContext for testing UploadFileStep.
func mockStepContextForUpload(t *testing.T, currentHostName string, mockRunner runner.Runner) step.StepContext {
	t.Helper()
	l, _ := logger.New(logger.DefaultOptions())
	tempGlobalWorkDir, err := ioutil.TempDir("", "test-gwd-upload-")
	require.NoError(t, err)

	var currentHost connector.Host
	var controlNode connector.Host
	var allHosts []v1alpha1.HostSpec

	controlHostSpec := v1alpha1.HostSpec{
		Name:    common.ControlNodeHostName,
		Type:    "local",
		Address: "127.0.0.1",
		Roles:   []string{common.ControlNodeRole},
	}
	allHosts = append(allHosts, controlHostSpec)
	controlNode = connector.NewHostFromSpec(controlHostSpec)

	if currentHostName == "" || currentHostName == common.ControlNodeHostName {
		currentHost = controlNode
	} else {
		remoteHostSpec := v1alpha1.HostSpec{Name: currentHostName, Address: "10.0.0.1", Type: "ssh", User: "test", Port: 22}
		allHosts = append(allHosts, remoteHostSpec)
		currentHost = connector.NewHostFromSpec(remoteHostSpec)
	}

	mainCtx := &runtime.Context{
		GoCtx:  context.Background(),
		Logger: l,
		ClusterConfig: &v1alpha1.Cluster{
			ObjectMeta: v1alpha1.ObjectMeta{Name: "test-cluster-upload"},
			Spec: v1alpha1.ClusterSpec{
				Global: &v1alpha1.GlobalSpec{WorkDir: filepath.Dir(filepath.Dir(tempGlobalWorkDir))},
				Hosts:  allHosts,
			},
		},
		Runner:        mockRunner, // Use the provided mockRunner
		StepCache:     cache.NewStepCache(),
		GlobalWorkDir: tempGlobalWorkDir,
		hostInfoMap:   make(map[string]*runtime.HostRuntimeInfo), // Initialize to avoid nil panics
	}

	// Populate hostInfoMap for GetConnectorForHost and GetHostFacts
	for _, hSpec := range allHosts {
		h := connector.NewHostFromSpec(hSpec)
		var conn connector.Connector
		if h.GetName() == common.ControlNodeHostName {
			conn = &connector.LocalConnector{}
		} else {
			// For remote hosts, tests will typically mock the runner's behavior directly
			// rather than mocking a specific SSH connector instance here.
			// If a step directly uses conn.Exec, then a mock connector is needed.
			// UploadFileStep uses runner.WriteFile, runner.Exists, runner.Remove.
			conn = &step.MockStepConnector{} // Generic mock for other hosts
		}
		mainCtx.GetHostInfoMap()[h.GetName()] = &runtime.HostRuntimeInfo{
			Host:  h,
			Conn:  conn,
			Facts: &runner.Facts{OS: &connector.OS{Arch: "amd64"}},
		}
	}

	mainCtx.SetControlNode(controlNode)
	mainCtx.SetCurrentHost(currentHost)
	return mainCtx
}

// mockRunnerForUploadFile provides a mock implementation of runner.Runner.
type mockRunnerForUploadFile struct {
	runner.Runner // Embed to satisfy the interface
	ExistsFunc    func(ctx context.Context, conn connector.Connector, path string) (bool, error)
	WriteFileFunc func(ctx context.Context, conn connector.Connector, content []byte, destPath, permissions string, sudo bool) error
	RemoveFunc    func(ctx context.Context, conn connector.Connector, path string, sudo bool) error
}

func (m *mockRunnerForUploadFile) Exists(ctx context.Context, conn connector.Connector, path string) (bool, error) {
	if m.ExistsFunc != nil {
		return m.ExistsFunc(ctx, conn, path)
	}
	return false, nil
}
func (m *mockRunnerForUploadFile) WriteFile(ctx context.Context, conn connector.Connector, content []byte, destPath, permissions string, sudo bool) error {
	if m.WriteFileFunc != nil {
		return m.WriteFileFunc(ctx, conn, content, destPath, permissions, sudo)
	}
	return fmt.Errorf("WriteFileFunc not implemented in mock")
}
func (m *mockRunnerForUploadFile) Remove(ctx context.Context, conn connector.Connector, path string, sudo bool) error {
	if m.RemoveFunc != nil {
		return m.RemoveFunc(ctx, conn, path, sudo)
	}
	return fmt.Errorf("RemoveFunc not implemented in mock")
}


func TestUploadFileStep_NewUploadFileStep(t *testing.T) {
	src := "/tmp/local.txt"
	dest := "/opt/remote.txt"
	perms := "0600"
	s := NewUploadFileStep("TestUpload", src, dest, perms, true, false)
	require.NotNil(t, s)
	meta := s.Meta()
	assert.Equal(t, "TestUpload", meta.Name)
	assert.Contains(t, meta.Description, src)
	assert.Contains(t, meta.Description, dest)

	ufs, ok := s.(*UploadFileStep)
	require.True(t, ok)
	assert.Equal(t, src, ufs.LocalSrcPath)
	assert.Equal(t, dest, ufs.RemoteDestPath)
	assert.Equal(t, perms, ufs.Permissions)
	assert.True(t, ufs.Sudo)
	assert.False(t, ufs.AllowMissingSrc)

	sDefaultName := NewUploadFileStep("", src, dest, perms, false, true)
	assert.Equal(t, fmt.Sprintf("UploadFile:%s_to_%s", src, dest), sDefaultName.Meta().Name)
	assert.True(t, sDefaultName.(*UploadFileStep).AllowMissingSrc)
}

func TestUploadFileStep_Precheck_LocalSourceMissing_NotAllowed(t *testing.T) {
	mockRunner := &mockRunnerForUploadFile{}
	mockCtx := mockStepContextForUpload(t, "remote-host", mockRunner)
	defer os.RemoveAll(mockCtx.GetGlobalWorkDir())

	nonExistentSrc := filepath.Join(mockCtx.GetGlobalWorkDir(), "nonexistent_src.txt")
	s := NewUploadFileStep("", nonExistentSrc, "/remote/dest.txt", "0644", false, false).(*UploadFileStep)

	done, err := s.Precheck(mockCtx, mockCtx.GetHost())
	require.Error(t, err)
	assert.False(t, done)
	assert.Contains(t, err.Error(), "local source file")
	assert.Contains(t, err.Error(), "does not exist")
}

func TestUploadFileStep_Precheck_LocalSourceMissing_Allowed(t *testing.T) {
	mockRunner := &mockRunnerForUploadFile{}
	mockCtx := mockStepContextForUpload(t, "remote-host", mockRunner)
	defer os.RemoveAll(mockCtx.GetGlobalWorkDir())

	nonExistentSrc := filepath.Join(mockCtx.GetGlobalWorkDir(), "nonexistent_src_allowed.txt")
	s := NewUploadFileStep("", nonExistentSrc, "/remote/dest.txt", "0644", false, true).(*UploadFileStep)

	done, err := s.Precheck(mockCtx, mockCtx.GetHost())
	require.NoError(t, err)
	assert.True(t, done, "Precheck should be done if local source missing and AllowMissingSrc is true")
}

func TestUploadFileStep_Precheck_RemoteExists(t *testing.T) {
	mockRunner := &mockRunnerForUploadFile{}
	mockCtx := mockStepContextForUpload(t, "remote-host", mockRunner)
	defer os.RemoveAll(mockCtx.GetGlobalWorkDir())

	localSrc := filepath.Join(mockCtx.GetGlobalWorkDir(), "local_src_for_remote_exists.txt")
	err := ioutil.WriteFile(localSrc, []byte("content"), 0644)
	require.NoError(t, err)

	remoteDest := "/remote/dest_exists.txt"
	s := NewUploadFileStep("", localSrc, remoteDest, "0644", false, false).(*UploadFileStep)

	mockRunner.ExistsFunc = func(ctx context.Context, conn connector.Connector, path string) (bool, error) {
		if path == remoteDest {
			return true, nil // Simulate remote file exists
		}
		return false, nil
	}

	done, err := s.Precheck(mockCtx, mockCtx.GetHost())
	require.NoError(t, err)
	assert.True(t, done, "Precheck should be done if remote file exists")
}

func TestUploadFileStep_Precheck_RemoteNotExists(t *testing.T) {
	mockRunner := &mockRunnerForUploadFile{}
	mockCtx := mockStepContextForUpload(t, "remote-host", mockRunner)
	defer os.RemoveAll(mockCtx.GetGlobalWorkDir())

	localSrc := filepath.Join(mockCtx.GetGlobalWorkDir(), "local_src_for_remote_not_exists.txt")
	err := ioutil.WriteFile(localSrc, []byte("content"), 0644)
	require.NoError(t, err)

	remoteDest := "/remote/dest_not_exists.txt"
	s := NewUploadFileStep("", localSrc, remoteDest, "0644", false, false).(*UploadFileStep)

	mockRunner.ExistsFunc = func(ctx context.Context, conn connector.Connector, path string) (bool, error) {
		if path == remoteDest {
			return false, nil // Simulate remote file does not exist
		}
		return false, nil
	}

	done, err := s.Precheck(mockCtx, mockCtx.GetHost())
	require.NoError(t, err)
	assert.False(t, done, "Precheck should not be done if remote file does not exist")
}

func TestUploadFileStep_Run_Success(t *testing.T) {
	mockRunner := &mockRunnerForUploadFile{}
	mockCtx := mockStepContextForUpload(t, "remote-host-run", mockRunner)
	defer os.RemoveAll(mockCtx.GetGlobalWorkDir())

	localSrcContent := "upload this content"
	localSrc := filepath.Join(mockCtx.GetGlobalWorkDir(), "local_to_upload.txt")
	err := ioutil.WriteFile(localSrc, []byte(localSrcContent), 0644)
	require.NoError(t, err)

	remoteDest := "/opt/uploaded_file.txt"
	permissions := "0600"
	useSudo := true
	s := NewUploadFileStep("", localSrc, remoteDest, permissions, useSudo, false).(*UploadFileStep)

	var writeFileCalled bool
	mockRunner.WriteFileFunc = func(ctx context.Context, conn connector.Connector, content []byte, destPath, perms string, sudo bool) error {
		writeFileCalled = true
		assert.Equal(t, localSrcContent, string(content))
		assert.Equal(t, remoteDest, destPath)
		assert.Equal(t, permissions, perms)
		assert.Equal(t, useSudo, sudo)
		return nil
	}

	errRun := s.Run(mockCtx, mockCtx.GetHost())
	require.NoError(t, errRun)
	assert.True(t, writeFileCalled, "Runner's WriteFile method was not called")
}

func TestUploadFileStep_Run_LocalSourceMissing_Allowed_Skips(t *testing.T) {
	mockRunner := &mockRunnerForUploadFile{}
	mockCtx := mockStepContextForUpload(t, "remote-host-skip", mockRunner)
	defer os.RemoveAll(mockCtx.GetGlobalWorkDir())

	nonExistentSrc := filepath.Join(mockCtx.GetGlobalWorkDir(), "nonexistent_src_for_run.txt")
	s := NewUploadFileStep("", nonExistentSrc, "/remote/dest.txt", "0644", false, true).(*UploadFileStep)

	var writeFileCalled bool
	mockRunner.WriteFileFunc = func(ctx context.Context, conn connector.Connector, content []byte, destPath, perms string, sudo bool) error {
		writeFileCalled = true
		return nil
	}

	errRun := s.Run(mockCtx, mockCtx.GetHost())
	require.NoError(t, errRun, "Run should succeed by skipping if AllowMissingSrc is true and file is missing")
	assert.False(t, writeFileCalled, "Runner's WriteFile should not be called if local source is missing and allowed")
}

func TestUploadFileStep_Run_SourceIsDirectory(t *testing.T) {
	mockRunner := &mockRunnerForUploadFile{}
	mockCtx := mockStepContextForUpload(t, "remote-host-dir", mockRunner)
	defer os.RemoveAll(mockCtx.GetGlobalWorkDir())

	localSrcDir := filepath.Join(mockCtx.GetGlobalWorkDir(), "local_src_dir")
	err := os.Mkdir(localSrcDir, 0755)
	require.NoError(t, err)

	s := NewUploadFileStep("", localSrcDir, "/remote/dest", "0644", false, false).(*UploadFileStep)

	errRun := s.Run(mockCtx, mockCtx.GetHost())
	require.Error(t, errRun)
	assert.Contains(t, errRun.Error(), "is a directory, UploadFileStep only supports single files")
}

func TestUploadFileStep_Rollback_Success(t *testing.T) {
	mockRunner := &mockRunnerForUploadFile{}
	mockCtx := mockStepContextForUpload(t, "remote-host-rollback", mockRunner)
	defer os.RemoveAll(mockCtx.GetGlobalWorkDir())

	remoteDestToClean := "/opt/file_to_clean.txt"
	useSudoRollback := true
	s := NewUploadFileStep("", "/local/any.txt", remoteDestToClean, "0644", useSudoRollback, false).(*UploadFileStep)

	var removeCalledWithPath string
	var removeCalledWithSudo bool
	mockRunner.RemoveFunc = func(ctx context.Context, conn connector.Connector, path string, sudo bool) error {
		removeCalledWithPath = path
		removeCalledWithSudo = sudo
		return nil
	}

	errRollback := s.Rollback(mockCtx, mockCtx.GetHost())
	require.NoError(t, errRollback)
	assert.Equal(t, remoteDestToClean, removeCalledWithPath)
	assert.Equal(t, useSudoRollback, removeCalledWithSudo)
}

func TestUploadFileStep_Rollback_RemoveError(t *testing.T) {
	mockRunner := &mockRunnerForUploadFile{}
	mockCtx := mockStepContextForUpload(t, "remote-host-rollback-err", mockRunner)
	defer os.RemoveAll(mockCtx.GetGlobalWorkDir())

	s := NewUploadFileStep("", "/local/any.txt", "/remote/file.txt", "0644", true, false).(*UploadFileStep)
	expectedErr := fmt.Errorf("failed to remove for test")

	mockRunner.RemoveFunc = func(ctx context.Context, conn connector.Connector, path string, sudo bool) error {
		return expectedErr
	}

	errRollback := s.Rollback(mockCtx, mockCtx.GetHost())
	// Rollback is best-effort, so it might not return the error from Remove directly.
	// The current implementation logs a warning but returns nil.
	assert.NoError(t, errRollback, "Rollback should return nil even if runner.Remove fails, as it's best-effort")
}

// Ensure mockRunnerForUploadFile implements runner.Runner
var _ runner.Runner = (*mockRunnerForUploadFile)(nil)

// Dummy implementations for the rest of runner.Runner for mockRunnerForUploadFile
func (m *mockRunnerForUploadFile) GatherFacts(ctx context.Context, conn connector.Connector) (*runner.Facts, error) { return nil, nil }
func (m *mockRunnerForUploadFile) Run(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (string, error) { return "", nil }
func (m *mockRunnerForUploadFile) MustRun(ctx context.Context, conn connector.Connector, cmd string, sudo bool) string { return "" }
func (m *mockRunnerForUploadFile) Check(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (bool, error) { return false, nil }
func (m *mockRunnerForUploadFile) RunWithOptions(ctx context.Context, conn connector.Connector, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) { return nil,nil, nil }
func (m *mockRunnerForUploadFile) Download(ctx context.Context, conn connector.Connector, facts *runner.Facts, url, destPath string, sudo bool) error { return nil }
func (m *mockRunnerForUploadFile) Extract(ctx context.Context, conn connector.Connector, facts *runner.Facts, archivePath, destDir string, sudo bool) error { return nil }
func (m *mockRunnerForUploadFile) DownloadAndExtract(ctx context.Context, conn connector.Connector, facts *runner.Facts, url, destDir string, sudo bool) error { return nil }
func (m *mockRunnerForUploadFile) IsDir(ctx context.Context, conn connector.Connector, path string) (bool, error) { return false, nil }
func (m *mockRunnerForUploadFile) ReadFile(ctx context.Context, conn connector.Connector, path string) ([]byte, error) { return nil, nil }
func (m *mockRunnerForUploadFile) Mkdirp(ctx context.Context, conn connector.Connector, path, permissions string, sudo bool) error { return nil }
func (m *mockRunnerForUploadFile) Chmod(ctx context.Context, conn connector.Connector, path string, permissions string, sudo bool) error { return nil }
func (m *mockRunnerForUploadFile) Chown(ctx context.Context, conn connector.Connector, path, owner, group string, recursive bool) error { return nil }
func (m *mockRunnerForUploadFile) GetSHA256(ctx context.Context, conn connector.Connector, path string) (string, error) { return "", nil }
func (m *mockRunnerForUploadFile) LookPath(ctx context.Context, conn connector.Connector, file string) (string, error) { return "", nil }
func (m *mockRunnerForUploadFile) IsPortOpen(ctx context.Context, conn connector.Connector, facts *runner.Facts, port int) (bool, error) { return false, nil }
func (m *mockRunnerForUploadFile) WaitForPort(ctx context.Context, conn connector.Connector, facts *runner.Facts, port int, timeout time.Duration) error { return nil }
func (m *mockRunnerForUploadFile) SetHostname(ctx context.Context, conn connector.Connector, facts *runner.Facts, hostname string) error { return nil }
func (m *mockRunnerForUploadFile) AddHostEntry(ctx context.Context, conn connector.Connector, ip, fqdn string, hostnames ...string) error { return nil }
func (m *mockRunnerForUploadFile) InstallPackages(ctx context.Context, conn connector.Connector, facts *runner.Facts, packages ...string) error { return nil }
func (m *mockRunnerForUploadFile) RemovePackages(ctx context.Context, conn connector.Connector, facts *runner.Facts, packages ...string) error { return nil }
func (m *mockRunnerForUploadFile) UpdatePackageCache(ctx context.Context, conn connector.Connector, facts *runner.Facts) error { return nil }
func (m *mockRunnerForUploadFile) IsPackageInstalled(ctx context.Context, conn connector.Connector, facts *runner.Facts, packageName string) (bool, error) { return false, nil }
func (m *mockRunnerForUploadFile) AddRepository(ctx context.Context, conn connector.Connector, facts *runner.Facts, repoConfig string, isFilePath bool) error { return nil }
func (m *mockRunnerForUploadFile) StartService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForUploadFile) StopService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForUploadFile) RestartService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForUploadFile) EnableService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForUploadFile) DisableService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForUploadFile) IsServiceActive(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) (bool, error) { return false, nil }
func (m *mockRunnerForUploadFile) DaemonReload(ctx context.Context, conn connector.Connector, facts *runner.Facts) error { return nil }
func (m *mockRunnerForUploadFile) Render(ctx context.Context, conn connector.Connector, tmpl *template.Template, data interface{}, destPath, permissions string, sudo bool) error { return nil }
func (m *mockRunnerForUploadFile) UserExists(ctx context.Context, conn connector.Connector, username string) (bool, error) { return false, nil }
func (m *mockRunnerForUploadFile) GroupExists(ctx context.Context, conn connector.Connector, groupname string) (bool, error) { return false, nil }
func (m *mockRunnerForUploadFile) AddUser(ctx context.Context, conn connector.Connector, username, group, shell string, homeDir string, createHome bool, systemUser bool) error { return nil }
func (m *mockRunnerForUploadFile) AddGroup(ctx context.Context, conn connector.Connector, groupname string, systemGroup bool) error { return nil }

// Ensure mockStepContextForUpload implements step.StepContext
var _ step.StepContext = (*mockStepContextForUpload)(nil)

// Dummy implementations for the rest of step.StepContext for mockStepContextForUpload
// Many are inherited from runtime.Context via embedding, but some might need explicit mock if behavior differs.
// For UploadFileStep, many of these are not directly used by the step logic itself, but by the runner it calls.
// The runner is mocked here, so these context methods are less critical for these specific tests.
// GetHost() is important for the step context.
func (m *mockStepContextForUpload) GetHost() connector.Host { return m.controlHost } // Assuming upload runs in context of control node for local path, targets remote via runner
// Other methods, if needed by a more complex step or for completeness:
// func (m *mockStepContextForUpload) StepCache() cache.StepCache { return nil }
// func (m *mockStepContextForUpload) TaskCache() cache.TaskCache { return nil }
// func (m *mockStepContextForUpload) ModuleCache() cache.ModuleCache { return nil }
// func (m *mockStepContextForUpload) GetCurrentHostFacts() (*runner.Facts, error) { return nil, nil }
// func (m *mockStepContextForUpload) GetCurrentHostConnector() (connector.Connector, error) { return nil, nil }
// func (m *mockStepContextForUpload) IsVerbose() bool { return false }
// func (m *mockStepContextForUpload) ShouldIgnoreErr() bool { return false }
// func (m *mockStepContextForUpload) GetGlobalConnectionTimeout() time.Duration { return 30 * time.Second }
// func (m *mockStepContextForUpload) GetClusterArtifactsDir() string { return filepath.Join(m.GetGlobalWorkDir(), m.GetClusterConfig().Name) }
// func (m *mockStepContextForUpload) GetCertsDir() string { return filepath.Join(m.GetClusterArtifactsDir(), "certs") }
// func (m *mockStepContextForUpload) GetEtcdCertsDir() string { return filepath.Join(m.GetCertsDir(), "etcd") }
// func (m *mockStepContextForUpload) GetComponentArtifactsDir(cn string) string { return filepath.Join(m.GetClusterArtifactsDir(), cn) }
// func (m *mockStepContextForUpload) GetEtcdArtifactsDir() string { return m.GetComponentArtifactsDir("etcd") }
// func (m *mockStepContextForUpload) GetContainerRuntimeArtifactsDir() string { return m.GetComponentArtifactsDir("container_runtime") }
// func (m *mockStepContextForUpload) GetKubernetesArtifactsDir() string { return m.GetComponentArtifactsDir("kubernetes") }
// func (m *mockStepContextForUpload) GetFileDownloadPath(c,v,a,f string) string { return "" }
// func (m *mockStepContextForUpload) GetHostDir(h string) string { return filepath.Join(m.GetClusterArtifactsDir(), h) }
// func (m *mockStepContextForUpload) WithGoContext(gCtx context.Context) step.StepContext { m.goCtx = gCtx; return m }

// Adding missing methods from StepContext for mockStepContextForUpload to satisfy the interface.
func (m *mockStepContextForUpload) GetLogger() *logger.Logger                                  { return m.logger }
func (m *mockStepContextForUpload) GoContext() context.Context                                   { return m.goCtx }
// GetRunner() is already defined by embedding runtime.Context and being implemented by the mock
// GetConnectorForHost(h connector.Host) is already defined by embedding runtime.Context
func (m *mockStepContextForUpload) GetHostFacts(h connector.Host) (*runner.Facts, error) {
	if m.hostInfoMap != nil {
		if hri, ok := m.hostInfoMap[h.GetName()]; ok && hri.Facts != nil {
			return hri.Facts, nil
		}
	}
	return &runner.Facts{OS: &connector.OS{Arch: "amd64"}}, nil // Default mock
}
func (m *mockStepContextForUpload) GetCurrentHostFacts() (*runner.Facts, error) {
	return m.GetHostFacts(m.controlHost) // Assuming current host is controlHost for these tests
}
func (m *mockStepContextForUpload) GetCurrentHostConnector() (connector.Connector, error) {
	if m.hostInfoMap != nil {
		if hri, ok := m.hostInfoMap[m.controlHost.GetName()]; ok && hri.Conn != nil {
			return hri.Conn, nil
		}
	}
	return &step.MockStepConnector{}, nil // Default mock
}
func (m *mockStepContextForUpload) StepCache() cache.StepCache                               { return nil }
func (m *mockStepContextForUpload) TaskCache() cache.TaskCache                               { return nil }
func (m *mockStepContextForUpload) ModuleCache() cache.ModuleCache                             { return nil }
func (m *mockStepContextForUpload) GetGlobalWorkDir() string                                   { return m.GlobalWorkDir }
func (m *mockStepContextForUpload) GetClusterConfig() *v1alpha1.Cluster                      { return m.ClusterConfig }
func (m *mockStepContextForUpload) IsVerbose() bool                                        { return false }
func (m *mockStepContextForUpload) ShouldIgnoreErr() bool                                  { return false }
func (m *mockStepContextForUpload) GetGlobalConnectionTimeout() time.Duration                { return 30 * time.Second }
func (m *mockStepContextForUpload) GetClusterArtifactsDir() string                         { return filepath.Join(m.GlobalWorkDir, m.ClusterConfig.Name) }
func (m *mockStepContextForUpload) GetCertsDir() string                                    { return filepath.Join(m.GetClusterArtifactsDir(), "certs") }
func (m *mockStepContextForUpload) GetEtcdCertsDir() string                                { return filepath.Join(m.GetCertsDir(), "etcd") }
func (m *mockStepContextForUpload) GetComponentArtifactsDir(componentName string) string     { return filepath.Join(m.GetClusterArtifactsDir(), componentName) }
func (m *mockStepContextForUpload) GetEtcdArtifactsDir() string                            { return m.GetComponentArtifactsDir("etcd") }
func (m *mockStepContextForUpload) GetContainerRuntimeArtifactsDir() string                { return m.GetComponentArtifactsDir("container_runtime") }
func (m *mockStepContextForUpload) GetKubernetesArtifactsDir() string                      { return m.GetComponentArtifactsDir("kubernetes") }
func (m *mockStepContextForUpload) GetFileDownloadPath(c, v, a, f string) string             { return "" } // Simplified
func (m *mockStepContextForUpload) GetHostDir(hostname string) string                      { return filepath.Join(m.GetClusterArtifactsDir(), hostname) }
func (m *mockStepContextForUpload) WithGoContext(gCtx context.Context) step.StepContext      { m.goCtx = gCtx; return m }

// Fields from runtime.Context that need to be on mockStepContextForUpload for the above to work
func (m *mockStepContextForUpload) GetHostInfoMap() map[string]*runtime.HostRuntimeInfo { return nil } // Simplified
func (m *mockStepContextForUpload) SetControlNode(cn connector.Host) {}
func (m *mockStepContextForUpload) SetCurrentHost(ch connector.Host) {}
func (m *mockStepContextForUpload) AsPipelineContext() runtime.PipelineContext { return nil }
func (m *mockStepContextForUpload) AsModuleContext() runtime.ModuleContext { return nil }
func (m *mockStepContextForUpload) AsTaskContext() runtime.TaskContext { return nil }

// GlobalWorkDir, ClusterConfig, etc., are direct fields on mockStepContextForUpload
// Logger, GoCtx are direct fields.

// Implement missing runner.Runner methods for mockRunnerForUploadFile
func (m *mockRunnerForUploadFile) Render(ctx context.Context, conn connector.Connector, tmpl *template.Template, data interface{}, destPath, permissions string, sudo bool) error { return nil }
func (m *mockRunnerForUploadFile) UserExists(ctx context.Context, conn connector.Connector, username string) (bool, error) { return false, nil }
func (m *mockRunnerForUploadFile) GroupExists(ctx context.Context, conn connector.Connector, groupname string) (bool, error) { return false, nil }
func (m *mockRunnerForUploadFile) AddUser(ctx context.Context, conn connector.Connector, username, group, shell string, homeDir string, createHome bool, systemUser bool) error { return nil }
func (m *mockRunnerForUploadFile) AddGroup(ctx context.Context, conn connector.Connector, groupname string, systemGroup bool) error { return nil }
func (m *mockRunnerForUploadFile) GetPipelineCache() cache.PipelineCache {return nil}

// Add dummy methods to mockStepContextForUpload to fully satisfy step.StepContext if runtime.Context methods were not directly used.
// These are now mostly covered by direct field access or specific implementations above.
