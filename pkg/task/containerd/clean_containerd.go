package containerd

import (
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step/containerd"
	"github.com/mensylisir/kubexm/pkg/task"
)

type CleanContainerdTask struct {
	task.Base
}

func NewCleanContainerdTask() task.Task {
	return &CleanContainerdTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "CleanContainerd",
				Description: "Stop, disable, and remove containerd and its related components",
			},
		},
	}
}

func (t *CleanContainerdTask) Name() string {
	return t.Meta.Name
}

func (t *CleanContainerdTask) Description() string {
	return t.Meta.Description
}

func (t *CleanContainerdTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return true, nil
}

func (t *CleanContainerdTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {

	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	deployHosts := append(ctx.GetHostsByRole(common.RoleMaster), ctx.GetHostsByRole(common.RoleWorker)...)
	if len(deployHosts) == 0 {
		return fragment, nil
	}

	stopContainerd := containerd.NewStopContainerdStepBuilder(*runtimeCtx, "StopContainerd").Build()
	disableContainerd := containerd.NewDisableContainerdStepBuilder(*runtimeCtx, "DisableContainerd").Build()
	cleanContainerdFiles := containerd.NewCleanupContainerdStepBuilder(*runtimeCtx, "CleanContainerdFiles").Build()

	stopNode := &plan.ExecutionNode{Name: "StopContainerd", Step: stopContainerd, Hosts: deployHosts}
	disableNode := &plan.ExecutionNode{Name: "DisableContainerd", Step: disableContainerd, Hosts: deployHosts}
	cleanFilesNode := &plan.ExecutionNode{Name: "CleanContainerdFiles", Step: cleanContainerdFiles, Hosts: deployHosts}

	fragment.AddNode(stopNode)
	fragment.AddNode(disableNode)
	fragment.AddNode(cleanFilesNode)

	fragment.AddDependency("StopContainerd", "DisableContainerd")
	fragment.AddDependency("DisableContainerd", "CleanContainerdFiles")
	fragment.CalculateEntryAndExitNodes()

	return fragment, nil
}
