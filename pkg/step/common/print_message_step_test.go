package common

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1" // Added import

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/cache"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/logger"
	"github.com/mensylisir/kubexm/pkg/runner"
	"github.com/mensylisir/kubexm/pkg/step"
)

// mockPMSContext provides a minimal context for testing PrintMessageStep.
type mockPMSContext struct {
	logger *logger.Logger
	goCtx  context.Context
	// No connector or runner needed as PrintMessageStep prints to local stdout
	// No host needed as it's a local operation.
}

func newMockPMSContext(t *testing.T) *mockPMSContext {
	l, _ := logger.NewLogger(logger.DefaultOptions())
	return &mockPMSContext{
		logger: l,
		goCtx:  context.Background(),
	}
}

// Implement step.StepContext
func (m *mockPMSContext) GoContext() context.Context    { return m.goCtx }
func (m *mockPMSContext) GetLogger() *logger.Logger     { return m.logger }
func (m *mockPMSContext) GetHost() connector.Host       { return nil } // PrintMessageStep runs locally
func (m *mockPMSContext) GetRunner() runner.Runner      { return nil }
func (m *mockPMSContext) GetControlNode() (connector.Host, error) {
	// Return a dummy control node
	dummyControlSpec := &v1alpha1.HostSpec{Name: "control-node", Type: "local", Address: "127.0.0.1", Arch: "amd64"}
	return connector.NewHostFromSpec(*dummyControlSpec), nil
}
func (m *mockPMSContext) GetConnectorForHost(h connector.Host) (connector.Connector, error) { return nil, nil }
func (m *mockPMSContext) GetCurrentHostConnector() (connector.Connector, error)        { return nil, nil }
func (m *mockPMSContext) GetHostFacts(h connector.Host) (*runner.Facts, error)           { return &runner.Facts{}, nil }
func (m *mockPMSContext) GetCurrentHostFacts() (*runner.Facts, error)                  { return &runner.Facts{}, nil }

func (m *mockPMSContext) GetStepCache() cache.StepCache          { return cache.NewStepCache() }
func (m *mockPMSContext) GetTaskCache() cache.TaskCache          { return cache.NewTaskCache() }
func (m *mockPMSContext) GetModuleCache() cache.ModuleCache      { return cache.NewModuleCache() }
func (m *mockPMSContext) GetPipelineCache() cache.PipelineCache  { return cache.NewPipelineCache() }

func (m *mockPMSContext) GetClusterConfig() *v1alpha1.Cluster { return &v1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "testcluster"}} }
func (m *mockPMSContext) GetHostsByRole(role string) ([]connector.Host, error) { return nil, nil }

func (m *mockPMSContext) GetGlobalWorkDir() string         { return "/tmp/kubexm_workdir_pms" }
func (m *mockPMSContext) IsVerbose() bool                  { return false }
func (m *mockPMSContext) ShouldIgnoreErr() bool            { return false }
func (m *mockPMSContext) GetGlobalConnectionTimeout() time.Duration { return 30 * time.Second }

func (m *mockPMSContext) GetClusterArtifactsDir() string       { return filepath.Join(m.GetGlobalWorkDir(), ".kubexm", m.GetClusterConfig().Name) }
func (m *mockPMSContext) GetCertsDir() string                  { return filepath.Join(m.GetClusterArtifactsDir(), "certs") }
func (m *mockPMSContext) GetEtcdCertsDir() string              { return filepath.Join(m.GetCertsDir(), "etcd") }
func (m *mockPMSContext) GetComponentArtifactsDir(componentName string) string {
	return filepath.Join(m.GetClusterArtifactsDir(), componentName)
}
func (m *mockPMSContext) GetEtcdArtifactsDir() string          { return m.GetComponentArtifactsDir("etcd") }
func (m *mockPMSContext) GetContainerRuntimeArtifactsDir() string { return m.GetComponentArtifactsDir("container_runtime") }
func (m *mockPMSContext) GetKubernetesArtifactsDir() string    { return m.GetComponentArtifactsDir("kubernetes") }
func (m *mockPMSContext) GetFileDownloadPath(cn, v, a, fn string) string { return "" }
func (m *mockPMSContext) GetHostDir(hostname string) string    { return filepath.Join(m.GetClusterArtifactsDir(), hostname) }

func (m *mockPMSContext) WithGoContext(goCtx context.Context) step.StepContext {
	m.goCtx = goCtx
	return m
}
var _ step.StepContext = (*mockPMSContext)(nil) // Verify interface satisfaction


func TestPrintMessageStep_NewPrintMessageStep(t *testing.T) {
	msg := "Hello KubeXM User!"
	s := NewPrintMessageStep("WelcomeMsg", msg)
	require.NotNil(t, s)
	meta := s.Meta()
	assert.Equal(t, "WelcomeMsg", meta.Name)
	assert.Equal(t, "Prints a message to the console.", meta.Description)

	pms, ok := s.(*PrintMessageStep)
	require.True(t, ok, "NewPrintMessageStep should return a *PrintMessageStep")
	assert.Equal(t, msg, pms.Message)

	sDefaultName := NewPrintMessageStep("", "Test message")
	assert.Equal(t, "PrintMessage", sDefaultName.Meta().Name)
}

func TestPrintMessageStep_Run(t *testing.T) {
	mockCtx := newMockPMSContext(t)
	testMessage := "This is a test message for stdout."
	pms := NewPrintMessageStep("", testMessage).(*PrintMessageStep)

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// For PrintMessageStep, the host argument to Run is often nil as it prints locally.
	err := pms.Run(mockCtx, nil)
	require.NoError(t, err)

	w.Close()
	os.Stdout = oldStdout // Restore stdout

	var buf bytes.Buffer
	_, copyErr := io.Copy(&buf, r)
	require.NoError(t, copyErr)
	r.Close()

	// fmt.Println in the step adds a newline
	assert.Equal(t, testMessage+"\n", buf.String())
}

func TestPrintMessageStep_Precheck(t *testing.T) {
	mockCtx := newMockPMSContext(t)
	pms := NewPrintMessageStep("", "any message").(*PrintMessageStep)

	done, err := pms.Precheck(mockCtx, nil)
	require.NoError(t, err)
	assert.False(t, done, "Precheck for PrintMessageStep should always return false.")
}

func TestPrintMessageStep_Rollback(t *testing.T) {
	mockCtx := newMockPMSContext(t)
	pms := NewPrintMessageStep("", "any message").(*PrintMessageStep)

	err := pms.Rollback(mockCtx, nil)
	assert.NoError(t, err, "Rollback for PrintMessageStep should be a no-op and not return an error.")
}
