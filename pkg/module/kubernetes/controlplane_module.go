package kubernetes

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/module"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/task"
	taskKube "github.com/mensylisir/kubexm/pkg/task/kubernetes"
)

// ControlPlaneModule is responsible for setting up the Kubernetes control plane.
type ControlPlaneModule struct {
	module.BaseModule
}

// NewControlPlaneModule creates a new ControlPlaneModule.
func NewControlPlaneModule() module.Module {
	// Define tasks. Actual instances created in Plan if needed.
	// These tasks will be planned sequentially.
	tasks := []task.Task{
		taskKube.NewInstallKubeBinariesTask(nil), // Roles for binaries: all nodes typically
		taskKube.NewPullImagesTask(nil),          // Roles for images: control-plane and workers
		taskKube.NewInitControlPlaneTask(),       // Runs on first master
		taskKube.NewJoinControlPlaneTask(),       // Runs on other masters (conditional)
	}
	base := module.NewBaseModule("KubernetesControlPlaneSetup", tasks)
	return &ControlPlaneModule{BaseModule: base}
}

func (m *ControlPlaneModule) Plan(ctx module.ModuleContext) (*task.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("module", m.Name())
	moduleFragment := task.NewExecutionFragment(m.Name() + "-Fragment")

	taskCtx, ok := ctx.(task.TaskContext)
	if !ok {
		return nil, fmt.Errorf("module context cannot be asserted to task.TaskContext for %s", m.Name())
	}

	var previousTaskExitNodes []plan.NodeID
	isFirstTaskInModule := true

	// Explicitly define task instances to manage their fragments and linking
	installBinariesTask := taskKube.NewInstallKubeBinariesTask(nil) // Roles can be refined if needed
	pullImagesTask := taskKube.NewPullImagesTask(nil)
	initCPTask := taskKube.NewInitControlPlaneTask()
	joinCPTask := taskKube.NewJoinControlPlaneTask()

	// 1. Install Kube Binaries (kubeadm, kubelet, kubectl) - runs on all nodes
	logger.Info("Planning task", "task_name", installBinariesTask.Name())
	binariesFrag, err := installBinariesTask.Plan(taskCtx)
	if err != nil { return nil, fmt.Errorf("failed to plan %s: %w", installBinariesTask.Name(), err) }
	if err := moduleFragment.MergeFragment(binariesFrag); err != nil { return nil, err }
	// This is an entry point for the module
	moduleFragment.EntryNodes = append(moduleFragment.EntryNodes, binariesFrag.EntryNodes...)

	// 2. Pull Core K8s Images - runs on all nodes, can be parallel to binaries install
	logger.Info("Planning task", "task_name", pullImagesTask.Name())
	imagesFrag, err := pullImagesTask.Plan(taskCtx)
	if err != nil { return nil, fmt.Errorf("failed to plan %s: %w", pullImagesTask.Name(), err) }
	if err := moduleFragment.MergeFragment(imagesFrag); err != nil { return nil, err }
	// Also an entry point for the module
	moduleFragment.EntryNodes = append(moduleFragment.EntryNodes, imagesFrag.EntryNodes...)

	// `previousTaskExitNodes` will be the combined exits of binaries and images tasks,
	// as InitControlPlaneTask depends on both being done on the first master.
	previousTaskExitNodes = append(previousTaskExitNodes, binariesFrag.ExitNodes...)
	previousTaskExitNodes = append(previousTaskExitNodes, imagesFrag.ExitNodes...)
	previousTaskExitNodes = plan.UniqueNodeIDs(previousTaskExitNodes)


	// 3. Init Control Plane (on first master)
	initCPRequired, err := initCPTask.IsRequired(taskCtx)
	if err != nil { return nil, fmt.Errorf("failed to check IsRequired for %s: %w", initCPTask.Name(), err) }
	if initCPRequired {
		logger.Info("Planning task", "task_name", initCPTask.Name())
		initCPFrag, err := initCPTask.Plan(taskCtx)
		if err != nil { return nil, fmt.Errorf("failed to plan %s: %w", initCPTask.Name(), err) }
		if err := moduleFragment.MergeFragment(initCPFrag); err != nil { return nil, err }
		plan.LinkFragments(moduleFragment, previousTaskExitNodes, initCPFrag.EntryNodes)
		previousTaskExitNodes = initCPFrag.ExitNodes
	} else {
		logger.Info("Skipping task as it's not required", "task_name", initCPTask.Name())
	}

	// 4. Join Other Control Plane Nodes (conditional, on other masters)
	joinCPRequired, err := joinCPTask.IsRequired(taskCtx)
	if err != nil { return nil, fmt.Errorf("failed to check IsRequired for %s: %w", joinCPTask.Name(), err) }
	if joinCPRequired {
		logger.Info("Planning task", "task_name", joinCPTask.Name())
		joinCPFrag, err := joinCPTask.Plan(taskCtx)
		if err != nil { return nil, fmt.Errorf("failed to plan %s: %w", joinCPTask.Name(), err) }
		if err := moduleFragment.MergeFragment(joinCPFrag); err != nil { return nil, err }
		plan.LinkFragments(moduleFragment, previousTaskExitNodes, joinCPFrag.EntryNodes)
		previousTaskExitNodes = joinCPFrag.ExitNodes
	} else {
		logger.Info("Skipping task as it's not required", "task_name", joinCPTask.Name())
	}

	moduleFragment.EntryNodes = task.UniqueNodeIDs(moduleFragment.EntryNodes)
	moduleFragment.ExitNodes = task.UniqueNodeIDs(previousTaskExitNodes)

	if len(moduleFragment.Nodes) == 0 {
		logger.Info("ControlPlaneModule planned no executable nodes.")
		return task.NewEmptyFragment(), nil
	}

	logger.Info("ControlPlane module planning complete.", "total_nodes", len(moduleFragment.Nodes))
	return moduleFragment, nil
}

var _ module.Module = (*ControlPlaneModule)(nil)
