package common

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io/ioutil"
	"os"
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
	"github.com/mensylisir/kubexm/pkg/runtime" // For runtime.Context full
	"github.com/mensylisir/kubexm/pkg/step"
)

// mockStepContextForExtract is a helper to create a StepContext for testing.
func mockStepContextForExtract(t *testing.T, host connector.Host) step.StepContext {
	t.Helper()
	l, _ := logger.New(logger.DefaultOptions())
	// Create a temp directory for GlobalWorkDir for this test context
	tempGlobalWorkDir, err := ioutil.TempDir("", "test-gwd-extract-")
	require.NoError(t, err)
	// t.Cleanup(func() { os.RemoveAll(tempGlobalWorkDir) }) // Cleanup temp dir

	mainCtx := &runtime.Context{
		GoCtx:  context.Background(),
		Logger: l,
		ClusterConfig: &v1alpha1.Cluster{
			ObjectMeta: v1alpha1.ObjectMeta{Name: "test-cluster-extract"},
			Spec: v1alpha1.ClusterSpec{
				Global: &v1alpha1.GlobalSpec{
					WorkDir: filepath.Dir(filepath.Dir(tempGlobalWorkDir)),
				},
			},
		},
		StepCache:     cache.NewStepCache(),
		GlobalWorkDir: tempGlobalWorkDir,
	}

	if host == nil { // Default to a mock local host if none provided
		hostSpec := v1alpha1.HostSpec{
			Name:    common.ControlNodeHostName, // Use common constant
			Type:    "local",
			Address: "127.0.0.1",
			Roles:   []string{common.ControlNodeRole}, // Use common constant
		}
		host = connector.NewHostFromSpec(hostSpec)
		mainCtx.SetControlNode(host) // Set the control node
	}
	mainCtx.SetCurrentHost(host) // Set the current host for the context

	return mainCtx // The full runtime.Context itself implements step.StepContext
}

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
		// Ensure parent directories for files within tar are created conceptually by full path name
		hdr := &tar.Header{
			Name: name, // This name can include directories e.g. "dir1/file1.txt"
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
	mockCtx := mockStepContextForExtract(t, nil)
	baseTestDir := mockCtx.GetGlobalWorkDir() // Use the temp dir from context

	sourceArchiveDir := filepath.Join(baseTestDir, "source_archive")
	destDir := filepath.Join(baseTestDir, "destination_extract")
	require.NoError(t, os.MkdirAll(sourceArchiveDir, 0755))
	// destDir will be created by the step

	filesToArchive := map[string]string{
		"file1.txt":        "content1",
		"dir1/file2.txt":   "content2",
		"dir1/dir2/file3.txt": "content3",
	}
	archivePath := createTestTarGz(t, sourceArchiveDir, filesToArchive, "test_archive.tar.gz")

	eStep := NewExtractArchiveStep("ExtractRun", archivePath, destDir, false, false).(*ExtractArchiveStep)

	// Precheck: destDir does not exist, so it should run
	done, errPre := eStep.Precheck(mockCtx, mockCtx.GetHost())
	require.NoError(t, errPre)
	assert.False(t, done)

	// Run
	errRun := eStep.Run(mockCtx, mockCtx.GetHost())
	require.NoError(t, errRun)

	// Verify extracted files
	content1, errRead1 := ioutil.ReadFile(filepath.Join(destDir, "file1.txt"))
	require.NoError(t, errRead1)
	assert.Equal(t, "content1", string(content1))

	content2, errRead2 := ioutil.ReadFile(filepath.Join(destDir, "dir1/file2.txt"))
	require.NoError(t, errRead2)
	assert.Equal(t, "content2", string(content2))

	content3, errRead3 := ioutil.ReadFile(filepath.Join(destDir, "dir1/dir2/file3.txt"))
	require.NoError(t, errRead3)
	assert.Equal(t, "content3", string(content3))

	// Precheck again: now destDir and its contents should exist (if ExpectedFiles were used)
	eStep.ExpectedFiles = []string{"file1.txt", "dir1/file2.txt", "dir1/dir2/file3.txt"}
	doneAfterRun, errPreAfterRun := eStep.Precheck(mockCtx, mockCtx.GetHost())
	require.NoError(t, errPreAfterRun)
	assert.True(t, doneAfterRun)

	// Rollback
	errRollback := eStep.Rollback(mockCtx, mockCtx.GetHost())
	require.NoError(t, errRollback)
	_, errStat := os.Stat(destDir)
	assert.True(t, os.IsNotExist(errStat), "Destination directory should be removed by rollback")

	os.RemoveAll(baseTestDir) // Cleanup test's base temp dir
}

func TestExtractArchiveStep_Precheck_DestinationExists_NoExpectedFiles(t *testing.T) {
	mockCtx := mockStepContextForExtract(t, nil)
	baseTestDir := mockCtx.GetGlobalWorkDir()
	destDir := filepath.Join(baseTestDir, "existing_dest_for_precheck")
	require.NoError(t, os.MkdirAll(destDir, 0755))

	eStep := NewExtractArchiveStep("PrecheckNoExpected", "/tmp/dummy.tar.gz", destDir, false, false).(*ExtractArchiveStep)

	done, errPre := eStep.Precheck(mockCtx, mockCtx.GetHost())
	require.NoError(t, errPre)
	assert.False(t, done, "Should run if dest dir exists but no expected files for verification")
	os.RemoveAll(baseTestDir)
}

func TestExtractArchiveStep_Precheck_DestinationExists_ExpectedFilesMissing(t *testing.T) {
	mockCtx := mockStepContextForExtract(t, nil)
	baseTestDir := mockCtx.GetGlobalWorkDir()
	destDir := filepath.Join(baseTestDir, "existing_dest_expect_missing")
	require.NoError(t, os.MkdirAll(destDir, 0755))
	require.NoError(t, ioutil.WriteFile(filepath.Join(destDir, "present.txt"), []byte("i am here"), 0644))

	eStep := NewExtractArchiveStep("PrecheckExpectedMissing", "/tmp/dummy.tar.gz", destDir, false, false).(*ExtractArchiveStep)
	eStep.ExpectedFiles = []string{"present.txt", "missing.txt"}

	done, errPre := eStep.Precheck(mockCtx, mockCtx.GetHost())
	require.NoError(t, errPre)
	assert.False(t, done, "Should run if some expected files are missing")
	os.RemoveAll(baseTestDir)
}

func TestExtractArchiveStep_Run_RemoveArchiveAfterExtract(t *testing.T) {
	mockCtx := mockStepContextForExtract(t, nil)
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
	os.RemoveAll(baseTestDir)
}

func TestExtractArchiveStep_Run_UnsupportedArchive(t *testing.T) {
	mockCtx := mockStepContextForExtract(t, nil)
	baseTestDir := mockCtx.GetGlobalWorkDir()
	unsupportedArchive := filepath.Join(baseTestDir, "archive.zip") // .zip is not supported by current Run
	require.NoError(t, ioutil.WriteFile(unsupportedArchive, []byte("dummy zip content"), 0644))

	destDir := filepath.Join(baseTestDir, "dest_unsupported")
	eStep := NewExtractArchiveStep("Unsupported", unsupportedArchive, destDir, false, false).(*ExtractArchiveStep)

	errRun := eStep.Run(mockCtx, mockCtx.GetHost())
	require.Error(t, errRun)
	assert.Contains(t, errRun.Error(), "unsupported archive format")
	os.RemoveAll(baseTestDir)
}

func TestExtractArchiveStep_Run_PathTraversalAttempt(t *testing.T) {
	mockCtx := mockStepContextForExtract(t, nil)
	baseTestDir := mockCtx.GetGlobalWorkDir()
	sourceArchiveDir := filepath.Join(baseTestDir, "source_traversal")
	destDir := filepath.Join(baseTestDir, "destination_traversal")
	require.NoError(t, os.MkdirAll(sourceArchiveDir, 0755))

	// Create a tar with a path traversal attempt
	maliciousFiles := map[string]string{
		"../../../../tmp/evil.txt": "evil content",
		"goodfile.txt":             "good content",
	}
	archivePath := createTestTarGz(t, sourceArchiveDir, maliciousFiles, "evil_archive.tar.gz")

	eStep := NewExtractArchiveStep("PathTraversalTest", archivePath, destDir, false, false).(*ExtractArchiveStep)

	errRun := eStep.Run(mockCtx, mockCtx.GetHost())
	require.Error(t, errRun, "Run should fail on path traversal attempt")
	assert.Contains(t, errRun.Error(), "attempts to escape destination directory")

	// Ensure goodfile was not created either if the process stops on error
	_, errStatGood := os.Stat(filepath.Join(destDir, "goodfile.txt"))
	assert.True(t, os.IsNotExist(errStatGood), "Good file should not be created if path traversal detected early")

	// Ensure evil.txt was not created outside the intended destination
	_, errStatEvil := os.Stat(filepath.Join(baseTestDir, "tmp/evil.txt")) // Check relative to baseTestDir
	assert.True(t, os.IsNotExist(errStatEvil), "Evil file should not be created outside destination")
	// Check one level up from baseTestDir as well, just in case.
	_, errStatEvilRoot := os.Stat(filepath.Join(filepath.Dir(baseTestDir), "tmp/evil.txt"))
	assert.True(t, os.IsNotExist(errStatEvilRoot))


	os.RemoveAll(baseTestDir)
}

// Ensure mockStepContextForExtract implements step.StepContext
var _ step.StepContext = (*mockStepContextForExtract)(nil)

// Dummy implementations for the rest of step.StepContext for mockStepContextForExtract
func (m *mockStepContextForExtract) GetRunner() runner.Runner                                   { return nil }
func (m *mockStepContextForExtract) GetConnectorForHost(h connector.Host) (connector.Connector, error) { return nil, nil }
func (m *mockStepContextForExtract) GetHostFacts(h connector.Host) (*runner.Facts, error)           { return nil, nil }
// GetHost() is implemented by embedding runtime.StepContext, but its underlying mainCtx.currentHost might be nil.
// For local steps, GetHost() might not be strictly needed if operations are on the control node's file system.
// If it were needed, a mock host should be set in the mainCtx or returned here.
func (m *mockStepContextForExtract) GetCurrentHostFacts() (*runner.Facts, error)                  { return nil, nil }
func (m *mockStepContextForExtract) GetCurrentHostConnector() (connector.Connector, error)        { return nil, nil }
func (m *mockStepContextForExtract) StepCache() cache.StepCache                               { return nil }
func (m *mockStepContextForExtract) TaskCache() cache.TaskCache                               { return nil }
func (m *mockStepContextForExtract) ModuleCache() cache.ModuleCache                             { return nil }
// GetGlobalWorkDir() is implemented.
func (m *mockStepContextForExtract) GetClusterConfig() *v1alpha1.Cluster                      { return nil }
func (m *mockStepContextForExtract) IsVerbose() bool                                        { return false }
func (m *mockStepContextForExtract) ShouldIgnoreErr() bool                                  { return false }
func (m *mockStepContextForExtract) GetGlobalConnectionTimeout() time.Duration                { return 0 }
func (m *mockStepContextForExtract) GetClusterArtifactsDir() string                         { return "" }
func (m *mockStepContextForExtract) GetCertsDir() string                                    { return "" }
func (m *mockStepContextForExtract) GetEtcdCertsDir() string                                { return "" }
func (m *mockStepContextForExtract) GetComponentArtifactsDir(componentName string) string     { return "" }
func (m *mockStepContextForExtract) GetEtcdArtifactsDir() string                            { return "" }
func (m *mockStepContextForExtract) GetContainerRuntimeArtifactsDir() string                { return "" }
func (m *mockStepContextForExtract) GetKubernetesArtifactsDir() string                      { return "" }
func (m *mockStepContextForExtract) GetFileDownloadPath(c, v, a, f string) string             { return "" }
func (m *mockStepContextForExtract) GetHostDir(hostname string) string                      { return "" }
func (m *mockStepContextForExtract) WithGoContext(gCtx context.Context) step.StepContext      {
	m.goCtx = gCtx
	return m
}
// GetControlNode() is implemented
func (m *mockStepContextForExtract) GetControlNode() (connector.Host, error) { // Added method for GetControlNode
	hostSpec := v1alpha1.HostSpec{Name: common.ControlNodeHostName, Type: "local", Address: "127.0.0.1", Roles: []string{common.ControlNodeRole}}
	return connector.NewHostFromSpec(hostSpec), nil
}
