package kubernetes

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/module"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/task"
	// taskK8s "github.com/mensylisir/kubexm/pkg/task/kubernetes" // For actual tasks later
)

// KubeletModule defines the module for setting up Kubelet on nodes.
type KubeletModule struct {
	module.BaseModule
}

// NewKubeletModule creates a new KubeletModule.
func NewKubeletModule() module.Module {
	// TODO: Define actual tasks:
	// - NewInstallKubeletTask() (on all nodes or master/worker)
	// - NewConfigureKubeletTask() (including joining cluster, certificates)
	// - NewManageKubeletServiceTask()
	moduleTasks := []task.Task{
		// Example: taskK8s.NewInstallKubeletTask(),
	}

	base := module.NewBaseModule("KubernetesKubeletSetup", moduleTasks)
	m := &KubeletModule{BaseModule: base}
	return m
}

// Plan generates the execution fragment for the Kubelet module.
func (m *KubeletModule) Plan(ctx module.ModuleContext) (*task.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("module", m.Name())
	// clusterConfig := ctx.GetClusterConfig() // Available for checks if needed

	logger.Info("Planning Kubelet Setup module (stub implementation)...")

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
		logger.Info("Kubelet Setup module planned no executable nodes (stub).")
	} else {
		logger.Info("Kubelet Setup module planning complete.", "total_nodes", len(moduleFragment.Nodes))
	}

	return moduleFragment, nil
}

var _ module.Module = (*KubeletModule)(nil)
