package docker

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

// Reusing mockStepContextForDockerCleanup and mockRunnerForDockerCleanup
// from cleanup_docker_config_step_test.go, adding UploadFileFunc to the runner mock.

type mockRunnerForDistributeCriD struct {
	runner.Runner
	ExistsFunc     func(ctx context.Context, conn connector.Connector, path string) (bool, error)
	MkdirpFunc     func(ctx context.Context, conn connector.Connector, path, permissions string, sudo bool) error
	UploadFileFunc func(ctx context.Context, localSrcPath, remoteDestPath string, options *connector.FileTransferOptions, targetHost connector.Host) error
	RemoveFunc     func(ctx context.Context, conn connector.Connector, path string, sudo bool) error
}

func (m *mockRunnerForDistributeCriD) Exists(ctx context.Context, conn connector.Connector, path string) (bool, error) {
	if m.ExistsFunc != nil {
		return m.ExistsFunc(ctx, conn, path)
	}
	return false, fmt.Errorf("ExistsFunc not implemented")
}
func (m *mockRunnerForDistributeCriD) Mkdirp(ctx context.Context, conn connector.Connector, path, permissions string, sudo bool) error {
	if m.MkdirpFunc != nil {
		return m.MkdirpFunc(ctx, conn, path, permissions, sudo)
	}
	return fmt.Errorf("MkdirpFunc not implemented")
}
func (m *mockRunnerForDistributeCriD) UploadFile(ctx context.Context, localSrcPath, remoteDestPath string, options *connector.FileTransferOptions, targetHost connector.Host) error {
	if m.UploadFileFunc != nil {
		return m.UploadFileFunc(ctx, localSrcPath, remoteDestPath, options, targetHost)
	}
	return fmt.Errorf("UploadFileFunc not implemented")
}
func (m *mockRunnerForDistributeCriD) Remove(ctx context.Context, conn connector.Connector, path string, sudo bool) error {
	if m.RemoveFunc != nil {
		return m.RemoveFunc(ctx, conn, path, sudo)
	}
	return fmt.Errorf("RemoveFunc not implemented")
}


func TestDistributeCriDockerdArchiveStep_New(t *testing.T) {
	s := NewDistributeCriDockerdArchiveStep("TestDistributeCriD", "localKeyCriD", "/remote/tmp/crid", "crid.tgz", "remoteKeyCriD", true)
	require.NotNil(t, s)
	meta := s.Meta()
	assert.Equal(t, "TestDistributeCriD", meta.Name)
	assert.Equal(t, "Uploads the cri-dockerd archive to target nodes.", meta.Description)

	dcas, ok := s.(*DistributeCriDockerdArchiveStep)
	require.True(t, ok)
	assert.Equal(t, "localKeyCriD", dcas.LocalArchivePathCacheKey)
	assert.Equal(t, "/remote/tmp/crid", dcas.RemoteTempDir)
	assert.Equal(t, "crid.tgz", dcas.RemoteArchiveName)
	assert.Equal(t, "remoteKeyCriD", dcas.OutputRemotePathCacheKey)
	assert.True(t, dcas.Sudo)

	sDefaults := NewDistributeCriDockerdArchiveStep("", "", "", "cri-d-default.tgz", "", false)
	dcasDefaults, _ := sDefaults.(*DistributeCriDockerdArchiveStep)
	assert.Equal(t, "DistributeCriDockerdArchive", dcasDefaults.Meta().Name)
	assert.Equal(t, CriDockerdArchiveLocalPathCacheKey, dcasDefaults.LocalArchivePathCacheKey)
	assert.Equal(t, "/tmp/kubexm-archives", dcasDefaults.RemoteTempDir)
	assert.Equal(t, CriDockerdArchiveRemotePathCacheKey, dcasDefaults.OutputRemotePathCacheKey)
	assert.False(t, dcasDefaults.Sudo)
}

func TestDistributeCriDockerdArchiveStep_Run_Success(t *testing.T) {
	mockRunner := &mockRunnerForDistributeCriD{}
	localArchivePath := "/control/path/cri-dockerd-v0.3.0.tgz"
	taskCache := map[string]interface{}{CriDockerdArchiveLocalPathCacheKey: localArchivePath}
	mockCtx := mockStepContextForDockerCleanup(t, mockRunner, "host-run-dist-crid")
	// Need to inject task cache into mockCtx if mockStepContextForDockerCleanup doesn't handle it
	if tc, ok := mockCtx.(*runtime.Context); ok { // Assuming mockStepContextForDockerCleanup returns *runtime.Context
		tc.SetTaskCache(cache.NewTaskCache()) // Ensure TaskCache is not nil
		for k, v := range taskCache {
			tc.TaskCache().Set(k, v)
		}
	}


	remoteTempDir := "/opt/kubexm_stage/cri-d"
	remoteArchiveName := "cri-dockerd-on-remote.tgz"
	s := NewDistributeCriDockerdArchiveStep("", CriDockerdArchiveLocalPathCacheKey, remoteTempDir, remoteArchiveName, CriDockerdArchiveRemotePathCacheKey, true).(*DistributeCriDockerdArchiveStep)

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
	assert.True(t, mkdirCalled, "Mkdirp should have been called")
	assert.True(t, uploadCalled, "UploadFile should have been called")

	cachedPath, found := mockCtx.TaskCache().Get(CriDockerdArchiveRemotePathCacheKey)
	assert.True(t, found)
	assert.Equal(t, expectedRemotePath, cachedPath)
}


// Ensure mockRunnerForDistributeCriD implements runner.Runner
var _ runner.Runner = (*mockRunnerForDistributeCriD)(nil)
// Ensure mockStepContextForDockerCleanup implements step.StepContext
var _ step.StepContext = (*mockStepContextForDockerCleanup)(t, nil, "")

// Add dummy implementations for other runner.Runner methods for mockRunnerForDistributeCriD
func (m *mockRunnerForDistributeCriD) GatherFacts(ctx context.Context, conn connector.Connector) (*runner.Facts, error) { return nil, nil }
func (m *mockRunnerForDistributeCriD) Run(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (string, error) { return "", nil }
func (m *mockRunnerForDistributeCriD) MustRun(ctx context.Context, conn connector.Connector, cmd string, sudo bool) string { return "" }
func (m *mockRunnerForDistributeCriD) Check(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (bool, error) { return false, nil }
func (m *mockRunnerForDistributeCriD) RunWithOptions(ctx context.Context, conn connector.Connector, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) { return nil,nil, nil }
func (m *mockRunnerForDistributeCriD) Download(ctx context.Context, conn connector.Connector, facts *runner.Facts, url, destPath string, sudo bool) error { return nil }
func (m *mockRunnerForDistributeCriD) Extract(ctx context.Context, conn connector.Connector, facts *runner.Facts, archivePath, destDir string, sudo bool) error { return nil }
func (m *mockRunnerForDistributeCriD) DownloadAndExtract(ctx context.Context, conn connector.Connector, facts *runner.Facts, url, destDir string, sudo bool) error { return nil }
func (m *mockRunnerForDistributeCriD) IsDir(ctx context.Context, conn connector.Connector, path string) (bool, error) { return false, nil }
func (m *mockRunnerForDistributeCriD) ReadFile(ctx context.Context, conn connector.Connector, path string) ([]byte, error) { return nil, nil }
func (m *mockRunnerForDistributeCriD) WriteFile(ctx context.Context, conn connector.Connector, content []byte, destPath, permissions string, sudo bool) error { return nil }
func (m *mockRunnerForDistributeCriD) Chmod(ctx context.Context, conn connector.Connector, path string, permissions string, sudo bool) error { return nil }
func (m *mockRunnerForDistributeCriD) Chown(ctx context.Context, conn connector.Connector, path, owner, group string, recursive bool) error { return nil }
func (m *mockRunnerForDistributeCriD) GetSHA256(ctx context.Context, conn connector.Connector, path string) (string, error) { return "", nil }
func (m *mockRunnerForDistributeCriD) LookPath(ctx context.Context, conn connector.Connector, file string) (string, error) { return "", nil }
func (m *mockRunnerForDistributeCriD) IsPortOpen(ctx context.Context, conn connector.Connector, facts *runner.Facts, port int) (bool, error) { return false, nil }
func (m *mockRunnerForDistributeCriD) WaitForPort(ctx context.Context, conn connector.Connector, facts *runner.Facts, port int, timeout time.Duration) error { return nil }
func (m *mockRunnerForDistributeCriD) SetHostname(ctx context.Context, conn connector.Connector, facts *runner.Facts, hostname string) error { return nil }
func (m *mockRunnerForDistributeCriD) AddHostEntry(ctx context.Context, conn connector.Connector, ip, fqdn string, hostnames ...string) error { return nil }
func (m *mockRunnerForDistributeCriD) InstallPackages(ctx context.Context, conn connector.Connector, facts *runner.Facts, packages ...string) error { return nil }
func (m *mockRunnerForDistributeCriD) RemovePackages(ctx context.Context, conn connector.Connector, facts *runner.Facts, packages ...string) error { return nil }
func (m *mockRunnerForDistributeCriD) UpdatePackageCache(ctx context.Context, conn connector.Connector, facts *runner.Facts) error { return nil }
func (m *mockRunnerForDistributeCriD) IsPackageInstalled(ctx context.Context, conn connector.Connector, facts *runner.Facts, packageName string) (bool, error) { return false, nil }
func (m *mockRunnerForDistributeCriD) AddRepository(ctx context.Context, conn connector.Connector, facts *runner.Facts, repoConfig string, isFilePath bool) error { return nil }
func (m *mockRunnerForDistributeCriD) StartService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForDistributeCriD) StopService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForDistributeCriD) RestartService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForDistributeCriD) EnableService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForDistributeCriD) DisableService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForDistributeCriD) IsServiceActive(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) (bool, error) { return false, nil }
func (m *mockRunnerForDistributeCriD) DaemonReload(ctx context.Context, conn connector.Connector, facts *runner.Facts) error { return nil }
func (m *mockRunnerForDistributeCriD) Render(ctx context.Context, conn connector.Connector, tmpl *template.Template, data interface{}, destPath, permissions string, sudo bool) error { return nil }
func (m *mockRunnerForDistributeCriD) UserExists(ctx context.Context, conn connector.Connector, username string) (bool, error) { return false, nil }
func (m *mockRunnerForDistributeCriD) GroupExists(ctx context.Context, conn connector.Connector, groupname string) (bool, error) { return false, nil }
func (m *mockRunnerForDistributeCriD) AddUser(ctx context.Context, conn connector.Connector, username, group, shell string, homeDir string, createHome bool, systemUser bool) error { return nil }
func (m *mockRunnerForDistributeCriD) AddGroup(ctx context.Context, conn connector.Connector, groupname string, systemGroup bool) error { return nil }
func (m *mockRunnerForDistributeCriD) GetPipelineCache() cache.PipelineCache { return nil }


func TestMockContextImplementation_DockerDistributeCriD(t *testing.T) {
	var _ step.StepContext = mockStepContextForDockerCleanup(t, &mockRunnerForDistributeCriD{}, "dummy")
}
