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

// Reusing mockStepContextForDistribute and mockRunnerForDistribute from
// distribute_cni_plugins_archive_step_test.go or distribute_containerd_archive_step_test.go

func TestDistributeRuncBinaryStep_New(t *testing.T) {
	s := NewDistributeRuncBinaryStep("TestDistributeRunc", "localRuncKey", "/remote/runc/tmp", "runc-v1.1", "remoteRuncKey", true)
	require.NotNil(t, s)
	meta := s.Meta()
	assert.Equal(t, "TestDistributeRunc", meta.Name)
	assert.Equal(t, "Uploads the runc binary to target nodes.", meta.Description)

	drbs, ok := s.(*DistributeRuncBinaryStep)
	require.True(t, ok)
	assert.Equal(t, "localRuncKey", drbs.LocalBinaryPathCacheKey)
	assert.Equal(t, "/remote/runc/tmp", drbs.RemoteTempDir)
	assert.Equal(t, "runc-v1.1", drbs.RemoteBinaryName)
	assert.Equal(t, "remoteRuncKey", drbs.OutputRemotePathCacheKey)
	assert.True(t, drbs.Sudo)

	sDefaults := NewDistributeRuncBinaryStep("", "", "", "runc.amd64", "", false)
	drbsDefaults, _ := sDefaults.(*DistributeRuncBinaryStep)
	assert.Equal(t, "DistributeRuncBinary", drbsDefaults.Meta().Name)
	assert.Equal(t, RuncBinaryLocalPathCacheKey, drbsDefaults.LocalBinaryPathCacheKey)
	assert.Equal(t, "/tmp/kubexm-binaries", drbsDefaults.RemoteTempDir)
	assert.Equal(t, RuncBinaryRemotePathCacheKey, drbsDefaults.OutputRemotePathCacheKey)
	assert.False(t, drbsDefaults.Sudo)
}

func TestDistributeRuncBinaryStep_Precheck_RemoteExists(t *testing.T) {
	mockRunner := &mockRunnerForDistribute{}
	mockCtx := mockStepContextForDistribute(t, mockRunner, "host-runc-precheck", nil)

	remoteTempDir := "/var/tmp/kubexm_runc_bin"
	remoteBinaryName := "runc-v1.1.0"
	expectedRemotePath := filepath.Join(remoteTempDir, remoteBinaryName)
	s := NewDistributeRuncBinaryStep("", "", remoteTempDir, remoteBinaryName, "", true).(*DistributeRuncBinaryStep)

	mockRunner.ExistsFunc = func(ctx context.Context, conn connector.Connector, path string) (bool, error) {
		if path == expectedRemotePath {
			return true, nil
		}
		return false, nil
	}

	done, err := s.Precheck(mockCtx, mockCtx.GetHost())
	require.NoError(t, err)
	assert.True(t, done, "Precheck should be done if remote binary exists")

	cachedPath, found := mockCtx.TaskCache().Get(s.OutputRemotePathCacheKey)
	assert.True(t, found)
	assert.Equal(t, expectedRemotePath, cachedPath)
}

func TestDistributeRuncBinaryStep_Run_Success(t *testing.T) {
	mockRunner := &mockRunnerForDistribute{}
	localBinaryPath := "/control/path/runc.amd64.v1.1.0"
	taskCache := map[string]interface{}{RuncBinaryLocalPathCacheKey: localBinaryPath}
	mockCtx := mockStepContextForDistribute(t, mockRunner, "host-run-runc", taskCache)

	remoteTempDir := "/opt/kubexm_stage/runc"
	remoteBinaryName := "runc-on-remote" // Explicitly set
	s := NewDistributeRuncBinaryStep("", RuncBinaryLocalPathCacheKey, remoteTempDir, remoteBinaryName, RuncBinaryRemotePathCacheKey, true).(*DistributeRuncBinaryStep)

	expectedRemotePath := filepath.Join(remoteTempDir, remoteBinaryName)
	var mkdirCalled, uploadCalled bool

	mockRunner.MkdirpFunc = func(ctx context.Context, conn connector.Connector, path, permissions string, sudo bool) error {
		if path == remoteTempDir {
			mkdirCalled = true
			assert.True(t, sudo)
			assert.Equal(t, "0750", permissions)
			return nil
		}
		return fmt.Errorf("unexpected Mkdirp call: %s", path)
	}
	mockRunner.UploadFileFunc = func(ctx context.Context, localSrc, remoteDest string, options *connector.FileTransferOptions, targetHost connector.Host) error {
		if localSrc == localBinaryPath && remoteDest == expectedRemotePath {
			uploadCalled = true
			assert.True(t, options.Sudo)
			assert.Equal(t, "0755", options.Permissions, "Runc binary should have executable permissions")
			assert.Equal(t, mockCtx.GetHost().GetName(), targetHost.GetName())
			return nil
		}
		return fmt.Errorf("unexpected UploadFile call: local=%s, remote=%s", localSrc, remoteDest)
	}

	err := s.Run(mockCtx, mockCtx.GetHost())
	require.NoError(t, err)
	assert.True(t, mkdirCalled, "Mkdirp should have been called for remote temp directory")
	assert.True(t, uploadCalled, "UploadFile should have been called")

	cachedPath, found := mockCtx.TaskCache().Get(RuncBinaryRemotePathCacheKey)
	assert.True(t, found)
	assert.Equal(t, expectedRemotePath, cachedPath)
}

func TestDistributeRuncBinaryStep_Rollback(t *testing.T) {
	mockRunner := &mockRunnerForDistribute{}
	remoteTempDir := "/tmp/cleanme_runc"
	remoteBinaryName := "runc-for-rollback"
	expectedRemotePath := filepath.Join(remoteTempDir, remoteBinaryName)

	taskCache := map[string]interface{}{RuncBinaryRemotePathCacheKey: expectedRemotePath}
	mockCtx := mockStepContextForDistribute(t, mockRunner, "host-rollback-runc", taskCache)

	s := NewDistributeRuncBinaryStep("", "", remoteTempDir, "", RuncBinaryRemotePathCacheKey, true).(*DistributeRuncBinaryStep)

	var removeCalledWithPath string
	mockRunner.RemoveFunc = func(ctx context.Context, conn connector.Connector, path string, sudo bool) error {
		removeCalledWithPath = path
		assert.True(t, sudo)
		return nil
	}

	err := s.Rollback(mockCtx, mockCtx.GetHost())
	require.NoError(t, err)
	assert.Equal(t, expectedRemotePath, removeCalledWithPath)
	_, found := mockCtx.TaskCache().Get(RuncBinaryRemotePathCacheKey)
	assert.False(t, found, "Cache key should be deleted on rollback")
}

// Ensure mockRunnerForDistribute implements runner.Runner if not already covered
var _ runner.Runner = (*mockRunnerForDistribute)(nil)
// Ensure mockStepContextForDistribute implements step.StepContext
var _ step.StepContext = (*mockStepContextForDistribute)(t, nil, "", nil)

// Add dummy implementations for other runner.Runner methods for mockRunnerForDistribute
// (These are already present in distribute_cni_plugins_archive_step_test.go,
// assuming they are in the same package `containerd` for testing.)

// Add remaining StepContext methods for mockStepContextForDistribute
// (Similar to above, these are likely already present from the CNI test file if in same package)

// Adding a type assertion for the context used in tests to be sure:
func TestMockContextImplementation_RuncDistribute(t *testing.T) {
	var _ step.StepContext = mockStepContextForDistribute(t, &mockRunnerForDistribute{}, "dummy", nil)
}

// Add dummy methods to mockStepContextForDistribute if they are not inherited from runtime.Context correctly
// For this test structure, mockStepContextForDistribute creates a runtime.Context which implements step.StepContext.
// The mockRunner is injected into this runtime.Context.
// So, the mockStepContextForDistribute helper itself returns a valid step.StepContext.

// Add dummy runner methods (if not already present from CNI/Containerd test file in same package)
func (m *mockRunnerForDistribute) Render(ctx context.Context, conn connector.Connector, tmpl *template.Template, data interface{}, destPath, permissions string, sudo bool) error { return nil }
func (m *mockRunnerForDistribute) UserExists(ctx context.Context, conn connector.Connector, username string) (bool, error) { return false, nil }
func (m *mockRunnerForDistribute) GroupExists(ctx context.Context, conn connector.Connector, groupname string) (bool, error) { return false, nil }
func (m *mockRunnerForDistribute) AddUser(ctx context.Context, conn connector.Connector, username, group, shell string, homeDir string, createHome bool, systemUser bool) error { return nil }
func (m *mockRunnerForDistribute) AddGroup(ctx context.Context, conn connector.Connector, groupname string, systemGroup bool) error { return nil }

// Add dummy step context methods (if not already present from CNI/Containerd test file in same package)
// These are just to satisfy the interface if the mock context is defined locally.
// The `mockStepContextForDistribute` used in the CNI test already does this by wrapping `runtime.Context`.
// func (m *mockStepContextForDistribute) GetHost() connector.Host { return m.mainCtx.GetHost() }
// ... and so on for all step.StepContext methods, delegating to m.mainCtx.
// The current `mockStepContextForDistribute` helper returns a `*runtime.Context` cast to `step.StepContext`,
// which is a valid approach.

// Adding missing runner methods for mockRunnerForDistribute
func (m *mockRunnerForDistribute) GetPipelineCache() cache.PipelineCache { return nil }

// Adding missing StepContext methods for mockStepContextForDistribute
// func (m *mockStepContextForDistribute) GetPipelineCache() cache.PipelineCache { return nil }

// GlobalWorkDir, ClusterConfig, etc., are direct fields on mockStepContextForDistribute
// Logger, GoCtx are direct fields.
// GetRunner is a method on mockStepContextForDistribute that returns its runner field.

// Ensure mockRunnerForDistribute implements runner.Runner
// var _ runner.Runner = (*mockRunnerForDistribute)(nil) // Already checked in other tests

// Ensure mockStepContextForDistribute implements step.StepContext
// var _ step.StepContext = (*mockStepContextForDistribute)(t, nil, "", nil) // Already checked in other tests
