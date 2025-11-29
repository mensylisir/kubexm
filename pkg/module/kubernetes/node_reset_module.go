package kubernetes

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/module"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/task"
	// taskK8s "github.com/mensylisir/kubexm/pkg/task/kubernetes" // For actual tasks later
)

// NodeResetModule defines the module for resetting Kubernetes nodes.
type NodeResetModule struct {
	module.BaseModule
}

// NewNodeResetModule creates a new NodeResetModule.
func NewNodeResetModule() module.Module {
	// TODO: Define actual tasks:
	// - NewDrainNodeTask() (optional, if not handled by user or higher-level process)
	// - NewStopKubeletTask()
	// - NewRemoveKubeletBinariesTask()
	// - NewKubeadmResetTask() (if applicable)
	// - NewCleanupCRIDataTask() (e.g., remove pods, images, volumes specific to this cluster)
	// - NewRemoveCNIConfigTask()
	moduleTasks := []task.Task{
		// Example: taskK8s.NewKubeadmResetTask(),
	}

	base := module.NewBaseModule("KubernetesNodeReset", moduleTasks)
	m := &NodeResetModule{BaseModule: base}
	return m
}

// Plan generates the execution fragment for the NodeReset module.
func (m *NodeResetModule) Plan(ctx runtime.ModuleContext) (*plan.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("module", m.Name())
	logger.Info("Planning Kubernetes Node Reset module (stub implementation)...")

	moduleFragment := plan.NewExecutionFragment(m.Name() + "-Fragment")
	var previousTaskExitNodes []plan.NodeID
	firstTaskProcessed := false // Renamed from isFirstEffectiveTask for clarity

	// Get all tasks for this module. If NewNodeResetModule populates BaseModule.ModuleTasks,
	// m.Tasks() (from BaseModule) will return them.
	// If NodeResetModule overrides GetTasks(ctx), that would be called instead by a generic loop.
	// For now, assuming m.Tasks() provides the static list.
	allTasks := m.BaseModule.Tasks() // Get tasks from embedded BaseModule

	taskCtx, ok := ctx.(runtime.TaskContext)
	if !ok {
		return nil, fmt.Errorf("module context cannot be asserted to runtime.TaskContext for %s", m.Name())
	}

	for _, currentTask := range allTasks {
		// ModuleContext (ctx) is passed directly as it satisfies task.TaskContext
		taskIsRequired, err := currentTask.IsRequired(taskCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to check if task %s is required in module %s: %w", currentTask.Name(), m.Name(), err)
		}
		if !taskIsRequired {
			logger.Info("Skipping non-required task", "task", currentTask.Name())
			continue
		}

		logger.Info("Planning task", "task", currentTask.Name())
		taskFrag, err := currentTask.Plan(taskCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to plan task %s in module %s: %w", currentTask.Name(), m.Name(), err)
		}
		if taskFrag == nil || taskFrag.IsEmpty() {
			logger.Debug("Task planned an empty fragment, skipping merge/link", "task", currentTask.Name())
			continue
		}

		err = moduleFragment.MergeFragment(taskFrag)
		if err != nil {
			return nil, fmt.Errorf("failed to merge fragment from task %s into module %s: %w", currentTask.Name(), m.Name(), err)
		}

		if !firstTaskProcessed {
			// Entry nodes of the first *processed* task fragment are potential module entries.
			// This will be finalized by CalculateEntryAndExitNodes.
			// No need to manage moduleFragment.EntryNodes directly here if CalculateEntryAndExitNodes is robust.
			firstTaskProcessed = true
		} else {
			// Link current task's entry nodes to previous task's exit nodes
			if len(previousTaskExitNodes) > 0 && len(taskFrag.EntryNodes) > 0 {
				for _, entryNodeID := range taskFrag.EntryNodes {
					targetNode, exists := moduleFragment.Nodes[entryNodeID]
					if !exists {
						return nil, fmt.Errorf("internal error: entry node %s from task %s not found in merged module fragment", entryNodeID, currentTask.Name())
					}
					targetNode.Dependencies = append(targetNode.Dependencies, previousTaskExitNodes...)
					// Uniqueness of dependencies within a node is implicitly handled if Dependencies is a map/set,
					// or needs plan.UniqueNodeIDs if it's a slice and can have duplicates from multiple links.
					// ExecutionNode.Dependencies is []plan.NodeID, so unique is good.
					targetNode.Dependencies = plan.UniqueNodeIDs(targetNode.Dependencies)
				}
			}
		}
		previousTaskExitNodes = taskFrag.ExitNodes // Update for the next sequential task
	}

	// After all tasks are processed and linked, calculate the final entry/exit nodes for the module fragment.
	moduleFragment.CalculateEntryAndExitNodes()

	if moduleFragment.IsEmpty() {
		logger.Info("Kubernetes Node Reset module planned no executable nodes.")
	} else {
		logger.Info("Kubernetes Node Reset module planning complete.", "total_nodes", len(moduleFragment.Nodes))
	}

	return moduleFragment, nil
}

var _ module.Module = (*NodeResetModule)(nil)
