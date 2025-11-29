package etcd

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/module"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/task"
	// taskEtcd "github.com/mensylisir/kubexm/pkg/task/etcd" // For actual tasks later
)

// EtcdCleanupModule defines the module for cleaning up etcd components.
type EtcdCleanupModule struct {
	module.BaseModule
}

// NewEtcdCleanupModule creates a new EtcdCleanupModule.
func NewEtcdCleanupModule() module.Module {
	// TODO: Define actual tasks:
	// - NewStopEtcdServiceTask()
	// - NewRemoveEtcdDataTask()
	// - NewRemoveEtcdBinariesTask()
	// - NewRemoveEtcdPKITask()
	moduleTasks := []task.Task{
		// Example: taskEtcd.NewRemoveEtcdDataTask(),
	}

	base := module.NewBaseModule("EtcdClusterCleanup", moduleTasks)
	m := &EtcdCleanupModule{BaseModule: base}
	return m
}

// Plan generates the execution fragment for the EtcdCleanup module.
func (m *EtcdCleanupModule) Plan(ctx runtime.ModuleContext) (*plan.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("module", m.Name())
	clusterConfig := ctx.GetClusterConfig()

	if clusterConfig.Spec.Etcd == nil || clusterConfig.Spec.Etcd.External != nil {
		logger.Info("Etcd is external or not specified. Skipping Etcd Cleanup module planning.")
		return plan.NewEmptyFragment(m.Name()), nil
	}
	logger.Info("Planning Etcd Cleanup module (stub implementation)...")


	moduleFragment := plan.NewExecutionFragment(m.Name() + "-Fragment")
	var previousTaskExitNodes []plan.NodeID
	isFirstEffectiveTask := true

	for _, currentTask := range m.Tasks() {
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
	moduleFragment.EntryNodes = plan.UniqueNodeIDs(moduleFragment.EntryNodes)
	moduleFragment.ExitNodes = plan.UniqueNodeIDs(moduleFragment.ExitNodes)

	if len(moduleFragment.Nodes) == 0 {
		logger.Info("Etcd Cleanup module planned no executable nodes (stub).")
	} else {
		logger.Info("Etcd Cleanup module planning complete.", "total_nodes", len(moduleFragment.Nodes))
	}

	return moduleFragment, nil
}

var _ module.Module = (*EtcdCleanupModule)(nil)
