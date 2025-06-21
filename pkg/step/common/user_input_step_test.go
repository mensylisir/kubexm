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

// mockUISContext provides a minimal context for testing UserInputStep.
type mockUISContext struct {
	runtime.StepContext
	logger *logger.Logger
	goCtx  context.Context
}

func newMockUISContext(t *testing.T) *mockUISContext {
	l, _ := logger.New(logger.DefaultConfig())
	return &mockUISContext{
		logger: l,
		goCtx:  context.Background(),
	}
}
func (m *mockUISContext) GetLogger() *logger.Logger { return m.logger }
func (m *mockUISContext) GoContext() context.Context  { return m.goCtx }
func (m *mockUISContext) GetHost() connector.Host   { return nil } // Assuming local/control node operation
// Implement other StepContext methods if any are called by the step, returning default/nil.
func (m *mockUISContext) GetRunner() runner.Runner                                   { return nil }
func (m *mockUISContext) GetConnectorForHost(h connector.Host) (connector.Connector, error) { return nil, nil }
func (m *mockUISContext) GetHostFacts(h connector.Host) (*runner.Facts, error)           { return nil, nil }
func (m *mockUISContext) GetCurrentHostFacts() (*runner.Facts, error)                  { return nil, nil }
func (m *mockUISContext) GetCurrentHostConnector() (connector.Connector, error)        { return nil, nil }
func (m *mockUISContext) StepCache() runtime.StepCache                               { return nil }
func (m *mockUISContext) TaskCache() runtime.TaskCache                               { return nil }
func (m *mockUISContext) ModuleCache() runtime.ModuleCache                             { return nil }
func (m *mockUISContext) GetGlobalWorkDir() string                                   { return "/tmp" }


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

	// Capture stdout to ensure nothing is printed
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := uis.Run(mockCtx, nil)
	require.NoError(t, err)

	w.Close()
	os.Stdout = oldStdout // Restore stdout
	var buf bytes.Buffer
	_, copyErr := io.Copy(&buf, r)
	require.NoError(t, copyErr)
	r.Close()

	assert.Empty(t, buf.String(), "Run should not print anything if AssumeYes is true")
}

func TestUserInputStep_Run_UserInput_Yes(t *testing.T) {
	mockCtx := newMockUISContext(t)
	prompt := "Confirm action?"
	uis := NewUserInputStep("", prompt, false).(*UserInputStep)

	// Simulate user input "yes"
	inputBuffer := bytes.NewBufferString("yes\n")
	oldStdin := os.Stdin
	os.Stdin = inputBuffer // Replace os.Stdin with our buffer
	defer func() { os.Stdin = oldStdin }() // Restore os.Stdin

	// Capture stdout to check prompt
	oldStdout := os.Stdout
	rStdout, wStdout, _ := os.Pipe()
	os.Stdout = wStdout

	err := uis.Run(mockCtx, nil)
	require.NoError(t, err)

	wStdout.Close()
	os.Stdout = oldStdout // Restore stdout
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

	inputBuffer := bytes.NewBufferString("no\n")
	oldStdin := os.Stdin
	os.Stdin = inputBuffer
	defer func() { os.Stdin = oldStdin }()

	err := uis.Run(mockCtx, nil)
	require.Error(t, err)
	assert.EqualError(t, err, "user declined confirmation")
}

func TestUserInputStep_Run_UserInput_InvalidThenYes(t *testing.T) {
	mockCtx := newMockUISContext(t)
	prompt := "Confirm action?"
	uis := NewUserInputStep("", prompt, false).(*UserInputStep)

	// Test with an initial invalid input, then "y"
	inputBuffer := bytes.NewBufferString("maybe\nyes\n") // User types "maybe", then "yes"
	oldStdin := os.Stdin
	os.Stdin = inputBuffer
	defer func() { os.Stdin = oldStdin }()

	// Capture stdout
	oldStdout := os.Stdout
	rStdout, wStdout, _ := os.Pipe()
	os.Stdout = wStdout

	// Note: The current implementation of UserInputStep only reads one line.
	// To test multi-line or retry logic, the step itself would need to change.
	// This test will effectively test with "maybe" as input, which will be treated as "no".
	err := uis.Run(mockCtx, nil)

	wStdout.Close()
	os.Stdout = oldStdout
	var outBuf bytes.Buffer
	_, copyErr := io.Copy(&outBuf, rStdout)
	require.NoError(t, copyErr)
	rStdout.Close()

	assert.Equal(t, prompt+" [yes/no]: ", outBuf.String())
	require.Error(t, err)
	assert.EqualError(t, err, "user declined confirmation") // Because "maybe" is not "yes" or "y"
}


func TestUserInputStep_Rollback(t *testing.T) {
	mockCtx := newMockUISContext(t)
	uis := NewUserInputStep("", "Prompt", false).(*UserInputStep)
	err := uis.Rollback(mockCtx, nil)
	assert.NoError(t, err, "Rollback for UserInputStep should be a no-op.")
}
