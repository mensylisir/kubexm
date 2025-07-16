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
	"go.uber.org/mock/gomock"                     // Changed gomock import
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1" // Added import

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/cache"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/logger"
	"github.com/mensylisir/kubexm/pkg/runner"
	"github.com/mensylisir/kubexm/pkg/step"
)

type mockUploadContext struct {
	ctrl          *gomock.Controller
	logger        *logger.Logger
	goCtx         context.Context
	currentHost   connector.Host
	controlNode   connector.Host
	mockRunner    *mock_runner.MockRunner
	mockConnector *mock_connector.MockConnector
	globalWorkDir string
	clusterConfig *v1alpha1.Cluster
}

func newMockUploadContext(t *testing.T, currentHostName string) *mockUploadContext {
	ctrl := gomock.NewController(t)
	l, _ := logger.NewLogger(logger.DefaultOptions())
	tempGlobalWorkDir, err := ioutil.TempDir("", "test-gwd-upload-")
	require.NoError(t, err)

	var currentHost connector.Host
	controlHostSpec := v1alpha1.HostSpec{Name: common.ControlNodeHostName, Type: "local", Address: "127.0.0.1", Roles: []string{common.ControlNodeRole}, Arch: "amd64"}
	controlNode := connector.NewHostFromSpec(controlHostSpec)

	if currentHostName == "" || currentHostName == common.ControlNodeHostName {
		currentHost = controlNode
	} else {
		remoteHostSpec := v1alpha1.HostSpec{Name: currentHostName, Address: "10.0.0.1", Type: "ssh", User: "test", Port: 22, Arch: "amd64"}
		currentHost = connector.NewHostFromSpec(remoteHostSpec)
	}

	clusterCfg := &v1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "test-cluster-upload"}, // Corrected to metav1.ObjectMeta
		Spec: v1alpha1.ClusterSpec{
			Global: &v1alpha1.GlobalSpec{WorkDir: filepath.Dir(filepath.Dir(tempGlobalWorkDir))}, // $(pwd)
			Hosts:  []v1alpha1.HostSpec{controlHostSpec},
		},
	}
	// If currentHost is remote, add it to clusterConfig.Spec.Hosts for completeness if needed by other methods
	if currentHostName != common.ControlNodeHostName {
		// For HostSpec, direct assignment is fine if it's not modified by the mock context.
		// If a true deep copy is needed, it has to be done field by field or via a helper.
		// For test setup, often direct use or shallow copy is sufficient.
		hostSpecCopy := currentHost.GetHostSpec()
		clusterCfg.Spec.Hosts = append(clusterCfg.Spec.Hosts, hostSpecCopy) // Removed .DeepCopy()
	}

	return &mockUploadContext{
		ctrl:          ctrl,
		logger:        l,
		goCtx:         context.Background(),
		currentHost:   currentHost,
		controlNode:   controlNode,
		mockRunner:    mock_runner.NewMockRunner(ctrl),
		mockConnector: mock_connector.NewMockConnector(ctrl),
		globalWorkDir: tempGlobalWorkDir, // This is $(pwd)/.kubexm/test-cluster-upload effectively
		clusterConfig: clusterCfg,
	}
}

// Implement step.StepContext
func (m *mockUploadContext) GoContext() context.Context              { return m.goCtx }
func (m *mockUploadContext) GetLogger() *logger.Logger               { return m.logger }
func (m *mockUploadContext) GetHost() connector.Host                 { return m.currentHost }
func (m *mockUploadContext) GetRunner() runner.Runner                { return m.mockRunner }
func (m *mockUploadContext) GetControlNode() (connector.Host, error) { return m.controlNode, nil }
func (m *mockUploadContext) GetConnectorForHost(h connector.Host) (connector.Connector, error) {
	return m.mockConnector, nil
}
func (m *mockUploadContext) GetCurrentHostConnector() (connector.Connector, error) {
	return m.mockConnector, nil
}
func (m *mockUploadContext) GetHostFacts(h connector.Host) (*runner.Facts, error) {
	return &runner.Facts{OS: &connector.OS{Arch: "amd64", ID: "linux"}}, nil
}
func (m *mockUploadContext) GetCurrentHostFacts() (*runner.Facts, error) {
	return &runner.Facts{OS: &connector.OS{Arch: "amd64", ID: "linux"}}, nil
}
func (m *mockUploadContext) GetStepCache() cache.StepCache                        { return cache.NewStepCache() }
func (m *mockUploadContext) GetTaskCache() cache.TaskCache                        { return cache.NewTaskCache() }
func (m *mockUploadContext) GetModuleCache() cache.ModuleCache                    { return cache.NewModuleCache() }
func (m *mockUploadContext) GetPipelineCache() cache.PipelineCache                { return cache.NewPipelineCache() }
func (m *mockUploadContext) GetClusterConfig() *v1alpha1.Cluster                  { return m.clusterConfig }
func (m *mockUploadContext) GetHostsByRole(role string) ([]connector.Host, error) { return nil, nil }
func (m *mockUploadContext) GetGlobalWorkDir() string                             { return m.globalWorkDir }
func (m *mockUploadContext) IsVerbose() bool                                      { return false }
func (m *mockUploadContext) ShouldIgnoreErr() bool                                { return false }
func (m *mockUploadContext) GetGlobalConnectionTimeout() time.Duration            { return 30 * time.Second }
func (m *mockUploadContext) GetClusterArtifactsDir() string                       { return m.globalWorkDir } // globalWorkDir is already cluster specific
func (m *mockUploadContext) GetCertsDir() string {
	return filepath.Join(m.GetClusterArtifactsDir(), "certs")
}
func (m *mockUploadContext) GetEtcdCertsDir() string { return filepath.Join(m.GetCertsDir(), "etcd") }
func (m *mockUploadContext) GetComponentArtifactsDir(componentName string) string {
	return filepath.Join(m.GetClusterArtifactsDir(), componentName)
}
func (m *mockUploadContext) GetEtcdArtifactsDir() string { return m.GetComponentArtifactsDir("etcd") }
func (m *mockUploadContext) GetContainerRuntimeArtifactsDir() string {
	return m.GetComponentArtifactsDir("container_runtime")
}
func (m *mockUploadContext) GetKubernetesArtifactsDir() string {
	return m.GetComponentArtifactsDir("kubernetes")
}
func (m *mockUploadContext) GetFileDownloadPath(cn, v, a, fn string) string { return "" }
func (m *mockUploadContext) GetHostDir(hostname string) string {
	return filepath.Join(m.GetClusterArtifactsDir(), hostname)
}
func (m *mockUploadContext) WithGoContext(goCtx context.Context) step.StepContext {
	m.goCtx = goCtx
	return m
}

var _ step.StepContext = (*mockUploadContext)(nil)

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
	mockCtx := newMockUploadContext(t, "remote-host")
	defer mockCtx.ctrl.Finish()
	defer os.RemoveAll(filepath.Dir(filepath.Dir(mockCtx.globalWorkDir))) // Clean up $(pwd)

	nonExistentSrc := filepath.Join(mockCtx.globalWorkDir, "nonexistent_src.txt")
	s := NewUploadFileStep("", nonExistentSrc, "/remote/dest.txt", "0644", false, false).(*UploadFileStep)

	done, err := s.Precheck(mockCtx, mockCtx.GetHost())
	require.Error(t, err)
	assert.False(t, done)
	assert.Contains(t, err.Error(), "local source file")
	assert.Contains(t, err.Error(), "does not exist")
}

func TestUploadFileStep_Precheck_LocalSourceMissing_Allowed(t *testing.T) {
	mockCtx := newMockUploadContext(t, "remote-host")
	defer mockCtx.ctrl.Finish()
	defer os.RemoveAll(filepath.Dir(filepath.Dir(mockCtx.globalWorkDir)))

	nonExistentSrc := filepath.Join(mockCtx.globalWorkDir, "nonexistent_src_allowed.txt")
	s := NewUploadFileStep("", nonExistentSrc, "/remote/dest.txt", "0644", false, true).(*UploadFileStep)

	done, err := s.Precheck(mockCtx, mockCtx.GetHost())
	require.NoError(t, err)
	assert.True(t, done, "Precheck should be done if local source missing and AllowMissingSrc is true")
}

func TestUploadFileStep_Precheck_RemoteExists(t *testing.T) {
	mockCtx := newMockUploadContext(t, "remote-host")
	defer mockCtx.ctrl.Finish()
	defer os.RemoveAll(filepath.Dir(filepath.Dir(mockCtx.globalWorkDir)))

	localSrc := filepath.Join(mockCtx.globalWorkDir, "local_src_for_remote_exists.txt")
	err := ioutil.WriteFile(localSrc, []byte("content"), 0644)
	require.NoError(t, err)

	remoteDest := "/remote/dest_exists.txt"
	s := NewUploadFileStep("", localSrc, remoteDest, "0644", false, false).(*UploadFileStep)

	mockCtx.mockRunner.EXPECT().Exists(mockCtx.GoContext(), mockCtx.mockConnector, remoteDest).Return(true, nil)

	done, err := s.Precheck(mockCtx, mockCtx.GetHost())
	require.NoError(t, err)
	assert.True(t, done, "Precheck should be done if remote file exists")
}

func TestUploadFileStep_Precheck_RemoteNotExists(t *testing.T) {
	mockCtx := newMockUploadContext(t, "remote-host")
	defer mockCtx.ctrl.Finish()
	defer os.RemoveAll(filepath.Dir(filepath.Dir(mockCtx.globalWorkDir)))

	localSrc := filepath.Join(mockCtx.globalWorkDir, "local_src_for_remote_not_exists.txt")
	err := ioutil.WriteFile(localSrc, []byte("content"), 0644)
	require.NoError(t, err)

	remoteDest := "/remote/dest_not_exists.txt"
	s := NewUploadFileStep("", localSrc, remoteDest, "0644", false, false).(*UploadFileStep)

	mockCtx.mockRunner.EXPECT().Exists(mockCtx.GoContext(), mockCtx.mockConnector, remoteDest).Return(false, nil)

	done, err := s.Precheck(mockCtx, mockCtx.GetHost())
	require.NoError(t, err)
	assert.False(t, done, "Precheck should not be done if remote file does not exist")
}

func TestUploadFileStep_Run_Success(t *testing.T) {
	mockCtx := newMockUploadContext(t, "remote-host-run")
	defer mockCtx.ctrl.Finish()
	defer os.RemoveAll(filepath.Dir(filepath.Dir(mockCtx.globalWorkDir)))

	localSrcContent := "upload this content"
	localSrc := filepath.Join(mockCtx.globalWorkDir, "local_to_upload.txt")
	err := ioutil.WriteFile(localSrc, []byte(localSrcContent), 0644)
	require.NoError(t, err)

	remoteDest := "/opt/uploaded_file.txt"
	permissions := "0600"
	useSudo := true
	s := NewUploadFileStep("", localSrc, remoteDest, permissions, useSudo, false).(*UploadFileStep)

	mockCtx.mockRunner.EXPECT().WriteFile(mockCtx.GoContext(), mockCtx.mockConnector, []byte(localSrcContent), remoteDest, permissions, useSudo).Return(nil)

	errRun := s.Run(mockCtx, mockCtx.GetHost())
	require.NoError(t, errRun)
}

func TestUploadFileStep_Run_LocalSourceMissing_Allowed_Skips(t *testing.T) {
	mockCtx := newMockUploadContext(t, "remote-host-skip")
	defer mockCtx.ctrl.Finish()
	defer os.RemoveAll(filepath.Dir(filepath.Dir(mockCtx.globalWorkDir)))

	nonExistentSrc := filepath.Join(mockCtx.globalWorkDir, "nonexistent_src_for_run.txt")
	s := NewUploadFileStep("", nonExistentSrc, "/remote/dest.txt", "0644", false, true).(*UploadFileStep)

	// Runner's WriteFile should not be called. No EXPECT needed for it.

	errRun := s.Run(mockCtx, mockCtx.GetHost())
	require.NoError(t, errRun, "Run should succeed by skipping if AllowMissingSrc is true and file is missing")
}

func TestUploadFileStep_Run_SourceIsDirectory(t *testing.T) {
	mockCtx := newMockUploadContext(t, "remote-host-dir")
	defer mockCtx.ctrl.Finish()
	defer os.RemoveAll(filepath.Dir(filepath.Dir(mockCtx.globalWorkDir)))

	localSrcDir := filepath.Join(mockCtx.globalWorkDir, "local_src_dir")
	err := os.Mkdir(localSrcDir, 0755)
	require.NoError(t, err)

	s := NewUploadFileStep("", localSrcDir, "/remote/dest", "0644", false, false).(*UploadFileStep)

	errRun := s.Run(mockCtx, mockCtx.GetHost())
	require.Error(t, errRun)
	assert.Contains(t, errRun.Error(), "is a directory, UploadFileStep only supports single files")
}

func TestUploadFileStep_Rollback_Success(t *testing.T) {
	mockCtx := newMockUploadContext(t, "remote-host-rollback")
	defer mockCtx.ctrl.Finish()
	defer os.RemoveAll(filepath.Dir(filepath.Dir(mockCtx.globalWorkDir)))

	remoteDestToClean := "/opt/file_to_clean.txt"
	useSudoRollback := true
	s := NewUploadFileStep("", "/local/any.txt", remoteDestToClean, "0644", useSudoRollback, false).(*UploadFileStep)

	mockCtx.mockRunner.EXPECT().Remove(mockCtx.GoContext(), mockCtx.mockConnector, remoteDestToClean, useSudoRollback).Return(nil)

	errRollback := s.Rollback(mockCtx, mockCtx.GetHost())
	require.NoError(t, errRollback)
}

func TestUploadFileStep_Rollback_RemoveError(t *testing.T) {
	mockCtx := newMockUploadContext(t, "remote-host-rollback-err")
	defer mockCtx.ctrl.Finish()
	defer os.RemoveAll(filepath.Dir(filepath.Dir(mockCtx.globalWorkDir)))

	s := NewUploadFileStep("", "/local/any.txt", "/remote/file.txt", "0644", true, false).(*UploadFileStep)
	expectedErr := fmt.Errorf("failed to remove for test")

	mockCtx.mockRunner.EXPECT().Remove(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(expectedErr)

	errRollback := s.Rollback(mockCtx, mockCtx.GetHost())
	assert.NoError(t, errRollback, "Rollback should return nil even if runner.Remove fails, as it's best-effort")
}
