package docker

import (
	"context"
	"fmt"
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
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step"
)

// Reusing mockRunnerForExtract from containerd tests (assuming same package for test helpers or defined locally)
// Reusing mockStepContextForDistribute helper from containerd tests for context setup

func TestExtractCriDockerdArchiveStep_New(t *testing.T) {
	s := NewExtractCriDockerdArchiveStep("TestExtractCriD", "remoteCriDKey", "/opt/extract/crid", "cri-d-0.3", "extractedCriDPathKey", true, true)
	require.NotNil(t, s)
	meta := s.Meta()
	assert.Equal(t, "TestExtractCriD", meta.Name)
	assert.Equal(t, "Extracts the cri-dockerd archive on the target node.", meta.Description)

	ecas, ok := s.(*ExtractCriDockerdArchiveStep)
	require.True(t, ok)
	assert.Equal(t, "remoteCriDKey", ecas.RemoteArchivePathCacheKey)
	assert.Equal(t, "/opt/extract/crid", ecas.TargetExtractBaseDir)
	assert.Equal(t, "cri-d-0.3", ecas.ArchiveInternalSubDir)
	assert.Equal(t, "extractedCriDPathKey", ecas.OutputExtractedPathCacheKey)
	assert.True(t, ecas.Sudo)
	assert.True(t, ecas.RemoveArchiveAfterExtract)

	sDefaults := NewExtractCriDockerdArchiveStep("", "", "", "", "", false, false)
	ecasDefaults, _ := sDefaults.(*ExtractCriDockerdArchiveStep)
	assert.Equal(t, "ExtractCriDockerdArchive", ecasDefaults.Meta().Name)
	assert.Equal(t, CriDockerdArchiveRemotePathCacheKey, ecasDefaults.RemoteArchivePathCacheKey)
	assert.Equal(t, "/tmp/kubexm-extracted/cri-dockerd", ecasDefaults.TargetExtractBaseDir)
	assert.Equal(t, "cri-dockerd", ecasDefaults.ArchiveInternalSubDir) // Default internal subdir
	assert.Equal(t, CriDockerdExtractedDirCacheKey, ecasDefaults.OutputExtractedPathCacheKey)
	assert.False(t, ecasDefaults.Sudo)
	assert.False(t, ecasDefaults.RemoveArchiveAfterExtract)
}

func TestExtractCriDockerdArchiveStep_Precheck_BinaryExists(t *testing.T) {
	mockRunner := &mockRunnerForExtract{} // Reusing from containerd
	mockCtx := mockStepContextForDistribute(t, mockRunner, "host-crid-precheck-exists", nil)

	baseDir := "/opt/cri_d_extract"
	internalSubDir := "cri-dockerd" // Default
	s := NewExtractCriDockerdArchiveStep("", "", baseDir, internalSubDir, "", true, false).(*ExtractCriDockerdArchiveStep)

	expectedContentDir := filepath.Join(baseDir, internalSubDir)
	expectedCriDockerdBinaryPath := filepath.Join(expectedContentDir, "cri-dockerd")

	mockRunner.ExistsFunc = func(ctx context.Context, conn connector.Connector, path string) (bool, error) {
		if path == expectedCriDockerdBinaryPath {
			return true, nil
		}
		return false, nil
	}

	done, err := s.Precheck(mockCtx, mockCtx.GetHost())
	require.NoError(t, err)
	assert.True(t, done, "Precheck should be done if key binary exists")

	cachedPath, found := mockCtx.TaskCache().Get(s.OutputExtractedPathCacheKey)
	assert.True(t, found)
	assert.Equal(t, expectedContentDir, cachedPath)
}

func TestExtractCriDockerdArchiveStep_Run_Success(t *testing.T) {
	mockRunner := &mockRunnerForExtract{}
	remoteArchivePath := "/tmp/archives/cri-dockerd-v0.3.0.tgz"
	taskCache := map[string]interface{}{CriDockerdArchiveRemotePathCacheKey: remoteArchivePath}
	mockCtx := mockStepContextForDistribute(t, mockRunner, "host-run-crid-extract", taskCache)

	baseDir := "/opt/cri_d_extract_run"
	internalSubDir := "cri-dockerd" // Default
	s := NewExtractCriDockerdArchiveStep("", CriDockerdArchiveRemotePathCacheKey, baseDir, internalSubDir, CriDockerdExtractedDirCacheKey, true, true).(*ExtractCriDockerdArchiveStep)

	expectedContentDir := filepath.Join(baseDir, internalSubDir)
	expectedCriDockerdBinaryPath := filepath.Join(expectedContentDir, "cri-dockerd")

	var mkdirCalled, extractCalled, removeCalled, binaryExistsCallCount int
	mockRunner.MkdirpFunc = func(ctx context.Context, conn connector.Connector, path, permissions string, sudo bool) error {
		if path == baseDir {
			mkdirCalled = true; assert.True(t, sudo); assert.Equal(t, "0755", permissions)
			return nil
		}
		return fmt.Errorf("unexpected Mkdirp call: %s", path)
	}
	mockRunner.ExtractFunc = func(ctx context.Context, conn connector.Connector, facts *runner.Facts, archivePath, destDir string, sudo bool) error {
		if archivePath == remoteArchivePath && destDir == baseDir {
			extractCalled = true; assert.True(t, sudo)
			// Simulate that extraction creates the internalSubDir
			// In a real scenario, the runner.Extract might handle this, or files appear in baseDir/internalSubDir
			return nil
		}
		return fmt.Errorf("unexpected Extract call: archive=%s, dest=%s", archivePath, destDir)
	}
	mockRunner.ExistsFunc = func(ctx context.Context, conn connector.Connector, path string) (bool, error) {
		if path == expectedCriDockerdBinaryPath {
			binaryExistsCallCount++
			return true, nil // Simulate binary exists after extraction
		}
		return false, nil
	}
	mockRunner.RemoveFunc = func(ctx context.Context, conn connector.Connector, path string, sudo bool) error {
		if path == remoteArchivePath {
			removeCalled = true; assert.True(t, sudo)
			return nil
		}
		return fmt.Errorf("unexpected Remove call: %s", path)
	}

	err := s.Run(mockCtx, mockCtx.GetHost())
	require.NoError(t, err)
	assert.True(t, mkdirCalled, "Mkdirp should have been called")
	assert.True(t, extractCalled, "Extract should have been called")
	assert.Equal(t, 1, binaryExistsCallCount, "Exists for cri-dockerd binary should be called once")
	assert.True(t, removeCalled, "Remove for archive should have been called")

	cachedPath, found := mockCtx.TaskCache().Get(CriDockerdExtractedDirCacheKey)
	assert.True(t, found)
	assert.Equal(t, expectedContentDir, cachedPath)
}

// Ensure mockRunnerForExtract implements runner.Runner (if not already covered in containerd tests)
var _ runner.Runner = (*mockRunnerForExtract)(nil)
// Ensure mockStepContextForDistribute implements step.StepContext
var _ step.StepContext = (*mockStepContextForDistribute)(t, nil, "", nil)

// Add dummy implementations for other runner.Runner methods for mockRunnerForExtract
// (These are likely already present from other test files in the same package `containerd` or `docker` if mocks are shared)
func (m *mockRunnerForExtract) GatherFacts(ctx context.Context, conn connector.Connector) (*runner.Facts, error) { return nil, nil }
func (m *mockRunnerForExtract) Run(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (string, error) { return "", nil }
func (m *mockRunnerForExtract) MustRun(ctx context.Context, conn connector.Connector, cmd string, sudo bool) string { return "" }
func (m *mockRunnerForExtract) Check(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (bool, error) { return false, nil }
// RunWithOptions is implemented for specific test cases
func (m *mockRunnerForExtract) Download(ctx context.Context, conn connector.Connector, facts *runner.Facts, url, destPath string, sudo bool) error { return nil }
func (m *mockRunnerForExtract) DownloadAndExtract(ctx context.Context, conn connector.Connector, facts *runner.Facts, url, destDir string, sudo bool) error { return nil }
func (m *mockRunnerForExtract) IsDir(ctx context.Context, conn connector.Connector, path string) (bool, error) { return false, nil }
func (m *mockRunnerForExtract) ReadFile(ctx context.Context, conn connector.Connector, path string) ([]byte, error) { return nil, nil }
func (m *mockRunnerForExtract) WriteFile(ctx context.Context, conn connector.Connector, content []byte, destPath, permissions string, sudo bool) error { return nil }
func (m *mockRunnerForExtract) Chmod(ctx context.Context, conn connector.Connector, path string, permissions string, sudo bool) error { return nil }
func (m *mockRunnerForExtract) Chown(ctx context.Context, conn connector.Connector, path, owner, group string, recursive bool) error { return nil }
func (m *mockRunnerForExtract) GetSHA256(ctx context.Context, conn connector.Connector, path string) (string, error) { return "", nil }
func (m *mockRunnerForExtract) LookPath(ctx context.Context, conn connector.Connector, file string) (string, error) { return "", nil }
func (m *mockRunnerForExtract) IsPortOpen(ctx context.Context, conn connector.Connector, facts *runner.Facts, port int) (bool, error) { return false, nil }
func (m *mockRunnerForExtract) WaitForPort(ctx context.Context, conn connector.Connector, facts *runner.Facts, port int, timeout time.Duration) error { return nil }
func (m *mockRunnerForExtract) SetHostname(ctx context.Context, conn connector.Connector, facts *runner.Facts, hostname string) error { return nil }
func (m *mockRunnerForExtract) AddHostEntry(ctx context.Context, conn connector.Connector, ip, fqdn string, hostnames ...string) error { return nil }
func (m *mockRunnerForExtract) InstallPackages(ctx context.Context, conn connector.Connector, facts *runner.Facts, packages ...string) error { return nil }
func (m *mockRunnerForExtract) RemovePackages(ctx context.Context, conn connector.Connector, facts *runner.Facts, packages ...string) error { return nil }
func (m *mockRunnerForExtract) UpdatePackageCache(ctx context.Context, conn connector.Connector, facts *runner.Facts) error { return nil }
func (m *mockRunnerForExtract) IsPackageInstalled(ctx context.Context, conn connector.Connector, facts *runner.Facts, packageName string) (bool, error) { return false, nil }
func (m *mockRunnerForExtract) AddRepository(ctx context.Context, conn connector.Connector, facts *runner.Facts, repoConfig string, isFilePath bool) error { return nil }
func (m *mockRunnerForExtract) StartService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForExtract) StopService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForExtract) RestartService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForExtract) EnableService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForExtract) DisableService(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) error { return nil }
func (m *mockRunnerForExtract) IsServiceActive(ctx context.Context, conn connector.Connector, facts *runner.Facts, serviceName string) (bool, error) { return false, nil }
func (m *mockRunnerForExtract) DaemonReload(ctx context.Context, conn connector.Connector, facts *runner.Facts) error { return nil }
func (m *mockRunnerForExtract) Render(ctx context.Context, conn connector.Connector, tmpl *template.Template, data interface{}, destPath, permissions string, sudo bool) error { return nil }
func (m *mockRunnerForExtract) UserExists(ctx context.Context, conn connector.Connector, username string) (bool, error) { return false, nil }
func (m *mockRunnerForExtract) GroupExists(ctx context.Context, conn connector.Connector, groupname string) (bool, error) { return false, nil }
func (m *mockRunnerForExtract) AddUser(ctx context.Context, conn connector.Connector, username, group, shell string, homeDir string, createHome bool, systemUser bool) error { return nil }
func (m *mockRunnerForExtract) AddGroup(ctx context.Context, conn connector.Connector, groupname string, systemGroup bool) error { return nil }
func (m *mockRunnerForExtract) GetPipelineCache() cache.PipelineCache { return nil }

func TestMockContextImplementation_DockerExtractCriD(t *testing.T) {
	var _ step.StepContext = mockStepContextForDistribute(t, &mockRunnerForExtract{}, "dummy", nil)
}

```text
The file `pkg/step/containerd/extract_containerd_archive_step_test.go` has been created with the provided content.
```

The unit tests for `ExtractContainerdArchiveStep` are complete.

Next up is `pkg/step/docker/extract_cri_dockerd_archive_step.go`.
