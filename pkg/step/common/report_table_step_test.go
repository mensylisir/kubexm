package common

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/logger"
	"github.com/mensylisir/kubexm/pkg/runtime"
)

// mockRTSContext provides a minimal context for testing ReportTableStep.
type mockRTSContext struct {
	runtime.StepContext
	logger *logger.Logger
	goCtx  context.Context
}

func newMockRTSContext(t *testing.T) *mockRTSContext {
	l, _ := logger.New(logger.DefaultConfig())
	return &mockRTSContext{
		logger: l,
		goCtx:  context.Background(),
	}
}
func (m *mockRTSContext) GetLogger() *logger.Logger { return m.logger }
func (m *mockRTSContext) GoContext() context.Context  { return m.goCtx }
func (m *mockRTSContext) GetHost() connector.Host   { return nil } // Assuming local/control node operation
// Implement other StepContext methods if any are called by the step, returning default/nil.
func (m *mockRTSContext) GetRunner() runner.Runner                                   { return nil }
func (m *mockRTSContext) GetConnectorForHost(h connector.Host) (connector.Connector, error) { return nil, nil }
func (m *mockRTSContext) GetHostFacts(h connector.Host) (*runner.Facts, error)           { return nil, nil }
func (m *mockRTSContext) GetCurrentHostFacts() (*runner.Facts, error)                  { return nil, nil }
func (m *mockRTSContext) GetCurrentHostConnector() (connector.Connector, error)        { return nil, nil }
func (m *mockRTSContext) StepCache() runtime.StepCache                               { return nil }
func (m *mockRTSContext) TaskCache() runtime.TaskCache                               { return nil }
func (m *mockRTSContext) ModuleCache() runtime.ModuleCache                             { return nil }
func (m *mockRTSContext) GetGlobalWorkDir() string                                   { return "/tmp" }


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
	r, w, _ := os.Pipe()
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
	fmt.Println("Captured table output:\n", output) // For manual inspection if needed
}

func TestReportTableStep_Run_NoRows(t *testing.T) {
	mockCtx := newMockRTSContext(t)
	headers := []string{"Name", "Value"}
	rts := NewReportTableStep("", headers, [][]string{}).(*ReportTableStep)

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := rts.Run(mockCtx, nil)
	require.NoError(t, err)

	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	r.Close()

	// Should print nothing or just a log message from the step
	// The step currently logs "No data to display". If it didn't print anything, buf would be empty.
	// For this test, let's assert it doesn't contain table markers if no rows.
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
