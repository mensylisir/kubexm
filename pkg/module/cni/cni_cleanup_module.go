package cni

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/module"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/task"
	// "github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1" // For CNI type check if needed
	// taskCNI "github.com/mensylisir/kubexm/pkg/task/cni" // For actual tasks later
)

// CNICleanupModule defines the module for cleaning up CNI components.
// This is a generic name; specific CNI cleanup might be in e.g. calico_cleanup_module.go
type CNICleanupModule struct {
	module.BaseModule
}

// NewCNICleanupModule creates a new CNICleanupModule.
// In a real scenario, this might be NewCalicoCleanupModule, NewFlannelCleanupModule etc.
func NewCNICleanupModule() module.Module {
	// TODO: Define actual tasks based on the CNI plugin used by the cluster.
	// - NewDeleteCNIManifestsTask()
	// - NewRemoveCNIConfigsFromNodesTask()
	// - NewCleanupIPAMDataTask()
	moduleTasks := []task.Task{
		// Example: taskCNI.NewDeleteCalicoManifestsTask(),
	}

	base := module.NewBaseModule("CNICleanup", moduleTasks)
	m := &CNICleanupModule{BaseModule: base}
	return m
}

// Plan generates the execution fragment for the CNI Cleanup module.
func (m *CNICleanupModule) Plan(ctx module.ModuleContext) (*task.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("module", m.Name())
	clusterConfig := ctx.GetClusterConfig()

	// Enablement Check: Only run if CNI was configured.
	// Specific logic for which CNI to cleanup would be inside the tasks or a more specific module.
	if clusterConfig.Spec.Network == nil || clusterConfig.Spec.Network.Plugin == "" {
		logger.Info("CNI plugin not specified or network spec missing. Skipping CNI Cleanup module planning.")
		return task.NewEmptyFragment(), nil
	}
	logger.Infof("Planning CNI Cleanup module for plugin: %s (stub implementation)...", clusterConfig.Spec.Network.Plugin)


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
		logger.Info("CNI Cleanup module planned no executable nodes (stub).")
	} else {
		logger.Info("CNI Cleanup module planning complete.", "total_nodes", len(moduleFragment.Nodes))
	}

	return moduleFragment, nil
}

var _ module.Module = (*CNICleanupModule)(nil)
