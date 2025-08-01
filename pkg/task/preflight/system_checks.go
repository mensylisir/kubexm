package preflight

import (
	"fmt"
	"strings"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	commonstep "github.com/mensylisir/kubexm/pkg/step/common"
	steppreflight "github.com/mensylisir/kubexm/pkg/step/preflight"
	"github.com/mensylisir/kubexm/pkg/task"
)

// SystemChecksTask performs common system preflight checks.
type SystemChecksTask struct {
	task.BaseTask
}

// NewSystemChecksTask creates a new SystemChecksTask.
func NewSystemChecksTask() task.Task {
	return &SystemChecksTask{
		BaseTask: task.NewBaseTask(
			"SystemPreflightChecks",
			"Performs common system preflight checks.",
			[]string{common.RoleMaster, common.RoleWorker},
			nil,
			false,
		),
	}
}

func (t *SystemChecksTask) Plan(ctx runtime.TaskContext) (*task.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("task", t.Name())
	fragment := task.NewExecutionFragment(t.Name())

	targetHosts, err := ctx.GetHostsByRole(t.GetRunOnRoles()...)
	if err != nil {
		return nil, fmt.Errorf("failed to get hosts for task %s: %w", t.Name(), err)
	}
	if len(targetHosts) == 0 {
		return task.NewEmptyFragment(), nil
	}

	clusterCfg := ctx.GetClusterConfig()
	minCores := int32(common.DefaultMinCPUCores)
	minMemoryMB := uint64(common.DefaultMinMemoryMB)
	if clusterCfg.Spec.Preflight != nil {
		if clusterCfg.Spec.Preflight.MinCPUCores != nil {
			minCores = *clusterCfg.Spec.Preflight.MinCPUCores
		}
		if clusterCfg.Spec.Preflight.MinMemoryMB != nil {
			minMemoryMB = *clusterCfg.Spec.Preflight.MinMemoryMB
		}
	}

	// --- Define all checks ---
	var checkNodes []plan.NodeID

	// Resource Checks
	cpuCheckStep := steppreflight.NewCheckCPUStep("CheckCPUCores", minCores, false)
	cpuCheckNodeID, _ := fragment.AddNode(&plan.ExecutionNode{Name: "CheckCPUCores", Step: cpuCheckStep, Hosts: targetHosts})
	checkNodes = append(checkNodes, cpuCheckNodeID)

	memCheckStep := steppreflight.NewCheckMemoryStep("CheckMemory", minMemoryMB, false)
	memCheckNodeID, _ := fragment.AddNode(&plan.ExecutionNode{Name: "CheckMemory", Step: memCheckStep, Hosts: targetHosts})
	checkNodes = append(checkNodes, memCheckNodeID)

	// Command-based checks from the old pre_task.go
	cmdChecks := []struct {
		Name    string
		Command string
	}{
		{"Hostname", "hostname"},
		{"OSRelease", "cat /etc/os-release"},
		{"KernelVersion", "uname -r"},
		{"FirewallStatus", "systemctl is-active firewalld || echo 'inactive'"},
		{"SELinuxStatus", "sestatus | grep -i 'SELinux status:'"},
		{"SwapStatus", "swapon --summary"},
		{"OverlayModule", "lsmod | grep overlay"},
		{"BrNetfilterModule", "lsmod | grep br_netfilter"},
		{"IPv4Forwarding", "sysctl net.ipv4.ip_forward"},
		{"BridgeNFCallIPTables", "sysctl net.bridge.bridge-nf-call-iptables"},
	}

	for _, check := range cmdChecks {
		cmdStep := commonstep.NewCommandStep(
			fmt.Sprintf("Check-%s", check.Name),
			check.Command,
			false, // sudo
			true,  // ignore error, we just want to see the output
			0, nil, 0, "", false, 0, "", false,
		)
		nodeID, _ := fragment.AddNode(&plan.ExecutionNode{
			Name:  fmt.Sprintf("Check-%s", check.Name),
			Step:  cmdStep,
			Hosts: targetHosts,
		})
		checkNodes = append(checkNodes, nodeID)
	}

	// All checks can run in parallel
	fragment.EntryNodes = checkNodes
	fragment.ExitNodes = checkNodes

	logger.Info("System preflight checks task planned.")
	return fragment, nil
}

var _ task.Task = (*SystemChecksTask)(nil)
