package common

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
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
	"github.com/mensylisir/kubexm/pkg/step"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type mockChecksumContext struct {
	logger        *logger.Logger
	goCtx         context.Context
	currentHost   connector.Host // Should be controlNode for this step
	controlNode   connector.Host
	globalWorkDir string
	clusterConfig *v1alpha1.Cluster
}

func newMockChecksumContext(t *testing.T) *mockChecksumContext {
	l, _ := logger.NewLogger(logger.DefaultOptions())
	tempGlobalWorkDir, err := ioutil.TempDir("", "test-gwd-checksum-")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(tempGlobalWorkDir) })

	controlHostSpec := v1alpha1.HostSpec{Name: common.ControlNodeHostName, Type: "local", Address: "127.0.0.1", Roles: []string{common.ControlNodeRole}, Arch: "amd64"}
	controlNode := connector.NewHostFromSpec(controlHostSpec)

	clusterName := "test-cluster-checksum"
	baseWorkDirForConfig := filepath.Dir(filepath.Dir(tempGlobalWorkDir))

	return &mockChecksumContext{
		logger:        l,
		goCtx:         context.Background(),
		currentHost:   controlNode, // FileChecksumStep runs on control node
		controlNode:   controlNode,
		globalWorkDir: tempGlobalWorkDir,
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
func (m *mockChecksumContext) GoContext() context.Context    { return m.goCtx }
func (m *mockChecksumContext) GetLogger() *logger.Logger     { return m.logger }
func (m *mockChecksumContext) GetHost() connector.Host       { return m.currentHost }
func (m *mockChecksumContext) GetRunner() runner.Runner      { return nil } // Not used by FileChecksumStep
func (m *mockChecksumContext) GetControlNode() (connector.Host, error)    { return m.controlNode, nil }
func (m *mockChecksumContext) GetConnectorForHost(h connector.Host) (connector.Connector, error) { return nil, nil }
func (m *mockChecksumContext) GetCurrentHostConnector() (connector.Connector, error)        { return nil, nil }
func (m *mockChecksumContext) GetHostFacts(h connector.Host) (*runner.Facts, error)           { return &runner.Facts{OS: &connector.OS{Arch: "amd64"}}, nil }
func (m *mockChecksumContext) GetCurrentHostFacts() (*runner.Facts, error)                  { return &runner.Facts{OS: &connector.OS{Arch: "amd64"}}, nil }
func (m *mockChecksumContext) GetStepCache() cache.StepCache          { return cache.NewStepCache() }
func (m *mockChecksumContext) GetTaskCache() cache.TaskCache          { return cache.NewTaskCache() }
func (m *mockChecksumContext) GetModuleCache() cache.ModuleCache      { return cache.NewModuleCache() }
func (m *mockChecksumContext) GetPipelineCache() cache.PipelineCache  { return cache.NewPipelineCache() }
func (m *mockChecksumContext) GetClusterConfig() *v1alpha1.Cluster { return m.clusterConfig }
func (m *mockChecksumContext) GetHostsByRole(role string) ([]connector.Host, error) { return nil, nil }
func (m *mockChecksumContext) GetGlobalWorkDir() string         { return m.globalWorkDir }
func (m *mockChecksumContext) IsVerbose() bool                  { return false }
func (m *mockChecksumContext) ShouldIgnoreErr() bool            { return false }
func (m *mockChecksumContext) GetGlobalConnectionTimeout() time.Duration { return 30 * time.Second }
func (m *mockChecksumContext) GetClusterArtifactsDir() string       { return m.globalWorkDir }
func (m *mockChecksumContext) GetCertsDir() string                  { return filepath.Join(m.GetClusterArtifactsDir(), "certs") }
func (m *mockChecksumContext) GetEtcdCertsDir() string              { return filepath.Join(m.GetCertsDir(), "etcd") }
func (m *mockChecksumContext) GetComponentArtifactsDir(componentName string) string {
	return filepath.Join(m.GetClusterArtifactsDir(), componentName)
}
func (m *mockChecksumContext) GetEtcdArtifactsDir() string          { return m.GetComponentArtifactsDir("etcd") }
func (m *mockChecksumContext) GetContainerRuntimeArtifactsDir() string { return m.GetComponentArtifactsDir("container_runtime") }
func (m *mockChecksumContext) GetKubernetesArtifactsDir() string    { return m.GetComponentArtifactsDir("kubernetes") }
func (m *mockChecksumContext) GetFileDownloadPath(cn, v, a, fn string) string { return "" }
func (m *mockChecksumContext) GetHostDir(hostname string) string    { return filepath.Join(m.GetClusterArtifactsDir(), hostname) }
func (m *mockChecksumContext) WithGoContext(goCtx context.Context) step.StepContext {
	m.goCtx = goCtx
	return m
}
var _ step.StepContext = (*mockChecksumContext)(nil)


func TestFileChecksumStep_NewFileChecksumStep(t *testing.T) {
	filePath := "/tmp/testfile.txt"
	expectedChecksum := "abc123def456"
	algo := "sha256"

	s := NewFileChecksumStep("TestChecksum", filePath, expectedChecksum, algo)
	require.NotNil(t, s)
	meta := s.Meta()
	assert.Equal(t, "TestChecksum", meta.Name)
	assert.Contains(t, meta.Description, filePath)
	assert.Contains(t, meta.Description, algo)

	fcs, ok := s.(*FileChecksumStep)
	require.True(t, ok)
	assert.Equal(t, filePath, fcs.FilePath)
	assert.Equal(t, expectedChecksum, fcs.ExpectedChecksum)
	assert.Equal(t, algo, fcs.ChecksumAlgorithm)

	sDefaultAlgo := NewFileChecksumStep("", filePath, expectedChecksum, "")
	fcsDefaultAlgo, _ := sDefaultAlgo.(*FileChecksumStep)
	assert.Equal(t, "sha256", fcsDefaultAlgo.ChecksumAlgorithm)
	assert.Equal(t, fmt.Sprintf("VerifyChecksum-%s", "testfile.txt"), sDefaultAlgo.Meta().Name)
}

func TestFileChecksumStep_Precheck_FileExists(t *testing.T) {
	mockCtx := newMockChecksumContext(t)
	hostForStep := mockCtx.GetHost() // FileChecksumStep runs locally

	tempFile, err := ioutil.TempFile(mockCtx.GetGlobalWorkDir(), "checksumtest*.txt")
	require.NoError(t, err)
	defer os.Remove(tempFile.Name())
	_, err = tempFile.WriteString("test content")
	require.NoError(t, err)
	tempFile.Close()

	s := NewFileChecksumStep("", tempFile.Name(), "somechecksum", "sha256").(*FileChecksumStep)

	done, errPre := s.Precheck(mockCtx, hostForStep)
	require.NoError(t, errPre)
	assert.False(t, done, "Precheck should be false if file exists, Run will verify")
}

func TestFileChecksumStep_Precheck_FileDoesNotExist(t *testing.T) {
	mockCtx := newMockChecksumContext(t)
	hostForStep := mockCtx.GetHost()

	nonExistentFilePath := filepath.Join(mockCtx.GetGlobalWorkDir(), "nonexistent.txt")
	s := NewFileChecksumStep("", nonExistentFilePath, "somechecksum", "sha256").(*FileChecksumStep)

	done, errPre := s.Precheck(mockCtx, hostForStep)
	require.NoError(t, errPre)
	assert.True(t, done, "Precheck should be true (skip Run) if file does not exist")
}

func TestFileChecksumStep_Run_SHA256_Success(t *testing.T) {
	mockCtx := newMockChecksumContext(t)
	hostForStep := mockCtx.GetHost()

	content := "kubexm test content"
	hasher := sha256.New()
	hasher.Write([]byte(content))
	expectedChecksum := hex.EncodeToString(hasher.Sum(nil))

	tempFile, err := ioutil.TempFile(mockCtx.GetGlobalWorkDir(), "sha256test*.txt")
	require.NoError(t, err)
	defer os.Remove(tempFile.Name())
	_, err = tempFile.WriteString(content)
	require.NoError(t, err)
	tempFile.Close()

	s := NewFileChecksumStep("", tempFile.Name(), expectedChecksum, "sha256").(*FileChecksumStep)
	errRun := s.Run(mockCtx, hostForStep)
	assert.NoError(t, errRun)
}

func TestFileChecksumStep_Run_MD5_Success(t *testing.T) {
	mockCtx := newMockChecksumContext(t)
	hostForStep := mockCtx.GetHost()

	content := "kubexm md5 test"
	hasher := md5.New()
	hasher.Write([]byte(content))
	expectedChecksum := hex.EncodeToString(hasher.Sum(nil))

	tempFile, err := ioutil.TempFile(mockCtx.GetGlobalWorkDir(), "md5test*.txt")
	require.NoError(t, err)
	defer os.Remove(tempFile.Name())
	_, err = tempFile.WriteString(content)
	require.NoError(t, err)
	tempFile.Close()

	s := NewFileChecksumStep("", tempFile.Name(), expectedChecksum, "md5").(*FileChecksumStep)
	errRun := s.Run(mockCtx, hostForStep)
	assert.NoError(t, errRun)
}

func TestFileChecksumStep_Run_ChecksumMismatch(t *testing.T) {
	mockCtx := newMockChecksumContext(t)
	hostForStep := mockCtx.GetHost()

	tempFile, err := ioutil.TempFile(mockCtx.GetGlobalWorkDir(), "mismatch*.txt")
	require.NoError(t, err)
	defer os.Remove(tempFile.Name())
	_, err = tempFile.WriteString("actual content")
	require.NoError(t, err)
	tempFile.Close()

	s := NewFileChecksumStep("", tempFile.Name(), "expected_checksum_that_will_not_match", "sha256").(*FileChecksumStep)
	errRun := s.Run(mockCtx, hostForStep)
	require.Error(t, errRun)
	assert.Contains(t, errRun.Error(), "checksum mismatch")
}

func TestFileChecksumStep_Run_UnsupportedAlgorithm(t *testing.T) {
	mockCtx := newMockChecksumContext(t)
	hostForStep := mockCtx.GetHost()

	tempFile, err := ioutil.TempFile(mockCtx.GetGlobalWorkDir(), "unsupported*.txt")
	require.NoError(t, err)
	defer os.Remove(tempFile.Name())
	_, err = tempFile.WriteString("content")
	require.NoError(t, err)
	tempFile.Close()

	s := NewFileChecksumStep("", tempFile.Name(), "checksum", "sha1").(*FileChecksumStep) // SHA1 not supported by step
	errRun := s.Run(mockCtx, hostForStep)
	require.Error(t, errRun)
	assert.Contains(t, errRun.Error(), "unsupported checksum algorithm 'sha1'")
}

func TestFileChecksumStep_Run_NoExpectedChecksum(t *testing.T) {
	mockCtx := newMockChecksumContext(t)
	hostForStep := mockCtx.GetHost()

	tempFile, err := ioutil.TempFile(mockCtx.GetGlobalWorkDir(), "noexpect*.txt")
	require.NoError(t, err)
	defer os.Remove(tempFile.Name())
	_, err = tempFile.WriteString("content")
	require.NoError(t, err)
	tempFile.Close()

	s := NewFileChecksumStep("", tempFile.Name(), "", "sha256").(*FileChecksumStep)
	errRun := s.Run(mockCtx, hostForStep)
	assert.NoError(t, errRun, "Run should succeed if no expected checksum is provided")
}

func TestFileChecksumStep_Rollback(t *testing.T) {
	mockCtx := newMockChecksumContext(t)
	hostForStep := mockCtx.GetHost()
	s := NewFileChecksumStep("", "/tmp/anyfile.txt", "", "").(*FileChecksumStep)
	err := s.Rollback(mockCtx, hostForStep)
	assert.NoError(t, err, "Rollback for FileChecksumStep should be a no-op")
}
