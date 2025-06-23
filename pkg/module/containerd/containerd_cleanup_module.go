package containerd

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/module"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/task"
	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1" // For runtime type check
	// taskContainerd "github.com/mensylisir/kubexm/pkg/task/containerd" // For actual tasks later
)

// ContainerdCleanupModule defines the module for cleaning up containerd components.
type ContainerdCleanupModule struct {
	module.BaseModule
}

// NewContainerdCleanupModule creates a new ContainerdCleanupModule.
func NewContainerdCleanupModule() module.Module {
	// TODO: Define actual tasks:
	// - NewStopContainerdServiceTask()
	// - NewRemoveContainerdPackagesTask() (if installed via package manager)
	// - NewRemoveContainerdBinariesTask() (if installed manually)
	// - NewRemoveContainerdDataTask() (images, containers, volumes - CAUTION: this is very destructive)
	// - NewRemoveContainerdConfigTask()
	moduleTasks := []task.Task{
		// Example: taskContainerd.NewStopContainerdServiceTask(),
	}

	base := module.NewBaseModule("ContainerdCleanup", moduleTasks)
	m := &ContainerdCleanupModule{BaseModule: base}
	return m
}

// Plan generates the execution fragment for the ContainerdCleanup module.
func (m *ContainerdCleanupModule) Plan(ctx module.ModuleContext) (*task.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("module", m.Name())
	clusterConfig := ctx.GetClusterConfig()

	// Enablement Check: Only run if container runtime was containerd.
	// This cleanup might be part of a more general NodeResetModule,
	// but if it's separate, this check is important.
	if clusterConfig.Spec.ContainerRuntime == nil || clusterConfig.Spec.ContainerRuntime.Type != v1alpha1.ContainerdRuntime {
		logger.Info("Container runtime is not containerd or not specified. Skipping Containerd Cleanup module planning.")
		return task.NewEmptyFragment(), nil
	}
	logger.Info("Planning Containerd Cleanup module (stub implementation)...")


	moduleFragment := task.NewExecutionFragment()
	var previousTaskExitNodes []plan.NodeID
	isFirstEffectiveTask := true

	for _, currentTask := range m.Tasks() {
		taskCtx, ok := ctx.(task.TaskContext)
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
		taskFrag, err := currentTask.Plan(taskCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to plan task %s in module %s: %w", currentTask.Name(), m.Name(), err)
		}

		if taskFrag == nil || len(taskFrag.Nodes) == 0 {
			logger.Info("Task returned an empty fragment, skipping merge", "task_name", currentTask.Name())
			continue
		}

		for id, node := range taskFrag.Nodes {
			if _, exists := moduleFragment.Nodes[id]; exists {
				return nil, fmt.Errorf("duplicate NodeID '%s' from task '%s' in module '%s'", id, currentTask.Name(), m.Name())
			}
			moduleFragment.Nodes[id] = node
		}

		if !isFirstEffectiveTask && len(previousTaskExitNodes) > 0 {
			for _, entryNodeID := range taskFrag.EntryNodes {
				if entryNode, ok := moduleFragment.Nodes[entryNodeID]; ok {
					entryNode.Dependencies = plan.UniqueNodeIDs(append(entryNode.Dependencies, previousTaskExitNodes...))
				} else {
					return nil, fmt.Errorf("entry node ID '%s' from task '%s' not found in module fragment", entryNodeID, currentTask.Name())
				}
			}
		} else if isFirstEffectiveTask {
			moduleFragment.EntryNodes = append(moduleFragment.EntryNodes, taskFrag.EntryNodes...)
		}

		if len(taskFrag.ExitNodes) > 0 {
			previousTaskExitNodes = taskFrag.ExitNodes
			isFirstEffectiveTask = false
		}
	}
	moduleFragment.ExitNodes = append(moduleFragment.ExitNodes, previousTaskExitNodes...)
	moduleFragment.RemoveDuplicateNodeIDs()

	if len(moduleFragment.Nodes) == 0 {
		logger.Info("Containerd Cleanup module planned no executable nodes (stub).")
	} else {
		logger.Info("Containerd Cleanup module planning complete.", "total_nodes", len(moduleFragment.Nodes))
	}

	return moduleFragment, nil
}

var _ module.Module = (*ContainerdCleanupModule)(nil)
