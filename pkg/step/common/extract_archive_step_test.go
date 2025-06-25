package common

import (
	"archive/tar"
	"compress/gzip"
	"context"
	// "fmt" // Removed unused import
	"io/ioutil"
	"os"
	"path/filepath"
	// "strings" // Removed unused import
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1" // Added import

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/cache"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/logger"
	"github.com/mensylisir/kubexm/pkg/runner"
	"github.com/mensylisir/kubexm/pkg/step"
)

type mockExtractContext struct {
	logger        *logger.Logger
	goCtx         context.Context
	currentHost   connector.Host
	controlNode   connector.Host
	globalWorkDir string
	clusterConfig *v1alpha1.Cluster
	// No runner or connector needed for ExtractArchiveStep as it's local FS ops
}

func newMockExtractContext(t *testing.T, hostName string) *mockExtractContext {
	l, _ := logger.NewLogger(logger.DefaultOptions())
	tempGlobalWorkDir, err := ioutil.TempDir("", "test-gwd-extract-")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(tempGlobalWorkDir) })

	var currentHost connector.Host
	controlHostSpec := v1alpha1.HostSpec{Name: common.ControlNodeHostName, Type: "local", Address: "127.0.0.1", Roles: []string{common.ControlNodeRole}, Arch: "amd64"}
	controlNode := connector.NewHostFromSpec(controlHostSpec)

	if hostName == "" || hostName == common.ControlNodeHostName {
		currentHost = controlNode
	} else {
		// This step primarily runs on control node, but mock can represent it.
		currentHost = connector.NewHostFromSpec(v1alpha1.HostSpec{Name: hostName, Address: "irrelevant", Type: "local", Arch: "amd64"})
	}

	clusterCfg := &v1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "test-cluster-extract"}, // Corrected to metav1.ObjectMeta
		Spec: v1alpha1.ClusterSpec{
			Global: &v1alpha1.GlobalSpec{WorkDir: filepath.Dir(filepath.Dir(tempGlobalWorkDir))}, // $(pwd)
		},
	}

	return &mockExtractContext{
		logger:        l,
		goCtx:         context.Background(),
		currentHost:   currentHost,
		controlNode:   controlNode,
		globalWorkDir: tempGlobalWorkDir, // This is $(pwd)/.kubexm/test-cluster-extract effectively
		clusterConfig: clusterCfg,
	}
}

// Implement step.StepContext
func (m *mockExtractContext) GoContext() context.Context    { return m.goCtx }
func (m *mockExtractContext) GetLogger() *logger.Logger     { return m.logger }
func (m *mockExtractContext) GetHost() connector.Host       { return m.currentHost }
func (m *mockExtractContext) GetRunner() runner.Runner      { return nil } // Not used by ExtractArchiveStep
func (m *mockExtractContext) GetControlNode() (connector.Host, error)    { return m.controlNode, nil }
func (m *mockExtractContext) GetConnectorForHost(h connector.Host) (connector.Connector, error) { return nil, nil }
func (m *mockExtractContext) GetCurrentHostConnector() (connector.Connector, error)        { return nil, nil }
func (m *mockExtractContext) GetHostFacts(h connector.Host) (*runner.Facts, error)           { return &runner.Facts{OS: &connector.OS{Arch: "amd64"}}, nil }
func (m *mockExtractContext) GetCurrentHostFacts() (*runner.Facts, error)                  { return &runner.Facts{OS: &connector.OS{Arch: "amd64"}}, nil }

func (m *mockExtractContext) GetStepCache() cache.StepCache          { return cache.NewStepCache() }
func (m *mockExtractContext) GetTaskCache() cache.TaskCache          { return cache.NewTaskCache() }
func (m *mockExtractContext) GetModuleCache() cache.ModuleCache      { return cache.NewModuleCache() }
func (m *mockExtractContext) GetPipelineCache() cache.PipelineCache  { return cache.NewPipelineCache() }

func (m *mockExtractContext) GetClusterConfig() *v1alpha1.Cluster { return m.clusterConfig }
func (m *mockExtractContext) GetHostsByRole(role string) ([]connector.Host, error) { return nil, nil }

func (m *mockExtractContext) GetGlobalWorkDir() string         { return m.globalWorkDir }
func (m *mockExtractContext) IsVerbose() bool                  { return false }
func (m *mockExtractContext) ShouldIgnoreErr() bool            { return false }
func (m *mockExtractContext) GetGlobalConnectionTimeout() time.Duration { return 30 * time.Second }

func (m *mockExtractContext) GetClusterArtifactsDir() string       { return m.globalWorkDir }
func (m *mockExtractContext) GetCertsDir() string                  { return filepath.Join(m.GetClusterArtifactsDir(), "certs") }
func (m *mockExtractContext) GetEtcdCertsDir() string              { return filepath.Join(m.GetCertsDir(), "etcd") }
func (m *mockExtractContext) GetComponentArtifactsDir(componentName string) string {
	return filepath.Join(m.GetClusterArtifactsDir(), componentName)
}
func (m *mockExtractContext) GetEtcdArtifactsDir() string          { return m.GetComponentArtifactsDir("etcd") }
func (m *mockExtractContext) GetContainerRuntimeArtifactsDir() string { return m.GetComponentArtifactsDir("container_runtime") }
func (m *mockExtractContext) GetKubernetesArtifactsDir() string    { return m.GetComponentArtifactsDir("kubernetes") }
func (m *mockExtractContext) GetFileDownloadPath(cn, v, a, fn string) string { return "" }
func (m *mockExtractContext) GetHostDir(hostname string) string    { return filepath.Join(m.GetClusterArtifactsDir(), hostname) }

func (m *mockExtractContext) WithGoContext(goCtx context.Context) step.StepContext {
	m.goCtx = goCtx
	return m
}
var _ step.StepContext = (*mockExtractContext)(nil) // Verify interface satisfaction


// Helper to create a simple tar.gz file for testing
func createTestTarGz(t *testing.T, dir string, files map[string]string, archiveName string) string {
	t.Helper()
	archivePath := filepath.Join(dir, archiveName)
	tarFile, err := os.Create(archivePath)
	require.NoError(t, err)
	defer tarFile.Close()

	gzipWriter := gzip.NewWriter(tarFile)
	defer gzipWriter.Close()

	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	for name, content := range files {
		hdr := &tar.Header{
			Name: name,
			Mode: 0644,
			Size: int64(len(content)),
		}
		require.NoError(t, tarWriter.WriteHeader(hdr))
		_, err = tarWriter.Write([]byte(content))
		require.NoError(t, err)
	}
	return archivePath
}

func TestExtractArchiveStep_NewExtractArchiveStep(t *testing.T) {
	s := NewExtractArchiveStep("TestExtract", "/tmp/archive.tar.gz", "/tmp/dest", false, false)
	require.NotNil(t, s)
	meta := s.Meta()
	assert.Equal(t, "TestExtract", meta.Name)
	assert.Contains(t, meta.Description, "/tmp/archive.tar.gz")
	assert.Contains(t, meta.Description, "/tmp/dest")
}

func TestExtractArchiveStep_Run_Success_TarGz(t *testing.T) {
	mockCtx := newMockExtractContext(t, common.ControlNodeHostName)
	// baseTestDir is now mockCtx.globalWorkDir which is auto-cleaned
	baseTestDir := mockCtx.GetGlobalWorkDir()

	sourceArchiveDir := filepath.Join(baseTestDir, "source_archive")
	destDir := filepath.Join(baseTestDir, "destination_extract")
	require.NoError(t, os.MkdirAll(sourceArchiveDir, 0755))

	filesToArchive := map[string]string{
		"file1.txt":           "content1",
		"dir1/file2.txt":      "content2",
		"dir1/dir2/file3.txt": "content3",
	}
	archivePath := createTestTarGz(t, sourceArchiveDir, filesToArchive, "test_archive.tar.gz")

	eStep := NewExtractArchiveStep("ExtractRun", archivePath, destDir, false, false).(*ExtractArchiveStep)

	done, errPre := eStep.Precheck(mockCtx, mockCtx.GetHost())
	require.NoError(t, errPre)
	assert.False(t, done)

	errRun := eStep.Run(mockCtx, mockCtx.GetHost())
	require.NoError(t, errRun)

	content1, errRead1 := ioutil.ReadFile(filepath.Join(destDir, "file1.txt"))
	require.NoError(t, errRead1)
	assert.Equal(t, "content1", string(content1))

	content2, errRead2 := ioutil.ReadFile(filepath.Join(destDir, "dir1/file2.txt"))
	require.NoError(t, errRead2)
	assert.Equal(t, "content2", string(content2))

	content3, errRead3 := ioutil.ReadFile(filepath.Join(destDir, "dir1/dir2/file3.txt"))
	require.NoError(t, errRead3)
	assert.Equal(t, "content3", string(content3))

	eStep.ExpectedFiles = []string{"file1.txt", "dir1/file2.txt", "dir1/dir2/file3.txt"}
	doneAfterRun, errPreAfterRun := eStep.Precheck(mockCtx, mockCtx.GetHost())
	require.NoError(t, errPreAfterRun)
	assert.True(t, doneAfterRun)

	errRollback := eStep.Rollback(mockCtx, mockCtx.GetHost())
	require.NoError(t, errRollback)
	_, errStat := os.Stat(destDir)
	assert.True(t, os.IsNotExist(errStat), "Destination directory should be removed by rollback")
}

func TestExtractArchiveStep_Precheck_DestinationExists_NoExpectedFiles(t *testing.T) {
	mockCtx := newMockExtractContext(t, common.ControlNodeHostName)
	baseTestDir := mockCtx.GetGlobalWorkDir()
	destDir := filepath.Join(baseTestDir, "existing_dest_for_precheck")
	require.NoError(t, os.MkdirAll(destDir, 0755))

	eStep := NewExtractArchiveStep("PrecheckNoExpected", "/tmp/dummy.tar.gz", destDir, false, false).(*ExtractArchiveStep)

	done, errPre := eStep.Precheck(mockCtx, mockCtx.GetHost())
	require.NoError(t, errPre)
	assert.False(t, done, "Should run if dest dir exists but no expected files for verification")
}

func TestExtractArchiveStep_Precheck_DestinationExists_ExpectedFilesMissing(t *testing.T) {
	mockCtx := newMockExtractContext(t, common.ControlNodeHostName)
	baseTestDir := mockCtx.GetGlobalWorkDir()
	destDir := filepath.Join(baseTestDir, "existing_dest_expect_missing")
	require.NoError(t, os.MkdirAll(destDir, 0755))
	require.NoError(t, ioutil.WriteFile(filepath.Join(destDir, "present.txt"), []byte("i am here"), 0644))

	eStep := NewExtractArchiveStep("PrecheckExpectedMissing", "/tmp/dummy.tar.gz", destDir, false, false).(*ExtractArchiveStep)
	eStep.ExpectedFiles = []string{"present.txt", "missing.txt"}

	done, errPre := eStep.Precheck(mockCtx, mockCtx.GetHost())
	require.NoError(t, errPre)
	assert.False(t, done, "Should run if some expected files are missing")
}

func TestExtractArchiveStep_Run_RemoveArchiveAfterExtract(t *testing.T) {
	mockCtx := newMockExtractContext(t, common.ControlNodeHostName)
	baseTestDir := mockCtx.GetGlobalWorkDir()
	sourceArchiveDir := filepath.Join(baseTestDir, "source_remove")
	destDir := filepath.Join(baseTestDir, "destination_remove")
	require.NoError(t, os.MkdirAll(sourceArchiveDir, 0755))

	archivePath := createTestTarGz(t, sourceArchiveDir, map[string]string{"file.txt": "data"}, "remove_me.tar.gz")

	eStep := NewExtractArchiveStep("ExtractAndRemove", archivePath, destDir, true, false).(*ExtractArchiveStep)

	errRun := eStep.Run(mockCtx, mockCtx.GetHost())
	require.NoError(t, errRun)

	_, errStatArchive := os.Stat(archivePath)
	assert.True(t, os.IsNotExist(errStatArchive), "Source archive should be removed")

	_, errStatExtracted := os.Stat(filepath.Join(destDir, "file.txt"))
	assert.NoError(t, errStatExtracted, "Extracted file should exist")
}

func TestExtractArchiveStep_Run_UnsupportedArchive(t *testing.T) {
	mockCtx := newMockExtractContext(t, common.ControlNodeHostName)
	baseTestDir := mockCtx.GetGlobalWorkDir()
	unsupportedArchive := filepath.Join(baseTestDir, "archive.zip")
	require.NoError(t, ioutil.WriteFile(unsupportedArchive, []byte("dummy zip content"), 0644))

	destDir := filepath.Join(baseTestDir, "dest_unsupported")
	eStep := NewExtractArchiveStep("Unsupported", unsupportedArchive, destDir, false, false).(*ExtractArchiveStep)

	errRun := eStep.Run(mockCtx, mockCtx.GetHost())
	require.Error(t, errRun)
	assert.Contains(t, errRun.Error(), "unsupported archive format")
}

func TestExtractArchiveStep_Run_PathTraversalAttempt(t *testing.T) {
	mockCtx := newMockExtractContext(t, common.ControlNodeHostName)
	baseTestDir := mockCtx.GetGlobalWorkDir()
	sourceArchiveDir := filepath.Join(baseTestDir, "source_traversal")
	destDir := filepath.Join(baseTestDir, "destination_traversal") // This is the intended, safe destination
	require.NoError(t, os.MkdirAll(sourceArchiveDir, 0755))
	// Crucially, ensure destDir itself exists for the test's internal logic,
	// though the step would also create it.
	require.NoError(t, os.MkdirAll(destDir, 0755))


	maliciousFiles := map[string]string{
		"../../../../tmp/evil.txt": "evil content", // Path traversal
		"goodfile.txt":             "good content",
	}
	archivePath := createTestTarGz(t, sourceArchiveDir, maliciousFiles, "evil_archive.tar.gz")

	eStep := NewExtractArchiveStep("PathTraversalTest", archivePath, destDir, false, false).(*ExtractArchiveStep)

	errRun := eStep.Run(mockCtx, mockCtx.GetHost())
	require.Error(t, errRun, "Run should fail on path traversal attempt")
	assert.Contains(t, errRun.Error(), "attempts to escape destination directory")

	_, errStatGood := os.Stat(filepath.Join(destDir, "goodfile.txt"))
	assert.True(t, os.IsNotExist(errStatGood), "Good file should not be created if path traversal detected early")

	// Construct the path where evil.txt would be if it escaped relative to baseTestDir
	// If baseTestDir = /tmp/test-gwd-extract-123/
	// ../../../../tmp/evil.txt would resolve from /tmp/test-gwd-extract-123/destination_traversal/
	// to /tmp/evil.txt (assuming baseTestDir is shallow enough)
	// We need to check a path that is truly outside `destDir`.
	// A simple check is whether `destDir` itself contains `evil.txt` (it shouldn't).
	_, errStatEvilInDest := os.Stat(filepath.Join(destDir, "evil.txt"))
	assert.True(t, os.IsNotExist(errStatEvilInDest))
	// And check a known external path if the traversal was successful.
	// This is harder to make platform-agnostic and reliable.
	// The error message is the primary indicator of the defense working.
}
