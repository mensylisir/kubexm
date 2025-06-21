package containerd

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/module"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/task"
	taskContainerd "github.com/mensylisir/kubexm/pkg/task/containerd"
)

// ContainerdModule installs and configures containerd.
type ContainerdModule struct {
	module.BaseModule
	// cfg *v1alpha1.Cluster // Stored if needed by Plan for IsEnabled logic, or tasks constructed in Plan
}

// NewContainerdModule creates a new module for containerd.
func NewContainerdModule() module.Module { // Removed *v1alpha1.Cluster from params
	// Define roles where containerd should be installed (typically all nodes that run containers)
	containerdRoles := []string{common.MasterRole, common.WorkerRole}

	// Instantiate tasks. NewInstallContainerdTask now only takes roles.
	// Specific configurations (version, mirrors, etc.) are pulled by the task from ClusterConfig via TaskContext.
	installTask := taskContainerd.NewInstallContainerdTask(containerdRoles)

	modTasks := []task.Task{installTask}
	base := module.NewBaseModule("ContainerdRuntime", modTasks)
	m := &ContainerdModule{BaseModule: base}
	return m
}

// Plan generates the execution fragment for the containerd module.
func (m *ContainerdModule) Plan(ctx runtime.ModuleContext) (*task.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("module", m.Name())
	clusterConfig := ctx.GetClusterConfig()

	// Enablement Check
	if clusterConfig.Spec.ContainerRuntime == nil || clusterConfig.Spec.ContainerRuntime.Type != v1alpha1.ContainerdRuntime { // Use v1alpha1 constant
		logger.Info("Containerd module is not enabled (ContainerRuntime.Type is not 'containerd'). Skipping.")
		return task.NewEmptyFragment(), nil
	}

	moduleFragment := task.NewExecutionFragment() // Initialize correctly

	var previousTaskExitNodes []plan.NodeID
	isFirstEffectiveTask := true

	for _, currentTask := range m.Tasks() { // m.Tasks() from BaseModule
		taskCtx, ok := ctx.(runtime.TaskContext)
		if !ok {
			return nil, fmt.Errorf("module context cannot be asserted to task context for module %s, task %s", m.Name(), currentTask.Name())
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
	moduleFragment.RemoveDuplicateNodeIDs() // Use helper method on ExecutionFragment

	logger.Info("Containerd module planning complete.", "total_nodes", len(moduleFragment.Nodes))
	return moduleFragment, nil
}

// uniqueNodeIDs helper removed, assuming ExecutionFragment has a RemoveDuplicateNodeIDs method.

// Ensure ContainerdModule implements the module.Module interface.
var _ module.Module = (*ContainerdModule)(nil)
