package common

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
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
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/util" // Added import for util
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// mockDownloadContext is a dedicated mock for step.StepContext for DownloadFileStep tests.
type mockDownloadContext struct {
	logger        *logger.Logger
	goCtx         context.Context
	currentHost   connector.Host // Should be controlNode for this step
	controlNode   connector.Host
	globalWorkDir string
	clusterConfig *v1alpha1.Cluster
	// No runner or connector needed by DownloadFileStep's core logic (uses net/http)
}

func newMockDownloadContext(t *testing.T) *mockDownloadContext {
	l, _ := logger.NewLogger(logger.DefaultOptions())
	tempGlobalWorkDir, err := ioutil.TempDir("", "test-gwd-download-")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(tempGlobalWorkDir) })

	controlHostSpec := v1alpha1.HostSpec{Name: common.ControlNodeHostName, Type: "local", Address: "127.0.0.1", Roles: []string{common.ControlNodeRole}, Arch: "amd64"}
	controlNode := connector.NewHostFromSpec(controlHostSpec)

	clusterName := "test-cluster-download"
	// baseWorkDirForConfig should be the parent of the .kubexm dir, effectively $(pwd)
	baseWorkDirForConfig := filepath.Dir(filepath.Dir(tempGlobalWorkDir))

	return &mockDownloadContext{
		logger:        l,
		goCtx:         context.Background(),
		currentHost:   controlNode, // DownloadFileStep always runs on currentHost, which is controlNode
		controlNode:   controlNode,
		globalWorkDir: tempGlobalWorkDir, // This is $(pwd)/.kubexm/test-cluster-download
		clusterConfig: &v1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: clusterName},
			Spec: v1alpha1.ClusterSpec{
				Global: &v1alpha1.GlobalSpec{WorkDir: baseWorkDirForConfig},
				Hosts:  []v1alpha1.HostSpec{controlHostSpec},
			},
		},
	}
}

// Implement step.StepContext
func (m *mockDownloadContext) GoContext() context.Context    { return m.goCtx }
func (m *mockDownloadContext) GetLogger() *logger.Logger     { return m.logger }
func (m *mockDownloadContext) GetHost() connector.Host       { return m.currentHost }
func (m *mockDownloadContext) GetRunner() runner.Runner      { return nil }
func (m *mockDownloadContext) GetControlNode() (connector.Host, error)    { return m.controlNode, nil }
func (m *mockDownloadContext) GetConnectorForHost(h connector.Host) (connector.Connector, error) { return nil, nil }
func (m *mockDownloadContext) GetCurrentHostConnector() (connector.Connector, error)        { return nil, nil }
func (m *mockDownloadContext) GetHostFacts(h connector.Host) (*runner.Facts, error)           { return &runner.Facts{OS: &connector.OS{Arch: "amd64"}}, nil }
func (m *mockDownloadContext) GetCurrentHostFacts() (*runner.Facts, error)                  { return &runner.Facts{OS: &connector.OS{Arch: "amd64"}}, nil }
func (m *mockDownloadContext) GetStepCache() cache.StepCache          { return cache.NewStepCache() }
func (m *mockDownloadContext) GetTaskCache() cache.TaskCache          { return cache.NewTaskCache() }
func (m *mockDownloadContext) GetModuleCache() cache.ModuleCache      { return cache.NewModuleCache() }
func (m *mockDownloadContext) GetPipelineCache() cache.PipelineCache  { return cache.NewPipelineCache() }
func (m *mockDownloadContext) GetClusterConfig() *v1alpha1.Cluster { return m.clusterConfig }
func (m *mockDownloadContext) GetHostsByRole(role string) ([]connector.Host, error) { return nil, nil }
func (m *mockDownloadContext) GetGlobalWorkDir() string         { return m.globalWorkDir }
func (m *mockDownloadContext) IsVerbose() bool                  { return false }
func (m *mockDownloadContext) ShouldIgnoreErr() bool            { return false }
func (m *mockDownloadContext) GetGlobalConnectionTimeout() time.Duration { return 30 * time.Second }
func (m *mockDownloadContext) GetClusterArtifactsDir() string       { return m.globalWorkDir }
func (m *mockDownloadContext) GetCertsDir() string                  { return filepath.Join(m.GetClusterArtifactsDir(), "certs") }
func (m *mockDownloadContext) GetEtcdCertsDir() string              { return filepath.Join(m.GetCertsDir(), "etcd") }
func (m *mockDownloadContext) GetComponentArtifactsDir(componentName string) string {
	return filepath.Join(m.GetClusterArtifactsDir(), componentName)
}
func (m *mockDownloadContext) GetEtcdArtifactsDir() string          { return m.GetComponentArtifactsDir("etcd") }
func (m *mockDownloadContext) GetContainerRuntimeArtifactsDir() string { return m.GetComponentArtifactsDir("container_runtime") }
func (m *mockDownloadContext) GetKubernetesArtifactsDir() string    { return m.GetComponentArtifactsDir("kubernetes") }
func (m *mockDownloadContext) GetFileDownloadPath(cn, v, a, fn string) string {
	// This mock context's globalWorkDir is already cluster-specific.
	// util.GetBinaryInfo expects the parent of .kubexm dir.
	pwdSuperDir := filepath.Dir(filepath.Dir(m.globalWorkDir))
	binInfo, _ := util.GetBinaryInfo(cn,v,a,util.GetZone(), pwdSuperDir, m.clusterConfig.Name)
	if binInfo != nil {
		return binInfo.FilePath
	}
	return ""
}
func (m *mockDownloadContext) GetHostDir(hostname string) string    { return filepath.Join(m.GetClusterArtifactsDir(), hostname) }
func (m *mockDownloadContext) WithGoContext(goCtx context.Context) step.StepContext {
	m.goCtx = goCtx
	return m
}
var _ step.StepContext = (*mockDownloadContext)(nil)


func TestDownloadFileStep_NewDownloadFileStep(t *testing.T) {
	s := NewDownloadFileStep("TestDownload", "http://example.com/file.txt", "/tmp/file.txt", "checksum", "sha256", false)
	require.NotNil(t, s)
	meta := s.Meta()
	assert.Equal(t, "TestDownload", meta.Name)
	assert.Contains(t, meta.Description, "http://example.com/file.txt")
	assert.Contains(t, meta.Description, "/tmp/file.txt")

	dfs, ok := s.(*DownloadFileStep)
	require.True(t, ok)
	assert.Equal(t, "sha256", dfs.ChecksumType)

	sNoType := NewDownloadFileStep("TestDownloadNoType", "http://example.com/file.txt", "/tmp/file.txt", "checksumval", "", false)
	dfsNoType, _ := sNoType.(*DownloadFileStep)
	assert.Equal(t, "sha256", dfsNoType.ChecksumType, "ChecksumType should default to sha256 if checksum is provided")

	sNoChecksum := NewDownloadFileStep("TestDownloadNoChecksum", "http://example.com/file.txt", "/tmp/file.txt", "", "", false)
	dfsNoChecksum, _ := sNoChecksum.(*DownloadFileStep)
	assert.Equal(t, "", dfsNoChecksum.ChecksumType, "ChecksumType should be empty if no checksum is provided")
}

func TestDownloadFileStep_Run_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Hello, client")
	}))
	defer server.Close()

	mockCtx := newMockDownloadContext(t)
	// Pass nil for host as DownloadFileStep runs locally on control node context
	hostForStep := mockCtx.GetHost() // This will be the controlNode

	destPath := filepath.Join(mockCtx.GetGlobalWorkDir(), "downloaded_file.txt")

	dStep := NewDownloadFileStep("DownloadTest", server.URL, destPath, "", "", false).(*DownloadFileStep)

	done, errPrecheck := dStep.Precheck(mockCtx, hostForStep)
	require.NoError(t, errPrecheck)
	assert.False(t, done)

	errRun := dStep.Run(mockCtx, hostForStep)
	require.NoError(t, errRun)

	content, errRead := ioutil.ReadFile(destPath)
	require.NoError(t, errRead)
	assert.Equal(t, "Hello, client\n", string(content))

	doneAfterRun, errPrecheckAfterRun := dStep.Precheck(mockCtx, hostForStep)
	require.NoError(t, errPrecheckAfterRun)
	assert.True(t, doneAfterRun)

	errRollback := dStep.Rollback(mockCtx, hostForStep)
	require.NoError(t, errRollback)
	_, errStat := os.Stat(destPath)
	assert.True(t, os.IsNotExist(errStat))
}

func TestDownloadFileStep_Run_ChecksumSuccess(t *testing.T) {
	fileContent := "Checksum content"
	hasher := sha256.New()
	hasher.Write([]byte(fileContent))
	expectedChecksum := hex.EncodeToString(hasher.Sum(nil))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, fileContent)
	}))
	defer server.Close()

	mockCtx := newMockDownloadContext(t)
	hostForStep := mockCtx.GetHost()
	destPath := filepath.Join(mockCtx.GetGlobalWorkDir(), "checksum_file.txt")

	dStep := NewDownloadFileStep("ChecksumDownload", server.URL, destPath, expectedChecksum, "sha256", false).(*DownloadFileStep)

	errRun := dStep.Run(mockCtx, hostForStep)
	require.NoError(t, errRun)

	done, errPrecheck := dStep.Precheck(mockCtx, hostForStep)
	require.NoError(t, errPrecheck)
	assert.True(t, done)
}

func TestDownloadFileStep_Run_ChecksumFail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Different content")
	}))
	defer server.Close()

	mockCtx := newMockDownloadContext(t)
	hostForStep := mockCtx.GetHost()
	destPath := filepath.Join(mockCtx.GetGlobalWorkDir(), "checksum_fail.txt")

	expectedChecksum := "thisisnotthecorrectchecksum"
	dStep := NewDownloadFileStep("ChecksumFail", server.URL, destPath, expectedChecksum, "sha256", false).(*DownloadFileStep)

	errRun := dStep.Run(mockCtx, hostForStep)
	require.Error(t, errRun)
	assert.Contains(t, errRun.Error(), "checksum mismatch")

	_, errStat := os.Stat(destPath)
	assert.True(t, os.IsNotExist(errStat), "File should be removed after checksum failure in Run")
}

func TestDownloadFileStep_Precheck_ExistingFile_BadChecksum_RemovesFile(t *testing.T) {
	mockCtx := newMockDownloadContext(t)
	hostForStep := mockCtx.GetHost()
	destPath := filepath.Join(mockCtx.GetGlobalWorkDir(), "existing_bad_checksum.txt")

	err := os.MkdirAll(filepath.Dir(destPath), 0755)
	require.NoError(t, err)
	err = ioutil.WriteFile(destPath, []byte("actual content"), 0644)
	require.NoError(t, err)

	expectedChecksum := "thisisnotthecorrectchecksumforactualcontent"
	dStep := NewDownloadFileStep("PrecheckBadChecksum", "http://example.com", destPath, expectedChecksum, "sha256", false).(*DownloadFileStep)

	done, errPrecheck := dStep.Precheck(mockCtx, hostForStep)
	require.NoError(t, errPrecheck)
	assert.False(t, done, "Precheck should indicate not done if checksum fails")

	_, errStat := os.Stat(destPath)
	assert.True(t, os.IsNotExist(errStat), "File should be removed by Precheck if checksum fails")
}

func TestDownloadFileStep_Run_HttpError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	mockCtx := newMockDownloadContext(t)
	hostForStep := mockCtx.GetHost()
	destPath := filepath.Join(mockCtx.GetGlobalWorkDir(), "http_error.txt")
	dStep := NewDownloadFileStep("HttpError", server.URL, destPath, "", "", false).(*DownloadFileStep)

	errRun := dStep.Run(mockCtx, hostForStep)
	require.Error(t, errRun)
	assert.Contains(t, errRun.Error(), "download request failed")
	assert.Contains(t, errRun.Error(), "404 Not Found")
}

func TestDownloadFileStep_Precheck_ErrorStatingFile(t *testing.T) {
	mockCtx := newMockDownloadContext(t)
	hostForStep := mockCtx.GetHost()
	destPath := "/root/no_permission_file_for_download_test.txt"
	dStep := NewDownloadFileStep("StatError", "http://example.com", destPath, "", "", false).(*DownloadFileStep)

	done, errPrecheck := dStep.Precheck(mockCtx, hostForStep)

	if os.Getuid() != 0 {
		require.Error(t, errPrecheck, "Expected an error from Precheck if stating fails with permission issues")
		assert.False(t, done)
		assert.Contains(t, errPrecheck.Error(), "precheck failed to stat destination file")
	} else {
		if errPrecheck == nil {
			assert.False(t, done, "If stat succeeded but file not there, done should be false.")
		} else {
			assert.False(t, done)
			t.Logf("Precheck error as root: %v", errPrecheck)
		}
	}
}

func TestDownloadFileStep_UnsupportedChecksumType(t *testing.T) {
	fileContent := "content"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, fileContent)
	}))
	defer server.Close()
	mockCtx := newMockDownloadContext(t)
	hostForStep := mockCtx.GetHost()
	destPath := filepath.Join(mockCtx.GetGlobalWorkDir(), "file.txt")

	dStepRun := NewDownloadFileStep("", server.URL, destPath, "checksum", "md5", false).(*DownloadFileStep)
	errRun := dStepRun.Run(mockCtx, hostForStep)
	require.Error(t, errRun)
	assert.Contains(t, errRun.Error(), "unsupported checksum type: md5")

	errWrite := ioutil.WriteFile(destPath, []byte(fileContent), 0644)
	require.NoError(t, errWrite)
	dStepPrecheck := NewDownloadFileStep("", server.URL, destPath, "checksum", "md5", false).(*DownloadFileStep)
	done, errPre := dStepPrecheck.Precheck(mockCtx, hostForStep)
	require.NoError(t, errPre, "Precheck should not error out itself for bad checksum type during verify")
	assert.False(t, done, "Precheck should return false if checksum verification within it fails (e.g. bad type)")
}
