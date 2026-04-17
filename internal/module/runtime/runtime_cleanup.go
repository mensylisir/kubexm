package runtime

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
)

// RuntimeCleanupModule handles cleanup of container runtime.
type RuntimeCleanupModule struct {
	module.BaseModule
}

// NewRuntimeCleanupModule creates a new RuntimeCleanupModule.
func NewRuntimeCleanupModule() module.Module {
	base := module.NewBaseModule("ContainerRuntimeCleanup", nil)
	return &RuntimeCleanupModule{BaseModule: base}
}

func (m *RuntimeCleanupModule) Plan(ctx runtime.ModuleContext) (*plan.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("module", m.Name())
	moduleFragment := plan.NewExecutionFragment(m.Name() + "-Fragment")

	taskCtx, ok := ctx.(runtime.TaskContext)
	if !ok {
		return nil, fmt.Errorf("context does not implement runtime.TaskContext")
	}

	clusterCfg := taskCtx.GetClusterConfig()
	if clusterCfg.Spec.Kubernetes == nil || clusterCfg.Spec.Kubernetes.ContainerRuntime == nil {
		logger.Info("No container runtime configured, skipping cleanup")
		return plan.NewEmptyFragment(m.Name()), nil
	}

	runtimeType := clusterCfg.Spec.Kubernetes.ContainerRuntime.Type
	logger.Info("Planning container runtime cleanup module", "type", runtimeType)

	var cleanupTask task.Task
	switch runtimeType {
	case common.RuntimeTypeContainerd:
		cleanupTask = taskContainerd.NewCleanContainerdTask()
	case common.RuntimeTypeDocker:
		cleanupTask = taskDocker.NewCleanDockerTask()
	case common.RuntimeTypeCRIO:
		cleanupTask = taskCrio.NewCleanCrioTask()
	default:
		logger.Info("Unsupported runtime type for cleanup", "type", runtimeType)
		return plan.NewEmptyFragment(m.Name()), nil
	}

	cleanupFrag, err := cleanupTask.Plan(taskCtx)
	if err != nil {
		return nil, err
	}

	if err := moduleFragment.MergeFragment(cleanupFrag); err != nil {
		return nil, err
	}

	moduleFragment.EntryNodes = cleanupFrag.EntryNodes
	moduleFragment.ExitNodes = cleanupFrag.ExitNodes

	logger.Info("Runtime cleanup module planning complete.", "total_nodes", len(moduleFragment.Nodes))
	return moduleFragment, nil
}

var _ module.Module = (*RuntimeCleanupModule)(nil)
