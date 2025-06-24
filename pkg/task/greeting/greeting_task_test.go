package greeting

import (
	"context"
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

// mockTaskContextForGreeting provides a mock runtime.TaskContext.
type mockTaskContextForGreeting struct {
	task.TaskContext // Embed for any future TaskContext methods not directly mocked
	logger           *logger.Logger
	goCtx            context.Context
	controlHost      connector.Host
}

func newMockTaskContextForGreeting() *mockTaskContextForGreeting {
	l, _ := logger.New(logger.DefaultConfig())
	// Ensure common.ControlNodeHostName and common.ControlNodeRole are defined
	// For testing, we can use placeholder strings if not readily available or crucial for this specific test
	ctrlHostSpec := v1alpha1.HostSpec{
		Name:    common.ControlNodeHostName, // Assume "control-node"
		Type:    "local",
		Address: "127.0.0.1",
		Roles:   []string{common.ControlNodeRole}, // Assume "control_node_role"
	}
	ctrlHost := connector.NewHostFromSpec(ctrlHostSpec)

	return &mockTaskContextForGreeting{
		logger:      l,
		goCtx:       context.Background(),
		controlHost: ctrlHost,
	}
}

// Implement runtime.TaskContext interface methods needed by GreetingTask
func (m *mockTaskContextForGreeting) GetLogger() *logger.Logger  { return m.logger }
func (m *mockTaskContextForGreeting) GoContext() context.Context { return m.goCtx }
func (m *mockTaskContextForGreeting) GetControlNode() (connector.Host, error) {
	return m.controlHost, nil
}
func (m *mockTaskContextForGreeting) GetClusterConfig() *v1alpha1.Cluster { return &v1alpha1.Cluster{} }
func (m *mockTaskContextForGreeting) GetWorkDir() string                  { return "/tmp/kubexm-test-workdir" }

// Mock other TaskContext methods as needed, returning defaults/nils
func (m *mockTaskContextForGreeting) GetHostsByRole(role string) ([]connector.Host, error) {
	return nil, nil
}
func (m *mockTaskContextForGreeting) GetHostFacts(host connector.Host) (*runtime.Facts, error) {
	return nil, nil
}
func (m *mockTaskContextForGreeting) PipelineCache() runtime.PipelineCache { return nil } // Assuming these return specific cache types
func (m *mockTaskContextForGreeting) ModuleCache() runtime.ModuleCache     { return nil }
func (m *mockTaskContextForGreeting) TaskCache() runtime.TaskCache         { return nil }

func TestGreetingTask_NewGreetingTask(t *testing.T) {
	gt := NewGreetingTask()
	require.NotNil(t, gt)
	// Assuming BaseTask provides Name() and Description(), or GreetingTask overrides them.
	// If GreetingTask directly provides them:
	assert.Equal(t, "DisplayWelcomeGreeting", gt.Name())
	assert.NotEmpty(t, gt.Description())

	if concreteTask, ok := gt.(*GreetingTask); ok {
		assert.Equal(t, DefaultLogo, concreteTask.LogoMessage)
	} else {
		t.Fatalf("NewGreetingTask did not return a *GreetingTask")
	}
}

func TestGreetingTask_IsRequired(t *testing.T) {
	gt := NewGreetingTask()
	mockCtx := newMockTaskContextForGreeting()
	required, err := gt.IsRequired(mockCtx)
	require.NoError(t, err)
	assert.True(t, required)
}

func TestGreetingTask_Plan(t *testing.T) {
	// Need to ensure BaseTask is properly defined or mocked if GreetingTask relies on it
	// For this test, we are testing GreetingTask's Plan method.
	// If BaseTask is in pkg/task/task.go and NewBaseTask is its constructor:
	// task.BaseTask = task.NewBaseTask(...) // This line is not how you'd init, it's done in NewGreetingTask

	gt := NewGreetingTask().(*GreetingTask) // Cast to access LogoMessage for assertion
	mockCtx := newMockTaskContextForGreeting()
	controlHost, err := mockCtx.GetControlNode()
	require.NoError(t, err) // Check error from GetControlNode

	fragment, err := gt.Plan(mockCtx)
	require.NoError(t, err)
	require.NotNil(t, fragment)

	require.Len(t, fragment.Nodes, 1, "GreetingTask plan should have one node")

	expectedNodeID := plan.NodeID("print-welcome-logo-node")
	node, ok := fragment.Nodes[expectedNodeID]
	require.True(t, ok, "Node with ID 'print-welcome-logo-node' not found")

	assert.Equal(t, "PrintWelcomeLogoNode", node.Name)
	require.IsType(t, &commonsteps.PrintMessageStep{}, node.Step)
	printStep := node.Step.(*commonsteps.PrintMessageStep)
	assert.Equal(t, gt.LogoMessage, printStep.Message)
	assert.Equal(t, "PrintWelcomeLogo", printStep.Meta().Name) // Check step instance name

	require.Len(t, node.Hosts, 1)
	assert.Equal(t, controlHost, node.Hosts[0])
	assert.Empty(t, node.Dependencies)

	require.ElementsMatch(t, []plan.NodeID{expectedNodeID}, fragment.EntryNodes)
	require.ElementsMatch(t, []plan.NodeID{expectedNodeID}, fragment.ExitNodes)
}
