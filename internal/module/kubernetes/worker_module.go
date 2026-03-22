package kubernetes

import (
	"fmt"

	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/module"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/task"
	taskKube "github.com/mensylisir/kubexm/internal/task/kubernetes/kubeadm"
	taskKubexm "github.com/mensylisir/kubexm/internal/task/kubernetes/kubexm"
)

// WorkerModule is responsible for setting up Kubernetes worker nodes.
type WorkerModule struct {
	module.BaseModule
}

// NewWorkerModule creates a new WorkerModule.
func NewWorkerModule() module.Module {
	// Tasks for worker nodes.
	// Note: InstallKubeComponentsTask might have already run on all nodes
	// as part of ControlPlaneModule or an earlier "all nodes setup" module.
	// If so, their IsRequired methods or Prechecks should make them no-ops on nodes where already done.
	tasks := []task.Task{
		taskKube.NewInstallKubeComponentsTask(), // Ensure binaries on workers
		// taskKube.NewPullImagesTask(nil),          // Ensure core images on workers (e.g. kube-proxy, pause, CNI)
		taskKube.NewJoinWorkersTask(), // The main task for joining workers
	}
	base := module.NewBaseModule("KubernetesWorkerSetup", tasks)
	return &WorkerModule{BaseModule: base}
}

func (m *WorkerModule) Plan(ctx runtime.ModuleContext) (*plan.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("module", m.Name())
	moduleFragment := plan.NewExecutionFragment(m.Name() + "-Fragment")

	taskCtx, ok := ctx.(runtime.TaskContext)
	if !ok {
		return nil, fmt.Errorf("module context cannot be asserted to runtime.TaskContext for %s", m.Name())
	}

	clusterCfg := taskCtx.GetClusterConfig()
	kubeType := clusterCfg.Spec.Kubernetes.Type
	if kubeType == "" {
		kubeType = string(common.KubernetesDeploymentTypeKubeadm)
	}

	logger.Info("Using Kubernetes deployment type", "type", kubeType)

	var previousTaskExitNodes []plan.NodeID
	var err error

	switch common.KubernetesDeploymentType(kubeType) {
	case common.KubernetesDeploymentTypeKubeadm:
		previousTaskExitNodes, err = m.planKubeadmWorkers(taskCtx, moduleFragment)
		if err != nil {
			return nil, err
		}
	case common.KubernetesDeploymentTypeKubexm:
		previousTaskExitNodes, err = m.planKubexmWorkers(taskCtx, moduleFragment)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported Kubernetes deployment type: %s", kubeType)
	}

	moduleFragment.EntryNodes = plan.UniqueNodeIDs(moduleFragment.EntryNodes)
	moduleFragment.ExitNodes = plan.UniqueNodeIDs(previousTaskExitNodes)

	if len(moduleFragment.Nodes) == 0 {
		logger.Info("WorkerModule planned no executable nodes.")
		return plan.NewEmptyFragment(m.Name()), nil
	}

	logger.Info("Worker module planning complete.", "total_nodes", len(moduleFragment.Nodes))
	return moduleFragment, nil
}

func (m *WorkerModule) planKubeadmWorkers(ctx runtime.TaskContext, moduleFragment *plan.ExecutionFragment) ([]plan.NodeID, error) {
	logger := ctx.GetLogger().With("module", m.Name())

	installBinariesTask := taskKube.NewInstallKubeComponentsTask()
	joinWorkersTask := taskKube.NewJoinWorkersTask()

	var lastBinariesExits []plan.NodeID

	// 1. Install Kube Binaries on workers
	binariesRequired, err := installBinariesTask.IsRequired(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to check IsRequired for %s: %w", installBinariesTask.Name(), err)
	}
	if binariesRequired {
		logger.Info("Planning task", "task_name", installBinariesTask.Name())
		binariesFrag, err := installBinariesTask.Plan(ctx)
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
	joinWorkersRequired, err := joinWorkersTask.IsRequired(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to check IsRequired for %s: %w", joinWorkersTask.Name(), err)
	}
	if joinWorkersRequired {
		logger.Info("Planning task", "task_name", joinWorkersTask.Name())
		joinWorkersFrag, err := joinWorkersTask.Plan(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to plan %s: %w", joinWorkersTask.Name(), err)
		}
		if err := moduleFragment.MergeFragment(joinWorkersFrag); err != nil {
			return nil, err
		}
		if len(joinWorkersFrag.EntryNodes) > 0 {
			plan.LinkFragments(moduleFragment, joinDependencies, joinWorkersFrag.EntryNodes)
			return joinWorkersFrag.ExitNodes, nil
		}
	}

	return joinDependencies, nil
}

func (m *WorkerModule) planKubexmWorkers(ctx runtime.TaskContext, moduleFragment *plan.ExecutionFragment) ([]plan.NodeID, error) {
	logger := ctx.GetLogger().With("module", m.Name())

	deployWorkersTask := taskKubexm.NewDeployWorkerNodesTask()

	// Deploy worker nodes using kubexm binary deployment
	deployRequired, err := deployWorkersTask.IsRequired(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to check IsRequired for %s: %w", deployWorkersTask.Name(), err)
	}
	if deployRequired {
		logger.Info("Planning task", "task_name", deployWorkersTask.Name())
		deployFrag, err := deployWorkersTask.Plan(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to plan %s: %w", deployWorkersTask.Name(), err)
		}
		if err := moduleFragment.MergeFragment(deployFrag); err != nil {
			return nil, err
		}
		moduleFragment.EntryNodes = append(moduleFragment.EntryNodes, deployFrag.EntryNodes...)
		return deployFrag.ExitNodes, nil
	}

	return nil, nil
}

var _ module.Module = (*WorkerModule)(nil)
