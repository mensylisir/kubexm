package kubeadm

import (
	"fmt"

	"github.com/mensylisir/kubexm/internal/module"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/task"
	taskKube "github.com/mensylisir/kubexm/internal/task/kubernetes/kubeadm"
)

// KubeadmControlPlaneModule is responsible for setting up the Kubernetes control plane using kubeadm.
type KubeadmControlPlaneModule struct {
	module.BaseModule
}

// NewKubeadmControlPlaneModule creates a new KubeadmControlPlaneModule.
func NewKubeadmControlPlaneModule() module.Module {
	tasks := []task.Task{
		taskKube.NewInstallKubeComponentsTask(),
		taskKube.NewBootstrapFirstMasterTask(),
		taskKube.NewJoinMastersTask(),
	}
	base := module.NewBaseModule("KubeadmControlPlane", tasks)
	return &KubeadmControlPlaneModule{BaseModule: base}
}

func (m *KubeadmControlPlaneModule) Plan(ctx runtime.ModuleContext) (*plan.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("module", m.Name())
	moduleFragment := plan.NewExecutionFragment(m.Name() + "-Fragment")

	taskCtx, ok := ctx.(runtime.TaskContext)
	if !ok {
		return nil, fmt.Errorf("module context cannot be asserted to runtime.TaskContext for %s", m.Name())
	}

	var previousTaskExitNodes []plan.NodeID

	// 1. Install Kube Binaries (kubeadm, kubelet, kubectl) - runs on all nodes
	installBinariesTask := taskKube.NewInstallKubeComponentsTask()
	logger.Info("Planning task", "task_name", installBinariesTask.Name())
	binariesFrag, err := installBinariesTask.Plan(taskCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to plan %s: %w", installBinariesTask.Name(), err)
	}
	if err := moduleFragment.MergeFragment(binariesFrag); err != nil {
		return nil, err
	}
	moduleFragment.EntryNodes = append(moduleFragment.EntryNodes, binariesFrag.EntryNodes...)
	previousTaskExitNodes = append(previousTaskExitNodes, binariesFrag.ExitNodes...)
	previousTaskExitNodes = plan.UniqueNodeIDs(previousTaskExitNodes)

	// 2. Init Control Plane (on first master)
	initCPTask := taskKube.NewBootstrapFirstMasterTask()
	initCPRequired, err := initCPTask.IsRequired(taskCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to check IsRequired for %s: %w", initCPTask.Name(), err)
	}
	if initCPRequired {
		logger.Info("Planning task", "task_name", initCPTask.Name())
		initCPFrag, err := initCPTask.Plan(taskCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to plan %s: %w", initCPTask.Name(), err)
		}
		if err := moduleFragment.MergeFragment(initCPFrag); err != nil {
			return nil, err
		}
		if err := plan.LinkFragments(moduleFragment, previousTaskExitNodes, initCPFrag.EntryNodes); err != nil {
			return nil, fmt.Errorf("failed to link init control plane fragment: %w", err)
		}
		previousTaskExitNodes = initCPFrag.ExitNodes
	}

	// 3. Join Other Control Plane Nodes (conditional, on other masters)
	joinCPTask := taskKube.NewJoinMastersTask()
	joinCPRequired, err := joinCPTask.IsRequired(taskCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to check IsRequired for %s: %w", joinCPTask.Name(), err)
	}
	if joinCPRequired {
		logger.Info("Planning task", "task_name", joinCPTask.Name())
		joinCPFrag, err := joinCPTask.Plan(taskCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to plan %s: %w", joinCPTask.Name(), err)
		}
		if err := moduleFragment.MergeFragment(joinCPFrag); err != nil {
			return nil, err
		}
		if err := plan.LinkFragments(moduleFragment, previousTaskExitNodes, joinCPFrag.EntryNodes); err != nil {
			return nil, fmt.Errorf("failed to link join control plane fragment: %w", err)
		}
		previousTaskExitNodes = joinCPFrag.ExitNodes
	}

	moduleFragment.EntryNodes = plan.UniqueNodeIDs(moduleFragment.EntryNodes)
	moduleFragment.ExitNodes = plan.UniqueNodeIDs(previousTaskExitNodes)

	if len(moduleFragment.Nodes) == 0 {
		logger.Info("KubeadmControlPlaneModule planned no executable nodes.")
		return plan.NewEmptyFragment(m.Name()), nil
	}

	logger.Info("KubeadmControlPlane module planning complete.", "total_nodes", len(moduleFragment.Nodes))
	return moduleFragment, nil
}

var _ module.Module = (*KubeadmControlPlaneModule)(nil)
