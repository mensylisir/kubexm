package pre

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/logger"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	commonsteps "github.com/mensylisir/kubexm/pkg/step/common"
	"github.com/mensylisir/kubexm/pkg/task"
)

// mockTaskContextForConfirm provides a mock runtime.TaskContext.
type mockTaskContextForConfirm struct {
	runtime.TaskContext
	logger      *logger.Logger
	goCtx       context.Context
	controlHost connector.Host
	// Add AssumeYes field to the mock context if ConfirmTask reads it from context
	// For now, AssumeYes is passed to NewConfirmTask directly.
}

func newMockTaskContextForConfirm(t *testing.T) *mockTaskContextForConfirm {
	l, _ := logger.New(logger.DefaultConfig())
	ctrlHostSpec := v1alpha1.Host{
		Name:    common.ControlNodeHostName,
		Type:    "local",
		Address: "127.0.0.1",
		Roles:   []string{common.ControlNodeRole},
	}
	ctrlHost := connector.NewHostFromSpec(ctrlHostSpec)

	return &mockTaskContextForConfirm{
		logger:      l,
		goCtx:       context.Background(),
		controlHost: ctrlHost,
	}
}

// Implement runtime.TaskContext interface
func (m *mockTaskContextForConfirm) GetLogger() *logger.Logger          { return m.logger }
func (m *mockTaskContextForConfirm) GoContext() context.Context            { return m.goCtx }
func (m *mockTaskContextForConfirm) GetControlNode() (connector.Host, error) { return m.controlHost, nil }
func (m *mockTaskContextForConfirm) GetClusterConfig() *v1alpha1.Cluster {
	// Return a minimal cluster config. If AssumeYes comes from global config, populate it here.
	return &v1alpha1.Cluster{
		Spec: v1alpha1.ClusterSpec{
			Global: &v1alpha1.Global{
				// AssumeYes: false, // Example if it were read from here
			},
		},
	}
}
func (m *mockTaskContextForConfirm) GetGlobalWorkDir() string { return "/tmp/kubexm-test-workdir" }

// Mock other TaskContext methods as needed
func (m *mockTaskContextForConfirm) GetHostsByRole(role string) ([]connector.Host, error) { return nil, nil }
func (m *mockTaskContextForConfirm) GetHostFacts(host connector.Host) (*runner.Facts, error) { return nil, nil }
func (m *mockTaskContextForConfirm) PipelineCache() runtime.PipelineCache                 { return nil }
func (m *mockTaskContextForConfirm) ModuleCache() runtime.ModuleCache                     { return nil }
func (m *mockTaskContextForConfirm) TaskCache() runtime.TaskCache                       { return nil }


func TestConfirmTask_NewConfirmTask(t *testing.T) {
	ct := NewConfirmTask("UserApproval", "Proceed?", false)
	require.NotNil(t, ct)
	assert.Equal(t, "UserApproval", ct.Name())
	assert.NotEmpty(t, ct.Description())

	concreteTask, ok := ct.(*ConfirmTask)
	require.True(t, ok)
	assert.Equal(t, "Proceed?", concreteTask.PromptMessage)
	assert.False(t, concreteTask.AssumeYes)

	ctDefault := NewConfirmTask("", "", true).(*ConfirmTask)
	assert.Equal(t, "UserConfirmation", ctDefault.Name())
	assert.Equal(t, DefaultConfirmationPrompt, ctDefault.PromptMessage)
	assert.True(t, ctDefault.AssumeYes)
}

func TestConfirmTask_IsRequired(t *testing.T) {
	ct := NewConfirmTask("", "", false)
	mockCtx := newMockTaskContextForConfirm(t)
	required, err := ct.IsRequired(mockCtx)
	require.NoError(t, err)
	assert.True(t, required)
}

func TestConfirmTask_Plan_AssumeYes_False(t *testing.T) {
	ct := NewConfirmTask("", "Test prompt", false).(*ConfirmTask)
	mockCtx := newMockTaskContextForConfirm(t)
	controlHost, _ := mockCtx.GetControlNode()

	fragment, err := ct.Plan(mockCtx)
	require.NoError(t, err)
	require.NotNil(t, fragment)
	require.Len(t, fragment.Nodes, 1)

	nodeID := plan.NodeID("user-confirmation-node-" + ct.Name())
	node, ok := fragment.Nodes[nodeID]
	require.True(t, ok)

	require.IsType(t, &commonsteps.UserInputStep{}, node.Step)
	userInputStep := node.Step.(*commonsteps.UserInputStep)
	assert.Equal(t, "Test prompt", userInputStep.Prompt)
	assert.False(t, userInputStep.AssumeYes, "UserInputStep's AssumeYes should be false")
	assert.Equal(t, controlHost, node.Hosts[0])
}

func TestConfirmTask_Plan_AssumeYes_True(t *testing.T) {
	ct := NewConfirmTask("", "Test prompt", true).(*ConfirmTask) // AssumeYes is true for the task
	mockCtx := newMockTaskContextForConfirm(t)

	fragment, err := ct.Plan(mockCtx)
	require.NoError(t, err)
	require.NotNil(t, fragment)
	require.Len(t, fragment.Nodes, 1)

	nodeID := plan.NodeID("user-confirmation-node-" + ct.Name())
	node, ok := fragment.Nodes[nodeID]
	require.True(t, ok)

	require.IsType(t, &commonsteps.UserInputStep{}, node.Step)
	userInputStep := node.Step.(*commonsteps.UserInputStep)
	assert.True(t, userInputStep.AssumeYes, "UserInputStep's AssumeYes should be true")

	// Simulate UserInputStep's Precheck behavior when AssumeYes is true
	// The step's Precheck itself would return 'done = true'
	// This test confirms the task correctly configures the step.
	stepIsDone, errPrecheck := userInputStep.Precheck(mockCtx, nil) // mockCtx can serve as StepContext here
	require.NoError(t, errPrecheck)
	assert.True(t, stepIsDone, "UserInputStep's Precheck should return true if AssumeYes is true")
}

// Test the actual execution of the UserInputStep via the ConfirmTask's plan
// This requires a bit more setup if we want to simulate the engine running the step.
// For now, the Plan tests above verify the step is configured correctly.
// A full integration test would be needed to test the flow through the engine.

// Example of testing the UserInputStep's Run method (as used by ConfirmTask)
func TestConfirmTask_Integration_UserInputStep_Run_Yes(t *testing.T) {
	ct := NewConfirmTask("", "Integration Test: Proceed?", false).(*ConfirmTask)
	mockCtx := newMockTaskContextForConfirm(t) // This also implements StepContext due to embedding

	fragment, err := ct.Plan(mockCtx)
	require.NoError(t, err)
	node := fragment.Nodes[plan.NodeID("user-confirmation-node-"+ct.Name())]
	require.NotNil(t, node)
	userInputStep, ok := node.Step.(*commonsteps.UserInputStep)
	require.True(t, ok)

	// Simulate user input "yes"
	inputBuffer := bytes.NewBufferString("yes\n")
	oldStdin := os.Stdin
	os.Stdin = inputBuffer
	defer func() { os.Stdin = oldStdin }()

	// Capture stdout
	oldStdout := os.Stdout
	rStdout, wStdout, _ := os.Pipe()
	os.Stdout = wStdout

	errRun := userInputStep.Run(mockCtx, nil) // host is nil for control node steps
	require.NoError(t, errRun)

	wStdout.Close()
	os.Stdout = oldStdout
	var outBuf bytes.Buffer
	_, _ = io.Copy(&outBuf, rStdout)
	rStdout.Close()

	assert.Equal(t, "Integration Test: Proceed? [yes/no]: ", outBuf.String())
}
