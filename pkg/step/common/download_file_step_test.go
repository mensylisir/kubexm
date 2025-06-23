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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/cache"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/logger"
	"github.com/mensylisir/kubexm/pkg/runtime" // For runtime.Context full
	"github.com/mensylisir/kubexm/pkg/step"
)

// mockStepContextForDownload is a helper to create a StepContext for testing.
// It uses the full runtime.Context and then gets a StepContext view from it.
func mockStepContextForDownload(t *testing.T, host connector.Host) step.StepContext {
	t.Helper()
	l, _ := logger.New(logger.DefaultOptions())
	// Create a temp directory for GlobalWorkDir for this test context
	tempGlobalWorkDir, err := ioutil.TempDir("", "test-gwd-download-")
	require.NoError(t, err)
	// t.Cleanup(func() { os.RemoveAll(tempGlobalWorkDir) }) // Cleanup temp dir

	mainCtx := &runtime.Context{
		GoCtx:  context.Background(),
		Logger: l,
		ClusterConfig: &v1alpha1.Cluster{
			ObjectMeta: v1alpha1.ObjectMeta{Name: "test-cluster-download"},
			Spec: v1alpha1.ClusterSpec{
				Global: &v1alpha1.GlobalSpec{
					WorkDir: filepath.Dir(filepath.Dir(tempGlobalWorkDir)), // $(pwd) for GetBinaryInfo
				},
			},
		},
		StepCache:     cache.NewStepCache(),
		GlobalWorkDir: tempGlobalWorkDir, // This is $(pwd)/.kubexm/${cluster_name}
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
	// SetCurrentHost is needed for StepContext to correctly return GetHost() etc.
	mainCtx.SetCurrentHost(host)

	// The full runtime.Context itself implements step.StepContext.
	return mainCtx
}

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

	mockCtx := mockStepContextForDownload(t, nil)
	// DestPath will be relative to the test's temp GlobalWorkDir
	destPath := filepath.Join(mockCtx.GetGlobalWorkDir(), "downloaded_file.txt")
	// Ensure parent of destPath is GlobalWorkDir for os.MkdirAll in Run to work as expected.
	// If destPath was /tmp/file.txt, MkdirAll("/tmp") is fine.
	// If destPath is complex, ensure the base for MkdirAll is appropriate.
	// Here, GetGlobalWorkDir() is the base, so destPath is inside it.

	dStep := NewDownloadFileStep("DownloadTest", server.URL, destPath, "", "", false).(*DownloadFileStep)

	// Precheck should indicate download is needed
	done, errPrecheck := dStep.Precheck(mockCtx, mockCtx.GetHost())
	require.NoError(t, errPrecheck)
	assert.False(t, done)

	errRun := dStep.Run(mockCtx, mockCtx.GetHost())
	require.NoError(t, errRun)

	content, errRead := ioutil.ReadFile(destPath)
	require.NoError(t, errRead)
	assert.Equal(t, "Hello, client\n", string(content))

	// Precheck again, should be done
	doneAfterRun, errPrecheckAfterRun := dStep.Precheck(mockCtx, mockCtx.GetHost())
	require.NoError(t, errPrecheckAfterRun)
	assert.True(t, doneAfterRun)

	// Rollback
	errRollback := dStep.Rollback(mockCtx, mockCtx.GetHost())
	require.NoError(t, errRollback)
	_, errStat := os.Stat(destPath)
	assert.True(t, os.IsNotExist(errStat))

	os.RemoveAll(mockCtx.GetGlobalWorkDir()) // Clean up temp dir for this test case
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

	mockCtx := mockStepContextForDownload(t, nil)
	destPath := filepath.Join(mockCtx.GetGlobalWorkDir(), "checksum_file.txt")

	dStep := NewDownloadFileStep("ChecksumDownload", server.URL, destPath, expectedChecksum, "sha256", false).(*DownloadFileStep)

	errRun := dStep.Run(mockCtx, mockCtx.GetHost())
	require.NoError(t, errRun)

	done, errPrecheck := dStep.Precheck(mockCtx, mockCtx.GetHost())
	require.NoError(t, errPrecheck)
	assert.True(t, done)
	os.RemoveAll(mockCtx.GetGlobalWorkDir())
}

func TestDownloadFileStep_Run_ChecksumFail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Different content")
	}))
	defer server.Close()

	mockCtx := mockStepContextForDownload(t, nil)
	destPath := filepath.Join(mockCtx.GetGlobalWorkDir(), "checksum_fail.txt")

	expectedChecksum := "thisisnotthecorrectchecksum"
	dStep := NewDownloadFileStep("ChecksumFail", server.URL, destPath, expectedChecksum, "sha256", false).(*DownloadFileStep)

	errRun := dStep.Run(mockCtx, mockCtx.GetHost())
	require.Error(t, errRun)
	assert.Contains(t, errRun.Error(), "checksum mismatch")

	_, errStat := os.Stat(destPath)
	assert.True(t, os.IsNotExist(errStat), "File should be removed after checksum failure in Run")
	os.RemoveAll(mockCtx.GetGlobalWorkDir())
}

func TestDownloadFileStep_Precheck_ExistingFile_BadChecksum_RemovesFile(t *testing.T) {
	mockCtx := mockStepContextForDownload(t, nil)
	destPath := filepath.Join(mockCtx.GetGlobalWorkDir(), "existing_bad_checksum.txt")

	err := os.MkdirAll(filepath.Dir(destPath), 0755) // Ensure dir exists
	require.NoError(t, err)
	err = ioutil.WriteFile(destPath, []byte("actual content"), 0644)
	require.NoError(t, err)

	expectedChecksum := "thisisnotthecorrectchecksumforactualcontent"
	dStep := NewDownloadFileStep("PrecheckBadChecksum", "http://example.com", destPath, expectedChecksum, "sha256", false).(*DownloadFileStep)

	done, errPrecheck := dStep.Precheck(mockCtx, mockCtx.GetHost())
	require.NoError(t, errPrecheck)
	assert.False(t, done, "Precheck should indicate not done if checksum fails")

	_, errStat := os.Stat(destPath)
	assert.True(t, os.IsNotExist(errStat), "File should be removed by Precheck if checksum fails")
	os.RemoveAll(mockCtx.GetGlobalWorkDir())
}

func TestDownloadFileStep_Run_HttpError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	mockCtx := mockStepContextForDownload(t, nil)
	destPath := filepath.Join(mockCtx.GetGlobalWorkDir(), "http_error.txt")
	dStep := NewDownloadFileStep("HttpError", server.URL, destPath, "", "", false).(*DownloadFileStep)

	errRun := dStep.Run(mockCtx, mockCtx.GetHost())
	require.Error(t, errRun)
	assert.Contains(t, errRun.Error(), "download request failed")
	assert.Contains(t, errRun.Error(), "404 Not Found")
	os.RemoveAll(mockCtx.GetGlobalWorkDir())
}

func TestDownloadFileStep_Precheck_ErrorStatingFile(t *testing.T) {
	mockCtx := mockStepContextForDownload(t, nil)
	// Use a path that is likely to cause a permission error if not run as root
	// Note: this test's behavior depends on the environment it runs in.
	destPath := "/root/no_permission_file_for_download_test.txt"
	// To make it more reliable, we could try to create a situation where Stat fails
	// not due to IsNotExist. However, os.Stat primarily fails with IsNotExist or permission.
	// If DestPath's parent doesn't exist, os.Stat(DestPath) will be IsNotExist.
	// If parent exists but DestPath is not accessible, it's a permission error.

	dStep := NewDownloadFileStep("StatError", "http://example.com", destPath, "", "", false).(*DownloadFileStep)

	done, errPrecheck := dStep.Precheck(mockCtx, mockCtx.GetHost())

	if os.Getuid() != 0 { // Only expect a specific error if not running as root
		require.Error(t, errPrecheck, "Expected an error from Precheck if stating fails with permission issues")
		assert.False(t, done)
		assert.Contains(t, errPrecheck.Error(), "precheck failed to stat destination file")
	} else {
		// If running as root, /root might exist, and the file might be IsNotExist,
		// in which case errPrecheck would be nil and done would be false.
		if errPrecheck == nil {
			assert.False(t, done, "If stat succeeded (e.g. root and /root exists), but file not there, done should be false.")
		} else {
			// If it was another error (e.g. non-existent /root dir even for root user, though unlikely)
			assert.False(t, done)
			t.Logf("Precheck error as root: %v", errPrecheck)
		}
	}
	os.RemoveAll(mockCtx.GetGlobalWorkDir()) // Cleanup temp dir
}

func TestDownloadFileStep_UnsupportedChecksumType(t *testing.T) {
	fileContent := "content"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, fileContent)
	}))
	defer server.Close()
	mockCtx := mockStepContextForDownload(t, nil)
	destPath := filepath.Join(mockCtx.GetGlobalWorkDir(), "file.txt")

	dStepRun := NewDownloadFileStep("", server.URL, destPath, "checksum", "md5", false).(*DownloadFileStep)
	errRun := dStepRun.Run(mockCtx, mockCtx.GetHost())
	require.Error(t, errRun)
	assert.Contains(t, errRun.Error(), "unsupported checksum type: md5")

	// Test Precheck path
	errWrite := ioutil.WriteFile(destPath, []byte(fileContent), 0644)
	require.NoError(t, errWrite)
	dStepPrecheck := NewDownloadFileStep("", server.URL, destPath, "checksum", "md5", false).(*DownloadFileStep)
	done, errPre := dStepPrecheck.Precheck(mockCtx, mockCtx.GetHost())
	// Precheck calls verifyChecksum. If verifyChecksum errors due to unsupported type,
	// Precheck logs a warning and returns (false, nil).
	require.NoError(t, errPre, "Precheck should not error out itself for bad checksum type during verify")
	assert.False(t, done, "Precheck should return false if checksum verification within it fails (e.g. bad type)")
	os.RemoveAll(mockCtx.GetGlobalWorkDir())
}
