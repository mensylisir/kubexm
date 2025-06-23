package containerd

import (
	"context"
	"fmt"
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

// Reusing mockStepContextForContainerdCleanup and mockRunnerForCleanup
// from cleanup_containerd_config_step_test.go as they are in the same package.

func TestRemoveContainerdBinariesStep_New(t *testing.T) {
	customPaths := []string{"/usr/bin/myctd", "/usr/bin/myctr"}
	s := NewRemoveContainerdBinariesStep("TestRemoveBins", customPaths, true)
	require.NotNil(t, s)
	meta := s.Meta()
	assert.Equal(t, "TestRemoveBins", meta.Name)
	assert.Contains(t, meta.Description, "/usr/bin/myctd")
	assert.Contains(t, meta.Description, "/usr/bin/myctr")

	rcbs, ok := s.(*RemoveContainerdBinariesStep)
	require.True(t, ok)
	assert.Equal(t, customPaths, rcbs.BinaryPaths)
	assert.True(t, rcbs.Sudo)

	// Test defaults
	sDefaults := NewRemoveContainerdBinariesStep("", nil, false) // Sudo defaults to true in constructor
	rcbsDefaults, _ := sDefaults.(*RemoveContainerdBinariesStep)
	assert.Equal(t, "RemoveContainerdBinaries", rcbsDefaults.Meta().Name)
	assert.NotEmpty(t, rcbsDefaults.BinaryPaths, "Default binary paths should be populated")
	assert.True(t, rcbsDefaults.Sudo) // Constructor overrides passed false with true
}

func TestRemoveContainerdBinariesStep_Precheck_AllMissing(t *testing.T) {
	mockRunner := &mockRunnerForCleanup{} // Reusing from cleanup_containerd_config_step_test.go
	mockCtx := mockStepContextForContainerdCleanup(t, mockRunner, "host-removebins-precheck-missing")

	s := NewRemoveContainerdBinariesStep("", nil, true).(*RemoveContainerdBinariesStep) // Use default paths

	mockRunner.ExistsFunc = func(ctx context.Context, conn connector.Connector, path string) (bool, error) {
		// Check if path is one of the default binary paths or /opt/cni/bin
		isDefaultPath := false
		for _, defaultBin := range s.BinaryPaths {
			if path == defaultBin {
				isDefaultPath = true
				break
			}
		}
		if path == "/opt/cni/bin" { // This path is not checked in Precheck, only in Run
			// For Precheck, only BinaryPaths are checked.
			// return false, fmt.Errorf("unexpected Exists call for /opt/cni/bin in Precheck")
		}
		if isDefaultPath {
			return false, nil // Simulate all default binaries are missing
		}
		return false, fmt.Errorf("unexpected Exists call for path in Precheck: %s", path)
	}

	done, err := s.Precheck(mockCtx, mockCtx.GetHost())
	require.NoError(t, err)
	assert.True(t, done, "Precheck should be done if all binaries are missing")
}

func TestRemoveContainerdBinariesStep_Precheck_SomeExist(t *testing.T) {
	mockRunner := &mockRunnerForCleanup{}
	mockCtx := mockStepContextForContainerdCleanup(t, mockRunner, "host-removebins-precheck-exists")
	s := NewRemoveContainerdBinariesStep("", nil, true).(*RemoveContainerdBinariesStep)

	mockRunner.ExistsFunc = func(ctx context.Context, conn connector.Connector, path string) (bool, error) {
		if path == s.BinaryPaths[0] { // Simulate first default binary exists
			return true, nil
		}
		// Other default binaries or /opt/cni/bin
		return false, nil
	}

	done, err := s.Precheck(mockCtx, mockCtx.GetHost())
	require.NoError(t, err)
	assert.False(t, done, "Precheck should not be done if some binaries still exist")
}

func TestRemoveContainerdBinariesStep_Run_Success(t *testing.T) {
	mockRunner := &mockRunnerForCleanup{}
	mockCtx := mockStepContextForContainerdCleanup(t, mockRunner, "host-run-removebins")

	defaultPaths := []string{
		"/usr/local/bin/containerd", "/usr/local/bin/ctr", // subset for test clarity
	}
	cniDir := "/opt/cni/bin"
	s := NewRemoveContainerdBinariesStep("", defaultPaths, true).(*RemoveContainerdBinariesStep)

	removedItems := make(map[string]bool)
	mockRunner.RemoveFunc = func(ctx context.Context, conn connector.Connector, path string, sudo bool) error {
		assert.True(t, sudo, "Remove should be called with sudo for path: %s", path)
		removedItems[path] = true
		return nil
	}

	err := s.Run(mockCtx, mockCtx.GetHost())
	require.NoError(t, err)

	for _, path := range defaultPaths {
		assert.True(t, removedItems[path], "Binary path %s should have been removed", path)
	}
	assert.True(t, removedItems[cniDir], "CNI bin directory %s should have been removed", cniDir)
}

func TestRemoveContainerdBinariesStep_Run_RemoveError_BestEffort(t *testing.T) {
	mockRunner := &mockRunnerForCleanup{}
	mockCtx := mockStepContextForContainerdCleanup(t, mockRunner, "host-run-removebins-err")

	pathsToTest := []string{"/usr/bin/bin1", "/usr/bin/bin2"}
	errorPath := "/usr/bin/bin1" // This path will cause an error
	s := NewRemoveContainerdBinariesStep("", pathsToTest, true).(*RemoveContainerdBinariesStep)
	expectedErrStr := fmt.Sprintf("failed to remove %s", errorPath)

	removedItems := make(map[string]bool)
	mockRunner.RemoveFunc = func(ctx context.Context, conn connector.Connector, path string, sudo bool) error {
		if path == errorPath {
			return fmt.Errorf("simulated remove error for %s", path)
		}
		removedItems[path] = true
		return nil
	}

	err := s.Run(mockCtx, mockCtx.GetHost())
	require.Error(t, err, "Run should return an error if any removal fails")
	assert.Contains(t, err.Error(), "one or more errors occurred")
	assert.Contains(t, err.Error(), expectedErrStr)

	assert.False(t, removedItems[errorPath], "Path that errored should not be marked as successfully removed (by this test's map logic)")
	assert.True(t, removedItems["/usr/bin/bin2"], "Other paths should still be attempted for removal")
	// CNI dir removal would also be attempted, check if needed
}

func TestRemoveContainerdBinariesStep_Rollback(t *testing.T) {
	mockRunner := &mockRunnerForCleanup{}
	mockCtx := mockStepContextForContainerdCleanup(t, mockRunner, "host-rollback-removebins")
	s := NewRemoveContainerdBinariesStep("", nil, true).(*RemoveContainerdBinariesStep)

	err := s.Rollback(mockCtx, mockCtx.GetHost())
	assert.NoError(t, err, "Rollback should be a no-op and not return an error")
}

// Ensure mockRunnerForCleanup implements runner.Runner
var _ runner.Runner = (*mockRunnerForCleanup)(nil)
// Ensure mockStepContextForContainerdCleanup implements step.StepContext
var _ step.StepContext = (*mockStepContextForContainerdCleanup)(t, nil, "")

// Add dummy implementations for other runner.Runner methods for mockRunnerForCleanup
// (These are already present in cleanup_containerd_config_step_test.go)

// Add dummy StepContext methods for mockStepContextForContainerdCleanup
// (These are already present in cleanup_containerd_config_step_test.go)

func TestMockContextImplementation_RemoveCtdBins(t *testing.T) {
	var _ step.StepContext = mockStepContextForContainerdCleanup(t, &mockRunnerForCleanup{}, "dummy")
}
