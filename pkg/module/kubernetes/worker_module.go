package kubernetes

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/module"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/task"
	taskKube "github.com/mensylisir/kubexm/pkg/task/kubernetes/kubeadm"
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

	// Define task instances
	installBinariesTask := taskKube.NewInstallKubeComponentsTask()
	// pullImagesTask := taskKube.NewPullImagesTask([]string{common.RoleWorker})
	joinWorkersTask := taskKube.NewJoinWorkersTask()

	var lastBinariesExits []plan.NodeID

	// 1. Install Kube Binaries on workers (might be a no-op if already done)
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

	// 2. Pull Core K8s Images on workers (might be a no-op)
	// imagesRequired, err := pullImagesTask.IsRequired(taskCtx)
	// if err != nil { return nil, fmt.Errorf("failed to check IsRequired for %s: %w", pullImagesTask.Name(), err) }
	// if imagesRequired {
	// 	logger.Info("Planning task", "task_name", pullImagesTask.Name())
	// 	imagesFrag, err := pullImagesTask.Plan(taskCtx)
	// 	if err != nil { return nil, fmt.Errorf("failed to plan %s: %w", pullImagesTask.Name(), err) }
	// 	if err := moduleFragment.MergeFragment(imagesFrag); err != nil { return nil, err }
	// 	moduleFragment.EntryNodes = append(moduleFragment.EntryNodes, imagesFrag.EntryNodes...)
	// 	lastImagesExits = imagesFrag.ExitNodes
	// }

	// Combine exits from binaries and images tasks as dependencies for joining
	joinDependencies := append([]plan.NodeID{}, lastBinariesExits...)
	// joinDependencies = append(joinDependencies, lastImagesExits...)
	joinDependencies = plan.UniqueNodeIDs(joinDependencies)

	// 3. Join Worker Nodes
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
		if len(joinWorkersFrag.EntryNodes) > 0 { // Only link if join task has entry nodes
			plan.LinkFragments(moduleFragment, joinDependencies, joinWorkersFrag.EntryNodes)
			moduleFragment.ExitNodes = append(moduleFragment.ExitNodes, joinWorkersFrag.ExitNodes...)
		} else if len(joinDependencies) > 0 { // If join task is empty but had dependencies, those exits are module exits
			moduleFragment.ExitNodes = append(moduleFragment.ExitNodes, joinDependencies...)
		}
	} else {
		// If join worker task is not required, the exits are from image/binary tasks
		moduleFragment.ExitNodes = append(moduleFragment.ExitNodes, joinDependencies...)
		logger.Info("Skipping task as it's not required", "task_name", joinWorkersTask.Name())
	}

	moduleFragment.EntryNodes = plan.UniqueNodeIDs(moduleFragment.EntryNodes)
	moduleFragment.ExitNodes = plan.UniqueNodeIDs(moduleFragment.ExitNodes)

	if len(moduleFragment.Nodes) == 0 {
		logger.Info("WorkerModule planned no executable nodes.")
		return plan.NewEmptyFragment(m.Name()), nil
	}

	logger.Info("Worker module planning complete.", "total_nodes", len(moduleFragment.Nodes))
	return moduleFragment, nil
}

var _ module.Module = (*WorkerModule)(nil)
