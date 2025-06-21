package common

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/logger"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step"
)

// mockExtractStepContext provides a minimal context for testing ExtractArchiveStep.
type mockExtractStepContext struct {
	runtime.StepContext
	logger *logger.Logger
	goCtx  context.Context
}

func newMockExtractStepContext(t *testing.T) *mockExtractStepContext {
	l, _ := logger.New(logger.DefaultConfig())
	return &mockExtractStepContext{
		logger: l,
		goCtx:  context.Background(),
	}
}
func (m *mockExtractStepContext) GetLogger() *logger.Logger { return m.logger }
func (m *mockExtractStepContext) GoContext() context.Context  { return m.goCtx }
func (m *mockExtractStepContext) GetHost() connector.Host   { return nil } // Assuming local/control node operation

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

func TestExtractArchiveStep_Run_Success(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "test-extract")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	sourceArchiveDir := filepath.Join(tempDir, "source")
	destDir := filepath.Join(tempDir, "destination")
	require.NoError(t, os.Mkdir(sourceArchiveDir, 0755))

	filesToArchive := map[string]string{
		"file1.txt": "content1",
		"dir1/file2.txt": "content2",
	}
	archivePath := createTestTarGz(t, sourceArchiveDir, filesToArchive, "test_archive.tar.gz")

	eStep := NewExtractArchiveStep("ExtractRun", archivePath, destDir, false, false).(*ExtractArchiveStep)
	mockCtx := newMockExtractStepContext(t)

	// Precheck: destDir does not exist, so it should run
	done, errPre := eStep.Precheck(mockCtx, nil)
	require.NoError(t, errPre)
	assert.False(t, done)

	// Run
	errRun := eStep.Run(mockCtx, nil)
	require.NoError(t, errRun)

	// Verify extracted files
	content1, errRead1 := ioutil.ReadFile(filepath.Join(destDir, "file1.txt"))
	require.NoError(t, errRead1)
	assert.Equal(t, "content1", string(content1))

	content2, errRead2 := ioutil.ReadFile(filepath.Join(destDir, "dir1/file2.txt"))
	require.NoError(t, errRead2)
	assert.Equal(t, "content2", string(content2))

	// Precheck again: now destDir and its contents should exist (if ExpectedFiles were used)
	// For this test, let's add ExpectedFiles to the step for a robust post-run precheck
	eStep.ExpectedFiles = []string{"file1.txt", "dir1/file2.txt"}
	done, errPre = eStep.Precheck(mockCtx, nil)
	require.NoError(t, errPre)
	assert.True(t, done)

	// Rollback
	errRollback := eStep.Rollback(mockCtx, nil)
	require.NoError(t, errRollback)
	_, errStat := os.Stat(destDir)
	assert.True(t, os.IsNotExist(errStat), "Destination directory should be removed by rollback")
}


func TestExtractArchiveStep_Precheck_DestinationExists_NoExpectedFiles(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "test-precheck-no-expected")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	destDir := filepath.Join(tempDir, "existing_dest")
	require.NoError(t, os.Mkdir(destDir, 0755))

	eStep := NewExtractArchiveStep("PrecheckNoExpected", "/tmp/dummy.tar.gz", destDir, false, false).(*ExtractArchiveStep)
	mockCtx := newMockExtractStepContext(t)

	done, errPre := eStep.Precheck(mockCtx, nil)
	require.NoError(t, errPre)
	// Default behavior: if dest dir exists but no expected files, it re-runs to ensure state.
	assert.False(t, done, "Should run if dest dir exists but no expected files for verification")
}

func TestExtractArchiveStep_Precheck_DestinationExists_ExpectedFilesMissing(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "test-precheck-expected-missing")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	destDir := filepath.Join(tempDir, "existing_dest_expect_missing")
	require.NoError(t, os.Mkdir(destDir, 0755))
	// Create one expected file, but not the other
	require.NoError(t, ioutil.WriteFile(filepath.Join(destDir, "present.txt"), []byte("i am here"), 0644))


	eStep := NewExtractArchiveStep("PrecheckExpectedMissing", "/tmp/dummy.tar.gz", destDir, false, false).(*ExtractArchiveStep)
	eStep.ExpectedFiles = []string{"present.txt", "missing.txt"}
	mockCtx := newMockExtractStepContext(t)

	done, errPre := eStep.Precheck(mockCtx, nil)
	require.NoError(t, errPre)
	assert.False(t, done, "Should run if some expected files are missing")
}

func TestExtractArchiveStep_Run_RemoveArchiveAfterExtract(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "test-extract-remove")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	sourceArchiveDir := filepath.Join(tempDir, "source_remove")
	destDir := filepath.Join(tempDir, "destination_remove")
	require.NoError(t, os.Mkdir(sourceArchiveDir, 0755))

	archivePath := createTestTarGz(t, sourceArchiveDir, map[string]string{"file.txt": "data"}, "remove_me.tar.gz")

	eStep := NewExtractArchiveStep("ExtractAndRemove", archivePath, destDir, true, false).(*ExtractArchiveStep)
	mockCtx := newMockExtractStepContext(t)

	errRun := eStep.Run(mockCtx, nil)
	require.NoError(t, errRun)

	_, errStatArchive := os.Stat(archivePath)
	assert.True(t, os.IsNotExist(errStatArchive), "Source archive should be removed")

	_, errStatExtracted := os.Stat(filepath.Join(destDir, "file.txt"))
	assert.NoError(t, errStatExtracted, "Extracted file should exist")
}

func TestExtractArchiveStep_Run_UnsupportedArchive(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "test-extract-unsupported")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	unsupportedArchive := filepath.Join(tempDir, "archive.zip")
	require.NoError(t, ioutil.WriteFile(unsupportedArchive, []byte("dummy zip content"), 0644))

	destDir := filepath.Join(tempDir, "dest_unsupported")
	eStep := NewExtractArchiveStep("Unsupported", unsupportedArchive, destDir, false, false).(*ExtractArchiveStep)
	mockCtx := newMockExtractStepContext(t)

	errRun := eStep.Run(mockCtx, nil)
	require.Error(t, errRun)
	assert.Contains(t, errRun.Error(), "unsupported archive format")
}
