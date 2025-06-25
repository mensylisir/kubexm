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

// mockRTSContext provides a minimal context for testing ReportTableStep.
type mockRTSContext struct {
	logger *logger.Logger
	goCtx  context.Context
	// No connector or runner needed as ReportTableStep prints to local stdout
}

func newMockRTSContext(t *testing.T) *mockRTSContext {
	l, _ := logger.NewLogger(logger.DefaultOptions())
	return &mockRTSContext{
		logger: l,
		goCtx:  context.Background(),
	}
}

// Implement step.StepContext
func (m *mockRTSContext) GoContext() context.Context    { return m.goCtx }
func (m *mockRTSContext) GetLogger() *logger.Logger     { return m.logger }
func (m *mockRTSContext) GetHost() connector.Host       { return nil } // ReportTableStep runs locally
func (m *mockRTSContext) GetRunner() runner.Runner      { return nil }
func (m *mockRTSContext) GetControlNode() (connector.Host, error) {
	dummyControlSpec := &v1alpha1.HostSpec{Name: "control-node", Type: "local", Address: "127.0.0.1", Arch: "amd64"}
	return connector.NewHostFromSpec(*dummyControlSpec), nil
}
func (m *mockRTSContext) GetConnectorForHost(h connector.Host) (connector.Connector, error) { return nil, nil }
func (m *mockRTSContext) GetCurrentHostConnector() (connector.Connector, error)        { return nil, nil }
func (m *mockRTSContext) GetHostFacts(h connector.Host) (*runner.Facts, error)           { return &runner.Facts{}, nil }
func (m *mockRTSContext) GetCurrentHostFacts() (*runner.Facts, error)                  { return &runner.Facts{}, nil }

func (m *mockRTSContext) GetStepCache() cache.StepCache          { return cache.NewStepCache() }
func (m *mockRTSContext) GetTaskCache() cache.TaskCache          { return cache.NewTaskCache() }
func (m *mockRTSContext) GetModuleCache() cache.ModuleCache      { return cache.NewModuleCache() }
func (m *mockRTSContext) GetPipelineCache() cache.PipelineCache  { return cache.NewPipelineCache() }

func (m *mockRTSContext) GetClusterConfig() *v1alpha1.Cluster { return &v1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "testcluster"}} }
func (m *mockRTSContext) GetHostsByRole(role string) ([]connector.Host, error) { return nil, nil }

func (m *mockRTSContext) GetGlobalWorkDir() string         { return "/tmp/kubexm_workdir_rts" }
func (m *mockRTSContext) IsVerbose() bool                  { return false }
func (m *mockRTSContext) ShouldIgnoreErr() bool            { return false }
func (m *mockRTSContext) GetGlobalConnectionTimeout() time.Duration { return 30 * time.Second }

func (m *mockRTSContext) GetClusterArtifactsDir() string       { return filepath.Join(m.GetGlobalWorkDir(), ".kubexm", m.GetClusterConfig().Name) }
func (m *mockRTSContext) GetCertsDir() string                  { return filepath.Join(m.GetClusterArtifactsDir(), "certs") }
func (m *mockRTSContext) GetEtcdCertsDir() string              { return filepath.Join(m.GetCertsDir(), "etcd") }
func (m *mockRTSContext) GetComponentArtifactsDir(componentName string) string {
	return filepath.Join(m.GetClusterArtifactsDir(), componentName)
}
func (m *mockRTSContext) GetEtcdArtifactsDir() string          { return m.GetComponentArtifactsDir("etcd") }
func (m *mockRTSContext) GetContainerRuntimeArtifactsDir() string { return m.GetComponentArtifactsDir("container_runtime") }
func (m *mockRTSContext) GetKubernetesArtifactsDir() string    { return m.GetComponentArtifactsDir("kubernetes") }
func (m *mockRTSContext) GetFileDownloadPath(cn, v, a, fn string) string { return "" }
func (m *mockRTSContext) GetHostDir(hostname string) string    { return filepath.Join(m.GetClusterArtifactsDir(), hostname) }

func (m *mockRTSContext) WithGoContext(goCtx context.Context) step.StepContext {
	m.goCtx = goCtx
	return m
}
var _ step.StepContext = (*mockRTSContext)(nil) // Verify interface satisfaction


func TestReportTableStep_NewReportTableStep(t *testing.T) {
	headers := []string{"ID", "Name", "Status"}
	rows := [][]string{
		{"1", "NodeA", "Ready"},
		{"2", "NodeB", "NotReady"},
	}
	s := NewReportTableStep("NodeStatusReport", headers, rows)
	require.NotNil(t, s)
	meta := s.Meta()
	assert.Equal(t, "NodeStatusReport", meta.Name)
	assert.Equal(t, "Displays a report in a table format.", meta.Description)

	rts, ok := s.(*ReportTableStep)
	require.True(t, ok)
	assert.Equal(t, headers, rts.Headers)
	assert.Equal(t, rows, rts.Rows)

	sDefaultName := NewReportTableStep("", headers, rows)
	assert.Equal(t, "DisplayTableReport", sDefaultName.Meta().Name)
}

func TestReportTableStep_Precheck(t *testing.T) {
	mockCtx := newMockRTSContext(t)
	headers := []string{"H1"}

	sWithRows := NewReportTableStep("", headers, [][]string{{"r1"}}).(*ReportTableStep)
	done, err := sWithRows.Precheck(mockCtx, nil)
	require.NoError(t, err)
	assert.False(t, done, "Precheck should return false if there are rows")

	sNoRows := NewReportTableStep("", headers, [][]string{}).(*ReportTableStep)
	done, err = sNoRows.Precheck(mockCtx, nil)
	require.NoError(t, err)
	assert.True(t, done, "Precheck should return true (done) if there are no rows")
}

func TestReportTableStep_Run_Success(t *testing.T) {
	mockCtx := newMockRTSContext(t)
	headers := []string{"Name", "Value"}
	rows := [][]string{
		{"CPU", "4 Cores"},
		{"Memory", "16GB"},
	}
	rts := NewReportTableStep("", headers, rows).(*ReportTableStep)

	// Capture stdout
	oldStdout := os.Stdout
	r, w, pipeErr := os.Pipe()
	require.NoError(t, pipeErr)
	os.Stdout = w

	err := rts.Run(mockCtx, nil) // Host is nil for control node operations
	require.NoError(t, err)

	w.Close()
	os.Stdout = oldStdout // Restore stdout

	var buf bytes.Buffer
	_, copyErr := io.Copy(&buf, r)
	require.NoError(t, copyErr)
	r.Close()

	output := buf.String()
	// Check for presence of headers and data
	assert.Contains(t, output, "Name")
	assert.Contains(t, output, "Value")
	assert.Contains(t, output, "CPU")
	assert.Contains(t, output, "4 Cores")
	assert.Contains(t, output, "Memory")
	assert.Contains(t, output, "16GB")
	// tablewriter adds lines and spaces, so direct string comparison is fragile.
	// Checking for key elements is more robust.
	// fmt.Println("Captured table output:\n", output) // For manual inspection if needed
}

func TestReportTableStep_Run_NoRows(t *testing.T) {
	mockCtx := newMockRTSContext(t)
	headers := []string{"Name", "Value"}
	rts := NewReportTableStep("", headers, [][]string{}).(*ReportTableStep)

	// Capture stdout
	oldStdout := os.Stdout
	r, w, pipeErr := os.Pipe()
	require.NoError(t, pipeErr)
	os.Stdout = w

	err := rts.Run(mockCtx, nil)
	require.NoError(t, err)

	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	r.Close()

	output := buf.String()
	assert.NotContains(t, output, "+---") // tablewriter border marker
	assert.NotContains(t, output, "Name") // Header
}


func TestReportTableStep_Rollback(t *testing.T) {
	mockCtx := newMockRTSContext(t)
	rts := NewReportTableStep("", []string{"H"}, [][]string{{"R"}}).(*ReportTableStep)
	err := rts.Rollback(mockCtx, nil)
	assert.NoError(t, err, "Rollback for ReportTableStep should be a no-op.")
}
