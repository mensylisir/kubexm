package cni

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/module"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/task"
)

// CalicoModule defines the module for installing Calico CNI.
type CalicoModule struct {
	module.BaseModule
}

// NewCalicoModule creates a new CalicoModule.
func NewCalicoModule() module.Module {
	// TODO: Define actual tasks:
	// - NewInstallCalicoOperatorTask() or NewApplyCalicoManifestsTask()
	// - NewConfigureCalicoTask() (if any specific config is needed post-install)
	moduleTasks := []task.Task{
		// Example: taskCNI.NewInstallCalicoTask(),
	}

	base := module.NewBaseModule("CNICalicoSetup", moduleTasks)
	m := &CalicoModule{BaseModule: base}
	return m
}

// Plan generates the execution fragment for the Calico CNI module.
func (m *CalicoModule) Plan(ctx runtime.ModuleContext) (*plan.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("module", m.Name())
	clusterConfig := ctx.GetClusterConfig()

	// Enablement Check: Only run if CNI is Calico
	if clusterConfig.Spec.Network == nil || clusterConfig.Spec.Network.Plugin != string(common.CNITypeCalico) {
		logger.Info("CNI plugin is not Calico, or CNI not specified. Skipping Calico module planning.", "configured_plugin", clusterConfig.Spec.Network.Plugin)
		return plan.NewEmptyFragment(m.Name()), nil
	}

	logger.Info("Planning Calico CNI module (stub implementation)...")

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

		if err := moduleFragment.MergeFragment(taskFrag); err != nil {
			return nil, err
		}

		if !isFirstEffectiveTask && len(previousTaskExitNodes) > 0 {
			plan.LinkFragments(moduleFragment, previousTaskExitNodes, taskFrag.EntryNodes)
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
		logger.Info("Calico CNI module planned no executable nodes (stub).")
	} else {
		logger.Info("Calico CNI module planning complete.", "total_nodes", len(moduleFragment.Nodes))
	}

	return moduleFragment, nil
}

var _ module.Module = (*CalicoModule)(nil)
