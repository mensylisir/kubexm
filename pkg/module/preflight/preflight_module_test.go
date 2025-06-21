package preflight

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/logger"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/task"
	// Assuming greeting and pre tasks are in these locations for mocking
	// "github.com/mensylisir/kubexm/pkg/task/greeting"
	// "github.com/mensylisir/kubexm/pkg/task/pre"
)

// mockTaskForModuleTest is a mock implementation of task.Task.
type mockTaskForModuleTest struct {
	task.BaseTask
	PlanFunc func(ctx runtime.TaskContext) (*task.ExecutionFragment, error)
	IsRequiredFunc func(ctx runtime.TaskContext) (bool, error)
}

func (m *mockTaskForModuleTest) Plan(ctx runtime.TaskContext) (*task.ExecutionFragment, error) {
	if m.PlanFunc != nil {
		return m.PlanFunc(ctx)
	}
	return task.NewEmptyFragment(), nil
}

func (m *mockTaskForModuleTest) IsRequired(ctx runtime.TaskContext) (bool, error) {
	if m.IsRequiredFunc != nil {
		return m.IsRequiredFunc(ctx)
	}
	return true, nil // Default to true if not set
}

// mockModuleContext provides a mock runtime.ModuleContext.
// It also needs to satisfy runtime.TaskContext because tasks will be planned.
type mockModuleContext struct {
	runtime.ModuleContext
	logger      *logger.Logger
	goCtx       context.Context
	controlHost connector.Host
	clusterCfg  *v1alpha1.Cluster
}

func newMockModuleContext(t *testing.T) *mockModuleContext {
	l, _ := logger.New(logger.DefaultConfig())
	ctrlHostSpec := v1alpha1.Host{Name: common.ControlNodeHostName, Type: "local", Address: "127.0.0.1", Roles: []string{common.ControlNodeRole}}
	ctrlHost := connector.NewHostFromSpec(ctrlHostSpec)

	return &mockModuleContext{
		logger:      l,
		goCtx:       context.Background(),
		controlHost: ctrlHost,
		clusterCfg:  &v1alpha1.Cluster{ObjectMeta: v1alpha1.ObjectMeta{Name: "test-cluster"}},
	}
}
func (m *mockModuleContext) GetLogger() *logger.Logger          { return m.logger }
func (m *mockModuleContext) GoContext() context.Context            { return m.goCtx }
func (m *mockModuleContext) GetControlNode() (connector.Host, error) { return m.controlHost, nil }
func (m *mockModuleContext) GetClusterConfig() *v1alpha1.Cluster { return m.clusterCfg }
func (m *mockModuleContext) GetGlobalWorkDir() string                  { return "/tmp/kubexm-test-workdir" }
func (m *mockModuleContext) GetHostsByRole(role string) ([]connector.Host, error) { return []connector.Host{m.controlHost}, nil } // Simplified
func (m *mockModuleContext) GetHostFacts(host connector.Host) (*runner.Facts, error) { return &runner.Facts{}, nil }
func (m *mockModuleContext) PipelineCache() runtime.PipelineCache                 { return nil }
func (m *mockModuleContext) ModuleCache() runtime.ModuleCache                     { return nil }
func (m *mockModuleContext) TaskCache() runtime.TaskCache                       { return nil }


func TestPreflightModule_NewPreflightModule(t *testing.T) {
	pm := NewPreflightModule(false) // AssumeYes = false
	require.NotNil(t, pm)
	assert.Equal(t, "PreflightChecksAndSetup", pm.Name())
	assert.NotEmpty(t, pm.Tasks()) // Should have Greeting, Confirm, Pre tasks by default

	if concreteMod, ok := pm.(*PreflightModule); ok {
		assert.False(t, concreteMod.AssumeYes)
	} else {
		t.Fatalf("NewPreflightModule did not return *PreflightModule")
	}
}

func TestPreflightModule_Plan_SequentialLinking(t *testing.T) {
	mockCtx := newMockModuleContext(t)

	// Create mock tasks
	task1Entry := plan.NodeID("task1-entry")
	task1Exit := plan.NodeID("task1-exit")
	task1Node := &plan.ExecutionNode{Name: "Task1Node"}
	mockTask1 := &mockTaskForModuleTest{
		BaseTask: task.BaseTask{TaskName: "MockTask1"},
		PlanFunc: func(ctx runtime.TaskContext) (*task.ExecutionFragment, error) {
			return &task.ExecutionFragment{
				Nodes:      map[plan.NodeID]*plan.ExecutionNode{task1Entry: task1Node, task1Exit: task1Node}, // Simplified node map
				EntryNodes: []plan.NodeID{task1Entry},
				ExitNodes:  []plan.NodeID{task1Exit},
			}, nil
		},
	}

	task2Entry := plan.NodeID("task2-entry")
	task2Exit := plan.NodeID("task2-exit")
	task2Node := &plan.ExecutionNode{Name: "Task2Node"}
	mockTask2 := &mockTaskForModuleTest{
		BaseTask: task.BaseTask{TaskName: "MockTask2"},
		PlanFunc: func(ctx runtime.TaskContext) (*task.ExecutionFragment, error) {
			return &task.ExecutionFragment{
				Nodes:      map[plan.NodeID]*plan.ExecutionNode{task2Entry: task2Node, task2Exit: task2Node},
				EntryNodes: []plan.NodeID{task2Entry},
				ExitNodes:  []plan.NodeID{task2Exit},
			}, nil
		},
	}

	task3Entry := plan.NodeID("task3-entry")
	task3Exit := plan.NodeID("task3-exit")
	task3Node := &plan.ExecutionNode{Name: "Task3Node"}
	mockTask3 := &mockTaskForModuleTest{
		BaseTask: task.BaseTask{TaskName: "MockTask3"},
		PlanFunc: func(ctx runtime.TaskContext) (*task.ExecutionFragment, error) {
			return &task.ExecutionFragment{
				Nodes:      map[plan.NodeID]*plan.ExecutionNode{task3Entry: task3Node, task3Exit: task3Node},
				EntryNodes: []plan.NodeID{task3Entry},
				ExitNodes:  []plan.NodeID{task3Exit},
			}, nil
		},
	}

	// Create PreflightModule with these mock tasks
	pm := &PreflightModule{
		BaseModule: task.BaseModule{
			ModuleName:  "TestPreflightWithMocks",
			ModuleTasks: []task.Task{mockTask1, mockTask2, mockTask3},
		},
		AssumeYes: false,
	}

	fullFragment, err := pm.Plan(mockCtx)
	require.NoError(t, err)
	require.NotNil(t, fullFragment)

	// Check total nodes
	assert.Len(t, fullFragment.Nodes, 6) // 2 nodes per task, 3 tasks

	// Check dependencies
	// Task2's entry should depend on Task1's exit
	nodeT2Entry, ok := fullFragment.Nodes[task2Entry]
	require.True(t, ok)
	assert.Contains(t, nodeT2Entry.Dependencies, task1Exit)

	// Task3's entry should depend on Task2's exit
	nodeT3Entry, ok := fullFragment.Nodes[task3Entry]
	require.True(t, ok)
	assert.Contains(t, nodeT3Entry.Dependencies, task2Exit)

	// Check module entry and exit nodes
	assert.ElementsMatch(t, []plan.NodeID{task1Entry}, fullFragment.EntryNodes, "Module entry should be task1's entry")
	assert.ElementsMatch(t, []plan.NodeID{task3Exit}, fullFragment.ExitNodes, "Module exit should be task3's exit")
}


func TestPreflightModule_Plan_TaskNotRequired(t *testing.T) {
	mockCtx := newMockModuleContext(t)

	task1Entry := plan.NodeID("task1-entry")
	task1Exit := plan.NodeID("task1-exit")
	mockTask1 := &mockTaskForModuleTest{
		BaseTask: task.BaseTask{TaskName: "MockTask1"},
		PlanFunc: func(ctx runtime.TaskContext) (*task.ExecutionFragment, error) {
			return &task.ExecutionFragment{
				Nodes:      map[plan.NodeID]*plan.ExecutionNode{task1Entry: {}, task1Exit: {}},
				EntryNodes: []plan.NodeID{task1Entry},
				ExitNodes:  []plan.NodeID{task1Exit},
			}, nil
		},
	}

	mockTask2Skipped := &mockTaskForModuleTest{ // This task will be skipped
		BaseTask: task.BaseTask{TaskName: "MockTask2Skipped"},
		IsRequiredFunc: func(ctx runtime.TaskContext) (bool, error) { return false, nil },
		PlanFunc: func(ctx runtime.TaskContext) (*task.ExecutionFragment, error) {
			t.Fatal("Plan should not be called for a task that is not required")
			return nil, nil
		},
	}

	task3Entry := plan.NodeID("task3-entry")
	task3Exit := plan.NodeID("task3-exit")
	mockTask3 := &mockTaskForModuleTest{
		BaseTask: task.BaseTask{TaskName: "MockTask3"},
		PlanFunc: func(ctx runtime.TaskContext) (*task.ExecutionFragment, error) {
			return &task.ExecutionFragment{
				Nodes:      map[plan.NodeID]*plan.ExecutionNode{task3Entry: {}, task3Exit: {}},
				EntryNodes: []plan.NodeID{task3Entry},
				ExitNodes:  []plan.NodeID{task3Exit},
			}, nil
		},
	}

	pm := &PreflightModule{
		BaseModule: task.BaseModule{
			ModuleName:  "TestPreflightSkippedTask",
			ModuleTasks: []task.Task{mockTask1, mockTask2Skipped, mockTask3},
		},
	}

	fullFragment, err := pm.Plan(mockCtx)
	require.NoError(t, err)
	require.NotNil(t, fullFragment)

	assert.Len(t, fullFragment.Nodes, 4) // Task2Skipped should contribute no nodes

	// Task3's entry should depend on Task1's exit (Task2 was skipped)
	nodeT3Entry, ok := fullFragment.Nodes[task3Entry]
	require.True(t, ok)
	assert.Contains(t, nodeT3Entry.Dependencies, task1Exit)

	assert.ElementsMatch(t, []plan.NodeID{task1Entry}, fullFragment.EntryNodes)
	assert.ElementsMatch(t, []plan.NodeID{task3Exit}, fullFragment.ExitNodes)
}

func TestPreflightModule_Plan_EmptyTaskFragment(t *testing.T) {
	mockCtx := newMockModuleContext(t)

	task1Entry := plan.NodeID("task1-entry")
	task1Exit := plan.NodeID("task1-exit")
	mockTask1 := &mockTaskForModuleTest{
		BaseTask: task.BaseTask{TaskName: "MockTask1"},
		PlanFunc: func(ctx runtime.TaskContext) (*task.ExecutionFragment, error) {
			return &task.ExecutionFragment{
				Nodes:      map[plan.NodeID]*plan.ExecutionNode{task1Entry: {}, task1Exit: {}},
				EntryNodes: []plan.NodeID{task1Entry},
				ExitNodes:  []plan.NodeID{task1Exit},
			}, nil
		},
	}

	mockTask2Empty := &mockTaskForModuleTest{ // This task returns an empty fragment
		BaseTask: task.BaseTask{TaskName: "MockTask2Empty"},
		PlanFunc: func(ctx runtime.TaskContext) (*task.ExecutionFragment, error) {
			return task.NewEmptyFragment(), nil
		},
	}

	task3Entry := plan.NodeID("task3-entry")
	task3Exit := plan.NodeID("task3-exit")
	mockTask3 := &mockTaskForModuleTest{
		BaseTask: task.BaseTask{TaskName: "MockTask3"},
		PlanFunc: func(ctx runtime.TaskContext) (*task.ExecutionFragment, error) {
			return &task.ExecutionFragment{
				Nodes:      map[plan.NodeID]*plan.ExecutionNode{task3Entry: {}, task3Exit: {}},
				EntryNodes: []plan.NodeID{task3Entry},
				ExitNodes:  []plan.NodeID{task3Exit},
			}, nil
		},
	}

	pm := &PreflightModule{
		BaseModule: task.BaseModule{
			ModuleName:  "TestPreflightEmptyFragment",
			ModuleTasks: []task.Task{mockTask1, mockTask2Empty, mockTask3},
		},
	}

	fullFragment, err := pm.Plan(mockCtx)
	require.NoError(t, err)
	require.NotNil(t, fullFragment)

	assert.Len(t, fullFragment.Nodes, 4) // Task2Empty should contribute no nodes

	// Task3's entry should depend on Task1's exit (Task2 was empty)
	nodeT3Entry, ok := fullFragment.Nodes[task3Entry]
	require.True(t, ok)
	assert.Contains(t, nodeT3Entry.Dependencies, task1Exit)

	assert.ElementsMatch(t, []plan.NodeID{task1Entry}, fullFragment.EntryNodes)
	assert.ElementsMatch(t, []plan.NodeID{task3Exit}, fullFragment.ExitNodes)
}
