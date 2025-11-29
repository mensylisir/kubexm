package infrastructure

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/module"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/task"
	taskContainerd "github.com/mensylisir/kubexm/pkg/task/containerd"
	taskDocker "github.com/mensylisir/kubexm/pkg/task/docker"
	taskEtcd "github.com/mensylisir/kubexm/pkg/task/etcd"
	taskos "github.com/mensylisir/kubexm/pkg/task/os"
	taskpackages "github.com/mensylisir/kubexm/pkg/task/packages"
	"github.com/pkg/errors"
)

// InfrastructureModule is responsible for setting up the core infrastructure:
// OS prerequisites, ETCD cluster, and Container Runtime on all nodes.
type InfrastructureModule struct {
	module.BaseModule
}

// NewInfrastructureModule creates a new InfrastructureModule.
func NewInfrastructureModule() module.Module {
	base := module.NewBaseModule("CoreInfrastructureSetup", nil)
	return &InfrastructureModule{BaseModule: base}
}

func (m *InfrastructureModule) Plan(ctx runtime.ModuleContext) (*plan.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("module", m.Name())
	moduleFragment := task.NewExecutionFragment(m.Name() + "-Fragment")

	// Phase 1: OS Preparation (grouped by function)
	osTasks := []task.Task{
		taskos.NewConfigureHostTask(),      // SetHostname + UpdateEtcHosts
		taskpackages.NewInstallPackagesTask(), // Install prerequisite packages
		taskos.NewDisableServicesTask(),    // DisableSwap, DisableFirewall, DisableSelinux
		taskos.NewConfigureKernelTask(),    // LoadKernelModules, ConfigureSysctl
	}

	var lastOsTaskExitNodes []plan.NodeID
	for _, t := range osTasks {
		frag, err := planTask(ctx, t)
		if err != nil {
			return nil, err
		}
		if frag.IsEmpty() {
			continue
		}
		if err := moduleFragment.MergeFragment(frag); err != nil {
			return nil, err
		}
		// Create a sequential dependency chain for the OS setup tasks
		if len(lastOsTaskExitNodes) > 0 {
			plan.LinkFragments(moduleFragment, lastOsTaskExitNodes, frag.EntryNodes)
		}
		lastOsTaskExitNodes = frag.ExitNodes
	}

	// Phase 2: ETCD Setup
	// Depends on OS setup being complete
	etcdPkiTask := taskEtcd.NewGenerateEtcdPkiTask()
	installEtcdTask := taskEtcd.NewInstallETCDTask()

	etcdPkiFrag, err := planTask(ctx, etcdPkiTask)
	if err != nil {
		return nil, err
	}
	if err := moduleFragment.MergeFragment(etcdPkiFrag); err != nil {
		return nil, err
	}
	plan.LinkFragments(moduleFragment, lastOsTaskExitNodes, etcdPkiFrag.EntryNodes)

	installEtcdFrag, err := planTask(ctx, installEtcdTask)
	if err != nil {
		return nil, err
	}
	if err := moduleFragment.MergeFragment(installEtcdFrag); err != nil {
		return nil, err
	}
	plan.LinkFragments(moduleFragment, etcdPkiFrag.ExitNodes, installEtcdFrag.EntryNodes)

	etcdExitNodes := installEtcdFrag.ExitNodes

	// Phase 3: Container Runtime Setup
	// Also depends on OS setup being complete, can run in parallel with ETCD setup
	var containerRuntimeTask task.Task
	clusterCfg := ctx.GetClusterConfig()
	if clusterCfg.Spec.ContainerRuntime == nil {
		return nil, errors.New("containerRuntime spec is nil, cannot determine runtime type")
	}
	switch clusterCfg.Spec.ContainerRuntime.Type {
	case v1alpha1.ContainerRuntimeContainerd:
		containerRuntimeTask = taskContainerd.NewInstallContainerdTask()
	case v1alpha1.ContainerRuntimeDocker:
		containerRuntimeTask = taskDocker.NewInstallDockerTask()
	default:
		return nil, fmt.Errorf("unsupported container runtime type: %s", clusterCfg.Spec.ContainerRuntime.Type)
	}

	runtimeFrag, err := planTask(ctx, containerRuntimeTask)
	if err != nil {
		return nil, err
	}
	if err := moduleFragment.MergeFragment(runtimeFrag); err != nil {
		return nil, err
	}
	plan.LinkFragments(moduleFragment, lastOsTaskExitNodes, runtimeFrag.EntryNodes)

	runtimeExitNodes := runtimeFrag.ExitNodes

	// Final module exits are the exits of the last parallel phases (ETCD and Container Runtime)
	finalExitNodes := append(etcdExitNodes, runtimeExitNodes...)
	moduleFragment.ExitNodes = plan.UniqueNodeIDs(finalExitNodes)

	moduleFragment.CalculateEntryAndExitNodes()

	if len(moduleFragment.Nodes) == 0 {
		logger.Info("InfrastructureModule planned no executable nodes.")
	} else {
		logger.Info("Infrastructure module planning complete.", "total_nodes", len(moduleFragment.Nodes))
	}

	return moduleFragment, nil
}

// planTask is a helper to reduce repetition.
func planTask(ctx module.ModuleContext, t task.Task) (*plan.ExecutionFragment, error) {
	isRequired, err := t.IsRequired(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to check if task %s is required: %w", t.Name(), err)
	}
	if !isRequired {
		ctx.GetLogger().Debug("Skipping non-required task", "task", t.Name())
		return plan.NewEmptyFragment(m.Name()), nil
	}

	ctx.GetLogger().Info("Planning task", "task_name", t.Name())
	taskFrag, err := t.Plan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to plan task %s: %w", t.Name(), err)
	}
	return taskFrag, nil
}

var _ module.Module = (*InfrastructureModule)(nil)
