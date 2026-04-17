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

// RuntimeModule manages container runtime installation and cleanup.
type RuntimeModule struct {
	module.BaseModule
}

// NewRuntimeModule creates a new RuntimeModule.
func NewRuntimeModule() module.Module {
	base := module.NewBaseModule("ContainerRuntime", nil)
	return &RuntimeModule{BaseModule: base}
}

func (m *RuntimeModule) Plan(ctx runtime.ModuleContext) (*plan.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("module", m.Name())
	moduleFragment := plan.NewExecutionFragment(m.Name() + "-Fragment")

	taskCtx, ok := ctx.(runtime.TaskContext)
	if !ok {
		return nil, fmt.Errorf("context does not implement runtime.TaskContext")
	}

	clusterCfg := taskCtx.GetClusterConfig()
	if clusterCfg.Spec.Kubernetes == nil || clusterCfg.Spec.Kubernetes.ContainerRuntime == nil {
		return nil, fmt.Errorf("containerRuntime spec is nil")
	}

	runtimeType := clusterCfg.Spec.Kubernetes.ContainerRuntime.Type
	logger.Info("Planning container runtime module", "type", runtimeType)

	var runtimeTask task.Task
	switch runtimeType {
	case common.RuntimeTypeContainerd:
		runtimeTask = taskContainerd.NewDeployContainerdTask()
	case common.RuntimeTypeDocker:
		runtimeTask = taskDocker.NewDeployDockerTask()
	case common.RuntimeTypeCRIO:
		runtimeTask = taskCrio.NewDeployCrioTask()
	case common.RuntimeTypeIsula:
		return nil, fmt.Errorf("unsupported container runtime: %s", runtimeType)
	default:
		return nil, fmt.Errorf("unsupported container runtime: %s", runtimeType)
	}

	runtimeFrag, err := runtimeTask.Plan(taskCtx)
	if err != nil {
		return nil, err
	}

	if err := moduleFragment.MergeFragment(runtimeFrag); err != nil {
		return nil, err
	}

	moduleFragment.EntryNodes = runtimeFrag.EntryNodes
	moduleFragment.ExitNodes = runtimeFrag.ExitNodes

	logger.Info("Runtime module planning complete.", "total_nodes", len(moduleFragment.Nodes))
	return moduleFragment, nil
}

var _ module.Module = (*RuntimeModule)(nil)
