package kubernetes

import (
	"fmt"

	"github.com/mensylisir/kubexm/internal/module"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	taskKubeadm "github.com/mensylisir/kubexm/internal/task/kubernetes/kubeadm"
)

type WorkerCleanupModule struct {
	module.BaseModule
}

func NewWorkerCleanupModule() module.Module {
	return &WorkerCleanupModule{
		BaseModule: module.NewBaseModule("WorkerCleanup", nil),
	}
}

func (m *WorkerCleanupModule) Plan(ctx runtime.ModuleContext) (*plan.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("module", m.Name())

	taskCtx, ok := ctx.(runtime.TaskContext)
	if !ok {
		return nil, fmt.Errorf("context does not implement runtime.TaskContext")
	}

	drainTask := taskKubeadm.NewDrainNodeTask()
	drainFrag, err := m.PlanSingleTask(taskCtx, drainTask)
	if err != nil {
		return nil, fmt.Errorf("failed to plan drain task: %w", err)
	}

	resetTask := taskKubeadm.NewResetNodeTask()
	resetFrag, err := m.PlanSingleTask(taskCtx, resetTask)
	if err != nil {
		return nil, fmt.Errorf("failed to plan reset task: %w", err)
	}

	moduleFragment := plan.NewExecutionFragment(m.Name() + "-Fragment")
	if err := moduleFragment.MergeFragment(drainFrag); err != nil {
		return nil, fmt.Errorf("failed to merge drain fragment: %w", err)
	}
	if err := moduleFragment.MergeFragment(resetFrag); err != nil {
		return nil, fmt.Errorf("failed to merge reset fragment: %w", err)
	}

	// Drain must complete before reset begins
	if err := plan.LinkFragments(moduleFragment, drainFrag.ExitNodes, resetFrag.EntryNodes); err != nil {
		return nil, fmt.Errorf("failed to link drain and reset fragments: %w", err)
	}

	moduleFragment.CalculateEntryAndExitNodes()
	logger.Info("WorkerCleanup module planning complete", "total_nodes", len(moduleFragment.Nodes))
	return moduleFragment, nil
}

var _ module.Module = (*WorkerCleanupModule)(nil)
