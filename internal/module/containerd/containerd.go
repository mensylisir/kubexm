package containerd

import (
	"fmt"

	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/module"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/task"
	taskContainerd "github.com/mensylisir/kubexm/internal/task/containerd"
)

// ContainerdModule installs and configures containerd.
type ContainerdModule struct {
	module.BaseModule
}

// NewContainerdModule creates a new module for containerd.
func NewContainerdModule() module.Module {
	// Instantiate tasks.
	installTask := taskContainerd.NewDeployContainerdTask()

	base := module.NewBaseModule("ContainerdRuntime", []task.Task{installTask})
	return &ContainerdModule{BaseModule: base}
}

// Plan generates the execution fragment for the containerd module.
func (m *ContainerdModule) Plan(ctx runtime.ModuleContext) (*plan.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("module", m.Name())
	clusterConfig := ctx.GetClusterConfig()

	// Enablement Check
	if clusterConfig.Spec.Kubernetes == nil || clusterConfig.Spec.Kubernetes.ContainerRuntime == nil ||
		clusterConfig.Spec.Kubernetes.ContainerRuntime.Type != common.RuntimeTypeContainerd {
		logger.Info("Containerd module is not enabled (ContainerRuntime.Type is not 'containerd'). Skipping.")
		return plan.NewEmptyFragment(m.Name()), nil
	}

	moduleFragment := plan.NewExecutionFragment(m.Name() + "-Fragment") // Initialize correctly

	var previousTaskExitNodes []plan.NodeID
	isFirstEffectiveTask := true

	// Get tasks from the module
	tasks := m.Tasks()

	for _, currentTask := range tasks {
		// The ModuleContext should also implement TaskContext
		// since it has all the required methods
		taskCtx, ok := ctx.(runtime.TaskContext)
		if !ok {
			return nil, fmt.Errorf("module context does not implement TaskContext for module %s, task %s", m.Name(), currentTask.Name())
		}

		taskIsRequired, err := currentTask.IsRequired(taskCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to check if task %s is required in module %s: %w", currentTask.Name(), m.Name(), err)
		}
		if !taskIsRequired {
			logger.Info("Skipping task as it's not required", "task_name", currentTask.Name())
			continue
		}

		logger.Info("Planning task", "task_name", currentTask.Name())
		taskFrag, err := currentTask.Plan(taskCtx) // taskFrag is the correct variable name
		if err != nil {
			return nil, fmt.Errorf("failed to plan task %s in module %s: %w", currentTask.Name(), m.Name(), err)
		}

		if taskFrag == nil || len(taskFrag.Nodes) == 0 {
			logger.Info("Task returned an empty fragment, skipping merge", "task_name", currentTask.Name())
			continue
		}

		// Merge nodes
		for id, node := range taskFrag.Nodes {
			if _, exists := moduleFragment.Nodes[id]; exists {
				return nil, fmt.Errorf("duplicate NodeID '%s' from task '%s' in module '%s'", id, currentTask.Name(), m.Name())
			}
			moduleFragment.Nodes[id] = node
		}

		// Link current task's entry nodes to previous task's exit nodes
		if !isFirstEffectiveTask && len(previousTaskExitNodes) > 0 {
			for _, entryNodeID := range taskFrag.EntryNodes {
				if entryNode, ok := moduleFragment.Nodes[entryNodeID]; ok {
					existingDeps := make(map[plan.NodeID]bool)
					for _, dep := range entryNode.Dependencies { existingDeps[dep] = true }
					for _, prevExitNodeID := range previousTaskExitNodes {
						if !existingDeps[prevExitNodeID] {
							entryNode.Dependencies = append(entryNode.Dependencies, prevExitNodeID)
						}
					}
				} else {
                    return nil, fmt.Errorf("entry node ID '%s' from task '%s' not found in module fragment", entryNodeID, currentTask.Name())
                }
			}
		} else if isFirstEffectiveTask { // Only add entry nodes from the very first task that runs
			moduleFragment.EntryNodes = append(moduleFragment.EntryNodes, taskFrag.EntryNodes...)
		}

		if len(taskFrag.ExitNodes) > 0 {
		    previousTaskExitNodes = taskFrag.ExitNodes
		    isFirstEffectiveTask = false
		}
	}

	moduleFragment.ExitNodes = append(moduleFragment.ExitNodes, previousTaskExitNodes...)
	moduleFragment.EntryNodes = plan.UniqueNodeIDs(moduleFragment.EntryNodes)
	moduleFragment.ExitNodes = plan.UniqueNodeIDs(moduleFragment.ExitNodes) // Use helper method on ExecutionFragment

	logger.Info("Containerd module planning complete.", "total_nodes", len(moduleFragment.Nodes))
	return moduleFragment, nil
}

// uniqueNodeIDs helper removed, assuming ExecutionFragment has a RemoveDuplicateNodeIDs method.

// Ensure ContainerdModule implements the module.Module interface.
var _ module.Module = (*ContainerdModule)(nil)
