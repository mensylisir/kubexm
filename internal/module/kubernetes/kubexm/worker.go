package kubexm

import (
	"fmt"

	"github.com/mensylisir/kubexm/internal/module"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/task"
	taskKubexm "github.com/mensylisir/kubexm/internal/task/kubernetes/kubexm"
)

// KubexmWorkerModule is responsible for setting up Kubernetes worker nodes using kubexm binary deployment.
type KubexmWorkerModule struct {
	module.BaseModule
}

// NewKubexmWorkerModule creates a new KubexmWorkerModule.
func NewKubexmWorkerModule() module.Module {
	tasks := []task.Task{
		taskKubexm.NewDeployWorkerNodesTask(),
	}
	base := module.NewBaseModule("KubexmWorker", tasks)
	return &KubexmWorkerModule{BaseModule: base}
}

func (m *KubexmWorkerModule) Plan(ctx runtime.ModuleContext) (*plan.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("module", m.Name())
	moduleFragment := plan.NewExecutionFragment(m.Name() + "-Fragment")

	taskCtx, ok := ctx.(runtime.TaskContext)
	if !ok {
		return nil, fmt.Errorf("module context cannot be asserted to runtime.TaskContext for %s", m.Name())
	}

	// Deploy worker nodes using kubexm binary deployment
	deployWorkersTask := taskKubexm.NewDeployWorkerNodesTask()
	deployRequired, err := deployWorkersTask.IsRequired(taskCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to check IsRequired for %s: %w", deployWorkersTask.Name(), err)
	}
	if deployRequired {
		logger.Info("Planning task", "task_name", deployWorkersTask.Name())
		deployFrag, err := deployWorkersTask.Plan(taskCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to plan %s: %w", deployWorkersTask.Name(), err)
		}
		if err := moduleFragment.MergeFragment(deployFrag); err != nil {
			return nil, err
		}
		moduleFragment.EntryNodes = deployFrag.EntryNodes
		moduleFragment.ExitNodes = deployFrag.ExitNodes
	}

	moduleFragment.EntryNodes = plan.UniqueNodeIDs(moduleFragment.EntryNodes)
	moduleFragment.ExitNodes = plan.UniqueNodeIDs(moduleFragment.ExitNodes)

	if len(moduleFragment.Nodes) == 0 {
		logger.Info("KubexmWorkerModule planned no executable nodes.")
		return plan.NewEmptyFragment(m.Name()), nil
	}

	logger.Info("KubexmWorker module planning complete.", "total_nodes", len(moduleFragment.Nodes))
	return moduleFragment, nil
}

var _ module.Module = (*KubexmWorkerModule)(nil)
