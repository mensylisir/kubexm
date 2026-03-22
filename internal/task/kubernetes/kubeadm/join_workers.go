package kubeadm

import (
	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step/kubernetes/kubeadm"
	"github.com/mensylisir/kubexm/internal/task"
)

type JoinWorkersTask struct {
	task.Base
}

func NewJoinWorkersTask() task.Task {
	return &JoinWorkersTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "JoinWorkers",
				Description: "Join worker nodes to the Kubernetes cluster",
			},
		},
	}
}

func (t *JoinWorkersTask) Name() string {
	return t.Meta.Name
}

func (t *JoinWorkersTask) Description() string {
	return t.Meta.Description
}

func (t *JoinWorkersTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return len(ctx.GetHostsByRole(common.RoleWorker)) > 0, nil
}

func (t *JoinWorkersTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	workerHosts := ctx.GetHostsByRole(common.RoleWorker)

	if len(workerHosts) == 0 {
		ctx.GetLogger().Info("No worker nodes to join, skipping task.")
		return fragment, nil
	}

	generateJoinConfig, err := kubeadm.NewGenerateJoinWorkerConfigStepBuilder(runtimeCtx, "GenerateJoinWorkerConfig").Build()
	if err != nil {
		return nil, err
	}
	kubeadmJoin, err := kubeadm.NewKubeadmJoinWorkerStepBuilder(runtimeCtx, "ExecuteKubeadmJoinWorker").Build()
	if err != nil {
		return nil, err
	}

	fragment.AddNode(&plan.ExecutionNode{Name: "GenerateJoinWorkerConfig", Step: generateJoinConfig, Hosts: workerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "ExecuteKubeadmJoinWorker", Step: kubeadmJoin, Hosts: workerHosts})

	fragment.AddDependency("GenerateJoinWorkerConfig", "ExecuteKubeadmJoinWorker")

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
