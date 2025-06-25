package common

import (
	"context"
	"errors"
	// "fmt" // Removed unused import
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.uber.org/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/cache"
	"github.com/mensylisir/kubexm/pkg/connector"
	mock_connector "github.com/mensylisir/kubexm/pkg/connector/mocks"
	"github.com/mensylisir/kubexm/pkg/logger"
	"github.com/mensylisir/kubexm/pkg/runner"
	mock_runner "github.com/mensylisir/kubexm/pkg/runner/mocks"
	"github.com/mensylisir/kubexm/pkg/step"
)

type mockIPSContext struct {
	logger     *logger.Logger
	goCtx      context.Context
	mockRunner *mock_runner.MockRunner
	mockHost   connector.Host
	mockConn   *mock_connector.MockConnector
	hostFacts  *runner.Facts
	ctrl       *gomock.Controller
}

func newMockIPSContext(t *testing.T) *mockIPSContext {
	ctrl := gomock.NewController(t)
	l, _ := logger.NewLogger(logger.DefaultOptions())

	mockHostSpec := &v1alpha1.HostSpec{Name: "test-host-ips", Address: "dummy-addr", Arch: "amd64"}
	mockHost := connector.NewHostFromSpec(*mockHostSpec)
	mockConn := mock_connector.NewMockConnector(ctrl)
	mockRun := mock_runner.NewMockRunner(ctrl)

	return &mockIPSContext{
		ctrl:       ctrl,
		logger:     l,
		goCtx:      context.Background(),
		mockRunner: mockRun,
		mockHost:   mockHost,
		mockConn:   mockConn,
		hostFacts:  &runner.Facts{OS: &connector.OS{ID: "linux", Arch: "amd64"}},
	}
}

// Implement step.StepContext
func (m *mockIPSContext) GoContext() context.Context                 { return m.goCtx }
func (m *mockIPSContext) GetLogger() *logger.Logger                  { return m.logger }
func (m *mockIPSContext) GetHost() connector.Host                    { return m.mockHost }
func (m *mockIPSContext) GetRunner() runner.Runner                   { return m.mockRunner }
func (m *mockIPSContext) GetControlNode() (connector.Host, error) {
	dummyControlSpec := &v1alpha1.HostSpec{Name: "control-node", Type: "local", Address: "127.0.0.1", Arch: "amd64"}
	return connector.NewHostFromSpec(*dummyControlSpec), nil
}
func (m *mockIPSContext) GetConnectorForHost(h connector.Host) (connector.Connector, error) { return m.mockConn, nil }
func (m *mockIPSContext) GetCurrentHostConnector() (connector.Connector, error)        { return m.mockConn, nil }
func (m *mockIPSContext) GetHostFacts(h connector.Host) (*runner.Facts, error)           { return m.hostFacts, nil }
func (m *mockIPSContext) GetCurrentHostFacts() (*runner.Facts, error)                  { return m.hostFacts, nil }

func (m *mockIPSContext) GetStepCache() cache.StepCache          { return cache.NewStepCache() }
func (m *mockIPSContext) GetTaskCache() cache.TaskCache          { return cache.NewTaskCache() }
func (m *mockIPSContext) GetModuleCache() cache.ModuleCache      { return cache.NewModuleCache() }
func (m *mockIPSContext) GetPipelineCache() cache.PipelineCache  { return cache.NewPipelineCache() }

func (m *mockIPSContext) GetClusterConfig() *v1alpha1.Cluster { return &v1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "testcluster"}} }
func (m *mockIPSContext) GetHostsByRole(role string) ([]connector.Host, error) { return nil, nil }

func (m *mockIPSContext) GetGlobalWorkDir() string         { return "/tmp/kubexm_workdir" }
func (m *mockIPSContext) IsVerbose() bool                  { return false }
func (m *mockIPSContext) ShouldIgnoreErr() bool            { return false }
func (m *mockIPSContext) GetGlobalConnectionTimeout() time.Duration { return 30 * time.Second }

func (m *mockIPSContext) GetClusterArtifactsDir() string       { return filepath.Join(m.GetGlobalWorkDir(), ".kubexm", m.GetClusterConfig().Name) }
func (m *mockIPSContext) GetCertsDir() string                  { return filepath.Join(m.GetClusterArtifactsDir(), "certs") }
func (m *mockIPSContext) GetEtcdCertsDir() string              { return filepath.Join(m.GetCertsDir(), "etcd") }
func (m *mockIPSContext) GetComponentArtifactsDir(componentName string) string {
	return filepath.Join(m.GetClusterArtifactsDir(), componentName)
}
func (m *mockIPSContext) GetEtcdArtifactsDir() string          { return m.GetComponentArtifactsDir("etcd") }
func (m *mockIPSContext) GetContainerRuntimeArtifactsDir() string { return m.GetComponentArtifactsDir("container_runtime") }
func (m *mockIPSContext) GetKubernetesArtifactsDir() string    { return m.GetComponentArtifactsDir("kubernetes") }
func (m *mockIPSContext) GetFileDownloadPath(cn, v, a, fn string) string { return "" }
func (m *mockIPSContext) GetHostDir(hostname string) string    { return filepath.Join(m.GetClusterArtifactsDir(), hostname) }

func (m *mockIPSContext) WithGoContext(goCtx context.Context) step.StepContext {
	m.goCtx = goCtx
	return m
}

var _ step.StepContext = (*mockIPSContext)(nil)


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
	defer mockCtx.ctrl.Finish()
	pkgs := []string{"nginx", "vim"}
	ips := NewInstallPackagesStep(pkgs, "").(*InstallPackagesStep)

	mockCtx.mockRunner.EXPECT().IsPackageInstalled(gomock.Any(), mockCtx.mockConn, mockCtx.hostFacts, "nginx").Return(true, nil)
	mockCtx.mockRunner.EXPECT().IsPackageInstalled(gomock.Any(), mockCtx.mockConn, mockCtx.hostFacts, "vim").Return(true, nil)

	done, err := ips.Precheck(mockCtx, mockCtx.mockHost)
	require.NoError(t, err)
	assert.True(t, done)
}

func TestInstallPackagesStep_Precheck_SomeNotInstalled(t *testing.T) {
	mockCtx := newMockIPSContext(t)
	defer mockCtx.ctrl.Finish()
	pkgs := []string{"nginx", "uncommon-pkg"}
	ips := NewInstallPackagesStep(pkgs, "").(*InstallPackagesStep)

	mockCtx.mockRunner.EXPECT().IsPackageInstalled(gomock.Any(), mockCtx.mockConn, mockCtx.hostFacts, "nginx").Return(true, nil)
	mockCtx.mockRunner.EXPECT().IsPackageInstalled(gomock.Any(), mockCtx.mockConn, mockCtx.hostFacts, "uncommon-pkg").Return(false, nil)

	done, err := ips.Precheck(mockCtx, mockCtx.mockHost)
	require.NoError(t, err)
	assert.False(t, done)
}

func TestInstallPackagesStep_Precheck_ErrorChecking(t *testing.T) {
	mockCtx := newMockIPSContext(t)
	defer mockCtx.ctrl.Finish()
	pkgs := []string{"nginx"}
	ips := NewInstallPackagesStep(pkgs, "").(*InstallPackagesStep)
	expectedErr := errors.New("failed to check package status")

	mockCtx.mockRunner.EXPECT().IsPackageInstalled(gomock.Any(), mockCtx.mockConn, mockCtx.hostFacts, "nginx").Return(false, expectedErr)

	done, err := ips.Precheck(mockCtx, mockCtx.mockHost)
	require.Error(t, err)
	assert.False(t, done)
	assert.True(t, errors.Is(err, expectedErr) || strings.Contains(err.Error(), expectedErr.Error()))
}

func TestInstallPackagesStep_Run_Success(t *testing.T) {
	mockCtx := newMockIPSContext(t)
	defer mockCtx.ctrl.Finish()
	pkgsToInstall := []string{"nginx", "vim"}
	ips := NewInstallPackagesStep(pkgsToInstall, "").(*InstallPackagesStep)

	mockCtx.mockRunner.EXPECT().InstallPackages(gomock.Any(), mockCtx.mockConn, mockCtx.hostFacts, pkgsToInstall).Return(nil)

	err := ips.Run(mockCtx, mockCtx.mockHost)
	require.NoError(t, err)
}

func TestInstallPackagesStep_Run_Error(t *testing.T) {
	mockCtx := newMockIPSContext(t)
	defer mockCtx.ctrl.Finish()
	pkgsToInstall := []string{"nginx"}
	ips := NewInstallPackagesStep(pkgsToInstall, "").(*InstallPackagesStep)
	expectedErr := errors.New("package manager failed")

	mockCtx.mockRunner.EXPECT().InstallPackages(gomock.Any(), mockCtx.mockConn, mockCtx.hostFacts, pkgsToInstall).Return(expectedErr)

	err := ips.Run(mockCtx, mockCtx.mockHost)
	require.Error(t, err)
	assert.True(t, errors.Is(err, expectedErr) || strings.Contains(err.Error(), expectedErr.Error()))
}

func TestInstallPackagesStep_Rollback_NoOp(t *testing.T) {
	mockCtx := newMockIPSContext(t)
	defer mockCtx.ctrl.Finish()
	ips := NewInstallPackagesStep([]string{"nginx"}, "").(*InstallPackagesStep)

	err := ips.Rollback(mockCtx, mockCtx.mockHost)
	assert.NoError(t, err)
}
