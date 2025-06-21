package pre

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

// mockTaskContextForPreTask provides a mock runtime.TaskContext.
type mockTaskContextForPreTask struct {
	runtime.TaskContext
	logger       *logger.Logger
	goCtx        context.Context
	controlHost  connector.Host
	remoteHosts  map[string]*runtime.HostRuntime // Using HostRuntime to include Host object
	clusterCfg   *v1alpha1.Cluster
}

func newMockTaskContextForPreTask(t *testing.T, numRemoteHosts int) *mockTaskContextForPreTask {
	l, _ := logger.New(logger.DefaultConfig())

	ctrlHostSpec := v1alpha1.Host{Name: common.ControlNodeHostName, Type: "local", Address: "127.0.0.1", Roles: []string{common.ControlNodeRole}}
	ctrlHost := connector.NewHostFromSpec(ctrlHostSpec)

	hostRuntimes := map[string]*runtime.HostRuntime{
		common.ControlNodeHostName: {Host: ctrlHost, Conn: &connector.LocalConnector{}},
	}

	var hostSpecs []*v1alpha1.Host
	hostSpecs = append(hostSpecs, &ctrlHostSpec) // Add control node spec for completeness if needed by GetClusterConfig()

	for i := 0; i < numRemoteHosts; i++ {
		remoteHostName := fmt.Sprintf("remote-host-%d", i+1)
		remoteHostSpec := v1alpha1.Host{Name: remoteHostName, Address: fmt.Sprintf("10.0.0.%d", i+1), Roles: []string{"worker"}}
		remoteHost := connector.NewHostFromSpec(remoteHostSpec)
		hostRuntimes[remoteHostName] = &runtime.HostRuntime{Host: remoteHost, Conn: &connector.LocalConnector{}} // Using Local for test simplicity
		hostSpecs = append(hostSpecs, &remoteHostSpec)
	}

	return &mockTaskContextForPreTask{
		logger:      l,
		goCtx:       context.Background(),
		controlHost: ctrlHost,
		remoteHosts: hostRuntimes,
		clusterCfg: &v1alpha1.Cluster{
			ObjectMeta: v1alpha1.ObjectMeta{Name: "test-cluster"},
			Spec:       v1alpha1.ClusterSpec{Hosts: hostSpecs},
		},
	}
}
func (m *mockTaskContextForPreTask) HostRuntimes() map[string]*runtime.HostRuntime { // Method to satisfy PreTask's current way of getting hosts
    return m.remoteHosts
}
func (m *mockTaskContextForPreTask) GetLogger() *logger.Logger          { return m.logger }
func (m *mockTaskContextForPreTask) GoContext() context.Context            { return m.goCtx }
func (m *mockTaskContextForPreTask) GetControlNode() (connector.Host, error) { return m.controlHost, nil }
func (m *mockTaskContextForPreTask) GetClusterConfig() *v1alpha1.Cluster { return m.clusterCfg }
func (m *mockTaskContextForPreTask) GetGlobalWorkDir() string                  { return "/tmp/kubexm-test-workdir" }
func (m *mockTaskContextForPreTask) GetHostsByRole(role string) ([]connector.Host, error) {
	var hosts []connector.Host
	for _, hr := range m.remoteHosts {
		for _, r := range hr.Host.GetRoles() {
			if r == role {
				hosts = append(hosts, hr.Host)
				break
			}
		}
	}
	return hosts, nil
}
func (m *mockTaskContextForPreTask) GetHostFacts(host connector.Host) (*runner.Facts, error) {
	return &runner.Facts{OS: &connector.OS{Arch: "amd64"}}, nil // Minimal facts
}
func (m *mockTaskContextForPreTask) PipelineCache() runtime.PipelineCache                 { return nil }
func (m *mockTaskContextForPreTask) ModuleCache() runtime.ModuleCache                     { return nil }
func (m *mockTaskContextForPreTask) TaskCache() runtime.TaskCache                       { return nil }


func TestPreTask_NewPreTask(t *testing.T) {
	pt := NewPreTask()
	require.NotNil(t, pt)
	assert.Equal(t, "PreFlightChecks", pt.Name())
	assert.NotEmpty(t, pt.Description())
}

func TestPreTask_IsRequired(t *testing.T) {
	pt := NewPreTask()
	mockCtx := newMockTaskContextForPreTask(t, 0)
	required, err := pt.IsRequired(mockCtx)
	require.NoError(t, err)
	assert.True(t, required)
}

func TestPreTask_Plan_NoRemoteHosts(t *testing.T) {
	pt := NewPreTask().(*PreTask)
	mockCtx := newMockTaskContextForPreTask(t, 0) // No remote hosts

	fragment, err := pt.Plan(mockCtx)
	require.NoError(t, err)
	require.NotNil(t, fragment)

	// Expect only the report node if no remote hosts and no local-only checks defined in task yet
	// Currently, checks default to controlNode if RunOnAll=true and no remote hosts.
	// Let's count how many checks are *not* RunOnAll (meaning they'd target control node by default)
	numControlNodeOnlyChecks := 0
	// This depends on the 'checks' definition in pre_task.go.
	// For now, all defined checks are RunOnAll=true. So if no remote hosts, they are skipped.

	if numControlNodeOnlyChecks == 0 {
		// Only the summary report node should exist
		require.Len(t, fragment.Nodes, 1, "Should have only summary report node if no remote hosts and no local checks")
		reportNodeID := plan.NodeID("preflight-summary-report")
		_, ok := fragment.Nodes[reportNodeID]
		assert.True(t, ok, "Summary report node should be present")
		assert.Empty(t, fragment.Nodes[reportNodeID].Dependencies, "Report node should have no dependencies if no checks ran")
		assert.ElementsMatch(t, []plan.NodeID{reportNodeID}, fragment.EntryNodes)
		assert.ElementsMatch(t, []plan.NodeID{reportNodeID}, fragment.ExitNodes)
	} else {
		// This branch would be more complex, depending on how many checks target control node
		t.Skip("Test logic for control-node-only checks needs specific check definitions.")
	}
}

func TestPreTask_Plan_WithRemoteHosts(t *testing.T) {
	pt := NewPreTask().(*PreTask)
	mockCtx := newMockTaskContextForPreTask(t, 2) // 2 remote hosts

	fragment, err := pt.Plan(mockCtx)
	require.NoError(t, err)
	require.NotNil(t, fragment)

	// Number of checks defined in PreTask.checks where RunOnAll = true
	// From current pre_task.go: Hostname, OSRelease, KernelVersion, TotalMemory, CPUInfo,
	// Firewall, SELinux, Swap, Overlay, BrNetfilter, IPv4Forward, BridgeNFCallIPTables = 12 checks
	expectedRemoteChecks := 12

	// Total nodes = (expectedRemoteChecks) + 1 (summary report)
	// This assumes all checks are RunOnAll=true and thus target the remote hosts.
	// If some checks were specific to controlNode, the count would differ.
	expectedNumNodes := expectedRemoteChecks + 1

	// Iterate through fragment.Nodes to count command steps and report step
	numCmdSteps := 0
	var reportNode *plan.ExecutionNode
	var cmdNodeIDs []plan.NodeID

	for id, node := range fragment.Nodes {
		if _, ok := node.Step.(*commonsteps.CommandStep); ok {
			numCmdSteps++
			cmdNodeIDs = append(cmdNodeIDs, id)
		} else if _, ok := node.Step.(*commonsteps.ReportTableStep); ok {
			reportNode = node
		}
	}

	assert.Equal(t, expectedRemoteChecks, numCmdSteps, "Number of command steps for checks is incorrect")
	require.NotNil(t, reportNode, "ReportTableStep node not found")
	assert.Len(t, fragment.Nodes, expectedNumNodes, "Total number of nodes in fragment is incorrect")


	// Check dependencies of the report node
	require.NotNil(t, reportNode)
	assert.ElementsMatch(t, cmdNodeIDs, reportNode.Dependencies, "Report node should depend on all command check nodes")

	// Check Entry and Exit nodes
	assert.ElementsMatch(t, cmdNodeIDs, fragment.EntryNodes, "Entry nodes should be all command check nodes")
	assert.ElementsMatch(t, []plan.NodeID{plan.NodeID("preflight-summary-report")}, fragment.ExitNodes, "Exit node should be the report node")

	// Verify hosts for command steps
	for _, node := range fragment.Nodes {
		if _, ok := node.Step.(*commonsteps.CommandStep); ok {
			assert.Len(t, node.Hosts, 2, "Command check steps should run on all 2 remote hosts")
			for _, h := range node.Hosts {
				assert.NotEqual(t, common.ControlNodeHostName, h.GetName(), "Command checks should not run on control node in this setup")
			}
		}
	}
}
