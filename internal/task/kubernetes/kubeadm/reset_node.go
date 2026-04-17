package kubeadm

import (
	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step/kubernetes/kubeadm"
	"github.com/mensylisir/kubexm/internal/task"
)

type ResetNodeTask struct {
	task.Base
}

func NewResetNodeTask() task.Task {
	return &ResetNodeTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "ResetNode",
				Description: "Resets kubeadm on worker nodes (kubeadm reset)",
			},
		},
	}
}

func (t *ResetNodeTask) Name() string        { return t.Meta.Name }
func (t *ResetNodeTask) Description() string { return t.Meta.Description }

func (t *ResetNodeTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return true, nil
}

func (t *ResetNodeTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	runtimeCtx := ctx.ForTask(t.Name())

	workerHosts := ctx.GetHostsByRole(common.RoleWorker)
	if len(workerHosts) == 0 {
		return fragment, nil
	}

	resetStep, err := kubeadm.NewKubeadmResetStepBuilder(runtimeCtx, "KubeadmReset").Build()
	if err != nil {
		return nil, err
	}

	fragment.AddNode(&plan.ExecutionNode{Name: "KubeadmReset", Step: resetStep, Hosts: workerHosts})
	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}

var _ task.Task = (*ResetNodeTask)(nil)
