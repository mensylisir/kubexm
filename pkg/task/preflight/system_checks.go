package preflight

import (
	"fmt"

	// "github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1" // No longer needed in constructor
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	steppreflight "github.com/mensylisir/kubexm/pkg/step/preflight" // Renamed import for clarity
	"github.com/mensylisir/kubexm/pkg/task"
)

// SystemChecksTask performs common system preflight checks.
type SystemChecksTask struct {
	task.BaseTask // Embed BaseTask
	// cfg *v1alpha1.Cluster // Config will be fetched from context in Plan
}

// NewSystemChecksTask creates a new SystemChecksTask.
// Roles can be passed to BaseTask if module wants to pre-filter.
func NewSystemChecksTask(roles []string) task.Task {
	return &SystemChecksTask{
		BaseTask: task.NewBaseTask(
			"SystemPreflightChecks",                                      // Name
			"Performs common system preflight checks like CPU, memory, and swap.", // Description
			roles, // RunOnRoles
			nil,   // HostFilter
			false, // IgnoreError
		),
	}
}

// IsRequired can use BaseTask's default or be overridden.
// func (t *SystemChecksTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
// 	 return t.BaseTask.IsRequired(ctx)
// }

// Plan generates the execution fragment for system checks.
func (t *SystemChecksTask) Plan(ctx runtime.TaskContext) (*task.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("task", t.Name())
	nodes := make(map[plan.NodeID]*plan.ExecutionNode)
	var entryNodes, exitNodes []plan.NodeID

	// Use RunOnRoles from BaseTask. If empty, GetHostsByRole("") should fetch all.
	targetHosts, err := ctx.GetHostsByRole(t.BaseTask.RunOnRoles...)
	if err != nil {
		return nil, fmt.Errorf("failed to get hosts for task %s: %w", t.Name(), err)
	}

	if len(targetHosts) == 0 {
		logger.Info("No target hosts for system checks task based on roles.")
		return task.NewEmptyFragment(), nil
	}

	clusterCfg := ctx.GetClusterConfig() // Get config from context

	// Default values for checks
	minCores := 2
	minMemoryMB := uint64(2048) // 2GB
	runDisableSwapStep := true
	sudoForSwap := true

	if clusterCfg != nil && clusterCfg.Spec.Preflight != nil {
		if clusterCfg.Spec.Preflight.MinCPUCores > 0 {
			minCores = clusterCfg.Spec.Preflight.MinCPUCores
		}
		if clusterCfg.Spec.Preflight.MinMemoryMB > 0 {
			minMemoryMB = clusterCfg.Spec.Preflight.MinMemoryMB
		}
		runDisableSwapStep = clusterCfg.Spec.Preflight.DisableSwap
	}

	// Node 1: CheckCPU
	cpuCheckStepName := fmt.Sprintf("%s-CPUCheck", t.Name())
	cpuCheckStep := steppreflight.NewCheckCPUStep(cpuCheckStepName, minCores, false)
	nodeIDCPUCheck := plan.NodeID(cpuCheckStepName)
	nodes[nodeIDCPUCheck] = &plan.ExecutionNode{
		Name:     "SystemCPUCheck",
		Step:     cpuCheckStep,
		Hosts:    targetHosts,
		StepName: cpuCheckStep.Meta().Name,
	}
	entryNodes = append(entryNodes, nodeIDCPUCheck)

	// Node 2: CheckMemory
	memCheckStepName := fmt.Sprintf("%s-MemoryCheck", t.Name())
	memCheckStep := steppreflight.NewCheckMemoryStep(memCheckStepName, minMemoryMB, false)
	nodeIDMemoryCheck := plan.NodeID(memCheckStepName)
	nodes[nodeIDMemoryCheck] = &plan.ExecutionNode{
		Name:     "SystemMemoryCheck",
		Step:     memCheckStep,
		Hosts:    targetHosts,
		StepName: memCheckStep.Meta().Name,
	}
	entryNodes = append(entryNodes, nodeIDMemoryCheck)

	// Node 3: CheckOSVersionStep (New Step to be implemented in pkg/step/preflight or pkg/step/os)
	// For now, this is a placeholder. Assume NewCheckOSVersionStep exists.
	/*
	osCheckStepName := fmt.Sprintf("%s-OSCheck", t.Name())
	// TODO: Get compatible OS list from config or constants
	compatibleOS := []string{"ubuntu_20.04_amd64", "centos_7_amd64"}
	osCheckStep := steppreflight.NewCheckOSVersionStep(osCheckStepName, compatibleOS)
	nodeIDOSCheck := plan.NodeID(osCheckStepName)
	nodes[nodeIDOSCheck] = &plan.ExecutionNode{
		Name:     "SystemOSVersionCheck",
		Step:     osCheckStep,
		Hosts:    targetHosts,
		StepName: osCheckStep.Meta().Name,
	}
	entryNodes = append(entryNodes, nodeIDOSCheck)
	*/

	// All checks can run in parallel, so they are all entry and exit nodes of this fragment.
	exitNodes = append(exitNodes, entryNodes...)

	finalEntryNodes := task.UniqueNodeIDs(entryNodes)
	finalExitNodes := task.UniqueNodeIDs(exitNodes)

	if len(nodes) == 0 {
		logger.Info("No system checks were planned for targeted hosts.")
		return task.NewEmptyFragment(), nil
	}

	logger.Info("Planned system preflight checks.", "hostCount", len(targetHosts), "nodesCount", len(nodes))
	return &task.ExecutionFragment{
		Name:       t.Name() + "-Fragment",
		Nodes:      nodes,
		EntryNodes: finalEntryNodes,
		ExitNodes:  finalExitNodes,
	}, nil
}

var _ task.Task = (*SystemChecksTask)(nil)
