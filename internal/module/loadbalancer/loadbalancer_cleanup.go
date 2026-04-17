package loadbalancer

import (
	"fmt"

	"github.com/mensylisir/kubexm/internal/module"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/task"
	"github.com/mensylisir/kubexm/internal/task/loadbalancer"
)

type LoadBalancerCleanupModule struct {
	module.BaseModule
}

func NewLoadBalancerCleanupModule() module.Module {
	return &LoadBalancerCleanupModule{
		BaseModule: module.NewBaseModule("LoadBalancerCleanup", []task.Task{}),
	}
}

// Tasks returns empty slice since tasks are built dynamically in Plan()
func (m *LoadBalancerCleanupModule) Tasks() []task.Task {
	return []task.Task{}
}

func (m *LoadBalancerCleanupModule) Plan(ctx runtime.ModuleContext) (*plan.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("module", m.Name())
	moduleFragment := plan.NewExecutionFragment(m.Name() + "-Fragment")

	taskCtx, ok := ctx.(runtime.TaskContext)
	if !ok {
		return nil, fmt.Errorf("context does not implement runtime.TaskContext")
	}

	cleanupTasks := loadbalancer.GetLoadBalancerCleanupTasks(taskCtx)

	var previousTaskExitNodes []plan.NodeID
	for _, cleanupTask := range cleanupTasks {
		taskFrag, err := cleanupTask.Plan(taskCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to plan load balancer cleanup task %s: %w", cleanupTask.Name(), err)
		}
		if taskFrag.IsEmpty() {
			continue
		}
		if err := moduleFragment.MergeFragment(taskFrag); err != nil {
			return nil, err
		}
		if len(previousTaskExitNodes) > 0 {
			if err := plan.LinkFragments(moduleFragment, previousTaskExitNodes, taskFrag.EntryNodes); err != nil {
				return nil, fmt.Errorf("failed to link fragments for task %s: %w", cleanupTask.Name(), err)
			}
		}
		previousTaskExitNodes = taskFrag.ExitNodes
	}

	if len(moduleFragment.Nodes) == 0 {
		logger.Info("LoadBalancerCleanup module returned empty fragment")
		return plan.NewEmptyFragment(m.Name()), nil
	}

	moduleFragment.CalculateEntryAndExitNodes()
	logger.Info("LoadBalancerCleanup module planning complete", "total_nodes", len(moduleFragment.Nodes))
	return moduleFragment, nil
}

var _ module.Module = (*LoadBalancerCleanupModule)(nil)
