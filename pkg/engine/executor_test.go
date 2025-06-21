package engine

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/logger"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// --- Mock Step Implementation ---
type mockEngineTestStep struct {
	step.NoOpStep
	name              string
	precheckFunc      func(ctx runtime.StepContext, host connector.Host) (bool, error)
	runFunc           func(ctx runtime.StepContext, host connector.Host) error
	rollbackFunc      func(ctx runtime.StepContext, host connector.Host) error
	runDuration       time.Duration
	precheckWillBeDone bool
	runWillFail       bool
	precheckWillFail  bool
	rollbackWillFail  bool
	executionOrder    *[]string // Pointer to a shared slice to record execution order
	orderMarker       string    // Marker for this step in executionOrder
	hostMutex         sync.Mutex // To protect hostExecutions map
	hostExecutions    map[string]int // Track executions per host
}

func (m *mockEngineTestStep) Meta() *spec.StepMeta {
	return &spec.StepMeta{Name: m.name, Description: "Mock step for engine tests"}
}

func (m *mockEngineTestStep) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	m.hostMutex.Lock()
	if m.hostExecutions == nil {
		m.hostExecutions = make(map[string]int)
	}
	m.hostExecutions[host.GetName()+"_precheck"]++
	m.hostMutex.Unlock()

	if m.executionOrder != nil && m.orderMarker != "" {
		*m.executionOrder = append(*m.executionOrder, m.orderMarker+"_precheck_"+host.GetName())
	}
	if m.precheckFunc != nil {
		return m.precheckFunc(ctx, host)
	}
	if m.precheckWillFail {
		return false, fmt.Errorf("mock precheck failed for %s on %s", m.name, host.GetName())
	}
	return m.precheckWillBeDone, nil
}

func (m *mockEngineTestStep) Run(ctx runtime.StepContext, host connector.Host) error {
	m.hostMutex.Lock()
	if m.hostExecutions == nil {
		m.hostExecutions = make(map[string]int)
	}
	m.hostExecutions[host.GetName()+"_run"]++
	m.hostMutex.Unlock()

	if m.runDuration > 0 {
		time.Sleep(m.runDuration)
	}
	if m.executionOrder != nil && m.orderMarker != "" {
		*m.executionOrder = append(*m.executionOrder, m.orderMarker+"_run_"+host.GetName())
	}
	if m.runFunc != nil {
		return m.runFunc(ctx, host)
	}
	if m.runWillFail {
		return fmt.Errorf("mock run failed for %s on %s", m.name, host.GetName())
	}
	return nil
}

func (m *mockEngineTestStep) Rollback(ctx runtime.StepContext, host connector.Host) error {
	m.hostMutex.Lock()
	if m.hostExecutions == nil {
		m.hostExecutions = make(map[string]int)
	}
	m.hostExecutions[host.GetName()+"_rollback"]++
	m.hostMutex.Unlock()

	if m.executionOrder != nil && m.orderMarker != "" {
		*m.executionOrder = append(*m.executionOrder, m.orderMarker+"_rollback_"+host.GetName())
	}
	if m.rollbackFunc != nil {
		return m.rollbackFunc(ctx, host)
	}
	if m.rollbackWillFail {
		return fmt.Errorf("mock rollback failed for %s on %s", m.name, host.GetName())
	}
	return nil
}

// --- Test Runtime Context Setup ---
func newTestEngineRuntimeContext(t *testing.T, hostsSpec ...v1alpha1.Host) *runtime.Context {
	t.Helper()
	l, _ := logger.New(logger.DefaultConfig())

	hostRuntimes := make(map[string]*runtime.HostRuntime)
	var clusterHosts []*v1alpha1.Host

	// Ensure control node is always present
	ctrlHostSpec := v1alpha1.Host{Name: common.ControlNodeHostName, Type: "local", Address: "127.0.0.1", Roles: []string{common.ControlNodeRole}}
	ctrlHost := connector.NewHostFromSpec(ctrlHostSpec) // This is connector.Host
	hostRuntimes[common.ControlNodeHostName] = &runtime.HostRuntime{
		Host:  ctrlHost,
		Conn:  &connector.LocalConnector{}, // Mock connector, or LocalConnector for tests
		Facts: &runner.Facts{OS: &connector.OS{Arch: "amd64"}},
	}
	clusterHosts = append(clusterHosts, &ctrlHostSpec)


	for _, hs := range hostsSpec {
		specCopy := hs // Capture range variable
		h := connector.NewHostFromSpec(specCopy)
		hostRuntimes[hs.Name] = &runtime.HostRuntime{
			Host:  h,
			Conn:  &connector.LocalConnector{}, // Using LocalConnector for simplicity in engine tests
			Facts: &runner.Facts{OS: &connector.OS{Arch: "amd64"}},
		}
		clusterHosts = append(clusterHosts, &specCopy)
	}

	return &runtime.Context{
		GoCtx:  context.Background(),
		Logger: l,
		// Runner and Engine are not directly used by what dagExecutor calls on context,
		// but steps might use runner via StepContext.
		Runner: runner.New(), // Real runner, steps will use its methods on the mock/local connectors.
		Engine: nil,       // Engine doesn't call itself.
		ClusterConfig: &v1alpha1.Cluster{
			ObjectMeta: v1alpha1.ObjectMeta{Name: "engine-test-cluster"},
			Spec:       v1alpha1.ClusterSpec{Hosts: clusterHosts},
		},
		HostRuntimes: hostRuntimes,
		ControlNode: ctrlHost, // Set the control node
	}
}

// Helper to create a simple host spec for tests
func testHost(name string, roles ...string) v1alpha1.Host {
	return v1alpha1.Host{Name: name, Address: "127.0.0.1", Type: "local", Roles: roles}
}
func testConnectorHost(name string, roles ...string) connector.Host {
	return connector.NewHostFromSpec(testHost(name, roles...))
}


// --- Basic Tests ---
func TestEngine_Execute_EmptyGraph(t *testing.T) {
	rtCtx := newTestEngineRuntimeContext(t)
	executor := NewExecutor().(*dagExecutor)

	graph := plan.NewExecutionGraph("EmptyTestGraph")
	result, err := executor.Execute(rtCtx, graph, false)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, plan.StatusSuccess, result.Status)
	assert.Empty(t, result.NodeResults)
}

func TestEngine_Execute_SingleNode_Success(t *testing.T) {
	host1 := testConnectorHost("host1")
	rtCtx := newTestEngineRuntimeContext(t, testHost("host1")) // Add host1 to runtime
	executor := NewExecutor().(*dagExecutor)

	execOrder := []string{}
	stepA := &mockEngineTestStep{name: "StepA", orderMarker: "A", executionOrder: &execOrder}

	graph := plan.NewExecutionGraph("SingleNodeGraph")
	nodeA := &plan.ExecutionNode{Name: "NodeA", Step: stepA, Hosts: []connector.Host{host1}, StepName: "StepA"}
	graph.Nodes["A"] = nodeA

	result, err := executor.Execute(rtCtx, graph, false)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, plan.StatusSuccess, result.Status)
	require.Len(t, result.NodeResults, 1)
	assert.Equal(t, plan.StatusSuccess, result.NodeResults["A"].Status)
	assert.Contains(t, *stepA.executionOrder, "A_precheck_host1")
	assert.Contains(t, *stepA.executionOrder, "A_run_host1")
	require.NotNil(t, result.NodeResults["A"].HostResults["host1"])
	assert.Equal(t, plan.StatusSuccess, result.NodeResults["A"].HostResults["host1"].Status)
}

func TestEngine_Execute_SingleNode_StepRunFails(t *testing.T) {
	host1 := testConnectorHost("host1")
	rtCtx := newTestEngineRuntimeContext(t, testHost("host1"))
	executor := NewExecutor().(*dagExecutor)

	execOrder := []string{}
	stepA := &mockEngineTestStep{name: "StepA", runWillFail: true, orderMarker: "A", executionOrder: &execOrder}

	graph := plan.NewExecutionGraph("SingleNodeFailGraph")
	nodeA := &plan.ExecutionNode{Name: "NodeA", Step: stepA, Hosts: []connector.Host{host1}, StepName: "StepA"}
	graph.Nodes["A"] = nodeA

	result, err := executor.Execute(rtCtx, graph, false)

	require.NoError(t, err) // Engine itself doesn't error, failure is in result
	require.NotNil(t, result)
	assert.Equal(t, plan.StatusFailed, result.Status)
	require.Len(t, result.NodeResults, 1)
	assert.Equal(t, plan.StatusFailed, result.NodeResults["A"].Status)
	assert.Contains(t, result.NodeResults["A"].Message, "mock run failed for StepA on host1")
	assert.Contains(t, *stepA.executionOrder, "A_precheck_host1")
	assert.Contains(t, *stepA.executionOrder, "A_run_host1")
	assert.Contains(t, *stepA.executionOrder, "A_rollback_host1") // Rollback should be called
	require.NotNil(t, result.NodeResults["A"].HostResults["host1"])
	assert.Equal(t, plan.StatusFailed, result.NodeResults["A"].HostResults["host1"].Status)
}

func TestEngine_Execute_SingleNode_PrecheckDone(t *testing.T) {
	host1 := testConnectorHost("host1")
	rtCtx := newTestEngineRuntimeContext(t, testHost("host1"))
	executor := NewExecutor().(*dagExecutor)

	execOrder := []string{}
	stepA := &mockEngineTestStep{name: "StepA", precheckWillBeDone: true, orderMarker: "A", executionOrder: &execOrder}

	graph := plan.NewExecutionGraph("SingleNodePrecheckDoneGraph")
	nodeA := &plan.ExecutionNode{Name: "NodeA", Step: stepA, Hosts: []connector.Host{host1}, StepName: "StepA"}
	graph.Nodes["A"] = nodeA

	result, err := executor.Execute(rtCtx, graph, false)

	require.NoError(t, err)
	assert.Equal(t, plan.StatusSkipped, result.Status, "Graph status should be Skipped if all nodes are skipped by precheck")
	require.Len(t, result.NodeResults, 1)
	assert.Equal(t, plan.StatusSkipped, result.NodeResults["A"].Status)
	assert.Contains(t, *stepA.executionOrder, "A_precheck_host1")
	assert.NotContains(t, *stepA.executionOrder, "A_run_host1") // Run should not be called
	require.NotNil(t, result.NodeResults["A"].HostResults["host1"])
	assert.Equal(t, plan.StatusSkipped, result.NodeResults["A"].HostResults["host1"].Status)
	assert.True(t, result.NodeResults["A"].HostResults["host1"].Skipped)
}


func TestEngine_Execute_SequentialNodes(t *testing.T) {
	host1 := testConnectorHost("host1")
	rtCtx := newTestEngineRuntimeContext(t, testHost("host1"))
	executor := NewExecutor().(*dagExecutor)
	execOrder := []string{}

	stepA := &mockEngineTestStep{name: "StepA", orderMarker: "A", executionOrder: &execOrder}
	stepB := &mockEngineTestStep{name: "StepB", orderMarker: "B", executionOrder: &execOrder}

	graph := plan.NewExecutionGraph("SequentialGraph")
	graph.Nodes["A"] = &plan.ExecutionNode{Name: "NodeA", Step: stepA, Hosts: []connector.Host{host1}, StepName: "StepA"}
	graph.Nodes["B"] = &plan.ExecutionNode{Name: "NodeB", Step: stepB, Hosts: []connector.Host{host1}, StepName: "StepB", Dependencies: []plan.NodeID{"A"}}

	result, err := executor.Execute(rtCtx, graph, false)
	require.NoError(t, err)
	assert.Equal(t, plan.StatusSuccess, result.Status)
	assert.Equal(t, plan.StatusSuccess, result.NodeResults["A"].Status)
	assert.Equal(t, plan.StatusSuccess, result.NodeResults["B"].Status)

	require.Len(t, execOrder, 4) // A_pre, A_run, B_pre, B_run for host1
	assert.Equal(t, "A_precheck_host1", execOrder[0])
	assert.Equal(t, "A_run_host1", execOrder[1])
	assert.Equal(t, "B_precheck_host1", execOrder[2])
	assert.Equal(t, "B_run_host1", execOrder[3])
}

func TestEngine_Execute_ParallelNodes(t *testing.T) {
	host1 := testConnectorHost("host1")
	rtCtx := newTestEngineRuntimeContext(t, testHost("host1"))
	executor := NewExecutor().(*dagExecutor)
	executor.maxWorkers = 2 // Allow parallel execution

	execOrder := []string{} // Order isn't strictly guaranteed for parallel, but calls should be present
	muOrder := sync.Mutex{} // Protect execOrder for parallel appends

	appender := func(marker string) {
		muOrder.Lock()
		defer muOrder.Unlock()
		execOrder = append(execOrder, marker)
	}

	stepA := &mockEngineTestStep{name: "StepA", runDuration: 50 * time.Millisecond,
		executionOrder: &execOrder, orderMarker:"A", // This direct append won't work well for parallel, use runFunc
		runFunc: func(ctx runtime.StepContext, host connector.Host) error { appender("A_run_"+host.GetName()); return nil},
		precheckFunc: func(ctx runtime.StepContext, host connector.Host) (bool, error) {appender("A_precheck_"+host.GetName()); return false, nil},
	}
	stepB := &mockEngineTestStep{name: "StepB", runDuration: 50 * time.Millisecond,
		executionOrder: &execOrder, orderMarker:"B",
		runFunc: func(ctx runtime.StepContext, host connector.Host) error { appender("B_run_"+host.GetName()); return nil},
		precheckFunc: func(ctx runtime.StepContext, host connector.Host) (bool, error) {appender("B_precheck_"+host.GetName()); return false, nil},
	}


	graph := plan.NewExecutionGraph("ParallelGraph")
	graph.Nodes["A"] = &plan.ExecutionNode{Name: "NodeA", Step: stepA, Hosts: []connector.Host{host1}, StepName: "StepA"}
	graph.Nodes["B"] = &plan.ExecutionNode{Name: "NodeB", Step: stepB, Hosts: []connector.Host{host1}, StepName: "StepB"}

	result, err := executor.Execute(rtCtx, graph, false)
	require.NoError(t, err)
	assert.Equal(t, plan.StatusSuccess, result.Status)
	assert.Equal(t, plan.StatusSuccess, result.NodeResults["A"].Status)
	assert.Equal(t, plan.StatusSuccess, result.NodeResults["B"].Status)

	muOrder.Lock() // Check execOrder after ensuring all goroutines are done
	assert.Contains(t, execOrder, "A_precheck_host1")
	assert.Contains(t, execOrder, "A_run_host1")
	assert.Contains(t, execOrder, "B_precheck_host1")
	assert.Contains(t, execOrder, "B_run_host1")
	muOrder.Unlock()
}

func TestEngine_Execute_FailurePropagation(t *testing.T) {
	host1 := testConnectorHost("host1")
	rtCtx := newTestEngineRuntimeContext(t, testHost("host1"))
	executor := NewExecutor().(*dagExecutor)

	stepA := &mockEngineTestStep{name: "StepA", runWillFail: true} // Step A will fail
	stepB := &mockEngineTestStep{name: "StepB"}                   // Step B depends on A
	stepC := &mockEngineTestStep{name: "StepC"}                   // Step C depends on B

	graph := plan.NewExecutionGraph("FailurePropagationGraph")
	graph.Nodes["A"] = &plan.ExecutionNode{Name: "NodeA", Step: stepA, Hosts: []connector.Host{host1}, StepName: "StepA"}
	graph.Nodes["B"] = &plan.ExecutionNode{Name: "NodeB", Step: stepB, Hosts: []connector.Host{host1}, StepName: "StepB", Dependencies: []plan.NodeID{"A"}}
	graph.Nodes["C"] = &plan.ExecutionNode{Name: "NodeC", Step: stepC, Hosts: []connector.Host{host1}, StepName: "StepC", Dependencies: []plan.NodeID{"B"}}
	graph.Nodes["D"] = &plan.ExecutionNode{Name: "NodeD_independent", Step: &mockEngineTestStep{name: "StepD"}, Hosts: []connector.Host{host1}, StepName: "StepD"}


	result, err := executor.Execute(rtCtx, graph, false)
	require.NoError(t, err) // Engine itself succeeds
	assert.Equal(t, plan.StatusFailed, result.Status) // Overall graph failed

	assert.Equal(t, plan.StatusFailed, result.NodeResults["A"].Status)
	assert.Equal(t, plan.StatusSkipped, result.NodeResults["B"].Status, "Node B should be skipped")
	assert.Contains(t, result.NodeResults["B"].Message, "Skipped due to failed prerequisite 'A'")
	assert.Equal(t, plan.StatusSkipped, result.NodeResults["C"].Status, "Node C should be skipped")
	assert.Contains(t, result.NodeResults["C"].Message, "Skipped due to failed prerequisite 'B'")
	assert.Equal(t, plan.StatusSuccess, result.NodeResults["D"].Status, "Independent Node D should succeed")

	// Check that Run was not called for B and C
	assert.Zero(t, stepB.hostExecutions["host1_run"], "StepB.Run should not have been called")
	assert.Zero(t, stepC.hostExecutions["host1_run"], "StepC.Run should not have been called")
}

func TestEngine_Execute_DryRun(t *testing.T) {
	host1 := testConnectorHost("host1")
	rtCtx := newTestEngineRuntimeContext(t, testHost("host1"))
	executor := NewExecutor().(*dagExecutor)

	stepA := &mockEngineTestStep{name: "StepA"}
	stepB := &mockEngineTestStep{name: "StepB"}

	graph := plan.NewExecutionGraph("DryRunTestGraph")
	graph.Nodes["A"] = &plan.ExecutionNode{Name: "NodeA", Step: stepA, Hosts: []connector.Host{host1}, StepName: "StepA"}
	graph.Nodes["B"] = &plan.ExecutionNode{Name: "NodeB", Step: stepB, Hosts: []connector.Host{host1}, StepName: "StepB", Dependencies: []plan.NodeID{"A"}}

	result, err := executor.Execute(rtCtx, graph, true) // dryRun = true
	require.NoError(t, err)
	assert.Equal(t, plan.StatusSuccess, result.Status) // Dry run itself is a success if no errors planning it

	require.Len(t, result.NodeResults, 2)
	for _, nodeRes := range result.NodeResults {
		assert.Equal(t, plan.StatusSkipped, nodeRes.Status)
		assert.Contains(t, nodeRes.Message, "Dry run: Node execution skipped")
		for _, hostRes := range nodeRes.HostResults {
			assert.Equal(t, plan.StatusSkipped, hostRes.Status)
			assert.True(t, hostRes.Skipped)
		}
	}
	// Ensure Run was not called on steps
	assert.Zero(t, stepA.hostExecutions["host1_run"], "StepA.Run should not be called in dry run")
	assert.Zero(t, stepB.hostExecutions["host1_run"], "StepB.Run should not be called in dry run")
}

// TODO: Add more tests:
// - Graph with cycles (ensure Validate catches it, or engine handles it gracefully if Validate is outside)
// - Multiple hosts per node
// - Mixed results on hosts for a single node (e.g., one host fails, one succeeds - node should be Failed)
// - Max workers limiting concurrency
// - Step Precheck failure
// - Step Rollback failure
// - Context cancellation during step execution

func TestEngine_Execute_NodeWithMultipleHosts_OneHostFails(t *testing.T) {
	host1 := testConnectorHost("host1")
	host2 := testConnectorHost("host2")
	rtCtx := newTestEngineRuntimeContext(t, testHost("host1"), testHost("host2"))
	executor := NewExecutor().(*dagExecutor)

	stepA := &mockEngineTestStep{
		name: "StepA_MultiHost",
		runFunc: func(ctx runtime.StepContext, host connector.Host) error {
			if host.GetName() == "host1" {
				return nil // Host1 succeeds
			}
			return fmt.Errorf("mock error on host2") // Host2 fails
		},
	}

	graph := plan.NewExecutionGraph("MultiHostNodeGraph")
	nodeA := &plan.ExecutionNode{Name: "NodeA", Step: stepA, Hosts: []connector.Host{host1, host2}, StepName: "StepA_MultiHost"}
	graph.Nodes["A"] = nodeA

	result, err := executor.Execute(rtCtx, graph, false)
	require.NoError(t, err) // Engine completes
	assert.Equal(t, plan.StatusFailed, result.Status, "Graph should fail if any node fails")

	nodeAResult, ok := result.NodeResults["A"]
	require.True(t, ok)
	assert.Equal(t, plan.StatusFailed, nodeAResult.Status, "NodeA should be marked as failed")
	assert.Contains(t, nodeAResult.Message, "step 'StepA_MultiHost' on host 'host2' failed")

	require.Len(t, nodeAResult.HostResults, 2)
	assert.Equal(t, plan.StatusSuccess, nodeAResult.HostResults["host1"].Status)
	assert.Equal(t, plan.StatusFailed, nodeAResult.HostResults["host2"].Status)
	assert.Contains(t, nodeAResult.HostResults["host2"].Message, "mock error on host2")
}

func TestEngine_Execute_GraphValidationFailure(t *testing.T) {
	rtCtx := newTestEngineRuntimeContext(t)
	executor := NewExecutor().(*dagExecutor)

	// Create a graph with a cycle for Validate() to catch
	graph := plan.NewExecutionGraph("CyclicGraph")
	graph.Nodes["A"] = &plan.ExecutionNode{Name: "NodeA", Step: &mockEngineTestStep{}, Dependencies: []plan.NodeID{"B"}}
	graph.Nodes["B"] = &plan.ExecutionNode{Name: "NodeB", Step: &mockEngineTestStep{}, Dependencies: []plan.NodeID{"A"}}

	// Mock g.Validate() to return an error
	// This requires ExecutionGraph to have a Validate method.
	// For this test, we assume if Validate (called by executor) returns error, it's handled.
	// Since we can't directly mock graph.Validate() here without changing plan.ExecutionGraph,
	// this test relies on the executor's behavior *if* Validate fails.
	// The current executor returns an error from Execute if g.Validate() fails.

	// If plan.ExecutionGraph.Validate is not yet implemented to detect cycles,
	// this test might show the engine deadlocking or behaving unexpectedly.
	// For now, we assume Validate works. If it doesn't detect this specific cycle,
	// the engine's deadlock detection ("Execution queue is empty, but not all nodes are processed")
	// might kick in after a timeout, or the test might hang.

	// Let's assume Validate works and returns an error for a cycle.
	// We can't easily inject that error here without modifying the graph object itself
	// or having a mockable Validate.
	// The test for Validate() itself should be in pkg/plan/graph_plan_test.go.

	// This test will check that if the graph has no entry points (a form of invalidity),
	// the engine handles it.
	graphNoEntry := plan.NewExecutionGraph("NoEntryGraph")
	graphNoEntry.Nodes["A"] = &plan.ExecutionNode{Name: "NodeA", Step: &mockEngineTestStep{}, Dependencies: []plan.NodeID{"A"}} // Self-loop

	result, err := executor.Execute(rtCtx, graphNoEntry, false)

	// The current executor's Validate() is a placeholder.
	// If Validate is a no-op, the engine will find no initial queue.
	require.Error(t, err, "Execute should error if graph has no entry points after validation")
	assert.Contains(t, err.Error(), "no entry nodes found")
	require.NotNil(t, result)
	assert.Equal(t, plan.StatusFailed, result.Status)
}
