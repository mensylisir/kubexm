package containerd

import (
	"context"
	"fmt"
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
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step"
)

// mockStepContextForDistribute is a helper to create a StepContext for testing distribution steps.
func mockStepContextForDistribute(t *testing.T, mockRunner runner.Runner, hostName string, taskCacheValues map[string]interface{}) step.StepContext {
	t.Helper()
	l, _ := logger.New(logger.DefaultOptions())
	mainCtx := &runtime.Context{
		GoCtx:         context.Background(),
		Logger:        l,
		ClusterConfig: &v1alpha1.Cluster{ObjectMeta: v1alpha1.ObjectMeta{Name: "test-cluster-distribute"}},
		Runner:        mockRunner,
		StepCache:     cache.NewStepCache(),
		TaskCache:     cache.NewTaskCache(), // Ensure TaskCache is initialized
		GlobalWorkDir: "/tmp/kubexm_distribute_test",
		hostInfoMap:   make(map[string]*runtime.HostRuntimeInfo),
	}

	if hostName == "" {
		hostName = "test-host-distribute"
	}
	hostSpec := v1alpha1.HostSpec{Name: hostName, Address: "127.0.0.1", Type: "local"}
	currentHost := connector.NewHostFromSpec(hostSpec)
	mainCtx.GetHostInfoMap()[hostName] = &runtime.HostRuntimeInfo{
		Host:  currentHost,
		Conn:  &connector.LocalConnector{},
		Facts: &runner.Facts{OS: &connector.OS{ID: "linux"}},
	}
	mainCtx.SetCurrentHost(currentHost)
	mainCtx.SetControlNode(currentHost) // Assuming this step might be orchestrated from control node context

	if taskCacheValues != nil {
		for k, v := range taskCacheValues {
			mainCtx.TaskCache().Set(k, v)
		}
	}
	return mainCtx
}

// mockRunnerForDistribute provides a mock implementation of runner.Runner.
type mockRunnerForDistribute struct {
	runner.Runner
	ExistsFunc     func(ctx context.Context, conn connector.Connector, path string) (bool, error)
	MkdirpFunc     func(ctx context.Context, conn connector.Connector, path, permissions string, sudo bool) error
	UploadFileFunc func(ctx context.Context, localSrcPath, remoteDestPath string, options *connector.FileTransferOptions, targetHost connector.Host) error
	RemoveFunc     func(ctx context.Context, conn connector.Connector, path string, sudo bool) error
}

func (m *mockRunnerForDistribute) Exists(ctx context.Context, conn connector.Connector, path string) (bool, error) {
	if m.ExistsFunc != nil {
		return m.ExistsFunc(ctx, conn, path)
	}
	return false, fmt.Errorf("ExistsFunc not implemented")
}
func (m *mockRunnerForDistribute) Mkdirp(ctx context.Context, conn connector.Connector, path, permissions string, sudo bool) error {
	if m.MkdirpFunc != nil {
		return m.MkdirpFunc(ctx, conn, path, permissions, sudo)
	}
	return fmt.Errorf("MkdirpFunc not implemented")
}
func (m *mockRunnerForDistribute) UploadFile(ctx context.Context, localSrcPath, remoteDestPath string, options *connector.FileTransferOptions, targetHost connector.Host) error {
	if m.UploadFileFunc != nil {
		return m.UploadFileFunc(ctx, localSrcPath, remoteDestPath, options, targetHost)
	}
	return fmt.Errorf("UploadFileFunc not implemented")
}
func (m *mockRunnerForDistribute) Remove(ctx context.Context, conn connector.Connector, path string, sudo bool) error {
	if m.RemoveFunc != nil {
		return m.RemoveFunc(ctx, conn, path, sudo)
	}
	return fmt.Errorf("RemoveFunc not implemented")
}

func TestDistributeCNIPluginsArchiveStep_New(t *testing.T) {
	s := NewDistributeCNIPluginsArchiveStep("TestDistributeCNI", "localKey", "/remote/tmp", "cni.tgz", "remoteKey", true)
	require.NotNil(t, s)
	meta := s.Meta()
	assert.Equal(t, "TestDistributeCNI", meta.Name)
	assert.Equal(t, "Uploads the CNI plugins archive to target nodes.", meta.Description)

	dcas, ok := s.(*DistributeCNIPluginsArchiveStep)
	require.True(t, ok)
	assert.Equal(t, "localKey", dcas.LocalArchivePathCacheKey)
	assert.Equal(t, "/remote/tmp", dcas.RemoteTempDir)
	assert.Equal(t, "cni.tgz", dcas.RemoteArchiveName)
	assert.Equal(t, "remoteKey", dcas.OutputRemotePathCacheKey)
	assert.True(t, dcas.Sudo)

	sDefaults := NewDistributeCNIPluginsArchiveStep("", "", "", "cni-plugins-default.tgz", "", false)
	dcasDefaults, _ := sDefaults.(*DistributeCNIPluginsArchiveStep)
	assert.Equal(t, "DistributeCNIPluginsArchive", dcasDefaults.Meta().Name)
	assert.Equal(t, CNIPluginsArchiveLocalPathCacheKey, dcasDefaults.LocalArchivePathCacheKey)
	assert.Equal(t, "/tmp/kubexm-archives", dcasDefaults.RemoteTempDir)
	assert.Equal(t, CNIPluginsArchiveRemotePathCacheKey, dcasDefaults.OutputRemotePathCacheKey)
	assert.False(t, dcasDefaults.Sudo)
}

func TestDistributeCNIPluginsArchiveStep_Precheck_RemoteExists(t *testing.T) {
	mockRunner := &mockRunnerForDistribute{}
	mockCtx := mockStepContextForDistribute(t, mockRunner, "host1", nil)

	remoteTempDir := "/var/tmp/kubexm_cni"
	remoteArchiveName := "cni_plugins_v1.tar.gz"
	expectedRemotePath := filepath.Join(remoteTempDir, remoteArchiveName)
	s := NewDistributeCNIPluginsArchiveStep("", "", remoteTempDir, remoteArchiveName, "", true).(*DistributeCNIPluginsArchiveStep)

	mockRunner.ExistsFunc = func(ctx context.Context, conn connector.Connector, path string) (bool, error) {
		if path == expectedRemotePath {
			return true, nil
		}
		return false, nil
	}

	done, err := s.Precheck(mockCtx, mockCtx.GetHost())
	require.NoError(t, err)
	assert.True(t, done, "Precheck should be done if remote archive exists")

	cachedPath, found := mockCtx.TaskCache().Get(s.OutputRemotePathCacheKey)
	assert.True(t, found)
	assert.Equal(t, expectedRemotePath, cachedPath)
}

func TestDistributeCNIPluginsArchiveStep_Precheck_RemoteNotExists(t *testing.T) {
	mockRunner := &mockRunnerForDistribute{}
	mockCtx := mockStepContextForDistribute(t, mockRunner, "host1", nil)

	remoteTempDir := "/var/tmp/kubexm_cni"
	remoteArchiveName := "cni_plugins_v1_notthere.tar.gz"
	s := NewDistributeCNIPluginsArchiveStep("", "", remoteTempDir, remoteArchiveName, "", true).(*DistributeCNIPluginsArchiveStep)

	mockRunner.ExistsFunc = func(ctx context.Context, conn connector.Connector, path string) (bool, error) {
		return false, nil
	}

	done, err := s.Precheck(mockCtx, mockCtx.GetHost())
	require.NoError(t, err)
	assert.False(t, done, "Precheck should not be done if remote archive does not exist")
}

func TestDistributeCNIPluginsArchiveStep_Run_Success(t *testing.T) {
	mockRunner := &mockRunnerForDistribute{}
	localArchivePath := "/control/path/cni-plugins.v1.0.0.tgz"
	taskCache := map[string]interface{}{CNIPluginsArchiveLocalPathCacheKey: localArchivePath}
	mockCtx := mockStepContextForDistribute(t, mockRunner, "host-run-cni", taskCache)

	remoteTempDir := "/opt/kubexm_stage"
	remoteArchiveName := "cni-plugins-on-remote.tgz" // Explicitly set
	s := NewDistributeCNIPluginsArchiveStep("", CNIPluginsArchiveLocalPathCacheKey, remoteTempDir, remoteArchiveName, CNIPluginsArchiveRemotePathCacheKey, true).(*DistributeCNIPluginsArchiveStep)

	expectedRemotePath := filepath.Join(remoteTempDir, remoteArchiveName)
	var mkdirCalled, uploadCalled bool

	mockRunner.MkdirpFunc = func(ctx context.Context, conn connector.Connector, path, permissions string, sudo bool) error {
		if path == remoteTempDir {
			mkdirCalled = true
			assert.True(t, sudo)
			assert.Equal(t, "0750", permissions)
			return nil
		}
		return fmt.Errorf("unexpected Mkdirp call: %s", path)
	}
	mockRunner.UploadFileFunc = func(ctx context.Context, localSrc, remoteDest string, options *connector.FileTransferOptions, targetHost connector.Host) error {
		if localSrc == localArchivePath && remoteDest == expectedRemotePath {
			uploadCalled = true
			assert.True(t, options.Sudo)
			assert.Equal(t, "0640", options.Permissions)
			assert.Equal(t, mockCtx.GetHost().GetName(), targetHost.GetName())
			return nil
		}
		return fmt.Errorf("unexpected UploadFile call: local=%s, remote=%s", localSrc, remoteDest)
	}

	err := s.Run(mockCtx, mockCtx.GetHost())
	require.NoError(t, err)
	assert.True(t, mkdirCalled, "Mkdirp should have been called for remote temp directory")
	assert.True(t, uploadCalled, "UploadFile should have been called")

	cachedPath, found := mockCtx.TaskCache().Get(CNIPluginsArchiveRemotePathCacheKey)
	assert.True(t, found)
	assert.Equal(t, expectedRemotePath, cachedPath)
}

func TestDistributeCNIPluginsArchiveStep_Run_DeriveRemoteName(t *testing.T) {
	mockRunner := &mockRunnerForDistribute{}
	localArchivePath := "/control/path/cni-plugins-special.v1.1.0.tgz"
	taskCache := map[string]interface{}{CNIPluginsArchiveLocalPathCacheKey: localArchivePath}
	mockCtx := mockStepContextForDistribute(t, mockRunner, "host-derive-name", taskCache)

	remoteTempDir := "/opt/kubexm_stage"
	// RemoteArchiveName is empty, should be derived from localArchivePath
	s := NewDistributeCNIPluginsArchiveStep("", CNIPluginsArchiveLocalPathCacheKey, remoteTempDir, "", CNIPluginsArchiveRemotePathCacheKey, false).(*DistributeCNIPluginsArchiveStep)

	expectedDerivedName := "cni-plugins-special.v1.1.0.tgz"
	expectedRemotePath := filepath.Join(remoteTempDir, expectedDerivedName)
	var uploadCalled bool

	mockRunner.MkdirpFunc = func(ctx context.Context, conn connector.Connector, path, permissions string, sudo bool) error { return nil }
	mockRunner.UploadFileFunc = func(ctx context.Context, localSrc, remoteDest string, options *connector.FileTransferOptions, targetHost connector.Host) error {
		if localSrc == localArchivePath && remoteDest == expectedRemotePath {
			uploadCalled = true
			assert.False(t, options.Sudo) // Sudo for step is false
			return nil
		}
		return fmt.Errorf("UploadFile call with unexpected remoteDest: %s, expected %s", remoteDest, expectedRemotePath)
	}

	err := s.Run(mockCtx, mockCtx.GetHost())
	require.NoError(t, err)
	assert.True(t, uploadCalled, "UploadFile should have been called with derived remote name")
	assert.Equal(t, expectedDerivedName, s.RemoteArchiveName, "RemoteArchiveName should be updated in the step struct")
}


func TestDistributeCNIPluginsArchiveStep_Rollback(t *testing.T) {
	mockRunner := &mockRunnerForDistribute{}
	remoteTempDir := "/tmp/to-clean"
	remoteArchiveName := "cni-plugins-for-rollback.tgz"
	expectedRemotePath := filepath.Join(remoteTempDir, remoteArchiveName)

	taskCache := map[string]interface{}{CNIPluginsArchiveRemotePathCacheKey: expectedRemotePath}
	mockCtx := mockStepContextForDistribute(t, mockRunner, "host-rollback-cni", taskCache)

	s := NewDistributeCNIPluginsArchiveStep("", "", remoteTempDir, "", CNIPluginsArchiveRemotePathCacheKey, true).(*DistributeCNIPluginsArchiveStep)
	// s.RemoteArchiveName will be derived in Rollback from cache key

	var removeCalledWithPath string
	mockRunner.RemoveFunc = func(ctx context.Context, conn connector.Connector, path string, sudo bool) error {
		removeCalledWithPath = path
		assert.True(t, sudo)
		return nil
	}

	err := s.Rollback(mockCtx, mockCtx.GetHost())
	require.NoError(t, err)
	assert.Equal(t, expectedRemotePath, removeCalledWithPath)
	_, found := mockCtx.TaskCache().Get(CNIPluginsArchiveRemotePathCacheKey)
	assert.False(t, found, "Cache key should be deleted on rollback")
}

var _ runner.Runner = (*mockRunnerForDistribute)(nil)
var _ step.StepContext = (*mockStepContextForDistribute)(t, nil, "", nil)

// Add dummy implementations for other runner.Runner methods for mockRunnerForDistribute
func (m *mockRunnerForDistribute) GatherFacts(ctx context.Context, conn connector.Connector) (*runner.Facts, error) { return nil, nil }
func (m *mockRunnerForDistribute) Run(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (string, error) { return "", nil }
func (m *mockRunnerForDistribute) MustRun(ctx context.Context, conn connector.Connector, cmd string, sudo bool) string { return "" }
func (m *mockRunnerForDistribute) Check(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (bool, error) { return false, nil }
func (m *mockRunnerForDistribute) RunWithOptions(ctx context.Context, conn connector.Connector, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) { return nil,nil, nil }
func (m *mockRunnerForDistribute) Download(ctx context.Context, conn connector.Connector, facts *runner.Facts, url, destPath string, sudo bool) error { return nil }
func (m *mockRunnerForDistribute) Extract(ctx context.Context, conn connector.Connector, facts *runner.Facts, archivePath, destDir string, sudo bool) error { return nil }
func (m *mockRunnerForDistribute) DownloadAndExtract(ctx context.Context, conn connector.Connector, facts *runner.Facts, url, destDir string, sudo bool) error { return nil }
func (m *mockRunnerForDistribute) IsDir(ctx context.Context, conn connector.Connector, path string) (bool, error) { return false, nil }
func (m *mockRunnerForDistribute) ReadFile(ctx context.Context, conn connector.Connector, path string) ([]byte, error) { return nil, nil }
func (m *mockRunnerForDistribute) WriteFile(ctx context.Context, conn connector.Connector, content []byte, destPath, permissions string, sudo bool) error { return nil }
func (m *mockRunnerForDistribute) Chmod(ctx context.Context, conn connector.Connector, path string, permissions string, sudo bool) error { return nil }
func (m *mockRunnerForDistribute) Chown(ctx context.Context, conn connector.Connector, path, owner, group string, recursive bool) error { return nil }
func (m *mockRunnerForDistribute) GetSHA256(ctx context.Context, conn connector.Connector, path string) (string, error) { return "", nil }
func (m *mockRunnerForDistribute) LookPath(ctx context.Context, conn connector.Connector, file string) (string, error) { return "", nil }
func (m *mockRunnerForDistribute) IsPortOpen(ctx context.Context, conn connector.Connector, facts *runner.Facts, port int) (bool, error) { return false, nil }
func (m *mockRunnerForDistribute) WaitForPort(ctx context.Context, conn connector.Connector, facts *runner.Facts, port int, timeout time.Duration) error { return nil }
func (m *mockRunnerForDistribute) SetHostname(ctx context.Context, conn connector.Connector, facts *runner.Facts, hostname string) error { return nil }
func (m *mockRunnerForDistribute) AddHostEntry(ctx context.Context, conn connector.Connector, ip, fqdn string, hostnames ...string) error { return nil }
func (m *mockRunnerForDistribute) InstallPackages(ctx context.Context, conn connector.Connector, facts *runner.Facts, packages ...string) error { return nil }
func (m *mockRunnerForDistribute) RemovePackages(ctx context.Context, conn connector.Connector, facts *runner.Facts, packages ...string) error { return nil }
func (m *mockRunnerForDistribute) UpdatePackageCache(ctx context.Context, conn connector.Connector, facts *runner.Facts) error { return nil }
func (m *mockRunnerForDistribute) IsPackageInstalled(ctx context.Context, conn connector.Connector, facts *runner.Facts, packageName string) (bool, error) { return false, nil }
func (m *mockRunnerForDistribute) AddRepository(ctx context.Context, conn connector.Connector, facts *runner.Facts, repoConfig string, isFilePath bool) error { return nil }
func (m *mockRunnerForDistribute) StartService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForDistribute) StopService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForDistribute) RestartService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForDistribute) EnableService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForDistribute) DisableService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForDistribute) IsServiceActive(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) (bool, error) { return false, nil }
func (m *mockRunnerForDistribute) DaemonReload(ctx context.Context, conn connector.Connector, facts *runner.Facts) error { return nil }
func (m *mockRunnerForDistribute) Render(ctx context.Context, conn connector.Connector, tmpl *template.Template, data interface{}, destPath, permissions string, sudo bool) error { return nil }
func (m *mockRunnerForDistribute) UserExists(ctx context.Context, conn connector.Connector, username string) (bool, error) { return false, nil }
func (m *mockRunnerForDistribute) GroupExists(ctx context.Context, conn connector.Connector, groupname string) (bool, error) { return false, nil }
func (m *mockRunnerForDistribute) AddUser(ctx context.Context, conn connector.Connector, username, group, shell string, homeDir string, createHome bool, systemUser bool) error { return nil }
func (m *mockRunnerForDistribute) AddGroup(ctx context.Context, conn connector.Connector, groupname string, systemGroup bool) error { return nil }

// Add remaining StepContext methods for mockStepContextForDistribute
func (m *mockStepContextForDistribute) GetHost() connector.Host { return m.mainCtx.GetHost() }
func (m *mockStepContextForDistribute) StepCache() cache.StepCache { return m.mainCtx.StepCache() }
func (m *mockStepContextForDistribute) ModuleCache() cache.ModuleCache { return m.mainCtx.ModuleCache() }
func (m *mockStepContextForDistribute) GetHostsByRole(role string) ([]connector.Host, error) { return m.mainCtx.GetHostsByRole(role) }
func (m *mockStepContextForDistribute) GetHostFacts(host connector.Host) (*runner.Facts, error) { return m.mainCtx.GetHostFacts(host) }
func (m *mockStepContextForDistribute) GetCurrentHostFacts() (*runner.Facts, error) { return m.mainCtx.GetCurrentHostFacts() }
func (m *mockStepContextForDistribute) GetCurrentHostConnector() (connector.Connector, error) { return m.mainCtx.GetCurrentHostConnector() }
func (m *mockStepContextForDistribute) IsVerbose() bool { return m.mainCtx.IsVerbose() }
func (m *mockStepContextForDistribute) ShouldIgnoreErr() bool { return m.mainCtx.ShouldIgnoreErr() }
func (m *mockStepContextForDistribute) GetGlobalConnectionTimeout() time.Duration { return m.mainCtx.GetGlobalConnectionTimeout() }
func (m *mockStepContextForDistribute) GetClusterArtifactsDir() string { return m.mainCtx.GetClusterArtifactsDir() }
func (m *mockStepContextForDistribute) GetCertsDir() string { return m.mainCtx.GetCertsDir() }
func (m *mockStepContextForDistribute) GetEtcdCertsDir() string { return m.mainCtx.GetEtcdCertsDir() }
func (m *mockStepContextForDistribute) GetComponentArtifactsDir(cn string) string { return m.mainCtx.GetComponentArtifactsDir(cn) }
func (m *mockStepContextForDistribute) GetEtcdArtifactsDir() string { return m.mainCtx.GetEtcdArtifactsDir() }
func (m *mockStepContextForDistribute) GetContainerRuntimeArtifactsDir() string { return m.mainCtx.GetContainerRuntimeArtifactsDir() }
func (m *mockStepContextForDistribute) GetKubernetesArtifactsDir() string { return m.mainCtx.GetKubernetesArtifactsDir() }
func (m *mockStepContextForDistribute) GetFileDownloadPath(c,v,a,f string) string { return m.mainCtx.GetFileDownloadPath(c,v,a,f) }
func (m *mockStepContextForDistribute) GetHostDir(hn string) string { return m.mainCtx.GetHostDir(hn) }
func (m *mockStepContextForDistribute) WithGoContext(gCtx context.Context) step.StepContext { return m.mainCtx.WithGoContext(gCtx) }
// Already implemented in helper
// func (m *mockStepContextForDistribute) GetLogger() *logger.Logger { return m.mainCtx.GetLogger() }
// func (m *mockStepContextForDistribute) GoContext() context.Context { return m.mainCtx.GoContext() }
// func (m *mockStepContextForDistribute) GetRunner() runner.Runner { return m.mainCtx.GetRunner() }
// func (m *mockStepContextForDistribute) GetClusterConfig() *v1alpha1.Cluster { return m.mainCtx.GetClusterConfig() }
// func (m *mockStepContextForDistribute) GetControlNode() (connector.Host, error) { return m.mainCtx.GetControlNode() }
// func (m *mockStepContextForDistribute) GetGlobalWorkDir() string { return m.mainCtx.GetGlobalWorkDir() }
// func (m *mockStepContextForDistribute) GetConnectorForHost(h connector.Host) (connector.Connector, error) { return m.mainCtx.GetConnectorForHost(h) }
// func (m *mockStepContextForDistribute) TaskCache() cache.TaskCache { return m.mainCtx.TaskCache() }

// Fields for mockStepContextForDistribute to match runtime.Context fields used by its methods if not directly embedding
type mockStepContextForDistribute struct {
	mainCtx *runtime.Context // Embed the actual runtime.Context or a mock that implements step.StepContext
}
