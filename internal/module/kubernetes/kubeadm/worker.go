package kubeadm

import (
	"fmt"

	"github.com/mensylisir/kubexm/internal/module"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/task"
	taskKube "github.com/mensylisir/kubexm/internal/task/kubernetes/kubeadm"
)

// KubeadmWorkerModule is responsible for setting up Kubernetes worker nodes using kubeadm.
type KubeadmWorkerModule struct {
	module.BaseModule
}

// NewKubeadmWorkerModule creates a new KubeadmWorkerModule.
func NewKubeadmWorkerModule() module.Module {
	tasks := []task.Task{
		taskKube.NewInstallKubeComponentsTask(),
		taskKube.NewJoinWorkersTask(),
	}
	base := module.NewBaseModule("KubeadmWorker", tasks)
	return &KubeadmWorkerModule{BaseModule: base}
}

func (m *KubeadmWorkerModule) Plan(ctx runtime.ModuleContext) (*plan.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("module", m.Name())
	moduleFragment := plan.NewExecutionFragment(m.Name() + "-Fragment")

	taskCtx, ok := ctx.(runtime.TaskContext)
	if !ok {
		return nil, fmt.Errorf("module context cannot be asserted to runtime.TaskContext for %s", m.Name())
	}

	var lastBinariesExits []plan.NodeID

	// 1. Install Kube Binaries on workers
	installBinariesTask := taskKube.NewInstallKubeComponentsTask()
	binariesRequired, err := installBinariesTask.IsRequired(taskCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to check IsRequired for %s: %w", installBinariesTask.Name(), err)
	}
	if binariesRequired {
		logger.Info("Planning task", "task_name", installBinariesTask.Name())
		binariesFrag, err := installBinariesTask.Plan(taskCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to plan %s: %w", installBinariesTask.Name(), err)
		}
		if err := moduleFragment.MergeFragment(binariesFrag); err != nil {
			return nil, err
		}
		moduleFragment.EntryNodes = append(moduleFragment.EntryNodes, binariesFrag.EntryNodes...)
		lastBinariesExits = binariesFrag.ExitNodes
	}

	joinDependencies := plan.UniqueNodeIDs(lastBinariesExits)

	// 2. Join Worker Nodes
	joinWorkersTask := taskKube.NewJoinWorkersTask()
	joinWorkersRequired, err := joinWorkersTask.IsRequired(taskCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to check IsRequired for %s: %w", joinWorkersTask.Name(), err)
	}
	if joinWorkersRequired {
		logger.Info("Planning task", "task_name", joinWorkersTask.Name())
		joinWorkersFrag, err := joinWorkersTask.Plan(taskCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to plan %s: %w", joinWorkersTask.Name(), err)
		}
		if err := moduleFragment.MergeFragment(joinWorkersFrag); err != nil {
			return nil, err
		}
		if len(joinWorkersFrag.EntryNodes) > 0 {
			if err := plan.LinkFragments(moduleFragment, joinDependencies, joinWorkersFrag.EntryNodes); err != nil {
				return nil, fmt.Errorf("failed to link join workers fragment: %w", err)
			}
		}
		moduleFragment.ExitNodes = joinWorkersFrag.ExitNodes
	} else {
		moduleFragment.ExitNodes = joinDependencies
	}

	moduleFragment.EntryNodes = plan.UniqueNodeIDs(moduleFragment.EntryNodes)
	moduleFragment.ExitNodes = plan.UniqueNodeIDs(moduleFragment.ExitNodes)

	if len(moduleFragment.Nodes) == 0 {
		logger.Info("KubeadmWorkerModule planned no executable nodes.")
		return plan.NewEmptyFragment(m.Name()), nil
	}

	logger.Info("KubeadmWorker module planning complete.", "total_nodes", len(moduleFragment.Nodes))
	return moduleFragment, nil
}

var _ module.Module = (*KubeadmWorkerModule)(nil)
