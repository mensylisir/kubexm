package kubernetes

import (
	"fmt"

	"github.com/mensylisir/kubexm/internal/module"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	taskkubernetes "github.com/mensylisir/kubexm/internal/task/kubernetes"
	"github.com/mensylisir/kubexm/internal/task"
)

// HealthModule handles cluster health checks.
type HealthModule struct {
	module.BaseModule
	component string
}

func NewHealthModule(component string) module.Module {
	var tasks []task.Task

	switch component {
	case "all":
		tasks = []task.Task{
			&healthTaskWrapper{taskkubernetes.NewCheckClusterHealthTask()},
			&healthTaskWrapper{taskkubernetes.NewCheckAPIServerHealthTask()},
			&healthTaskWrapper{taskkubernetes.NewCheckSchedulerHealthTask()},
			&healthTaskWrapper{taskkubernetes.NewCheckControllerManagerHealthTask()},
			&healthTaskWrapper{taskkubernetes.NewCheckKubeletHealthTask()},
		}
	case "apiserver":
		tasks = []task.Task{&healthTaskWrapper{taskkubernetes.NewCheckAPIServerHealthTask()}}
	case "scheduler":
		tasks = []task.Task{&healthTaskWrapper{taskkubernetes.NewCheckSchedulerHealthTask()}}
	case "controller-manager":
		tasks = []task.Task{&healthTaskWrapper{taskkubernetes.NewCheckControllerManagerHealthTask()}}
	case "kubelet":
		tasks = []task.Task{&healthTaskWrapper{taskkubernetes.NewCheckKubeletHealthTask()}}
	case "cluster":
		tasks = []task.Task{&healthTaskWrapper{taskkubernetes.NewCheckClusterHealthTask()}}
	default:
		tasks = []task.Task{&healthTaskWrapper{taskkubernetes.NewCheckClusterHealthTask()}}
	}

	return &HealthModule{
		BaseModule: module.NewBaseModule("HealthCheck", tasks),
		component:  component,
	}
}

func (m *HealthModule) Name() string        { return "HealthCheck" }
func (m *HealthModule) Description() string { return fmt.Sprintf("Check health status of cluster components (%s)", m.component) }

func (m *HealthModule) Plan(ctx runtime.ModuleContext) (*plan.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("module", m.Name(), "component", m.component)
	moduleFragment := plan.NewExecutionFragment(m.Name() + "-Fragment")

	taskCtx, ok := ctx.(runtime.TaskContext)
	if !ok {
		return nil, fmt.Errorf("context does not implement runtime.TaskContext")
	}

	var previousTaskExitNodes []plan.NodeID

	for _, t := range m.Tasks() {
		isRequired, err := t.IsRequired(taskCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to check if task %s is required: %w", t.Name(), err)
		}
		if !isRequired {
			logger.Info("Health check task is not required, skipping", "task", t.Name())
			continue
		}

		taskFrag, err := t.Plan(taskCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to plan task %s: %w", t.Name(), err)
		}
		if taskFrag.IsEmpty() {
			continue
		}

		if err := moduleFragment.MergeFragment(taskFrag); err != nil {
			return nil, fmt.Errorf("failed to merge fragment from task %s: %w", t.Name(), err)
		}

		if len(previousTaskExitNodes) > 0 {
			if err := plan.LinkFragments(moduleFragment, previousTaskExitNodes, taskFrag.EntryNodes); err != nil {
				return nil, fmt.Errorf("failed to link fragments for task %s: %w", t.Name(), err)
			}
		}
		previousTaskExitNodes = taskFrag.ExitNodes
	}

	if len(previousTaskExitNodes) == 0 {
		logger.Info("Health module returned empty fragment")
		return plan.NewEmptyFragment(m.Name()), nil
	}

	moduleFragment.CalculateEntryAndExitNodes()
	logger.Info("Health module planning complete.", "total_nodes", len(moduleFragment.Nodes))
	return moduleFragment, nil
}

type healthTaskWrapper struct{ task.Task }

var _ module.Module = (*HealthModule)(nil)
