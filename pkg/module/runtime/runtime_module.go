package runtime

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/module"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/task"
	"github.com/mensylisir/kubexm/pkg/task/containerd"
	"github.com/mensylisir/kubexm/pkg/task/docker"
)

type ContainerRuntimeModule struct {
	module.Base
}

func NewContainerRuntimeModule(ctx *module.ModuleContext) (module.Interface, error) {
	s := &ContainerRuntimeModule{
		Base: module.Base{
			Name: "ContainerRuntime",
			Desc: "Install and configure the container runtime",
		},
	}

	runtimeType := ctx.GetClusterConfig().Spec.Kubernetes.ContainerRuntime.Type
	var selectedTask task.Interface
	var err error

	switch runtimeType {
	case common.RuntimeTypeContainerd:
		selectedTask, err = containerd.NewInstallContainerdTask(ctx)
	case common.RuntimeTypeDocker:
		selectedTask, err = docker.NewInstallDockerTask(ctx)
	default:
		// Default to containerd if not specified or unknown
		selectedTask, err = containerd.NewInstallContainerdTask(ctx)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create task for runtime type %s: %w", runtimeType, err)
	}

	s.SetTasks([]task.Interface{selectedTask})
	return s, nil
}

func (m *ContainerRuntimeModule) Plan(ctx module.ModuleContext) (*plan.ExecutionGraph, error) {
	if len(m.GetTasks()) == 0 {
		return plan.NewExecutionGraph("Empty Container Runtime Module"), nil
	}

	runtimeTask := m.GetTasks()[0]
	return runtimeTask.Plan(ctx)
}
