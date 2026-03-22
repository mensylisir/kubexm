package kubeadm

import (
	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step/kubernetes/kubeadm"
	"github.com/mensylisir/kubexm/internal/task"
)

type CleanKubeComponentsTask struct {
	task.Base
}

func NewCleanKubeComponentsTask() task.Task {
	return &CleanKubeComponentsTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "CleanKubeComponents",
				Description: "Stop and disable kubelet service, remove kube binaries and service files",
			},
		},
	}
}

func (t *CleanKubeComponentsTask) Name() string {
	return t.Meta.Name
}

func (t *CleanKubeComponentsTask) Description() string {
	return t.Meta.Description
}

func (t *CleanKubeComponentsTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return true, nil
}

func (t *CleanKubeComponentsTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	deployHosts := append(ctx.GetHostsByRole(common.RoleMaster), ctx.GetHostsByRole(common.RoleWorker)...)
	if len(deployHosts) == 0 {
		return fragment, nil
	}

	cleanStep, err := kubeadm.NewCleanKubeComponentsStepBuilder(runtimeCtx, "CleanKubeComponents").Build()
	if err != nil {
		return nil, err
	}

	node := &plan.ExecutionNode{Name: "CleanKubeComponents", Step: cleanStep, Hosts: deployHosts}

	fragment.AddNode(node)

	fragment.CalculateEntryAndExitNodes()

	return fragment, nil
}
