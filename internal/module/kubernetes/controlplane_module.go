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

// ControlPlaneModule is responsible for setting up the Kubernetes control plane.
type ControlPlaneModule struct {
	module.BaseModule
}

// NewControlPlaneModule creates a new ControlPlaneModule.
func NewControlPlaneModule() module.Module {
	// Define tasks. Actual instances created in Plan if needed.
	// These tasks will be planned sequentially.
	tasks := []task.Task{
		taskKube.NewInstallKubeComponentsTask(), // Roles for binaries: all nodes typically
		// taskKube.NewPullImagesTask(nil),          // Roles for images: control-plane and workers
		taskKube.NewBootstrapFirstMasterTask(), // Runs on first master
		taskKube.NewJoinMastersTask(),          // Runs on other masters (conditional)
	}
	base := module.NewBaseModule("KubernetesControlPlaneSetup", tasks)
	return &ControlPlaneModule{BaseModule: base}
}

func (m *ControlPlaneModule) Plan(ctx runtime.ModuleContext) (*plan.ExecutionFragment, error) {
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

	// Determine which tasks to use based on Kubernetes deployment type
	switch common.KubernetesDeploymentType(kubeType) {
	case common.KubernetesDeploymentTypeKubeadm:
		previousTaskExitNodes, err = m.planKubeadmControlPlane(taskCtx, moduleFragment)
		if err != nil {
			return nil, err
		}
	case common.KubernetesDeploymentTypeKubexm:
		previousTaskExitNodes, err = m.planKubexmControlPlane(taskCtx, moduleFragment)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported Kubernetes deployment type: %s", kubeType)
	}

	moduleFragment.EntryNodes = plan.UniqueNodeIDs(moduleFragment.EntryNodes)
	moduleFragment.ExitNodes = plan.UniqueNodeIDs(previousTaskExitNodes)

	if len(moduleFragment.Nodes) == 0 {
		logger.Info("ControlPlaneModule planned no executable nodes.")
		return plan.NewEmptyFragment(m.Name()), nil
	}

	logger.Info("ControlPlane module planning complete.", "total_nodes", len(moduleFragment.Nodes))
	return moduleFragment, nil
}

func (m *ControlPlaneModule) planKubeadmControlPlane(ctx runtime.TaskContext, moduleFragment *plan.ExecutionFragment) ([]plan.NodeID, error) {
	logger := ctx.GetLogger().With("module", m.Name())
	var previousTaskExitNodes []plan.NodeID

	// Explicitly define task instances to manage their fragments and linking
	installBinariesTask := taskKube.NewInstallKubeComponentsTask()
	initCPTask := taskKube.NewBootstrapFirstMasterTask()
	joinCPTask := taskKube.NewJoinMastersTask()

	// 1. Install Kube Binaries (kubeadm, kubelet, kubectl) - runs on all nodes
	logger.Info("Planning task", "task_name", installBinariesTask.Name())
	binariesFrag, err := installBinariesTask.Plan(ctx)
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
	initCPRequired, err := initCPTask.IsRequired(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to check IsRequired for %s: %w", initCPTask.Name(), err)
	}
	if initCPRequired {
		logger.Info("Planning task", "task_name", initCPTask.Name())
		initCPFrag, err := initCPTask.Plan(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to plan %s: %w", initCPTask.Name(), err)
		}
		if err := moduleFragment.MergeFragment(initCPFrag); err != nil {
			return nil, err
		}
		plan.LinkFragments(moduleFragment, previousTaskExitNodes, initCPFrag.EntryNodes)
		previousTaskExitNodes = initCPFrag.ExitNodes
	} else {
		logger.Info("Skipping task as it's not required", "task_name", initCPTask.Name())
	}

	// 3. Join Other Control Plane Nodes (conditional, on other masters)
	joinCPRequired, err := joinCPTask.IsRequired(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to check IsRequired for %s: %w", joinCPTask.Name(), err)
	}
	if joinCPRequired {
		logger.Info("Planning task", "task_name", joinCPTask.Name())
		joinCPFrag, err := joinCPTask.Plan(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to plan %s: %w", joinCPTask.Name(), err)
		}
		if err := moduleFragment.MergeFragment(joinCPFrag); err != nil {
			return nil, err
		}
		plan.LinkFragments(moduleFragment, previousTaskExitNodes, joinCPFrag.EntryNodes)
		previousTaskExitNodes = joinCPFrag.ExitNodes
	} else {
		logger.Info("Skipping task as it's not required", "task_name", joinCPTask.Name())
	}

	return previousTaskExitNodes, nil
}

func (m *ControlPlaneModule) planKubexmControlPlane(ctx runtime.TaskContext, moduleFragment *plan.ExecutionFragment) ([]plan.NodeID, error) {
	logger := ctx.GetLogger().With("module", m.Name())
	var previousTaskExitNodes []plan.NodeID

	// For kubexm type, use binary deployment tasks
	configureCPTask := taskKubexm.NewConfigureControlPlaneTask()
	startCPTask := taskKubexm.NewStartControlPlaneTask()
	generateAdminKubeconfig := taskKubexm.NewGenerateAdminKubeconfigTask()

	// 1. Configure Control Plane components
	configureCPRequired, err := configureCPTask.IsRequired(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to check IsRequired for %s: %w", configureCPTask.Name(), err)
	}
	if configureCPRequired {
		logger.Info("Planning task", "task_name", configureCPTask.Name())
		configureCPFrag, err := configureCPTask.Plan(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to plan %s: %w", configureCPTask.Name(), err)
		}
		if err := moduleFragment.MergeFragment(configureCPFrag); err != nil {
			return nil, err
		}
		moduleFragment.EntryNodes = append(moduleFragment.EntryNodes, configureCPFrag.EntryNodes...)
		previousTaskExitNodes = configureCPFrag.ExitNodes
	}

	// 2. Start Control Plane components
	startCPRequired, err := startCPTask.IsRequired(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to check IsRequired for %s: %w", startCPTask.Name(), err)
	}
	if startCPRequired {
		logger.Info("Planning task", "task_name", startCPTask.Name())
		startCPFrag, err := startCPTask.Plan(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to plan %s: %w", startCPTask.Name(), err)
		}
		if err := moduleFragment.MergeFragment(startCPFrag); err != nil {
			return nil, err
		}
		if len(previousTaskExitNodes) > 0 {
			plan.LinkFragments(moduleFragment, previousTaskExitNodes, startCPFrag.EntryNodes)
		}
		previousTaskExitNodes = startCPFrag.ExitNodes
	}

	// 3. Generate admin kubeconfig
	generateKubeconfigRequired, err := generateAdminKubeconfig.IsRequired(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to check IsRequired for %s: %w", generateAdminKubeconfig.Name(), err)
	}
	if generateKubeconfigRequired {
		logger.Info("Planning task", "task_name", generateAdminKubeconfig.Name())
		generateKubeconfigFrag, err := generateAdminKubeconfig.Plan(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to plan %s: %w", generateAdminKubeconfig.Name(), err)
		}
		if err := moduleFragment.MergeFragment(generateKubeconfigFrag); err != nil {
			return nil, err
		}
		if len(previousTaskExitNodes) > 0 {
			plan.LinkFragments(moduleFragment, previousTaskExitNodes, generateKubeconfigFrag.EntryNodes)
		}
		previousTaskExitNodes = generateKubeconfigFrag.ExitNodes
	}

	return previousTaskExitNodes, nil
}

var _ module.Module = (*ControlPlaneModule)(nil)
