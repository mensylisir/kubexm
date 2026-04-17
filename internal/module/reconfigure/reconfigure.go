package reconfigure

import (
	"fmt"

	"github.com/mensylisir/kubexm/internal/module"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/task"
	reconfiguretask "github.com/mensylisir/kubexm/internal/task/reconfigure"
)

// ReconfigureModule handles cluster component reconfiguration.
type ReconfigureModule struct {
	module.BaseModule
	component string
}

func NewReconfigureModule(component string) module.Module {
	var tasks []task.Task

	switch component {
	case "all":
		tasks = []task.Task{
			&reconfigureTaskWrapper{reconfiguretask.NewReconfigureAPIServerTask()},
			&reconfigureTaskWrapper{reconfiguretask.NewReconfigureSchedulerTask()},
			&reconfigureTaskWrapper{reconfiguretask.NewReconfigureControllerManagerTask()},
			&reconfigureTaskWrapper{reconfiguretask.NewReconfigureKubeletTask()},
			&reconfigureTaskWrapper{reconfiguretask.NewReconfigureProxyTask()},
		}
	case "apiserver":
		tasks = []task.Task{&reconfigureTaskWrapper{reconfiguretask.NewReconfigureAPIServerTask()}}
	case "scheduler":
		tasks = []task.Task{&reconfigureTaskWrapper{reconfiguretask.NewReconfigureSchedulerTask()}}
	case "controller-manager":
		tasks = []task.Task{&reconfigureTaskWrapper{reconfiguretask.NewReconfigureControllerManagerTask()}}
	case "kubelet":
		tasks = []task.Task{&reconfigureTaskWrapper{reconfiguretask.NewReconfigureKubeletTask()}}
	case "proxy":
		tasks = []task.Task{&reconfigureTaskWrapper{reconfiguretask.NewReconfigureProxyTask()}}
	case "network":
		// Network reconfiguration handled separately
		tasks = []task.Task{}
	default:
		tasks = []task.Task{
			&reconfigureTaskWrapper{reconfiguretask.NewReconfigureAPIServerTask()},
			&reconfigureTaskWrapper{reconfiguretask.NewReconfigureSchedulerTask()},
			&reconfigureTaskWrapper{reconfiguretask.NewReconfigureControllerManagerTask()},
			&reconfigureTaskWrapper{reconfiguretask.NewReconfigureKubeletTask()},
			&reconfigureTaskWrapper{reconfiguretask.NewReconfigureProxyTask()},
		}
	}

	return &ReconfigureModule{
		BaseModule: module.NewBaseModule("Reconfigure", tasks),
		component:  component,
	}
}

func (m *ReconfigureModule) Name() string        { return "Reconfigure" }
func (m *ReconfigureModule) Description() string { return fmt.Sprintf("Reconfigure cluster components (%s)", m.component) }

func (m *ReconfigureModule) Plan(ctx runtime.ModuleContext) (*plan.ExecutionFragment, error) {
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
			logger.Info("Reconfigure task is not required, skipping", "task", t.Name())
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
		logger.Info("Reconfigure module returned empty fragment")
		return plan.NewEmptyFragment(m.Name()), nil
	}

	moduleFragment.CalculateEntryAndExitNodes()
	logger.Info("Reconfigure module planning complete.", "total_nodes", len(moduleFragment.Nodes))
	return moduleFragment, nil
}

type reconfigureTaskWrapper struct{ task.Task }

var _ module.Module = (*ReconfigureModule)(nil)
