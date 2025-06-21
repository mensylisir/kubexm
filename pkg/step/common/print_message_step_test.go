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
	// No step import needed as we are testing a concrete step type
)

// mockPMSContext provides a minimal context for testing PrintMessageStep.
type mockPMSContext struct {
	runtime.StepContext
	logger *logger.Logger
	goCtx  context.Context
	// No connector or runner needed as PrintMessageStep prints to local stdout
}

func newMockPMSContext(t *testing.T) *mockPMSContext {
	l, _ := logger.New(logger.DefaultConfig())
	return &mockPMSContext{
		logger: l,
		goCtx:  context.Background(),
	}
}
func (m *mockPMSContext) GetLogger() *logger.Logger { return m.logger }
func (m *mockPMSContext) GoContext() context.Context  { return m.goCtx }
func (m *mockPMSContext) GetHost() connector.Host   { return nil } // Assuming local/control node operation
// Implement other StepContext methods if any are called by the step, returning default/nil.
func (m *mockPMSContext) GetRunner() runner.Runner                                   { return nil }
func (m *mockPMSContext) GetConnectorForHost(h connector.Host) (connector.Connector, error) { return nil, nil }
func (m *mockPMSContext) GetHostFacts(h connector.Host) (*runner.Facts, error)           { return nil, nil }
func (m *mockPMSContext) GetCurrentHostFacts() (*runner.Facts, error)                  { return nil, nil }
func (m *mockPMSContext) GetCurrentHostConnector() (connector.Connector, error)        { return nil, nil }
func (m *mockPMSContext) StepCache() runtime.StepCache                               { return nil }
func (m *mockPMSContext) TaskCache() runtime.TaskCache                               { return nil }
func (m *mockPMSContext) ModuleCache() runtime.ModuleCache                             { return nil }
func (m *mockPMSContext) GetGlobalWorkDir() string                                   { return "/tmp" }


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

	err := pms.Run(mockCtx, nil) // Host is nil for control node operations
	require.NoError(t, err)

	w.Close()
	os.Stdout = oldStdout // Restore stdout

	var buf bytes.Buffer
	_, copyErr := io.Copy(&buf, r)
	require.NoError(t, copyErr)
	r.Close()

	// fmt.Println adds a newline
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
