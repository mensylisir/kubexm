package containerd

import (
	"context"
	"fmt"
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
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step"
)

// mockRunnerForExtract is a helper mock for runner.Runner focused on extract step needs.
type mockRunnerForExtract struct {
	runner.Runner // Embed to satisfy the interface
	ExistsFunc    func(ctx context.Context, conn connector.Connector, path string) (bool, error)
	MkdirpFunc    func(ctx context.Context, conn connector.Connector, path, permissions string, sudo bool) error
	ExtractFunc   func(ctx context.Context, conn connector.Connector, facts *runner.Facts, archivePath, destDir string, sudo bool) error
	RemoveFunc    func(ctx context.Context, conn connector.Connector, path string, sudo bool) error
	// Add RunWithOptions if ls -lR in Run needs to be mocked for specific tests
	RunWithOptionsFunc func(ctx context.Context, conn connector.Connector, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error)
}

func (m *mockRunnerForExtract) Exists(ctx context.Context, conn connector.Connector, path string) (bool, error) {
	if m.ExistsFunc != nil {
		return m.ExistsFunc(ctx, conn, path)
	}
	return false, fmt.Errorf("ExistsFunc not implemented")
}
func (m *mockRunnerForExtract) Mkdirp(ctx context.Context, conn connector.Connector, path, permissions string, sudo bool) error {
	if m.MkdirpFunc != nil {
		return m.MkdirpFunc(ctx, conn, path, permissions, sudo)
	}
	return fmt.Errorf("MkdirpFunc not implemented")
}
func (m *mockRunnerForExtract) Extract(ctx context.Context, conn connector.Connector, facts *runner.Facts, archivePath, destDir string, sudo bool) error {
	if m.ExtractFunc != nil {
		return m.ExtractFunc(ctx, conn, facts, archivePath, destDir, sudo)
	}
	return fmt.Errorf("ExtractFunc not implemented")
}
func (m *mockRunnerForExtract) Remove(ctx context.Context, conn connector.Connector, path string, sudo bool) error {
	if m.RemoveFunc != nil {
		return m.RemoveFunc(ctx, conn, path, sudo)
	}
	return fmt.Errorf("RemoveFunc not implemented")
}
func (m *mockRunnerForExtract) RunWithOptions(ctx context.Context, conn connector.Connector, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) {
	if m.RunWithOptionsFunc != nil {
		return m.RunWithOptionsFunc(ctx, conn, cmd, opts)
	}
	return nil, nil, fmt.Errorf("RunWithOptionsFunc not implemented")
}


func TestExtractCNIPluginsArchiveStep_New(t *testing.T) {
	s := NewExtractCNIPluginsArchiveStep("TestExtractCNI", "remoteCNIKey", "/opt/cni/custom_bin", "extractedCNIPathKey", true, true)
	require.NotNil(t, s)
	meta := s.Meta()
	assert.Equal(t, "TestExtractCNI", meta.Name)
	assert.Contains(t, meta.Description, "/opt/cni/custom_bin")

	ecas, ok := s.(*ExtractCNIPluginsArchiveStep)
	require.True(t, ok)
	assert.Equal(t, "remoteCNIKey", ecas.RemoteArchivePathCacheKey)
	assert.Equal(t, "/opt/cni/custom_bin", ecas.TargetCNIBinDir)
	assert.Equal(t, "extractedCNIPathKey", ecas.OutputExtractedPathCacheKey)
	assert.True(t, ecas.Sudo)
	assert.True(t, ecas.RemoveArchiveAfterExtract)

	sDefaults := NewExtractCNIPluginsArchiveStep("", "", "", "", false, false)
	ecasDefaults, _ := sDefaults.(*ExtractCNIPluginsArchiveStep)
	assert.Equal(t, "ExtractCNIPluginsArchive", ecasDefaults.Meta().Name)
	assert.Equal(t, CNIPluginsArchiveRemotePathCacheKey, ecasDefaults.RemoteArchivePathCacheKey)
	assert.Equal(t, "/opt/cni/bin", ecasDefaults.TargetCNIBinDir)
	assert.Equal(t, CNIPluginsExtractedDirCacheKey, ecasDefaults.OutputExtractedPathCacheKey)
	assert.False(t, ecasDefaults.Sudo)
	assert.False(t, ecasDefaults.RemoveArchiveAfterExtract)
}

func TestExtractCNIPluginsArchiveStep_Precheck_PluginsExist(t *testing.T) {
	mockRunner := &mockRunnerForExtract{}
	// Using mockStepContextForDistribute as it's compatible enough for these tests
	mockCtx := mockStepContextForDistribute(t, mockRunner, "host-cni-precheck-exists", nil)

	targetDir := "/opt/cni/bin"
	s := NewExtractCNIPluginsArchiveStep("", "", targetDir, "", true, false).(*ExtractCNIPluginsArchiveStep)
	expectedBridgePath := filepath.Join(targetDir, "bridge")

	mockRunner.ExistsFunc = func(ctx context.Context, conn connector.Connector, path string) (bool, error) {
		if path == expectedBridgePath {
			return true, nil // Simulate bridge plugin exists
		}
		return false, nil
	}

	done, err := s.Precheck(mockCtx, mockCtx.GetHost())
	require.NoError(t, err)
	assert.True(t, done, "Precheck should be done if key CNI plugin exists")

	cachedPath, found := mockCtx.TaskCache().Get(s.OutputExtractedPathCacheKey)
	assert.True(t, found)
	assert.Equal(t, targetDir, cachedPath)
}

func TestExtractCNIPluginsArchiveStep_Precheck_PluginsNotExist(t *testing.T) {
	mockRunner := &mockRunnerForExtract{}
	mockCtx := mockStepContextForDistribute(t, mockRunner, "host-cni-precheck-noexist", nil)
	targetDir := "/opt/cni/bin"
	s := NewExtractCNIPluginsArchiveStep("", "", targetDir, "", true, false).(*ExtractCNIPluginsArchiveStep)
	expectedBridgePath := filepath.Join(targetDir, "bridge")

	mockRunner.ExistsFunc = func(ctx context.Context, conn connector.Connector, path string) (bool, error) {
		if path == expectedBridgePath {
			return false, nil // Simulate bridge plugin does not exist
		}
		return false, nil
	}

	done, err := s.Precheck(mockCtx, mockCtx.GetHost())
	require.NoError(t, err)
	assert.False(t, done, "Precheck should not be done if key CNI plugin does not exist")
}

func TestExtractCNIPluginsArchiveStep_Run_Success(t *testing.T) {
	mockRunner := &mockRunnerForExtract{}
	remoteArchivePath := "/tmp/archives/cni-plugins-v1.0.0.tgz"
	taskCache := map[string]interface{}{CNIPluginsArchiveRemotePathCacheKey: remoteArchivePath}
	mockCtx := mockStepContextForDistribute(t, mockRunner, "host-run-cni-extract", taskCache)

	targetDir := "/opt/cni/bin"
	s := NewExtractCNIPluginsArchiveStep("", CNIPluginsArchiveRemotePathCacheKey, targetDir, CNIPluginsExtractedDirCacheKey, true, true).(*ExtractCNIPluginsArchiveStep)

	var mkdirCalled, extractCalled, removeCalled bool
	mockRunner.MkdirpFunc = func(ctx context.Context, conn connector.Connector, path, permissions string, sudo bool) error {
		if path == targetDir {
			mkdirCalled = true
			assert.True(t, sudo)
			assert.Equal(t, "0755", permissions)
			return nil
		}
		return fmt.Errorf("unexpected Mkdirp call: %s", path)
	}
	mockRunner.ExtractFunc = func(ctx context.Context, conn connector.Connector, facts *runner.Facts, archivePath, destDir string, sudo bool) error {
		if archivePath == remoteArchivePath && destDir == targetDir {
			extractCalled = true
			assert.True(t, sudo)
			return nil
		}
		return fmt.Errorf("unexpected Extract call: archive=%s, dest=%s", archivePath, destDir)
	}
	// Mock Exists for post-extraction check
	mockRunner.ExistsFunc = func(ctx context.Context, conn connector.Connector, path string) (bool, error) {
		if path == filepath.Join(targetDir, "bridge") {
			return true, nil // Simulate bridge plugin exists after extraction
		}
		return false, nil
	}
	mockRunner.RemoveFunc = func(ctx context.Context, conn connector.Connector, path string, sudo bool) error {
		if path == remoteArchivePath {
			removeCalled = true
			assert.True(t, sudo)
			return nil
		}
		return fmt.Errorf("unexpected Remove call: %s", path)
	}

	err := s.Run(mockCtx, mockCtx.GetHost())
	require.NoError(t, err)
	assert.True(t, mkdirCalled, "Mkdirp should have been called")
	assert.True(t, extractCalled, "Extract should have been called")
	assert.True(t, removeCalled, "Remove for archive should have been called because RemoveArchiveAfterExtract is true")

	cachedPath, found := mockCtx.TaskCache().Get(CNIPluginsExtractedDirCacheKey)
	assert.True(t, found)
	assert.Equal(t, targetDir, cachedPath)
}

func TestExtractCNIPluginsArchiveStep_Run_PostExtractionCheckFails(t *testing.T) {
	mockRunner := &mockRunnerForExtract{}
	remoteArchivePath := "/tmp/archives/cni-plugins-bad.tgz"
	taskCache := map[string]interface{}{CNIPluginsArchiveRemotePathCacheKey: remoteArchivePath}
	mockCtx := mockStepContextForDistribute(t, mockRunner, "host-run-cni-extract-fail", taskCache)

	targetDir := "/opt/cni/bin"
	s := NewExtractCNIPluginsArchiveStep("", CNIPluginsArchiveRemotePathCacheKey, targetDir, CNIPluginsExtractedDirCacheKey, true, false).(*ExtractCNIPluginsArchiveStep)

	mockRunner.MkdirpFunc = func(ctx context.Context, conn connector.Connector, path, permissions string, sudo bool) error { return nil }
	mockRunner.ExtractFunc = func(ctx context.Context, conn connector.Connector, facts *runner.Facts, archivePath, destDir string, sudo bool) error { return nil }
	mockRunner.ExistsFunc = func(ctx context.Context, conn connector.Connector, path string) (bool, error) {
		if path == filepath.Join(targetDir, "bridge") {
			return false, nil // Simulate bridge plugin does NOT exist after extraction
		}
		return false, nil
	}
	// Mock RunWithOptions for ls -lR if it gets called
	mockRunner.RunWithOptionsFunc = func(ctx context.Context, conn connector.Connector, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) {
		if strings.Contains(cmd, "ls -lR") {
			return []byte("drwxr-xr-x someuser somegroup 0 Jan 1 00:00 ."), nil, nil
		}
		return nil, nil, fmt.Errorf("unexpected RunWithOptions call")
	}


	err := s.Run(mockCtx, mockCtx.GetHost())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "CNI bridge plugin not found")
	assert.Contains(t, err.Error(), "after extraction")
}


func TestExtractCNIPluginsArchiveStep_Rollback(t *testing.T) {
	mockRunner := &mockRunnerForExtract{}
	targetDir := "/opt/cni/bin"
	taskCache := map[string]interface{}{CNIPluginsExtractedDirCacheKey: targetDir}
	mockCtx := mockStepContextForDistribute(t, mockRunner, "host-rollback-cni-extract", taskCache)

	s := NewExtractCNIPluginsArchiveStep("", "", targetDir, CNIPluginsExtractedDirCacheKey, true, false).(*ExtractCNIPluginsArchiveStep)

	err := s.Rollback(mockCtx, mockCtx.GetHost())
	require.NoError(t, err, "Rollback is a no-op for file removal but should clear cache")

	_, found := mockCtx.TaskCache().Get(CNIPluginsExtractedDirCacheKey)
	assert.False(t, found, "Cache key should be deleted on rollback")
}

// Ensure mockRunnerForExtract implements runner.Runner
var _ runner.Runner = (*mockRunnerForExtract)(nil)
// Ensure mockStepContextForDistribute implements step.StepContext (it does via runtime.Context)
var _ step.StepContext = (*mockStepContextForDistribute)(t, nil, "", nil)

// Add dummy implementations for other runner.Runner methods for mockRunnerForExtract
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

// Adding missing runner methods for mockRunnerForExtract
func (m *mockRunnerForExtract) GetPipelineCache() cache.PipelineCache { return nil }

// Adding missing StepContext methods for mockStepContextForExtract
// func (m *mockStepContextForExtract) GetPipelineCache() cache.PipelineCache { return nil }

// GlobalWorkDir, ClusterConfig, etc., are direct fields on mockStepContextForExtract
// Logger, GoCtx are direct fields.
// GetRunner is a method on mockStepContextForExtract that returns its runner field.

// Ensure mockRunnerForExtract implements runner.Runner
// var _ runner.Runner = (*mockRunnerForExtract)(nil) // Already checked in other tests

// Ensure mockStepContextForExtract implements step.StepContext
// var _ step.StepContext = (*mockStepContextForExtract)(t, nil, "", nil) // Already checked in other tests

// Add dummy methods to mockStepContextForDistribute (if it were redefined locally)
// type mockStepContextForDistribute struct { mainCtx *runtime.Context }
// ... (implementations delegating to mainCtx)
// For tests in the same package, the original mockStepContextForDistribute helper from
// distribute_cni_plugins_archive_step_test.go is used, which correctly implements step.StepContext.
// The type assertion for mockStepContextForDistribute can be done once.
func TestMockContextImplementation_CNIExtract(t *testing.T) {
	var _ step.StepContext = mockStepContextForDistribute(t, &mockRunnerForDistribute{}, "dummy", nil)
}

```text
The file `pkg/step/containerd/distribute_runc_binary_step_test.go` has been created with the provided content.
```

The unit tests for `DistributeRuncBinaryStep` are complete.

Next up is `pkg/step/containerd/extract_cni_plugins_archive_step.go`.
