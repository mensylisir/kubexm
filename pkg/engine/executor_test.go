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
	// "github.com/mensylisir/kubexm/pkg/runtime" // REMOVE THIS IMPORT
	"github.com/mensylisir/kubexm/pkg/runner" // Added for runner.Facts, runner.Runner
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1" // Added
	"path/filepath"                               // Added
	"github.com/mensylisir/kubexm/pkg/cache"      // Added
)

// --- Mock Step Implementation ---
type mockEngineTestStep struct {
	step.NoOpStep
	name              string
	precheckFunc      func(ctx step.StepContext, host connector.Host) (bool, error)
	runFunc           func(ctx step.StepContext, host connector.Host) error
	rollbackFunc      func(ctx step.StepContext, host connector.Host) error
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

func (m *mockEngineTestStep) Precheck(ctx step.StepContext, host connector.Host) (bool, error) {
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

func (m *mockEngineTestStep) Run(ctx step.StepContext, host connector.Host) error {
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

func (m *mockEngineTestStep) Rollback(ctx step.StepContext, host connector.Host) error {
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

// --- Mock Contexts for Engine Test ---

type mockExecutorTestStepContext struct {
	goCtx         context.Context
	logger        *logger.Logger
	currentHost   connector.Host
	runner        runner.Runner // Changed to runner.Runner
	clusterConfig *v1alpha1.Cluster
	// Simplified caches for testing
	internalStepCache   cache.Cache
	internalTaskCache   cache.Cache
	internalModuleCache cache.Cache
}

func newMockExecutorTestStepContext(logger *logger.Logger, host connector.Host, cfg *v1alpha1.Cluster, goCtx context.Context) *mockExecutorTestStepContext {
	return &mockExecutorTestStepContext{
		goCtx: goCtx, logger: logger, currentHost: host, clusterConfig: cfg, runner: runner.New(), // Use real runner for now, can be mocked
		internalStepCache:   cache.NewMemoryCache(), // Assuming NewMemoryCache returns cache.Cache
		internalTaskCache:   cache.NewMemoryCache(),
		internalModuleCache: cache.NewMemoryCache(),
	}
}

func (m *mockExecutorTestStepContext) GoContext() context.Context                        { return m.goCtx }
func (m *mockExecutorTestStepContext) GetLogger() *logger.Logger                         { return m.logger }
func (m *mockExecutorTestStepContext) GetHost() connector.Host                         { return m.currentHost }
func (m *mockExecutorTestStepContext) GetRunner() runner.Runner                          { return m.runner }
func (m *mockExecutorTestStepContext) GetClusterConfig() *v1alpha1.Cluster               { return m.clusterConfig }
func (m *mockExecutorTestStepContext) StepCache() cache.StepCache                       { return m.internalStepCache }
func (m *mockExecutorTestStepContext) TaskCache() cache.TaskCache                       { return m.internalTaskCache }
func (m *mockExecutorTestStepContext) ModuleCache() cache.ModuleCache                     { return m.internalModuleCache }
func (m *mockExecutorTestStepContext) GetHostsByRole(role string) ([]connector.Host, error) {
	var hosts []connector.Host
	// Define the control node spec within this function's scope or as a package-level test helper
	controlNodeSpec := v1alpha1.HostSpec{Name: common.ControlNodeHostName, Type: "local", Address: "127.0.0.1", Roles: []string{common.ControlNodeRole}}
	controlNode := connector.NewHostFromSpec(controlNodeSpec)

	if m.clusterConfig != nil && m.clusterConfig.Spec.Hosts != nil {
		for _, hSpec := range m.clusterConfig.Spec.Hosts {
			// Check if this host spec is the control node to avoid double-adding
			if hSpec.Name == common.ControlNodeHostName {
				// If roles match, add it and mark control node as found in spec
				isRoleMatched := false
				for _, r := range hSpec.Roles {
					if r == role {
						isRoleMatched = true
						break
					}
				}
				if isRoleMatched {
					// Ensure we use the unified controlNode object if it's the one from spec
					hosts = append(hosts, connector.NewHostFromSpec(hSpec))
				}
				continue // Skip further processing for this host if it's the control node from spec
			}

			// For other hosts
			for _, r := range hSpec.Roles {
				if r == role {
					hosts = append(hosts, connector.NewHostFromSpec(hSpec))
					break
				}
			}
		}
	}

	// Handle explicit request for ControlNodeRole if not found in Spec.Hosts
	if role == common.ControlNodeRole {
		alreadyAdded := false
		for _, h := range hosts {
			if h.GetName() == common.ControlNodeHostName {
				alreadyAdded = true
				break
			}
		}
		if !alreadyAdded {
			hosts = append(hosts, controlNode)
		}
	}
	return hosts, nil
}
func (m *mockExecutorTestStepContext) GetHostFacts(host connector.Host) (*runner.Facts, error) { return &runner.Facts{OS: &connector.OS{Arch: "amd64"}}, nil }
func (m *mockExecutorTestStepContext) GetCurrentHostFacts() (*runner.Facts, error)             { return m.GetHostFacts(m.currentHost) }
func (m *mockExecutorTestStepContext) GetConnectorForHost(host connector.Host) (connector.Connector, error) { return &connector.LocalConnector{}, nil }
func (m *mockExecutorTestStepContext) GetCurrentHostConnector() (connector.Connector, error) { return &connector.LocalConnector{}, nil }
func (m *mockExecutorTestStepContext) GetControlNode() (connector.Host, error) {
	// Consistently return a control node, similar to how RuntimeBuilder ensures one.
	controlNodeSpec := v1alpha1.HostSpec{Name: common.ControlNodeHostName, Type: "local", Address: "127.0.0.1", Roles: []string{common.ControlNodeRole}}
	return connector.NewHostFromSpec(controlNodeSpec), nil
}
func (m *mockExecutorTestStepContext) GetGlobalWorkDir() string                        { return "/tmp/_kubexm_engine_test" }
func (m *mockExecutorTestStepContext) IsVerbose() bool                               { return false }
func (m *mockExecutorTestStepContext) ShouldIgnoreErr() bool                         { return false }
func (m *mockExecutorTestStepContext) GetGlobalConnectionTimeout() time.Duration         { return 30 * time.Second }
func (m *mockExecutorTestStepContext) GetClusterArtifactsDir() string                  { return filepath.Join(m.GetGlobalWorkDir(), m.clusterConfig.Name) }
func (m *mockExecutorTestStepContext) GetCertsDir() string                           { return filepath.Join(m.GetClusterArtifactsDir(), "certs") }
func (m *mockExecutorTestStepContext) GetEtcdCertsDir() string                       { return filepath.Join(m.GetCertsDir(), "etcd") }
func (m *mockExecutorTestStepContext) GetComponentArtifactsDir(name string) string     { return filepath.Join(m.GetClusterArtifactsDir(), name) }
func (m *mockExecutorTestStepContext) GetEtcdArtifactsDir() string                   { return m.GetComponentArtifactsDir("etcd") }
func (m *mockExecutorTestStepContext) GetContainerRuntimeArtifactsDir() string         { return m.GetComponentArtifactsDir("container_runtime") }
func (m *mockExecutorTestStepContext) GetKubernetesArtifactsDir() string               { return m.GetComponentArtifactsDir("kubernetes") }
func (m *mockExecutorTestStepContext) GetFileDownloadPath(c, v, a, f string) string      { return filepath.Join(m.GetComponentArtifactsDir(c), v, a, f) }
func (m *mockExecutorTestStepContext) GetHostDir(hostname string) string               { return filepath.Join(m.GetGlobalWorkDir(), hostname) }
func (m *mockExecutorTestStepContext) WithGoContext(gCtx context.Context) step.StepContext {
	copyCtx := *m
	copyCtx.goCtx = gCtx
	return &copyCtx
}

var _ step.StepContext = (*mockExecutorTestStepContext)(nil) // Ensure interface is implemented

type mockEngineTestEngineExecuteContext struct {
	logger        *logger.Logger
	clusterConfig *v1alpha1.Cluster
	hosts         []v1alpha1.HostSpec // Changed to HostSpec
}

func newTestEngineExecuteContext(t *testing.T, hostsSpec ...v1alpha1.HostSpec) *mockEngineTestEngineExecuteContext { // Changed to HostSpec
	t.Helper()
	l, _ := logger.New(logger.DefaultConfig())

	allHostSpecs := append([]v1alpha1.HostSpec{ // Changed to HostSpec
		{Name: common.ControlNodeHostName, Type: "local", Address: "127.0.0.1", Roles: []string{common.ControlNodeRole}}},
		hostsSpec...)

	return &mockEngineTestEngineExecuteContext{
		logger: l,
		clusterConfig: &v1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: "engine-test-cluster"}, // Changed to metav1.ObjectMeta
			Spec:       v1alpha1.ClusterSpec{Hosts: allHostSpecs},
		},
		hosts: allHostSpecs,
	}
}

func (m *mockEngineTestEngineExecuteContext) GoContext() context.Context                        { return context.Background() }
func (m *mockEngineTestEngineExecuteContext) GetLogger() *logger.Logger                         { return m.logger }
func (m *mockEngineTestEngineExecuteContext) GetClusterConfig() *v1alpha1.Cluster               { return m.clusterConfig }
func (m *mockEngineTestEngineExecuteContext) ForHost(host connector.Host) step.StepContext {
	var foundHostSpec v1alpha1.HostSpec // Changed to HostSpec
	for _, hs := range m.hosts {
		if hs.Name == host.GetName() {
			foundHostSpec = hs
			break
		}
	}
	// Note: foundHostSpec is not directly used in newMockExecutorTestStepContext,
	// but the loop is good for verifying the host exists in the test setup.
	// The clusterConfig passed to newMockExecutorTestStepContext contains all hosts.
	if foundHostSpec.Name == "" && host.GetName() != common.ControlNodeHostName { // Allow control node not in m.hosts explicitly
		panic(fmt.Sprintf("Host %s not found in mock context setup", host.GetName()))
	}
	return newMockExecutorTestStepContext(m.logger.With("host", host.GetName()), host, m.clusterConfig, context.Background())
}

var _ EngineExecuteContext = (*mockEngineTestEngineExecuteContext)(nil) // Ensure interface is implemented

// Helper to create a simple host spec for tests
func testHost(name string, roles ...string) v1alpha1.HostSpec { // Changed to return HostSpec
	return v1alpha1.HostSpec{Name: name, Address: "127.0.0.1", Type: "local", Roles: roles, Port: 22, User: "test"}
}
func testConnectorHost(name string, roles ...string) connector.Host {
	return connector.NewHostFromSpec(testHost(name, roles...))
}


// --- Basic Tests ---
func TestEngine_Execute_EmptyGraph(t *testing.T) {
	rtCtx := newTestEngineExecuteContext(t)
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
	rtCtx := newTestEngineExecuteContext(t, testHost("host1")) // Add host1 to runtime
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
	rtCtx := newTestEngineExecuteContext(t, testHost("host1"))
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
	rtCtx := newTestEngineExecuteContext(t, testHost("host1"))
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
	rtCtx := newTestEngineExecuteContext(t, testHost("host1"))
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
	rtCtx := newTestEngineExecuteContext(t, testHost("host1"))
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
		// executionOrder: &execOrder, orderMarker:"A", // This direct append won't work well for parallel, use runFunc
		runFunc: func(ctx step.StepContext, host connector.Host) error { appender("A_run_"+host.GetName()); return nil},
		precheckFunc: func(ctx step.StepContext, host connector.Host) (bool, error) {appender("A_precheck_"+host.GetName()); return false, nil},
	}
	stepB := &mockEngineTestStep{name: "StepB", runDuration: 50 * time.Millisecond,
		// executionOrder: &execOrder, orderMarker:"B",
		runFunc: func(ctx step.StepContext, host connector.Host) error { appender("B_run_"+host.GetName()); return nil},
		precheckFunc: func(ctx step.StepContext, host connector.Host) (bool, error) {appender("B_precheck_"+host.GetName()); return false, nil},
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
	rtCtx := newTestEngineExecuteContext(t, testHost("host1"))
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
	rtCtx := newTestEngineExecuteContext(t, testHost("host1"))
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
