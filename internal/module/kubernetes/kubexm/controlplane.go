package kubexm

import (
	"fmt"

	"github.com/mensylisir/kubexm/internal/module"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/task"
	taskKubexm "github.com/mensylisir/kubexm/internal/task/kubernetes/kubexm"
)

// KubexmControlPlaneModule is responsible for setting up the Kubernetes control plane using kubexm binary deployment.
type KubexmControlPlaneModule struct {
	module.BaseModule
}

// NewKubexmControlPlaneModule creates a new KubexmControlPlaneModule.
func NewKubexmControlPlaneModule() module.Module {
	tasks := []task.Task{
		taskKubexm.NewGenerateKubePKITask(),               // Generate CA and certificates
		taskKubexm.NewGenerateControlPlaneKubeconfigsTask(), // Generate kubeconfigs for control plane components
		taskKubexm.NewGenerateNodePKITask(),               // Generate node-level certificates (kubelet, kube-proxy)
		taskKubexm.NewConfigureControlPlaneTask(),         // Generate config files and systemd services
		taskKubexm.NewStartControlPlaneTask(),             // Enable, start, and health-check services
		taskKubexm.NewGenerateAdminKubeconfigTask(),       // Generate admin.conf kubeconfig
		taskKubexm.NewJoinMastersTask(),                   // Join additional master nodes (if any)
	}
	base := module.NewBaseModule("KubexmControlPlane", tasks)
	return &KubexmControlPlaneModule{BaseModule: base}
}

func (m *KubexmControlPlaneModule) Plan(ctx runtime.ModuleContext) (*plan.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("module", m.Name())
	moduleFragment := plan.NewExecutionFragment(m.Name() + "-Fragment")

	taskCtx, ok := ctx.(runtime.TaskContext)
	if !ok {
		return nil, fmt.Errorf("module context cannot be asserted to runtime.TaskContext for %s", m.Name())
	}

	var previousTaskExitNodes []plan.NodeID

	// 1. Generate PKI (CA and certificates)
	generatePKITask := taskKubexm.NewGenerateKubePKITask()
	generatePKIRequired, err := generatePKITask.IsRequired(taskCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to check IsRequired for %s: %w", generatePKITask.Name(), err)
	}
	if generatePKIRequired {
		logger.Info("Planning task", "task_name", generatePKITask.Name())
		generatePKIFrag, err := generatePKITask.Plan(taskCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to plan %s: %w", generatePKITask.Name(), err)
		}
		if err := moduleFragment.MergeFragment(generatePKIFrag); err != nil {
			return nil, err
		}
		moduleFragment.EntryNodes = append(moduleFragment.EntryNodes, generatePKIFrag.EntryNodes...)
		previousTaskExitNodes = generatePKIFrag.ExitNodes
	}

	// 2. Generate Controlplane Kubeconfigs
	generateKubeconfigsTask := taskKubexm.NewGenerateControlPlaneKubeconfigsTask()
	generateKubeconfigsRequired, err := generateKubeconfigsTask.IsRequired(taskCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to check IsRequired for %s: %w", generateKubeconfigsTask.Name(), err)
	}
	if generateKubeconfigsRequired {
		logger.Info("Planning task", "task_name", generateKubeconfigsTask.Name())
		generateKubeconfigsFrag, err := generateKubeconfigsTask.Plan(taskCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to plan %s: %w", generateKubeconfigsTask.Name(), err)
		}
		if err := moduleFragment.MergeFragment(generateKubeconfigsFrag); err != nil {
			return nil, err
		}
		if len(previousTaskExitNodes) > 0 {
			if err := plan.LinkFragments(moduleFragment, previousTaskExitNodes, generateKubeconfigsFrag.EntryNodes); err != nil {
				return nil, fmt.Errorf("failed to link kubeconfigs fragment: %w", err)
			}
		}
		previousTaskExitNodes = generateKubeconfigsFrag.ExitNodes
	}

	// 3. Configure Control Plane components
	configureCPTask := taskKubexm.NewConfigureControlPlaneTask()
	configureCPRequired, err := configureCPTask.IsRequired(taskCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to check IsRequired for %s: %w", configureCPTask.Name(), err)
	}
	if configureCPRequired {
		logger.Info("Planning task", "task_name", configureCPTask.Name())
		configureCPFrag, err := configureCPTask.Plan(taskCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to plan %s: %w", configureCPTask.Name(), err)
		}
		if err := moduleFragment.MergeFragment(configureCPFrag); err != nil {
			return nil, err
		}
		moduleFragment.EntryNodes = append(moduleFragment.EntryNodes, configureCPFrag.EntryNodes...)
		previousTaskExitNodes = configureCPFrag.ExitNodes
	}

	// 4. Start Control Plane components
	startCPTask := taskKubexm.NewStartControlPlaneTask()
	startCPRequired, err := startCPTask.IsRequired(taskCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to check IsRequired for %s: %w", startCPTask.Name(), err)
	}
	if startCPRequired {
		logger.Info("Planning task", "task_name", startCPTask.Name())
		startCPFrag, err := startCPTask.Plan(taskCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to plan %s: %w", startCPTask.Name(), err)
		}
		if err := moduleFragment.MergeFragment(startCPFrag); err != nil {
			return nil, err
		}
		if len(previousTaskExitNodes) > 0 {
			if err := plan.LinkFragments(moduleFragment, previousTaskExitNodes, startCPFrag.EntryNodes); err != nil {
				return nil, fmt.Errorf("failed to link start control plane fragment: %w", err)
			}
		}
		previousTaskExitNodes = startCPFrag.ExitNodes
	}

	// 5. Generate admin kubeconfig
	generateAdminKubeconfig := taskKubexm.NewGenerateAdminKubeconfigTask()
	generateKubeconfigRequired, err := generateAdminKubeconfig.IsRequired(taskCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to check IsRequired for %s: %w", generateAdminKubeconfig.Name(), err)
	}
	if generateKubeconfigRequired {
		logger.Info("Planning task", "task_name", generateAdminKubeconfig.Name())
		generateKubeconfigFrag, err := generateAdminKubeconfig.Plan(taskCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to plan %s: %w", generateAdminKubeconfig.Name(), err)
		}
		if err := moduleFragment.MergeFragment(generateKubeconfigFrag); err != nil {
			return nil, err
		}
		if len(previousTaskExitNodes) > 0 {
			if err := plan.LinkFragments(moduleFragment, previousTaskExitNodes, generateKubeconfigFrag.EntryNodes); err != nil {
				return nil, fmt.Errorf("failed to link kubeconfig fragment: %w", err)
			}
		}
		previousTaskExitNodes = generateKubeconfigFrag.ExitNodes
	}

	// 6. Join additional master nodes (if any)
	joinMastersTask := taskKubexm.NewJoinMastersTask()
	joinMastersRequired, err := joinMastersTask.IsRequired(taskCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to check IsRequired for %s: %w", joinMastersTask.Name(), err)
	}
	if joinMastersRequired {
		logger.Info("Planning task", "task_name", joinMastersTask.Name())
		joinMastersFrag, err := joinMastersTask.Plan(taskCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to plan %s: %w", joinMastersTask.Name(), err)
		}
		if err := moduleFragment.MergeFragment(joinMastersFrag); err != nil {
			return nil, err
		}
		if len(previousTaskExitNodes) > 0 {
			if err := plan.LinkFragments(moduleFragment, previousTaskExitNodes, joinMastersFrag.EntryNodes); err != nil {
				return nil, fmt.Errorf("failed to link join masters fragment: %w", err)
			}
		}
		previousTaskExitNodes = joinMastersFrag.ExitNodes
	}

	moduleFragment.EntryNodes = plan.UniqueNodeIDs(moduleFragment.EntryNodes)
	moduleFragment.ExitNodes = plan.UniqueNodeIDs(previousTaskExitNodes)

	if len(moduleFragment.Nodes) == 0 {
		logger.Info("KubexmControlPlaneModule planned no executable nodes.")
		return plan.NewEmptyFragment(m.Name()), nil
	}

	logger.Info("KubexmControlPlane module planning complete.", "total_nodes", len(moduleFragment.Nodes))
	return moduleFragment, nil
}

var _ module.Module = (*KubexmControlPlaneModule)(nil)
