package common

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"strings"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mensylisir/kubexm/pkg/connector" // For connector.Host if needed by mock
	"github.com/mensylisir/kubexm/pkg/logger"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// MockStepContext for DownloadFileStep tests
type mockDownloadStepContext struct {
	runtime.StepContext // Embed for any methods not overridden
	logger              *logger.Logger
	goCtx               context.Context
}

func newMockDownloadStepContext() *mockDownloadStepContext {
	l, _ := logger.New(logger.DefaultConfig())
	return &mockDownloadStepContext{
		logger: l,
		goCtx:  context.Background(),
	}
}

func (m *mockDownloadStepContext) GetLogger() *logger.Logger                               { return m.logger }
func (m *mockDownloadStepContext) GoContext() context.Context                                { return m.goCtx }
func (m *mockDownloadStepContext) GetRunner() runtime.Runner                                 { return nil } // Not used by DownloadFileStep directly
func (m *mockDownloadStepContext) GetConnectorForHost(h connector.Host) (connector.Connector, error) { return nil, nil }
func (m *mockDownloadStepContext) GetHostFacts(h connector.Host) (*runtime.Facts, error)         { return nil, nil }
func (m *mockDownloadStepContext) GetHost() connector.Host                                   { return nil } // Assumes control node context
func (m *mockDownloadStepContext) GetCurrentHostFacts() (*runtime.Facts, error)                { return nil, nil }
func (m *mockDownloadStepContext) GetCurrentHostConnector() (connector.Connector, error)       { return nil, nil }

func (m *mockDownloadStepContext) StepCache() runtime.StepCache     { return nil } // Mock if needed
func (m *mockDownloadStepContext) TaskCache() runtime.TaskCache     { return nil }
func (m *mockDownloadStepContext) ModuleCache() runtime.ModuleCache   { return nil }
func (m *mockDownloadStepContext) GetGlobalWorkDir() string         { tempDir, _ := ioutil.TempDir("", "gwd"); return tempDir }


func TestDownloadFileStep_NewDownloadFileStep(t *testing.T) {
	s := NewDownloadFileStep("TestDownload", "http://example.com/file.txt", "/tmp/file.txt", "checksum", "sha256", false)
	require.NotNil(t, s)
	meta := s.Meta()
	assert.Equal(t, "TestDownload", meta.Name)
	assert.Contains(t, meta.Description, "http://example.com/file.txt")
	assert.Contains(t, meta.Description, "/tmp/file.txt")
}

func TestDownloadFileStep_Run_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Hello, client")
	}))
	defer server.Close()

	tempDir, err := ioutil.TempDir("", "test-download")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	destPath := filepath.Join(tempDir, "downloaded_file.txt")
	dStep := NewDownloadFileStep("DownloadTest", server.URL, destPath, "", "", false).(*DownloadFileStep)

	mockCtx := newMockDownloadStepContext()

	// Precheck should indicate download is needed
	done, errPrecheck := dStep.Precheck(mockCtx, nil)
	require.NoError(t, errPrecheck)
	assert.False(t, done)

	errRun := dStep.Run(mockCtx, nil) // host is nil as it's a local operation
	require.NoError(t, errRun)

	content, errRead := ioutil.ReadFile(destPath)
	require.NoError(t, errRead)
	assert.Equal(t, "Hello, client\n", string(content))

	// Precheck again, should be done
	done, errPrecheck = dStep.Precheck(mockCtx, nil)
	require.NoError(t, errPrecheck)
	assert.True(t, done)

	// Rollback
	errRollback := dStep.Rollback(mockCtx, nil)
	require.NoError(t, errRollback)
	_, errStat := os.Stat(destPath)
	assert.True(t, os.IsNotExist(errStat))
}

func TestDownloadFileStep_Run_ChecksumSuccess(t *testing.T) {
	fileContent := "Checksum content"
	// SHA256 for "Checksum content" is e14931ignoring_rest_of_hash...
	// Correct SHA256 for "Checksum content" is 1f7f29cf8071dda9985699ace5994993ada2ef40fa36a8d0f307408336579085
	expectedChecksum := "1f7f29cf8071dda9985699ace5994993ada2ef40fa36a8d0f307408336579085"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, fileContent) // Use Fprint to avoid extra newline if not desired
	}))
	defer server.Close()

	tempDir, err := ioutil.TempDir("", "test-checksum")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	destPath := filepath.Join(tempDir, "checksum_file.txt")
	dStep := NewDownloadFileStep("ChecksumDownload", server.URL, destPath, expectedChecksum, "sha256", false).(*DownloadFileStep)
	mockCtx := newMockDownloadStepContext()

	errRun := dStep.Run(mockCtx, nil)
	require.NoError(t, errRun)

	// Precheck should now be done
	done, errPrecheck := dStep.Precheck(mockCtx, nil)
	require.NoError(t, errPrecheck)
	assert.True(t, done)
}

func TestDownloadFileStep_Run_ChecksumFail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Different content")
	}))
	defer server.Close()

	tempDir, err := ioutil.TempDir("", "test-checksum-fail")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	destPath := filepath.Join(tempDir, "checksum_fail.txt")
	// SHA256 for "Checksum content"
	expectedChecksum := "1f7f29cf8071dda9985699ace5994993ada2ef40fa36a8d0f307408336579085"
	dStep := NewDownloadFileStep("ChecksumFail", server.URL, destPath, expectedChecksum, "sha256", false).(*DownloadFileStep)
	mockCtx := newMockDownloadStepContext()

	errRun := dStep.Run(mockCtx, nil)
	require.Error(t, errRun)
	assert.Contains(t, errRun.Error(), "checksum mismatch")

	// File should have been removed after checksum failure
	_, errStat := os.Stat(destPath)
	assert.True(t, os.IsNotExist(errStat))
}

func TestDownloadFileStep_Precheck_ExistingFile_BadChecksum(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "test-precheck-bad-checksum")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	destPath := filepath.Join(tempDir, "existing_bad_checksum.txt")
	err = ioutil.WriteFile(destPath, []byte("actual content"), 0644)
	require.NoError(t, err)

	// SHA256 for "expected content"
	expectedChecksum := "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
	dStep := NewDownloadFileStep("PrecheckBadChecksum", "http://example.com", destPath, expectedChecksum, "sha256", false).(*DownloadFileStep)
	mockCtx := newMockDownloadStepContext()

	done, errPrecheck := dStep.Precheck(mockCtx, nil)
	require.NoError(t, errPrecheck) // Precheck itself doesn't error, but indicates Run is needed
	assert.False(t, done) // Not done, because checksum failed

	// File should have been removed by Precheck due to bad checksum
	_, errStat := os.Stat(destPath)
	assert.True(t, os.IsNotExist(errStat), "File should be removed by Precheck if checksum fails")
}


func TestDownloadFileStep_Run_HttpError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	tempDir, err := ioutil.TempDir("", "test-http-error")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	destPath := filepath.Join(tempDir, "http_error.txt")
	dStep := NewDownloadFileStep("HttpError", server.URL, destPath, "", "", false).(*DownloadFileStep)
	mockCtx := newMockDownloadStepContext()

	errRun := dStep.Run(mockCtx, nil)
	require.Error(t, errRun)
	assert.Contains(t, errRun.Error(), "download request failed")
	assert.Contains(t, errRun.Error(), "404 Not Found")
}
