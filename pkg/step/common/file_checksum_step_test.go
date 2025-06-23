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
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step"
)

// mockStepContextForChecksum is a helper to create a StepContext for testing.
func mockStepContextForChecksum(t *testing.T, host connector.Host) step.StepContext {
	t.Helper()
	l, _ := logger.New(logger.DefaultOptions())
	tempGlobalWorkDir, err := ioutil.TempDir("", "test-gwd-checksum-")
	require.NoError(t, err)

	mainCtx := &runtime.Context{
		GoCtx:  context.Background(),
		Logger: l,
		ClusterConfig: &v1alpha1.Cluster{
			ObjectMeta: v1alpha1.ObjectMeta{Name: "test-cluster-checksum"},
			Spec: v1alpha1.ClusterSpec{
				Global: &v1alpha1.GlobalSpec{
					WorkDir: filepath.Dir(filepath.Dir(tempGlobalWorkDir)),
				},
			},
		},
		StepCache:     cache.NewStepCache(),
		GlobalWorkDir: tempGlobalWorkDir,
	}

	if host == nil {
		hostSpec := v1alpha1.HostSpec{
			Name:    common.ControlNodeHostName,
			Type:    "local",
			Address: "127.0.0.1",
			Roles:   []string{common.ControlNodeRole},
		}
		host = connector.NewHostFromSpec(hostSpec)
		mainCtx.SetControlNode(host)
	}
	mainCtx.SetCurrentHost(host)
	return mainCtx
}

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

	// Test default algorithm
	sDefaultAlgo := NewFileChecksumStep("", filePath, expectedChecksum, "")
	fcsDefaultAlgo, _ := sDefaultAlgo.(*FileChecksumStep)
	assert.Equal(t, "sha256", fcsDefaultAlgo.ChecksumAlgorithm)
	assert.Equal(t, fmt.Sprintf("VerifyChecksum-%s", "testfile.txt"), sDefaultAlgo.Meta().Name)
}

func TestFileChecksumStep_Precheck_FileExists(t *testing.T) {
	mockCtx := mockStepContextForChecksum(t, nil)
	defer os.RemoveAll(mockCtx.GetGlobalWorkDir())

	tempFile, err := ioutil.TempFile(mockCtx.GetGlobalWorkDir(), "checksumtest*.txt")
	require.NoError(t, err)
	defer os.Remove(tempFile.Name())
	_, err = tempFile.WriteString("test content")
	require.NoError(t, err)
	tempFile.Close()

	s := NewFileChecksumStep("", tempFile.Name(), "somechecksum", "sha256").(*FileChecksumStep)

	done, errPre := s.Precheck(mockCtx, mockCtx.GetHost())
	require.NoError(t, errPre)
	assert.False(t, done, "Precheck should be false if file exists, Run will verify")
}

func TestFileChecksumStep_Precheck_FileDoesNotExist(t *testing.T) {
	mockCtx := mockStepContextForChecksum(t, nil)
	defer os.RemoveAll(mockCtx.GetGlobalWorkDir())

	nonExistentFilePath := filepath.Join(mockCtx.GetGlobalWorkDir(), "nonexistent.txt")
	s := NewFileChecksumStep("", nonExistentFilePath, "somechecksum", "sha256").(*FileChecksumStep)

	done, errPre := s.Precheck(mockCtx, mockCtx.GetHost())
	require.NoError(t, errPre)
	assert.True(t, done, "Precheck should be true (skip Run) if file does not exist")
}

func TestFileChecksumStep_Run_SHA256_Success(t *testing.T) {
	mockCtx := mockStepContextForChecksum(t, nil)
	defer os.RemoveAll(mockCtx.GetGlobalWorkDir())

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
	errRun := s.Run(mockCtx, mockCtx.GetHost())
	assert.NoError(t, errRun)
}

func TestFileChecksumStep_Run_MD5_Success(t *testing.T) {
	mockCtx := mockStepContextForChecksum(t, nil)
	defer os.RemoveAll(mockCtx.GetGlobalWorkDir())

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
	errRun := s.Run(mockCtx, mockCtx.GetHost())
	assert.NoError(t, errRun)
}

func TestFileChecksumStep_Run_ChecksumMismatch(t *testing.T) {
	mockCtx := mockStepContextForChecksum(t, nil)
	defer os.RemoveAll(mockCtx.GetGlobalWorkDir())

	tempFile, err := ioutil.TempFile(mockCtx.GetGlobalWorkDir(), "mismatch*.txt")
	require.NoError(t, err)
	defer os.Remove(tempFile.Name())
	_, err = tempFile.WriteString("actual content")
	require.NoError(t, err)
	tempFile.Close()

	s := NewFileChecksumStep("", tempFile.Name(), "expected_checksum_that_will_not_match", "sha256").(*FileChecksumStep)
	errRun := s.Run(mockCtx, mockCtx.GetHost())
	require.Error(t, errRun)
	assert.Contains(t, errRun.Error(), "checksum mismatch")
}

func TestFileChecksumStep_Run_UnsupportedAlgorithm(t *testing.T) {
	mockCtx := mockStepContextForChecksum(t, nil)
	defer os.RemoveAll(mockCtx.GetGlobalWorkDir())

	tempFile, err := ioutil.TempFile(mockCtx.GetGlobalWorkDir(), "unsupported*.txt")
	require.NoError(t, err)
	defer os.Remove(tempFile.Name())
	_, err = tempFile.WriteString("content")
	require.NoError(t, err)
	tempFile.Close()

	s := NewFileChecksumStep("", tempFile.Name(), "checksum", "sha1").(*FileChecksumStep) // SHA1 not supported by step
	errRun := s.Run(mockCtx, mockCtx.GetHost())
	require.Error(t, errRun)
	assert.Contains(t, errRun.Error(), "unsupported checksum algorithm 'sha1'")
}

func TestFileChecksumStep_Run_NoExpectedChecksum(t *testing.T) {
	mockCtx := mockStepContextForChecksum(t, nil)
	defer os.RemoveAll(mockCtx.GetGlobalWorkDir())

	tempFile, err := ioutil.TempFile(mockCtx.GetGlobalWorkDir(), "noexpect*.txt")
	require.NoError(t, err)
	defer os.Remove(tempFile.Name())
	_, err = tempFile.WriteString("content")
	require.NoError(t, err)
	tempFile.Close()

	s := NewFileChecksumStep("", tempFile.Name(), "", "sha256").(*FileChecksumStep)
	errRun := s.Run(mockCtx, mockCtx.GetHost())
	assert.NoError(t, errRun, "Run should succeed if no expected checksum is provided")
}

func TestFileChecksumStep_Rollback(t *testing.T) {
	mockCtx := mockStepContextForChecksum(t, nil)
	defer os.RemoveAll(mockCtx.GetGlobalWorkDir())
	s := NewFileChecksumStep("", "/tmp/anyfile.txt", "", "").(*FileChecksumStep)
	err := s.Rollback(mockCtx, mockCtx.GetHost())
	assert.NoError(t, err, "Rollback for FileChecksumStep should be a no-op")
}

// Ensure mockStepContextForChecksum implements step.StepContext
var _ step.StepContext = (*mockStepContextForChecksum)(nil)

// Dummy implementations for the rest of step.StepContext for mockStepContextForChecksum
func (m *mockStepContextForChecksum) GetRunner() runner.Runner                                   { return nil }
func (m *mockStepContextForChecksum) GetConnectorForHost(h connector.Host) (connector.Connector, error) { return nil, nil }
func (m *mockStepContextForChecksum) GetHostFacts(h connector.Host) (*runner.Facts, error)           { return nil, nil }
func (m *mockStepContextForChecksum) GetHost() connector.Host                                      { return m.controlHost } // Assuming it runs on control node
func (m *mockStepContextForChecksum) GetCurrentHostFacts() (*runner.Facts, error)                  { return nil, nil }
func (m *mockStepContextForChecksum) GetCurrentHostConnector() (connector.Connector, error)        { return nil, nil }
func (m *mockStepContextForChecksum) StepCache() cache.StepCache                               { return nil }
func (m *mockStepContextForChecksum) TaskCache() cache.TaskCache                               { return nil }
func (m *mockStepContextForChecksum) ModuleCache() cache.ModuleCache                             { return nil }
// GetGlobalWorkDir() is implemented
func (m *mockStepContextForChecksum) IsVerbose() bool                                        { return false }
func (m *mockStepContextForChecksum) ShouldIgnoreErr() bool                                  { return false }
func (m *mockStepContextForChecksum) GetGlobalConnectionTimeout() time.Duration                { return 0 }
func (m *mockStepContextForChecksum) GetClusterArtifactsDir() string                         { return "" }
func (m *mockStepContextForChecksum) GetCertsDir() string                                    { return "" }
func (m *mockStepContextForChecksum) GetEtcdCertsDir() string                                { return "" }
func (m *mockStepContextForChecksum) GetComponentArtifactsDir(componentName string) string     { return "" }
func (m *mockStepContextForChecksum) GetEtcdArtifactsDir() string                            { return "" }
func (m *mockStepContextForChecksum) GetContainerRuntimeArtifactsDir() string                { return "" }
func (m *mockStepContextForChecksum) GetKubernetesArtifactsDir() string                      { return "" }
func (m *mockStepContextForChecksum) GetFileDownloadPath(c, v, a, f string) string             { return "" }
func (m *mockStepContextForChecksum) GetHostDir(hostname string) string                      { return "" }
func (m *mockStepContextForChecksum) WithGoContext(gCtx context.Context) step.StepContext      {
	m.goCtx = gCtx
	return m
}
func (m *mockStepContextForChecksum) GetControlNode() (connector.Host, error) {
	return m.controlHost, nil
}
