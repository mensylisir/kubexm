package common

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath" // Added filepath
	"strings"       // Added strings
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock" // Changed gomock import

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/cache"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/logger"
	"github.com/mensylisir/kubexm/pkg/runner"
	// "github.com/mensylisir/kubexm/pkg/spec" // Removed unused import
	"github.com/mensylisir/kubexm/pkg/step"
)

// mockStepContextForMkdir provides a mockable implementation of step.StepContext for MkdirStep tests.
type mockStepContextForMkdir struct {
	ctrl          *gomock.Controller
	mockLogger    *logger.Logger
	currentHost   connector.Host
	controlNode   connector.Host
	mockRunner    *mock_runner.MockRunner
	mockConnector *mock_connector.MockConnector
	goCtx         context.Context
}

func newMockStepContextForMkdir(t *testing.T, hostName string, isControlNode bool) *mockStepContextForMkdir {
	ctrl := gomock.NewController(t)
	log := logger.Get() // Use actual logger

	hostSpec := v1alpha1.HostSpec{Name: hostName, Address: "1.2.3.4"}
	if isControlNode {
		hostSpec.Name = common.ControlNodeHostName
		hostSpec.Address = "127.0.0.1"
		hostSpec.Type = "local"
	}
	currentHost := connector.NewHostFromSpec(hostSpec)

	controlNodeSpec := v1alpha1.HostSpec{Name: common.ControlNodeHostName, Address: "127.0.0.1", Type: "local"}
	controlNode := connector.NewHostFromSpec(controlNodeSpec)

	return &mockStepContextForMkdir{
		ctrl:          ctrl,
		mockLogger:    log,
		currentHost:   currentHost,
		controlNode:   controlNode,
		mockRunner:    mock_runner.NewMockRunner(ctrl),
		mockConnector: mock_connector.NewMockConnector(ctrl),
		goCtx:         context.Background(),
	}
}

// Implement step.StepContext interface
func (m *mockStepContextForMkdir) GoContext() context.Context              { return m.goCtx }
func (m *mockStepContextForMkdir) GetLogger() *logger.Logger               { return m.mockLogger }
func (m *mockStepContextForMkdir) GetHost() connector.Host                 { return m.currentHost }
func (m *mockStepContextForMkdir) GetRunner() runner.Runner                { return m.mockRunner }
func (m *mockStepContextForMkdir) GetControlNode() (connector.Host, error) { return m.controlNode, nil }
func (m *mockStepContextForMkdir) GetConnectorForHost(host connector.Host) (connector.Connector, error) {
	return m.mockConnector, nil
}
func (m *mockStepContextForMkdir) GetCurrentHostConnector() (connector.Connector, error) {
	return m.mockConnector, nil
}

// Add other StepContext methods with dummy implementations as needed by the step
func (m *mockStepContextForMkdir) GetClusterConfig() *v1alpha1.Cluster   { return &v1alpha1.Cluster{} }
func (m *mockStepContextForMkdir) GetStepCache() cache.StepCache         { return nil }
func (m *mockStepContextForMkdir) GetTaskCache() cache.TaskCache         { return nil }
func (m *mockStepContextForMkdir) GetModuleCache() cache.ModuleCache     { return nil }
func (m *mockStepContextForMkdir) GetPipelineCache() cache.PipelineCache { return nil }
func (m *mockStepContextForMkdir) GetHostsByRole(role string) ([]connector.Host, error) {
	return nil, nil
}
func (m *mockStepContextForMkdir) GetHostFacts(host connector.Host) (*runner.Facts, error) {
	return &runner.Facts{}, nil
}
func (m *mockStepContextForMkdir) GetCurrentHostFacts() (*runner.Facts, error) {
	return &runner.Facts{}, nil
}
func (m *mockStepContextForMkdir) GetGlobalWorkDir() string                  { return "/tmp/kubexm_workdir" }
func (m *mockStepContextForMkdir) IsVerbose() bool                           { return false }
func (m *mockStepContextForMkdir) ShouldIgnoreErr() bool                     { return false }
func (m *mockStepContextForMkdir) GetGlobalConnectionTimeout() time.Duration { return 30 * time.Second }
func (m *mockStepContextForMkdir) GetClusterArtifactsDir() string {
	return "/tmp/kubexm_workdir/.kubexm/testcluster"
}
func (m *mockStepContextForMkdir) GetCertsDir() string                                  { return "" }
func (m *mockStepContextForMkdir) GetEtcdCertsDir() string                              { return "" }
func (m *mockStepContextForMkdir) GetComponentArtifactsDir(componentName string) string { return "" }
func (m *mockStepContextForMkdir) GetEtcdArtifactsDir() string                          { return "" }
func (m *mockStepContextForMkdir) GetContainerRuntimeArtifactsDir() string              { return "" }
func (m *mockStepContextForMkdir) GetKubernetesArtifactsDir() string                    { return "" }
func (m *mockStepContextForMkdir) GetFileDownloadPath(componentName, version, arch, fileName string) string {
	return ""
}
func (m *mockStepContextForMkdir) GetHostDir(hostname string) string { return "" }
func (m *mockStepContextForMkdir) WithGoContext(goCtx context.Context) step.StepContext {
	m.goCtx = goCtx
	return m
}

var _ step.StepContext = (*mockStepContextForMkdir)(nil)

func TestMkdirStep_Meta(t *testing.T) {
	testPath := "/test/dir"
	s := NewMkdirStep("TestMkdir", testPath, 0755, false)
	meta := s.Meta()
	assert.Equal(t, "TestMkdir", meta.Name)
	assert.Contains(t, meta.Description, testPath)

	sDefaultName := NewMkdirStep("", testPath, 0755, false)
	metaDefault := sDefaultName.Meta()
	assert.Equal(t, fmt.Sprintf("Mkdir-%s", testPath), metaDefault.Name)
}

func TestMkdirStep_Precheck_Local_Exists(t *testing.T) {
	mockCtx := newMockStepContextForMkdir(t, common.ControlNodeHostName, true)
	testPath := filepath.Join(t.TempDir(), "mydir")
	err := os.Mkdir(testPath, 0755)
	require.NoError(t, err)

	s := NewMkdirStep("", testPath, 0755, false) // Sudo false for local
	done, err := s.Precheck(mockCtx, mockCtx.currentHost)
	assert.NoError(t, err)
	assert.True(t, done, "Precheck should return true if directory exists locally")
}

func TestMkdirStep_Precheck_Local_NotExists(t *testing.T) {
	mockCtx := newMockStepContextForMkdir(t, common.ControlNodeHostName, true)
	testPath := filepath.Join(t.TempDir(), "notexistdir")

	s := NewMkdirStep("", testPath, 0755, false)
	done, err := s.Precheck(mockCtx, mockCtx.currentHost)
	assert.NoError(t, err)
	assert.False(t, done, "Precheck should return false if directory does not exist locally")
}

func TestMkdirStep_Precheck_Local_FileExists(t *testing.T) {
	mockCtx := newMockStepContextForMkdir(t, common.ControlNodeHostName, true)
	testPath := filepath.Join(t.TempDir(), "myfile")
	_, err := os.Create(testPath) // Create a file, not a directory
	require.NoError(t, err)

	s := NewMkdirStep("", testPath, 0755, false)
	done, err := s.Precheck(mockCtx, mockCtx.currentHost)
	assert.NoError(t, err) // Precheck returns (false, nil) if path exists but is not a dir
	assert.False(t, done, "Precheck should return false if path exists but is a file")
}

func TestMkdirStep_Precheck_Remote_Exists(t *testing.T) {
	mockCtx := newMockStepContextForMkdir(t, "remote-host-1", false)
	testPath := "/remote/test/dir"

	mockCtx.mockRunner.EXPECT().IsDir(mockCtx.goCtx, mockCtx.mockConnector, testPath).Return(true, nil)

	s := NewMkdirStep("", testPath, 0755, true) // Sudo true for remote
	done, err := s.Precheck(mockCtx, mockCtx.currentHost)
	assert.NoError(t, err)
	assert.True(t, done, "Precheck should return true if runner.IsDir is true")
}

func TestMkdirStep_Precheck_Remote_NotExists(t *testing.T) {
	mockCtx := newMockStepContextForMkdir(t, "remote-host-1", false)
	testPath := "/remote/notexist"

	mockCtx.mockRunner.EXPECT().IsDir(mockCtx.goCtx, mockCtx.mockConnector, testPath).Return(false, nil)

	s := NewMkdirStep("", testPath, 0755, true)
	done, err := s.Precheck(mockCtx, mockCtx.currentHost)
	assert.NoError(t, err)
	assert.False(t, done, "Precheck should return false if runner.IsDir is false")
}

func TestMkdirStep_Precheck_Remote_RunnerError(t *testing.T) {
	mockCtx := newMockStepContextForMkdir(t, "remote-host-1", false)
	testPath := "/remote/errorpath"
	expectedErr := errors.New("runner error")

	mockCtx.mockRunner.EXPECT().IsDir(mockCtx.goCtx, mockCtx.mockConnector, testPath).Return(false, expectedErr)

	s := NewMkdirStep("", testPath, 0755, true)
	done, err := s.Precheck(mockCtx, mockCtx.currentHost)
	assert.NoError(t, err, "Precheck itself shouldn't error on runner IsDir error, but return false for done")
	assert.False(t, done)
}

func TestMkdirStep_Run_Local(t *testing.T) {
	mockCtx := newMockStepContextForMkdir(t, common.ControlNodeHostName, true)
	tempDir := t.TempDir()
	testPath := filepath.Join(tempDir, "newlocaldimkdi")

	s := NewMkdirStep("", testPath, 0750, false) // Sudo false for local
	err := s.Run(mockCtx, mockCtx.currentHost)
	assert.NoError(t, err)

	fi, statErr := os.Stat(testPath)
	assert.NoError(t, statErr, "Directory should have been created")
	if statErr == nil {
		assert.True(t, fi.IsDir(), "Path should be a directory")
		assert.Equal(t, os.FileMode(0750), fi.Mode().Perm(), "Permissions should match")
	}
}

func TestMkdirStep_Run_Remote(t *testing.T) {
	mockCtx := newMockStepContextForMkdir(t, "remote-host-run", false)
	testPath := "/var/mymkdir"
	permissions := os.FileMode(0700)
	permStr := fmt.Sprintf("%o", permissions)
	useSudo := true

	mockCtx.mockRunner.EXPECT().Mkdirp(mockCtx.goCtx, mockCtx.mockConnector, testPath, permStr, useSudo).Return(nil)

	s := NewMkdirStep("", testPath, permissions, useSudo)
	err := s.Run(mockCtx, mockCtx.currentHost)
	assert.NoError(t, err)
}

func TestMkdirStep_Run_Remote_RunnerFails(t *testing.T) {
	mockCtx := newMockStepContextForMkdir(t, "remote-host-fail", false)
	testPath := "/var/mymkdirfail"
	expectedErr := errors.New("runner Mkdirp failed")

	mockCtx.mockRunner.EXPECT().Mkdirp(gomock.Any(), gomock.Any(), testPath, gomock.Any(), gomock.Any()).Return(expectedErr)

	s := NewMkdirStep("", testPath, 0755, true)
	err := s.Run(mockCtx, mockCtx.currentHost)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, expectedErr) || strings.Contains(err.Error(), expectedErr.Error()))
}

func TestMkdirStep_Rollback_Local_Success(t *testing.T) {
	mockCtx := newMockStepContextForMkdir(t, common.ControlNodeHostName, true)
	tempDir := t.TempDir()
	testPath := filepath.Join(tempDir, "toberemoved")

	err := os.Mkdir(testPath, 0755) // Create it first
	require.NoError(t, err)

	s := NewMkdirStep("", testPath, 0755, false)
	err = s.Rollback(mockCtx, mockCtx.currentHost)
	assert.NoError(t, err)

	_, statErr := os.Stat(testPath)
	assert.True(t, os.IsNotExist(statErr), "Directory should have been removed by rollback")
}

func TestMkdirStep_Rollback_Remote_Success(t *testing.T) {
	mockCtx := newMockStepContextForMkdir(t, "remote-host-rollback", false)
	testPath := "/var/toberemovedremote"
	useSudo := true

	mockCtx.mockRunner.EXPECT().Remove(mockCtx.goCtx, mockCtx.mockConnector, testPath, useSudo).Return(nil)

	s := NewMkdirStep("", testPath, 0755, useSudo)
	err := s.Rollback(mockCtx, mockCtx.currentHost)
	assert.NoError(t, err)
}
