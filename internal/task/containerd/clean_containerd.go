package containerd

import (
	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step/containerd"
	"github.com/mensylisir/kubexm/internal/task"
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

	stopContainerd, err := containerd.NewStopContainerdStepBuilder(runtimeCtx, "StopContainerd").Build()
	if err != nil {
		return nil, err
	}
	disableContainerd, err := containerd.NewDisableContainerdStepBuilder(runtimeCtx, "DisableContainerd").Build()
	if err != nil {
		return nil, err
	}
	cleanContainerdFiles, err := containerd.NewCleanupContainerdStepBuilder(runtimeCtx, "CleanContainerdFiles").Build()
	if err != nil {
		return nil, err
	}

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
