package common

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	// "strings" // Removed unused import
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/cache"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/logger"
	"github.com/mensylisir/kubexm/pkg/runner"
	"github.com/mensylisir/kubexm/pkg/step"
)

// mockUISContext provides a minimal context for testing UserInputStep.
type mockUISContext struct {
	logger        *logger.Logger
	goCtx         context.Context
	clusterConfig *v1alpha1.Cluster
	globalWorkDir string
	controlNode   connector.Host
}

func newMockUISContext(t *testing.T) *mockUISContext {
	l, _ := logger.NewLogger(logger.DefaultOptions())
	tempGlobalWorkDir, err := os.MkdirTemp("", "test-gwd-uis-") // Changed ioutil.TempDir to os.MkdirTemp
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(tempGlobalWorkDir) })

	controlHostSpec := v1alpha1.HostSpec{Name: "control-node", Type: "local", Address: "127.0.0.1", Roles: []string{"control-node"}, Arch: "amd64"}
	controlNode := connector.NewHostFromSpec(controlHostSpec)

	clusterName := "testcluster"
	baseWorkDirForConfig := filepath.Dir(filepath.Dir(tempGlobalWorkDir))


	return &mockUISContext{
		logger: l,
		goCtx:  context.Background(),
		globalWorkDir: tempGlobalWorkDir,
		controlNode: controlNode,
		clusterConfig: &v1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: clusterName}, // Corrected
			Spec: v1alpha1.ClusterSpec{
				Global: &v1alpha1.GlobalSpec{WorkDir: baseWorkDirForConfig},
			},
		},
	}
}

// Implement step.StepContext
func (m *mockUISContext) GoContext() context.Context    { return m.goCtx }
func (m *mockUISContext) GetLogger() *logger.Logger     { return m.logger }
func (m *mockUISContext) GetHost() connector.Host       { return nil }
func (m *mockUISContext) GetRunner() runner.Runner      { return nil }
func (m *mockUISContext) GetControlNode() (connector.Host, error)    { return m.controlNode, nil }
func (m *mockUISContext) GetConnectorForHost(h connector.Host) (connector.Connector, error) { return nil, nil }
func (m *mockUISContext) GetCurrentHostConnector() (connector.Connector, error)        { return nil, nil }
func (m *mockUISContext) GetHostFacts(h connector.Host) (*runner.Facts, error)           { return &runner.Facts{}, nil }
func (m *mockUISContext) GetCurrentHostFacts() (*runner.Facts, error)                  { return &runner.Facts{}, nil }
func (m *mockUISContext) GetStepCache() cache.StepCache          { return cache.NewStepCache() }
func (m *mockUISContext) GetTaskCache() cache.TaskCache          { return cache.NewTaskCache() }
func (m *mockUISContext) GetModuleCache() cache.ModuleCache      { return cache.NewModuleCache() }
func (m *mockUISContext) GetPipelineCache() cache.PipelineCache  { return cache.NewPipelineCache() }
func (m *mockUISContext) GetClusterConfig() *v1alpha1.Cluster { return m.clusterConfig }
func (m *mockUISContext) GetHostsByRole(role string) ([]connector.Host, error) { return nil, nil }
func (m *mockUISContext) GetGlobalWorkDir() string         { return m.globalWorkDir }
func (m *mockUISContext) IsVerbose() bool                  { return false }
func (m *mockUISContext) ShouldIgnoreErr() bool            { return false }
func (m *mockUISContext) GetGlobalConnectionTimeout() time.Duration { return 30 * time.Second }
func (m *mockUISContext) GetClusterArtifactsDir() string       { return m.globalWorkDir }
func (m *mockUISContext) GetCertsDir() string                  { return filepath.Join(m.GetClusterArtifactsDir(), "certs") }
func (m *mockUISContext) GetEtcdCertsDir() string              { return filepath.Join(m.GetCertsDir(), "etcd") }
func (m *mockUISContext) GetComponentArtifactsDir(componentName string) string {
	return filepath.Join(m.GetClusterArtifactsDir(), componentName)
}
func (m *mockUISContext) GetEtcdArtifactsDir() string          { return m.GetComponentArtifactsDir("etcd") }
func (m *mockUISContext) GetContainerRuntimeArtifactsDir() string { return m.GetComponentArtifactsDir("container_runtime") }
func (m *mockUISContext) GetKubernetesArtifactsDir() string    { return m.GetComponentArtifactsDir("kubernetes") }
func (m *mockUISContext) GetFileDownloadPath(cn, v, a, fn string) string { return "" }
func (m *mockUISContext) GetHostDir(hostname string) string    { return filepath.Join(m.GetClusterArtifactsDir(), hostname) }
func (m *mockUISContext) WithGoContext(goCtx context.Context) step.StepContext {
	m.goCtx = goCtx
	return m
}
var _ step.StepContext = (*mockUISContext)(nil)


func TestUserInputStep_NewUserInputStep(t *testing.T) {
	prompt := "Proceed with installation?"
	s := NewUserInputStep("TestConfirm", prompt, false)
	require.NotNil(t, s)
	meta := s.Meta()
	assert.Equal(t, "TestConfirm", meta.Name)
	assert.Contains(t, meta.Description, prompt)

	uis, ok := s.(*UserInputStep)
	require.True(t, ok)
	assert.Equal(t, prompt, uis.Prompt)
	assert.False(t, uis.AssumeYes)

	sDefaultName := NewUserInputStep("", "Default prompt", true)
	assert.Equal(t, "GetUserConfirmation", sDefaultName.Meta().Name)
	assert.True(t, sDefaultName.(*UserInputStep).AssumeYes)
}

func TestUserInputStep_Precheck(t *testing.T) {
	mockCtx := newMockUISContext(t)

	sAssumeYes := NewUserInputStep("", "Prompt", true).(*UserInputStep)
	done, err := sAssumeYes.Precheck(mockCtx, nil)
	require.NoError(t, err)
	assert.True(t, done, "Precheck should return true (done) if AssumeYes is true")

	sNoAssumeYes := NewUserInputStep("", "Prompt", false).(*UserInputStep)
	done, err = sNoAssumeYes.Precheck(mockCtx, nil)
	require.NoError(t, err)
	assert.False(t, done, "Precheck should return false (not done) if AssumeYes is false")
}

func TestUserInputStep_Run_AssumeYes(t *testing.T) {
	mockCtx := newMockUISContext(t)
	uis := NewUserInputStep("", "Prompt", true).(*UserInputStep)

	oldStdout := os.Stdout
	r, w, pipeErr := os.Pipe()
	require.NoError(t, pipeErr)
	os.Stdout = w

	err := uis.Run(mockCtx, nil)
	require.NoError(t, err)

	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	_, copyErr := io.Copy(&buf, r)
	require.NoError(t, copyErr)
	r.Close()

	assert.Empty(t, buf.String(), "Run should not print anything if AssumeYes is true")
}

func simulateUserInput(t *testing.T, input string) (originalStdin *os.File, cleanup func()) {
	t.Helper()
	originalStdin = os.Stdin
	r, w, err := os.Pipe()
	require.NoError(t, err)

	os.Stdin = r
	_, err = w.WriteString(input)
	require.NoError(t, err)
	w.Close() // Close writer so reader gets EOF

	return originalStdin, func() {
		os.Stdin = originalStdin
		r.Close()
	}
}

func TestUserInputStep_Run_UserInput_Yes(t *testing.T) {
	mockCtx := newMockUISContext(t)
	prompt := "Confirm action?"
	uis := NewUserInputStep("", prompt, false).(*UserInputStep)

	_, cleanup := simulateUserInput(t, "yes\n")
	defer cleanup()

	oldStdout := os.Stdout
	rStdout, wStdout, pipeErr := os.Pipe()
	require.NoError(t, pipeErr)
	os.Stdout = wStdout

	err := uis.Run(mockCtx, nil)
	require.NoError(t, err)

	wStdout.Close()
	os.Stdout = oldStdout
	var outBuf bytes.Buffer
	_, copyErr := io.Copy(&outBuf, rStdout)
	require.NoError(t, copyErr)
	rStdout.Close()

	assert.Equal(t, prompt+" [yes/no]: ", outBuf.String())
}

func TestUserInputStep_Run_UserInput_No(t *testing.T) {
	mockCtx := newMockUISContext(t)
	prompt := "Confirm action?"
	uis := NewUserInputStep("", prompt, false).(*UserInputStep)

	_, cleanup := simulateUserInput(t, "no\n")
	defer cleanup()

	err := uis.Run(mockCtx, nil)
	require.Error(t, err)
	assert.EqualError(t, err, "user declined confirmation")
}

func TestUserInputStep_Run_UserInput_InvalidThenYes(t *testing.T) {
	mockCtx := newMockUISContext(t)
	prompt := "Confirm action?"
	uis := NewUserInputStep("", prompt, false).(*UserInputStep)

	// Current UserInputStep reads only one line.
	// To test retry logic, UserInputStep.Run would need a loop.
	// This test will show current behavior with "maybe" as input.
	_, cleanup := simulateUserInput(t, "maybe\nyes\n")
	defer cleanup()

	oldStdout := os.Stdout
	rStdout, wStdout, pipeErr := os.Pipe()
	require.NoError(t, pipeErr)
	os.Stdout = wStdout

	err := uis.Run(mockCtx, nil)

	wStdout.Close()
	os.Stdout = oldStdout
	var outBuf bytes.Buffer
	_, copyErr := io.Copy(&outBuf, rStdout)
	require.NoError(t, copyErr)
	rStdout.Close()

	assert.Equal(t, prompt+" [yes/no]: ", outBuf.String())
	require.Error(t, err)
	assert.EqualError(t, err, "user declined confirmation") // "maybe" is not "y" or "yes"
}


func TestUserInputStep_Rollback(t *testing.T) {
	mockCtx := newMockUISContext(t)
	uis := NewUserInputStep("", "Prompt", false).(*UserInputStep)
	err := uis.Rollback(mockCtx, nil)
	assert.NoError(t, err, "Rollback for UserInputStep should be a no-op.")
}
