package etcd

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time" // Added for MockTestRunner
	"text/template" // Added for MockTestRunner

	"github.com/stretchr/testify/assert"
	testmock "github.com/stretchr/testify/mock" // Alias for testify's mock

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/logger" // Added
	"github.com/mensylisir/kubexm/pkg/runner" // Added
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step"
)

// MockTestRunner (copied from copy_etcd_binaries_to_path_test.go, consider moving to a shared test helper)
type MockTestRunner struct {
	testmock.Mock
	Connector connector.Connector
}
func (m *MockTestRunner) Exists(ctx context.Context, conn connector.Connector, path string) (bool, error) {
	args := m.Called(ctx, conn, path)
	return args.Bool(0), args.Error(1)
}
func (m *MockTestRunner) Mkdirp(ctx context.Context, conn connector.Connector, path, permissions string, sudo bool) error {
	args := m.Called(ctx, conn, path, permissions, sudo)
	return args.Error(0)
}
func (m *MockTestRunner) UploadFile(ctx context.Context, conn connector.Connector, src, dst string, options *connector.FileTransferOptions) error {
	args := m.Called(ctx, conn, src, dst, options)
	return args.Error(0)
}
func (m *MockTestRunner) Remove(ctx context.Context, conn connector.Connector, path string, sudo bool) error {
	args := m.Called(ctx, conn, path, sudo)
	return args.Error(0)
}
// Add other runner.Runner methods if they get called, returning nil or default values.
func (m *MockTestRunner) GatherFacts(ctx context.Context, conn connector.Connector) (*runner.Facts, error) { return nil, nil }
func (m *MockTestRunner) Run(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (string, error) { return "", nil }
func (m *MockTestRunner) MustRun(ctx context.Context, conn connector.Connector, cmd string, sudo bool) string { return "" }
func (m *MockTestRunner) Check(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (bool, error) { return false, nil }
func (m *MockTestRunner) RunWithOptions(ctx context.Context, conn connector.Connector, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) { return nil,nil, nil }
func (m *MockTestRunner) Download(ctx context.Context, conn connector.Connector, facts *runner.Facts, url, destPath string, sudo bool) error { return nil }
func (m *MockTestRunner) Extract(ctx context.Context, conn connector.Connector, facts *runner.Facts, archivePath, destDir string, sudo bool) error { return nil }
func (m *MockTestRunner) DownloadAndExtract(ctx context.Context, conn connector.Connector, facts *runner.Facts, url, destDir string, sudo bool) error { return nil }
func (m *MockTestRunner) IsDir(ctx context.Context, conn connector.Connector, path string) (bool, error) { return false, nil }
func (m *MockTestRunner) ReadFile(ctx context.Context, conn connector.Connector, path string) ([]byte, error) { return nil, nil }
func (m *MockTestRunner) WriteFile(ctx context.Context, conn connector.Connector, content []byte, destPath, permissions string, sudo bool) error { return nil }
func (m *MockTestRunner) Chmod(ctx context.Context, conn connector.Connector, path string, permissions string, sudo bool) error { return nil }
func (m *MockTestRunner) Chown(ctx context.Context, conn connector.Connector, path, owner, group string, recursive bool) error { return nil }
func (m *MockTestRunner) GetSHA256(ctx context.Context, conn connector.Connector, path string) (string, error) { return "", nil }
func (m *MockTestRunner) LookPath(ctx context.Context, conn connector.Connector, file string) (string, error) { return "", nil }
func (m *MockTestRunner) IsPortOpen(ctx context.Context, conn connector.Connector, facts *runner.Facts, port int) (bool, error) { return false, nil }
func (m *MockTestRunner) WaitForPort(ctx context.Context, conn connector.Connector, facts *runner.Facts, port int, timeout time.Duration) error { return nil }
func (m *MockTestRunner) SetHostname(ctx context.Context, conn connector.Connector, facts *runner.Facts, hostname string) error { return nil }
func (m *MockTestRunner) AddHostEntry(ctx context.Context, conn connector.Connector, ip, fqdn string, hostnames ...string) error { return nil }
func (m *MockTestRunner) InstallPackages(ctx context.Context, conn connector.Connector, facts *runner.Facts, packages ...string) error { return nil }
func (m *MockTestRunner) RemovePackages(ctx context.Context, conn connector.Connector, facts *runner.Facts, packages ...string) error { return nil }
func (m *MockTestRunner) UpdatePackageCache(ctx context.Context, conn connector.Connector, facts *runner.Facts) error { return nil }
func (m *MockTestRunner) IsPackageInstalled(ctx context.Context, conn connector.Connector, facts *runner.Facts, packageName string) (bool, error) { return false, nil }
func (m *MockTestRunner) AddRepository(ctx context.Context, conn connector.Connector, facts *runner.Facts, repoConfig string, isFilePath bool) error { return nil }
func (m *MockTestRunner) StartService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *MockTestRunner) StopService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *MockTestRunner) RestartService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *MockTestRunner) EnableService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *MockTestRunner) DisableService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *MockTestRunner) IsServiceActive(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) (bool, error) { return false, nil }
func (m *MockTestRunner) DaemonReload(ctx context.Context, conn connector.Connector, facts *runner.Facts) error { return nil }
func (m *MockTestRunner) Render(ctx context.Context, conn connector.Connector, tmpl *text.template.Template, data interface{}, destPath, permissions string, sudo bool) error { return nil }
func (m *MockTestRunner) UserExists(ctx context.Context, conn connector.Connector, username string) (bool, error) { return false, nil }
func (m *MockTestRunner) GroupExists(ctx context.Context, conn connector.Connector, groupname string) (bool, error) { return false, nil }
func (m *MockTestRunner) AddUser(ctx context.Context, conn connector.Connector, username, group, shell string, homeDir string, createHome bool, systemUser bool) error { return nil }
func (m *MockTestRunner) AddGroup(ctx context.Context, conn connector.Connector, groupname string, systemGroup bool) error { return nil }


// newTestStepEtcdContext (copied from copy_etcd_binaries_to_path_test.go, consider moving to a shared test helper)
func newTestStepEtcdContext(t *testing.T, mockRunner *MockTestRunner, cacheValues map[string]interface{}) runtime.StepContext {
	l, _ := logger.New(logger.DefaultConfig())
	mockConn := &step.MockStepConnector{}
	if mockRunner != nil {
		mockRunner.Connector = mockConn
	}
	mockHost := connector.NewHostFromSpec(v1alpha1.Host{Name: "test-etcd-host", Address: "1.2.3.4"})
	rtCtx := &runtime.Context{
		GoCtx:  context.Background(),
		Logger: l,
		Runner: mockRunner,
		HostRuntimes: map[string]*runtime.HostRuntime{
			"test-etcd-host": {
				Host:  mockHost,
				Conn:  mockConn,
				Facts: &runner.Facts{OS: &connector.OS{ID: "linux", Arch: "amd64"}},
			},
		},
		CurrentHost: mockHost,
		TaskCache:   runtime.NewTaskCache(),
	}
	if cacheValues != nil {
		for k, v := range cacheValues {
			rtCtx.TaskCache.Set(k, v)
		}
	}
	return rtCtx
}


func TestDistributeEtcdBinaryStep_Run_Success(t *testing.T) {
	mockRunner := new(MockTestRunner)
	mockConn := &step.MockStepConnector{}
	mockRunner.Connector = mockConn

	stepCtx := newTestStepEtcdContext(t, mockRunner, nil) // Pass mockRunner to context

	// Populate cache with the local archive path
	localArchivePath := "/control/node/path/etcd-v3.5.9.tar.gz"
	stepCtx.TaskCache().Set(EtcdArchiveLocalPathCacheKey, localArchivePath)

	// Step configuration
	remoteTempDir := "/tmp/kubexm-archives-test"
	remoteArchiveName := "etcd-archive-on-remote.tar.gz"

	s := NewDistributeEtcdBinaryStep(
		"TestDistributeEtcd",
		EtcdArchiveLocalPathCacheKey,
		remoteTempDir,
		remoteArchiveName,
		EtcdArchiveRemotePathCacheKey,
		true, // sudo
	)

	expectedRemoteArchivePath := filepath.Join(remoteTempDir, remoteArchiveName)

	// Mock expectations
	mockRunner.On("Mkdirp", context.TODO(), mockConn, remoteTempDir, "0750", true).Return(nil)
	mockRunner.On("UploadFile", context.TODO(), mockConn, localArchivePath, expectedRemoteArchivePath, testmock.AnythingOfType("*connector.FileTransferOptions")).Return(nil).Run(func(args testmock.Arguments) {
		opts := args.Get(3).(*connector.FileTransferOptions)
		assert.Equal(t, "0640", opts.Permissions)
		assert.True(t, opts.Sudo)
	})


	err := s.Run(stepCtx, stepCtx.GetHost())
	assert.NoError(t, err)

	// Verify cache was set
	cachedPath, found := stepCtx.TaskCache().Get(EtcdArchiveRemotePathCacheKey)
	assert.True(t, found)
	assert.Equal(t, expectedRemoteArchivePath, cachedPath)

	mockRunner.AssertExpectations(t)
}

func TestDistributeEtcdBinaryStep_Run_LocalPathNotCached(t *testing.T) {
	mockRunner := new(MockTestRunner)
	stepCtx := newTestStepEtcdContext(t, mockRunner, nil)

	s := NewDistributeEtcdBinaryStep("TestDistributeEtcd", EtcdArchiveLocalPathCacheKey, "/tmp/kubexm", "etcd.tar.gz", EtcdArchiveRemotePathCacheKey, true)

	err := s.Run(stepCtx, stepCtx.GetHost())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "local etcd archive path not found in task cache")
	mockRunner.AssertNotCalled(t, "UploadFile", context.TODO(), testmock.Anything, testmock.Anything, testmock.Anything, testmock.Anything)
}


func TestDistributeEtcdBinaryStep_Precheck_Exists(t *testing.T) {
	mockRunner := new(MockTestRunner)
	mockConn := &step.MockStepConnector{}
	mockRunner.Connector = mockConn
	stepCtx := newTestStepEtcdContext(t, mockRunner, nil)


	remoteTempDir := "/tmp/kubexm-archives-test"
	remoteArchiveName := "etcd-exists.tar.gz"
	expectedRemotePath := filepath.Join(remoteTempDir, remoteArchiveName)

	s := NewDistributeEtcdBinaryStep("TestDistributeEtcd", "", remoteTempDir, remoteArchiveName, EtcdArchiveRemotePathCacheKey, true)

	mockRunner.On("Exists", context.TODO(), mockConn, expectedRemotePath).Return(true, nil)

	done, err := s.Precheck(stepCtx, stepCtx.GetHost())
	assert.NoError(t, err)
	assert.True(t, done)

	cachedPath, found := stepCtx.TaskCache().Get(EtcdArchiveRemotePathCacheKey)
	assert.True(t, found)
	assert.Equal(t, expectedRemotePath, cachedPath)

	mockRunner.AssertExpectations(t)
}

func TestDistributeEtcdBinaryStep_Precheck_NotExists(t *testing.T) {
	mockRunner := new(MockTestRunner)
	mockConn := &step.MockStepConnector{}
	mockRunner.Connector = mockConn
	stepCtx := newTestStepEtcdContext(t, mockRunner, nil)


	remoteTempDir := "/tmp/kubexm-archives-test"
	remoteArchiveName := "etcd-notexists.tar.gz"
	expectedRemotePath := filepath.Join(remoteTempDir, remoteArchiveName)

	s := NewDistributeEtcdBinaryStep("TestDistributeEtcd", "", remoteTempDir, remoteArchiveName, EtcdArchiveRemotePathCacheKey, true)

	mockRunner.On("Exists", context.TODO(), mockConn, expectedRemotePath).Return(false, nil)

	done, err := s.Precheck(stepCtx, stepCtx.GetHost())
	assert.NoError(t, err)
	assert.False(t, done)
	mockRunner.AssertExpectations(t)
}

func TestDistributeEtcdBinaryStep_Rollback(t *testing.T) {
	mockRunner := new(MockTestRunner)
	mockConn := &step.MockStepConnector{}
	mockRunner.Connector = mockConn
	stepCtx := newTestStepEtcdContext(t, mockRunner, nil)


	remoteTempDir := "/tmp/kubexm-archives-test"
	remoteArchiveName := "etcd-to-rollback.tar.gz"
	expectedRemotePath := filepath.Join(remoteTempDir, remoteArchiveName)

	stepCtx.TaskCache().Set(EtcdArchiveRemotePathCacheKey, expectedRemotePath)

	s := NewDistributeEtcdBinaryStep(
		"TestDistributeEtcdRollback",
		EtcdArchiveLocalPathCacheKey,
		remoteTempDir,
		"",
		EtcdArchiveRemotePathCacheKey,
		true,
	)

	s.(*DistributeEtcdBinaryStep).RemoteArchiveName = remoteArchiveName


	mockRunner.On("Remove", context.TODO(), mockConn, expectedRemotePath, true).Return(nil)

	err := s.Rollback(stepCtx, stepCtx.GetHost())
	assert.NoError(t, err)

	_, found := stepCtx.TaskCache().Get(EtcdArchiveRemotePathCacheKey)
	assert.False(t, found, "Cache key should be deleted on rollback")

	mockRunner.AssertExpectations(t)
}
