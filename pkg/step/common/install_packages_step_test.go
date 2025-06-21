package common

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/logger"
	"github.com/mensylisir/kubexm/pkg/runner"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step"
)

// mockInstallPackagesStepContext and newMockInstallPackagesStepContext can be similar to those in other common step tests
type mockIPSContext struct {
	runtime.StepContext
	logger     *logger.Logger
	goCtx      context.Context
	mockRunner *mockIPSRunner
	mockHost   connector.Host
	mockConn   connector.Connector
	hostFacts  *runner.Facts
}

type mockIPSRunner struct {
	runner.Runner // Embed the interface
	IsPackageInstalledFunc func(ctx context.Context, conn connector.Connector, facts *runner.Facts, packageName string) (bool, error)
	InstallPackagesFunc    func(ctx context.Context, conn connector.Connector, facts *runner.Facts, packages ...string) error
	RemovePackagesFunc     func(ctx context.Context, conn connector.Connector, facts *runner.Facts, packages ...string) error
}

func (m *mockIPSRunner) IsPackageInstalled(ctx context.Context, conn connector.Connector, facts *runner.Facts, packageName string) (bool, error) {
	if m.IsPackageInstalledFunc != nil {
		return m.IsPackageInstalledFunc(ctx, conn, facts, packageName)
	}
	return false, fmt.Errorf("IsPackageInstalledFunc not implemented")
}

func (m *mockIPSRunner) InstallPackages(ctx context.Context, conn connector.Connector, facts *runner.Facts, packages ...string) error {
	if m.InstallPackagesFunc != nil {
		return m.InstallPackagesFunc(ctx, conn, facts, packages...)
	}
	return fmt.Errorf("InstallPackagesFunc not implemented")
}
func (m *mockIPSRunner) RemovePackages(ctx context.Context, conn connector.Connector, facts *runner.Facts, packages ...string) error {
	if m.RemovePackagesFunc != nil {
		return m.RemovePackagesFunc(ctx, conn, facts, packages...)
	}
	return fmt.Errorf("RemovePackagesFunc not implemented")
}


func newMockIPSContext(t *testing.T) *mockIPSContext {
	l, _ := logger.New(logger.DefaultConfig())
	mockHost := &step.MockHost{MockName: "test-host-ips"}
	mockConn := &step.MockStepConnector{} // Using the general mock connector
	mockRun := &mockIPSRunner{}

	return &mockIPSContext{
		logger:     l,
		goCtx:      context.Background(),
		mockRunner: mockRun,
		mockHost:   mockHost,
		mockConn:   mockConn,
		hostFacts:  &runner.Facts{OS: &connector.OS{ID: "linux"}}, // Basic facts
	}
}
func (m *mockIPSContext) GetLogger() *logger.Logger                                  { return m.logger }
func (m *mockIPSContext) GoContext() context.Context                                   { return m.goCtx }
func (m *mockIPSContext) GetRunner() runner.Runner                                   { return m.mockRunner }
func (m *mockIPSContext) GetConnectorForHost(h connector.Host) (connector.Connector, error) { return m.mockConn, nil }
func (m *mockIPSContext) GetHostFacts(h connector.Host) (*runner.Facts, error)           { return m.hostFacts, nil }
func (m *mockIPSContext) GetHost() connector.Host                                      { return m.mockHost }
func (m *mockIPSContext) GetCurrentHostFacts() (*runner.Facts, error)                  { return m.hostFacts, nil }
func (m *mockIPSContext) GetCurrentHostConnector() (connector.Connector, error)        { return m.mockConn, nil }
// Add other StepContext methods if needed, returning nil or default values
func (m *mockIPSContext) StepCache() runtime.StepCache     { return nil }
func (m *mockIPSContext) TaskCache() runtime.TaskCache     { return nil }
func (m *mockIPSContext) ModuleCache() runtime.ModuleCache   { return nil }
func (m *mockIPSContext) GetGlobalWorkDir() string         { return "/tmp/gwd" }


func TestInstallPackagesStep_NewInstallPackagesStep(t *testing.T) {
	pkgs := []string{"nginx", "vim"}
	s := NewInstallPackagesStep(pkgs, "Install Essential Tools")
	require.NotNil(t, s)
	meta := s.Meta()
	assert.Equal(t, "Install Essential Tools", meta.Name)
	assert.Contains(t, meta.Description, "nginx")
	assert.Contains(t, meta.Description, "vim")

	sDefaultName := NewInstallPackagesStep(pkgs, "")
	assert.Equal(t, "InstallPackages", sDefaultName.Meta().Name)
}

func TestInstallPackagesStep_Precheck_AllInstalled(t *testing.T) {
	mockCtx := newMockIPSContext(t)
	pkgs := []string{"nginx", "vim"}
	ips := NewInstallPackagesStep(pkgs, "").(*InstallPackagesStep)

	mockCtx.mockRunner.IsPackageInstalledFunc = func(ctx context.Context, conn connector.Connector, facts *runner.Facts, pkgName string) (bool, error) {
		assert.Contains(t, pkgs, pkgName)
		return true, nil // All packages are installed
	}

	done, err := ips.Precheck(mockCtx, mockCtx.mockHost)
	require.NoError(t, err)
	assert.True(t, done)
}

func TestInstallPackagesStep_Precheck_SomeNotInstalled(t *testing.T) {
	mockCtx := newMockIPSContext(t)
	pkgs := []string{"nginx", "uncommon-pkg"}
	ips := NewInstallPackagesStep(pkgs, "").(*InstallPackagesStep)

	mockCtx.mockRunner.IsPackageInstalledFunc = func(ctx context.Context, conn connector.Connector, facts *runner.Facts, pkgName string) (bool, error) {
		if pkgName == "nginx" {
			return true, nil
		}
		if pkgName == "uncommon-pkg" {
			return false, nil // This one is not installed
		}
		return false, fmt.Errorf("unexpected package check: %s", pkgName)
	}

	done, err := ips.Precheck(mockCtx, mockCtx.mockHost)
	require.NoError(t, err)
	assert.False(t, done)
}

func TestInstallPackagesStep_Precheck_ErrorChecking(t *testing.T) {
	mockCtx := newMockIPSContext(t)
	pkgs := []string{"nginx"}
	ips := NewInstallPackagesStep(pkgs, "").(*InstallPackagesStep)
	expectedErr := errors.New("failed to check package status")

	mockCtx.mockRunner.IsPackageInstalledFunc = func(ctx context.Context, conn connector.Connector, facts *runner.Facts, pkgName string) (bool, error) {
		return false, expectedErr
	}

	done, err := ips.Precheck(mockCtx, mockCtx.mockHost)
	require.Error(t, err)
	assert.False(t, done)
	assert.Contains(t, err.Error(), expectedErr.Error())
}

func TestInstallPackagesStep_Run_Success(t *testing.T) {
	mockCtx := newMockIPSContext(t)
	pkgsToInstall := []string{"nginx", "vim"}
	ips := NewInstallPackagesStep(pkgsToInstall, "").(*InstallPackagesStep)

	var installedPkgs []string
	mockCtx.mockRunner.InstallPackagesFunc = func(ctx context.Context, conn connector.Connector, facts *runner.Facts, packages ...string) error {
		installedPkgs = packages
		return nil
	}

	err := ips.Run(mockCtx, mockCtx.mockHost)
	require.NoError(t, err)
	assert.Equal(t, pkgsToInstall, installedPkgs)
}

func TestInstallPackagesStep_Run_Error(t *testing.T) {
	mockCtx := newMockIPSContext(t)
	pkgsToInstall := []string{"nginx"}
	ips := NewInstallPackagesStep(pkgsToInstall, "").(*InstallPackagesStep)
	expectedErr := errors.New("package manager failed")

	mockCtx.mockRunner.InstallPackagesFunc = func(ctx context.Context, conn connector.Connector, facts *runner.Facts, packages ...string) error {
		return expectedErr
	}

	err := ips.Run(mockCtx, mockCtx.mockHost)
	require.Error(t, err)
	assert.Contains(t, err.Error(), expectedErr.Error())
}

func TestInstallPackagesStep_Rollback_NoOp(t *testing.T) {
	mockCtx := newMockIPSContext(t)
	ips := NewInstallPackagesStep([]string{"nginx"}, "").(*InstallPackagesStep)

	// Verify no panic and no error for the default no-op rollback
	err := ips.Rollback(mockCtx, mockCtx.mockHost)
	assert.NoError(t, err)
}

// Example of how you might test a real rollback if it were implemented:
/*
func TestInstallPackagesStep_Rollback_RemovesPackages(t *testing.T) {
	mockCtx := newMockIPSContext(t)
	pkgsToRollback := []string{"nginx", "vim"}
	ips := NewInstallPackagesStep(pkgsToRollback, "").(*InstallPackagesStep)

	var removedPkgs []string
	mockCtx.mockRunner.RemovePackagesFunc = func(ctx context.Context, conn connector.Connector, facts *runner.Facts, packages ...string) error {
		removedPkgs = packages
		return nil
	}

	// To test actual rollback, you'd need to modify InstallPackagesStep.Rollback
	// For now, this test would fail or test the no-op.
	// If Rollback were implemented:
	// err := ips.Rollback(mockCtx, mockCtx.mockHost)
	// require.NoError(t, err)
	// assert.Equal(t, pkgsToRollback, removedPkgs)
}
*/
