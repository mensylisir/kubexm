package containerd

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

// Reusing mockStepContextForInstallContainerd and mockRunnerForInstallContainerd
// from install_containerd_test.go as they are in the same package and suitable.

func TestInstallRuncBinaryStep_New(t *testing.T) {
	s := NewInstallRuncBinaryStep("TestInstallRunc", "runcRemoteKey", "/usr/custom/bin/runc", true)
	require.NotNil(t, s)
	meta := s.Meta()
	assert.Equal(t, "TestInstallRunc", meta.Name)
	assert.Contains(t, meta.Description, "/usr/custom/bin/runc")

	irbs, ok := s.(*InstallRuncBinaryStep)
	require.True(t, ok)
	assert.Equal(t, "runcRemoteKey", irbs.RemoteBinaryPathCacheKey)
	assert.Equal(t, "/usr/custom/bin/runc", irbs.TargetSystemPath)
	assert.True(t, irbs.Sudo)

	sDefaults := NewInstallRuncBinaryStep("", "", "", false)
	irbsDefaults, _ := sDefaults.(*InstallRuncBinaryStep)
	assert.Equal(t, "InstallRuncBinary", irbsDefaults.Meta().Name)
	assert.Equal(t, RuncBinaryRemotePathCacheKey, irbsDefaults.RemoteBinaryPathCacheKey)
	assert.Equal(t, "/usr/local/sbin/runc", irbsDefaults.TargetSystemPath)
	assert.False(t, irbsDefaults.Sudo)
}

func TestInstallRuncBinaryStep_Precheck_Exists(t *testing.T) {
	mockRunner := &mockRunnerForInstallContainerd{} // Reusing from install_containerd_test
	mockCtx := mockStepContextForInstallContainerd(t, mockRunner, "host-runc-precheck-exists", nil, nil)

	targetPath := "/usr/local/sbin/runc-test"
	s := NewInstallRuncBinaryStep("", "", targetPath, true).(*InstallRuncBinaryStep)

	mockRunner.ExistsFunc = func(ctx context.Context, conn connector.Connector, path string) (bool, error) {
		if path == targetPath {
			return true, nil
		}
		return false, nil
	}

	done, err := s.Precheck(mockCtx, mockCtx.GetHost())
	require.NoError(t, err)
	assert.True(t, done, "Precheck should be done if target binary exists")
}

func TestInstallRuncBinaryStep_Run_Success(t *testing.T) {
	mockRunner := &mockRunnerForInstallContainerd{}
	remoteBinaryPath := "/tmp/kubexm-binaries/runc.amd64.v1.1"
	taskCache := map[string]interface{}{RuncBinaryRemotePathCacheKey: remoteBinaryPath}
	mockCtx := mockStepContextForInstallContainerd(t, mockRunner, "host-run-install-runc", taskCache)

	targetPath := "/opt/bin/runc-custom"
	s := NewInstallRuncBinaryStep("", RuncBinaryRemotePathCacheKey, targetPath, true).(*InstallRuncBinaryStep)

	var mkdirPath, cpCmd, chmodPath string
	mockRunner.MkdirpFunc = func(ctx context.Context, conn connector.Connector, path, permissions string, sudo bool) error {
		mkdirPath = path
		assert.True(t, sudo)
		assert.Equal(t, "0755", permissions)
		return nil
	}
	mockRunner.RunFunc = func(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (string, error) {
		if strings.HasPrefix(cmd, "cp -fp") {
			cpCmd = cmd
			assert.True(t, sudo)
			return "", nil
		}
		return "", fmt.Errorf("unexpected Run call: %s", cmd)
	}
	mockRunner.ChmodFunc = func(ctx context.Context, conn connector.Connector, path, permissions string, sudo bool) error {
		chmodPath = path
		assert.True(t, sudo)
		assert.Equal(t, "0755", permissions)
		return nil
	}

	err := s.Run(mockCtx, mockCtx.GetHost())
	require.NoError(t, err)

	assert.Equal(t, filepath.Dir(targetPath), mkdirPath)
	assert.Equal(t, fmt.Sprintf("cp -fp %s %s", remoteBinaryPath, targetPath), cpCmd)
	assert.Equal(t, targetPath, chmodPath)
}

func TestInstallRuncBinaryStep_Rollback(t *testing.T) {
	mockRunner := &mockRunnerForInstallContainerd{}
	mockCtx := mockStepContextForInstallContainerd(t, mockRunner, "host-rollback-install-runc", nil)

	targetPath := "/usr/sbin/runc_to_remove"
	s := NewInstallRuncBinaryStep("", "", targetPath, true).(*InstallRuncBinaryStep)

	var removeCalledWithPath string
	var removeCalledWithSudo bool
	mockRunner.RemoveFunc = func(ctx context.Context, conn connector.Connector, path string, sudo bool) error {
		removeCalledWithPath = path
		removeCalledWithSudo = sudo
		return nil
	}

	err := s.Rollback(mockCtx, mockCtx.GetHost())
	require.NoError(t, err)
	assert.Equal(t, targetPath, removeCalledWithPath)
	assert.True(t, removeCalledWithSudo)
}

// Ensure mockRunnerForInstallContainerd implements runner.Runner
var _ runner.Runner = (*mockRunnerForInstallContainerd)(nil)
// Ensure mockStepContextForInstallContainerd implements step.StepContext
var _ step.StepContext = (*mockStepContextForInstallContainerd)(t, nil, "", nil, nil)

// Add dummy implementations for other runner.Runner methods for mockRunnerForInstallContainerd
// (These are already present in install_containerd_test.go)

func TestMockContextImplementation_InstallRunc(t *testing.T) {
	var _ step.StepContext = mockStepContextForInstallContainerd(t, &mockRunnerForInstallContainerd{}, "dummy", nil, nil)
}
