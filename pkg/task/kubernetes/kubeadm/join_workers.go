package kubeadm

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step/kubernetes/kubeadm"
	"github.com/mensylisir/kubexm/pkg/task"
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
	runtimeCtx, ok := ctx.(*runtime.Context)
	if !ok {
		return nil, fmt.Errorf("internal error: TaskContext is not of type *runtime.Context")
	}

	workerHosts := ctx.GetHostsByRole(common.RoleWorker)

	if len(workerHosts) == 0 {
		ctx.GetLogger().Info("No worker nodes to join, skipping task.")
		return fragment, nil
	}

	generateJoinConfig := kubeadm.NewGenerateJoinWorkerConfigStepBuilder(*runtimeCtx, "GenerateJoinWorkerConfig").Build()
	kubeadmJoin := kubeadm.NewKubeadmJoinWorkerStepBuilder(*runtimeCtx, "ExecuteKubeadmJoinWorker").Build()

	fragment.AddNode(&plan.ExecutionNode{Name: "GenerateJoinWorkerConfig", Step: generateJoinConfig, Hosts: workerHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "ExecuteKubeadmJoinWorker", Step: kubeadmJoin, Hosts: workerHosts})

	fragment.AddDependency("GenerateJoinWorkerConfig", "ExecuteKubeadmJoinWorker")

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
