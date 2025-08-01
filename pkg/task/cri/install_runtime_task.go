package cri

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/task"
	"github.com/mensylisir/kubexm/pkg/task/containerd"
	"github.com/mensylisir/kubexm/pkg/task/docker"
)

// InstallRuntimeTask is a dispatcher task for installing the container runtime.
type InstallRuntimeTask struct {
	task.BaseTask
}

// NewInstallRuntimeTask creates a new InstallRuntimeTask.
func NewInstallRuntimeTask() task.Task {
	return &InstallRuntimeTask{
		BaseTask: task.NewBaseTask(
			"InstallContainerRuntime",
			"Dispatches to the correct container runtime installation task.",
			nil,
			nil,
			false,
		),
	}
}

func (t *InstallRuntimeTask) IsRequired(ctx task.TaskContext) (bool, error) {
	return ctx.GetClusterConfig().Spec.ContainerRuntime != nil, nil
}

// Plan is a dispatcher that selects the appropriate runtime-specific task.
func (t *InstallRuntimeTask) Plan(ctx task.TaskContext) (*task.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("task", t.Name())
	runtimeType := ctx.GetClusterConfig().Spec.ContainerRuntime.Type

	var runtimeTask task.Task

	switch runtimeType {
	case v1alpha1.ContainerRuntimeContainerd:
		runtimeTask = containerd.NewInstallContainerdTask()
	case v1alpha1.ContainerRuntimeDocker:
		runtimeTask = docker.NewInstallDockerTask()
	default:
		return nil, fmt.Errorf("unsupported container runtime type '%s'", runtimeType)
	}

	logger.Info("Dispatching to container runtime installation task.", "runtime", runtimeType)
	return runtimeTask.Plan(ctx)
}

var _ task.Task = (*InstallRuntimeTask)(nil)
