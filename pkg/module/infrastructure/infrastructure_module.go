package infrastructure

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/module"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/task"
	taskContainerd "github.com/mensylisir/kubexm/pkg/task/containerd"
	taskDocker "github.com/mensylisir/kubexm/pkg/task/docker"
	taskEtcd "github.com/mensylisir/kubexm/pkg/task/etcd"
)

// InfrastructureModule is responsible for setting up the core infrastructure:
// ETCD cluster and Container Runtime on all nodes.
type InfrastructureModule struct {
	module.BaseModule
}

// NewInfrastructureModule creates a new InfrastructureModule.
func NewInfrastructureModule() module.Module {
	// Tasks will be dynamically chosen in Plan based on config.
	base := module.NewBaseModule("CoreInfrastructureSetup", nil) // Tasks are dynamic
	return &InfrastructureModule{BaseModule: base}
}

func (m *InfrastructureModule) Plan(ctx module.ModuleContext) (*task.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("module", m.Name())
	moduleFragment := task.NewExecutionFragment(m.Name() + "-Fragment")

	taskCtx, ok := ctx.(task.TaskContext)
	if !ok {
		return nil, fmt.Errorf("module context cannot be asserted to task.TaskContext for %s", m.Name())
	}

	clusterCfg := ctx.GetClusterConfig()
	var allModuleEntryNodes []plan.NodeID
	var allModuleExitNodes []plan.NodeID

	// 1. ETCD Setup (PKI Generation + Installation)
	// ControlPlaneEndpoint.Domain and a default LB domain are passed for SAN generation if needed.
	cpDomain := ""
	if clusterCfg.Spec.ControlPlaneEndpoint != nil {
		cpDomain = clusterCfg.Spec.ControlPlaneEndpoint.Domain
	}
	// TODO: Make "lb.kubexm.internal" a configurable default or derive from cluster domain.
	generatePkiTask := taskEtcd.NewGenerateEtcdPkiTask(cpDomain, "lb.kubexm.internal")
	installEtcdTask := taskEtcd.NewInstallETCDTask()

	etcdTasks := []task.Task{generatePkiTask, installEtcdTask}
	etcdPkiExits := []plan.NodeID{}

	for i, etcdRelatedTask := range etcdTasks {
		isRequired, err := etcdRelatedTask.IsRequired(taskCtx)
		if err != nil { return nil, fmt.Errorf("failed to check IsRequired for task %s: %w", etcdRelatedTask.Name(), err) }
		if !isRequired {
			logger.Info("Skipping infrastructure task as it's not required", "task", etcdRelatedTask.Name())
			continue
		}

		logger.Info("Planning infrastructure task", "task", etcdRelatedTask.Name())
		taskFrag, err := etcdRelatedTask.Plan(taskCtx)
		if err != nil { return nil, fmt.Errorf("failed to plan task %s: %w", etcdRelatedTask.Name(), err) }
		if err := moduleFragment.MergeFragment(taskFrag); err != nil { return nil, fmt.Errorf("failed to merge fragment from task %s: %w", etcdRelatedTask.Name(), err) }

		if i == 0 { // GenerateEtcdPkiTask
			allModuleEntryNodes = append(allModuleEntryNodes, taskFrag.EntryNodes...)
			etcdPkiExits = taskFrag.ExitNodes
		} else { // InstallETCDTask
			plan.LinkFragments(moduleFragment, etcdPkiExits, taskFrag.EntryNodes)
			allModuleExitNodes = append(allModuleExitNodes, taskFrag.ExitNodes...) // ETCD setup is an exit of this module part
		}
	}

	// 2. Container Runtime Setup
	var containerRuntimeTask task.Task
	// According to API design, ContainerRuntime is directly in Spec
	if clusterCfg.Spec.ContainerRuntime == nil {
		return nil, fmt.Errorf("containerRuntime spec is nil, cannot determine runtime type")
	}
	switch clusterCfg.Spec.ContainerRuntime.Type {
	case v1alpha1.ContainerRuntimeContainerd: // Use defined constant
		containerRuntimeTask = taskContainerd.NewInstallContainerdTask([]string{common.AllHostsRole})
	case v1alpha1.ContainerRuntimeDocker:    // Use defined constant
		containerRuntimeTask = taskDocker.NewInstallDockerTask([]string{common.AllHostsRole})
	default:
		return nil, fmt.Errorf("unsupported container runtime type: %s", clusterCfg.Spec.ContainerRuntime.Type)
	}

	isRequired, err := containerRuntimeTask.IsRequired(taskCtx)
	if err != nil { return nil, fmt.Errorf("failed to check IsRequired for task %s: %w", containerRuntimeTask.Name(), err) }

	if isRequired {
		logger.Info("Planning infrastructure task", "task", containerRuntimeTask.Name())
		taskFrag, err := containerRuntimeTask.Plan(taskCtx)
		if err != nil { return nil, fmt.Errorf("failed to plan task %s: %w", containerRuntimeTask.Name(), err) }
		if err := moduleFragment.MergeFragment(taskFrag); err != nil { return nil, fmt.Errorf("failed to merge fragment from task %s: %w", containerRuntimeTask.Name(), err) }

		// Container runtime setup can also be an initial entry point for the module, parallel to ETCD PKI.
		allModuleEntryNodes = append(allModuleEntryNodes, taskFrag.EntryNodes...)
		allModuleExitNodes = append(allModuleExitNodes, taskFrag.ExitNodes...)
	}

	moduleFragment.EntryNodes = task.UniqueNodeIDs(allModuleEntryNodes)
	moduleFragment.ExitNodes = task.UniqueNodeIDs(allModuleExitNodes) // Exits are combined from ETCD and Runtime tasks

	if len(moduleFragment.Nodes) == 0 {
		logger.Info("InfrastructureModule planned no executable nodes.")
		return task.NewEmptyFragment(), nil
	}

	logger.Info("Infrastructure module planning complete.", "total_nodes", len(moduleFragment.Nodes))
	return moduleFragment, nil
}

var _ module.Module = (*InfrastructureModule)(nil)
