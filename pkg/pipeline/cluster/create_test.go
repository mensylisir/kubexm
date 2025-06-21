package cluster

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/engine"
	"github.com/mensylisir/kubexm/pkg/logger"
	"github.com/mensylisir/kubexm/pkg/module"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/task"
)

// mockModuleForPipelineTest is a mock implementation of module.Module.
type mockModuleForPipelineTest struct {
	module.BaseModule // Embed BaseModule
	PlanFunc          func(ctx runtime.ModuleContext) (*task.ExecutionFragment, error)
}

func (m *mockModuleForPipelineTest) Plan(ctx runtime.ModuleContext) (*task.ExecutionFragment, error) {
	if m.PlanFunc != nil {
		return m.PlanFunc(ctx)
	}
	return task.NewEmptyFragment(), nil
}

// mockPipelineContext provides a mock runtime.PipelineContext.
// It also needs to satisfy runtime.ModuleContext and runtime.TaskContext for underlying layers.
type mockPipelineContext struct {
	runtime.PipelineContext
	logger      *logger.Logger
	goCtx       context.Context
	controlHost connector.Host
	clusterCfg  *v1alpha1.Cluster
	// For engine mocking in Run tests
	mockEngine *mockEngineForPipelineTest
}

type mockEngineForPipelineTest struct {
	engine.Engine
	ExecuteFunc func(ctx *runtime.Context, g *plan.ExecutionGraph, dryRun bool) (*plan.GraphExecutionResult, error)
}

func (m *mockEngineForPipelineTest) Execute(ctx *runtime.Context, g *plan.ExecutionGraph, dryRun bool) (*plan.GraphExecutionResult, error) {
	if m.ExecuteFunc != nil {
		return m.ExecuteFunc(ctx, g, dryRun)
	}
	return &plan.GraphExecutionResult{Status: plan.StatusSuccess}, nil
}


func newMockPipelineContext(t *testing.T) *mockPipelineContext {
	l, _ := logger.New(logger.DefaultConfig())
	ctrlHostSpec := v1alpha1.Host{Name: common.ControlNodeHostName, Type: "local", Address: "127.0.0.1", Roles: []string{common.ControlNodeRole}}
	ctrlHost := connector.NewHostFromSpec(ctrlHostSpec)

	return &mockPipelineContext{
		logger:      l,
		goCtx:       context.Background(),
		controlHost: ctrlHost,
		clusterCfg:  &v1alpha1.Cluster{ObjectMeta: v1alpha1.ObjectMeta{Name: "pipe-test-cluster"}},
		mockEngine:  &mockEngineForPipelineTest{},
	}
}
func (m *mockPipelineContext) GetLogger() *logger.Logger          { return m.logger }
func (m *mockPipelineContext) GoContext() context.Context            { return m.goCtx }
func (m *mockPipelineContext) GetControlNode() (connector.Host, error) { return m.controlHost, nil }
func (m *mockPipelineContext) GetClusterConfig() *v1alpha1.Cluster { return m.clusterCfg }
func (m *mockPipelineContext) GetGlobalWorkDir() string                  { return "/tmp/kubexm-pipe-test-workdir" }
func (m *mockPipelineContext) GetHostsByRole(role string) ([]connector.Host, error) { return []connector.Host{m.controlHost}, nil }
func (m *mockPipelineContext) GetHostFacts(host connector.Host) (*runner.Facts, error) { return &runner.Facts{}, nil }
func (m *mockPipelineContext) PipelineCache() runtime.PipelineCache                 { return nil }
func (m *mockPipelineContext) ModuleCache() runtime.ModuleCache                     { return nil }
func (m *mockPipelineContext) TaskCache() runtime.TaskCache                       { return nil }
func (m *mockPipelineContext) GetEngine() engine.Engine                           { return m.mockEngine } // For Run test


func TestCreateClusterPipeline_NewCreateClusterPipeline(t *testing.T) {
	p := NewCreateClusterPipeline(false) // assumeYes = false
	require.NotNil(t, p)
	assert.Equal(t, "CreateNewCluster", p.Name())
	// Initially, only PreflightModule might be present.
	// This test might need adjustment as more modules are added to the constructor.
	assert.NotEmpty(t, p.Modules(), "Pipeline should have at least one module")
}

func TestCreateClusterPipeline_Plan_SequentialLinking(t *testing.T) {
	mockPipeCtx := newMockPipelineContext(t)

	// Mock modules
	mod1Entry := plan.NodeID("mod1-entry")
	mod1Exit := plan.NodeID("mod1-exit")
	mod1Node := &plan.ExecutionNode{Name: "Mod1Node"}
	mockModule1 := &mockModuleForPipelineTest{
		BaseModule: module.BaseModule{ModuleName: "MockModule1"},
		PlanFunc: func(ctx runtime.ModuleContext) (*task.ExecutionFragment, error) {
			return &task.ExecutionFragment{
				Nodes:      map[plan.NodeID]*plan.ExecutionNode{mod1Entry: mod1Node, mod1Exit: mod1Node},
				EntryNodes: []plan.NodeID{mod1Entry},
				ExitNodes:  []plan.NodeID{mod1Exit},
			}, nil
		},
	}

	mod2Entry := plan.NodeID("mod2-entry")
	mod2Exit := plan.NodeID("mod2-exit")
	mod2Node := &plan.ExecutionNode{Name: "Mod2Node"}
	mockModule2 := &mockModuleForPipelineTest{
		BaseModule: module.BaseModule{ModuleName: "MockModule2"},
		PlanFunc: func(ctx runtime.ModuleContext) (*task.ExecutionFragment, error) {
			return &task.ExecutionFragment{
				Nodes:      map[plan.NodeID]*plan.ExecutionNode{mod2Entry: mod2Node, mod2Exit: mod2Node},
				EntryNodes: []plan.NodeID{mod2Entry},
				ExitNodes:  []plan.NodeID{mod2Exit},
			}, nil
		},
	}

	// Create pipeline with these mock modules
	ccp := &CreateClusterPipeline{
		PipelineName:    "TestPipelineWithMocks",
		PipelineModules: []module.Module{mockModule1, mockModule2},
	}

	finalGraph, err := ccp.Plan(mockPipeCtx)
	require.NoError(t, err)
	require.NotNil(t, finalGraph)
	assert.Equal(t, "TestPipelineWithMocks", finalGraph.Name)
	assert.Len(t, finalGraph.Nodes, 4) // 2 nodes per module

	// Check dependencies: mod2's entry should depend on mod1's exit
	nodeM2Entry, ok := finalGraph.Nodes[mod2Entry]
	require.True(t, ok)
	assert.Contains(t, nodeM2Entry.Dependencies, mod1Exit)

	// The overall graph entry/exit nodes are not explicitly set on ExecutionGraph struct.
	// The engine infers them. But we can check that mod1Entry has no dependencies.
	nodeM1Entry, ok := finalGraph.Nodes[mod1Entry]
	require.True(t, ok)
	assert.Empty(t, nodeM1Entry.Dependencies)
}

func TestCreateClusterPipeline_Plan_ModuleReturnsEmptyFragment(t *testing.T) {
	mockPipeCtx := newMockPipelineContext(t)

	mod1Entry := plan.NodeID("mod1-entry")
	mod1Exit := plan.NodeID("mod1-exit")
	mockModule1 := &mockModuleForPipelineTest{
		BaseModule: module.BaseModule{ModuleName: "MockModule1"},
		PlanFunc: func(ctx runtime.ModuleContext) (*task.ExecutionFragment, error) {
			return &task.ExecutionFragment{
				Nodes:      map[plan.NodeID]*plan.ExecutionNode{mod1Entry: {}, mod1Exit: {}},
				EntryNodes: []plan.NodeID{mod1Entry},
				ExitNodes:  []plan.NodeID{mod1Exit},
			}, nil
		},
	}
	mockModuleEmpty := &mockModuleForPipelineTest{
		BaseModule: module.BaseModule{ModuleName: "MockModuleEmpty"},
		PlanFunc: func(ctx runtime.ModuleContext) (*task.ExecutionFragment, error) {
			return task.NewEmptyFragment(), nil // Returns empty
		},
	}
	mod3Entry := plan.NodeID("mod3-entry")
	mod3Exit := plan.NodeID("mod3-exit")
	mockModule3 := &mockModuleForPipelineTest{
		BaseModule: module.BaseModule{ModuleName: "MockModule3"},
		PlanFunc: func(ctx runtime.ModuleContext) (*task.ExecutionFragment, error) {
			return &task.ExecutionFragment{
				Nodes:      map[plan.NodeID]*plan.ExecutionNode{mod3Entry: {}, mod3Exit: {}},
				EntryNodes: []plan.NodeID{mod3Entry},
				ExitNodes:  []plan.NodeID{mod3Exit},
			}, nil
		},
	}
	ccp := &CreateClusterPipeline{
		PipelineName:    "TestPipelineEmptyModule",
		PipelineModules: []module.Module{mockModule1, mockModuleEmpty, mockModule3},
	}
	finalGraph, err := ccp.Plan(mockPipeCtx)
	require.NoError(t, err)
	assert.Len(t, finalGraph.Nodes, 4) // ModuleEmpty contributes no nodes

	nodeM3Entry, ok := finalGraph.Nodes[mod3Entry]
	require.True(t, ok)
	assert.Contains(t, nodeM3Entry.Dependencies, mod1Exit, "Mod3 should depend on Mod1 as ModEmpty was skipped")
}

func TestCreateClusterPipeline_Run_Success(t *testing.T) {
	mockPipeCtx := newMockPipelineContext(t) // This context has a mockEngine

	// Create a pipeline with a simple mock module that returns one node
	nodeID := plan.NodeID("test-node")
	mockMod := &mockModuleForPipelineTest{
		BaseModule: module.BaseModule{ModuleName: "RunTestModule"},
		PlanFunc: func(ctx runtime.ModuleContext) (*task.ExecutionFragment, error) {
			return &task.ExecutionFragment{
				Nodes:      map[plan.NodeID]*plan.ExecutionNode{nodeID: {Name: "TestNodeForRun"}},
				EntryNodes: []plan.NodeID{nodeID},
				ExitNodes:  []plan.NodeID{nodeID},
			}, nil
		},
	}
	ccp := &CreateClusterPipeline{
		PipelineName:    "TestRunSuccess",
		PipelineModules: []module.Module{mockMod},
	}

	// Setup mock engine's ExecuteFunc
	var executedGraph *plan.ExecutionGraph
	mockPipeCtx.mockEngine.ExecuteFunc = func(ctx *runtime.Context, g *plan.ExecutionGraph, dryRun bool) (*plan.GraphExecutionResult, error) {
		executedGraph = g
		return &plan.GraphExecutionResult{GraphName: g.Name, Status: plan.StatusSuccess}, nil
	}

	// The Run method needs a full *runtime.Context
	// We'll construct one for the test, using the mockPipelineContext's parts.
	fullRuntimeCtx := &runtime.Context{
		GoCtx:         mockPipeCtx.GoContext(),
		Logger:        mockPipeCtx.GetLogger(),
		ClusterConfig: mockPipeCtx.GetClusterConfig(),
		Engine:        mockPipeCtx.GetEngine(), // This is our mockEngine
		// Other fields like Runner, HostRuntimes can be minimal if not directly used by Plan/Execute path being tested
	}


	result, err := ccp.Run(fullRuntimeCtx, false) // dryRun = false
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, plan.StatusSuccess, result.Status)
	require.NotNil(t, executedGraph)
	assert.Equal(t, "TestRunSuccess", executedGraph.Name)
	assert.Contains(t, executedGraph.Nodes, nodeID)
}

func TestCreateClusterPipeline_Run_PlanningError(t *testing.T) {
	mockPipeCtx := newMockPipelineContext(t)
	expectedErr := fmt.Errorf("module planning failed")
	mockMod := &mockModuleForPipelineTest{
		BaseModule: module.BaseModule{ModuleName: "ErrorModule"},
		PlanFunc: func(ctx runtime.ModuleContext) (*task.ExecutionFragment, error) {
			return nil, expectedErr
		},
	}
	ccp := &CreateClusterPipeline{
		PipelineName:    "TestRunPlanningError",
		PipelineModules: []module.Module{mockMod},
	}
	fullRuntimeCtx := &runtime.Context{Logger: mockPipeCtx.GetLogger(), Engine: mockPipeCtx.GetEngine()}


	_, err := ccp.Run(fullRuntimeCtx, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), expectedErr.Error())
}

func TestCreateClusterPipeline_Run_ExecutionError(t *testing.T) {
	mockPipeCtx := newMockPipelineContext(t)
	nodeID := plan.NodeID("exec-error-node")
	mockMod := &mockModuleForPipelineTest{
		BaseModule: module.BaseModule{ModuleName: "ExecErrorModule"},
		PlanFunc: func(ctx runtime.ModuleContext) (*task.ExecutionFragment, error) {
			return &task.ExecutionFragment{
				Nodes:      map[plan.NodeID]*plan.ExecutionNode{nodeID: {Name: "NodeThatFails"}},
				EntryNodes: []plan.NodeID{nodeID}, ExitNodes: []plan.NodeID{nodeID}}, nil
		},
	}
	ccp := &CreateClusterPipeline{
		PipelineName:    "TestRunExecError",
		PipelineModules: []module.Module{mockMod},
	}

	expectedExecErr := fmt.Errorf("engine execution failed")
	mockPipeCtx.mockEngine.ExecuteFunc = func(ctx *runtime.Context, g *plan.ExecutionGraph, dryRun bool) (*plan.GraphExecutionResult, error) {
		return &plan.GraphExecutionResult{GraphName: g.Name, Status: plan.StatusFailed}, expectedExecErr
	}
	fullRuntimeCtx := &runtime.Context{Logger: mockPipeCtx.GetLogger(), Engine: mockPipeCtx.GetEngine(), ClusterConfig: mockPipeCtx.GetClusterConfig()}


	_, err := ccp.Run(fullRuntimeCtx, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), expectedExecErr.Error())
}
