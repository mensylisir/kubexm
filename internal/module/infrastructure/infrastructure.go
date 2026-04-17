package infrastructure

import (
	"fmt"

	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/module"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/task"
	taskContainerd "github.com/mensylisir/kubexm/internal/task/containerd"
	taskCrio "github.com/mensylisir/kubexm/internal/task/crio"
	taskDocker "github.com/mensylisir/kubexm/internal/task/docker"
	taskEtcd "github.com/mensylisir/kubexm/internal/task/etcd"
	taskos "github.com/mensylisir/kubexm/internal/task/os"
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
	moduleFragment := plan.NewExecutionFragment(m.Name() + "-Fragment")

	// Phase 1: OS Preparation (grouped by function)
	osTasks := []task.Task{
		taskos.NewConfigureHostTask(),      // SetHostname + UpdateEtcHosts
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
			if err := plan.LinkFragments(moduleFragment, lastOsTaskExitNodes, frag.EntryNodes); err != nil {
				return nil, fmt.Errorf("failed to link OS task fragment: %w", err)
			}
		}
		lastOsTaskExitNodes = frag.ExitNodes
	}

	// Phase 2: ETCD Setup
	// Depends on OS setup being complete
	etcdPkiTask := taskEtcd.NewGenerateEtcdPKITask()
	installEtcdTask := taskEtcd.NewDeployEtcdClusterTask()

	etcdPkiFrag, err := planTask(ctx, etcdPkiTask)
	if err != nil {
		return nil, err
	}
	if err := moduleFragment.MergeFragment(etcdPkiFrag); err != nil {
		return nil, err
	}
	if err := plan.LinkFragments(moduleFragment, lastOsTaskExitNodes, etcdPkiFrag.EntryNodes); err != nil {
		return nil, fmt.Errorf("failed to link etcd PKI fragment: %w", err)
	}

	installEtcdFrag, err := planTask(ctx, installEtcdTask)
	if err != nil {
		return nil, err
	}
	if err := moduleFragment.MergeFragment(installEtcdFrag); err != nil {
		return nil, err
	}
	if err := plan.LinkFragments(moduleFragment, etcdPkiFrag.ExitNodes, installEtcdFrag.EntryNodes); err != nil {
		return nil, fmt.Errorf("failed to link etcd install fragment: %w", err)
	}

	etcdExitNodes := installEtcdFrag.ExitNodes

	// Phase 3: Container Runtime Setup
	// Also depends on OS setup being complete, can run in parallel with ETCD setup
	var containerRuntimeTask task.Task
	clusterCfg := ctx.GetClusterConfig()
	if clusterCfg.Spec.Kubernetes == nil || clusterCfg.Spec.Kubernetes.ContainerRuntime == nil {
		return nil, errors.New("containerRuntime spec is nil, cannot determine runtime type")
	}
	switch clusterCfg.Spec.Kubernetes.ContainerRuntime.Type {
	case common.RuntimeTypeContainerd:
		containerRuntimeTask = taskContainerd.NewDeployContainerdTask()
	case common.RuntimeTypeDocker:
		containerRuntimeTask = taskDocker.NewDeployDockerTask()
	case common.RuntimeTypeCRIO:
		containerRuntimeTask = taskCrio.NewDeployCrioTask()
	default:
		return nil, fmt.Errorf("unsupported container runtime type: %s (supported: containerd, docker, crio)", clusterCfg.Spec.Kubernetes.ContainerRuntime.Type)
	}

	runtimeFrag, err := planTask(ctx, containerRuntimeTask)
	if err != nil {
		return nil, err
	}
	if err := moduleFragment.MergeFragment(runtimeFrag); err != nil {
		return nil, err
	}
	if err := plan.LinkFragments(moduleFragment, lastOsTaskExitNodes, runtimeFrag.EntryNodes); err != nil {
		return nil, fmt.Errorf("failed to link container runtime fragment: %w", err)
	}

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
func planTask(ctx runtime.ModuleContext, t task.Task) (*plan.ExecutionFragment, error) {
	taskCtx, ok := ctx.(runtime.TaskContext)
	if !ok {
		return nil, fmt.Errorf("context does not implement runtime.TaskContext")
	}
	isRequired, err := t.IsRequired(taskCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to check if task %s is required: %w", t.Name(), err)
	}
	if !isRequired {
		ctx.GetLogger().Debug("Skipping non-required task", "task", t.Name())
		return plan.NewEmptyFragment(t.Name()), nil
	}

	ctx.GetLogger().Info("Planning task", "task_name", t.Name())
	taskFrag, err := t.Plan(taskCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to plan task %s: %w", t.Name(), err)
	}
	return taskFrag, nil
}

var _ module.Module = (*InfrastructureModule)(nil)
