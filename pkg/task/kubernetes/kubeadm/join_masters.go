package kubeadm

import (
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step/kubernetes/kubeadm"
	"github.com/mensylisir/kubexm/pkg/task"
)

type JoinMastersTask struct {
	task.Base
}

func NewJoinMastersTask() task.Task {
	return &JoinMastersTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "JoinMasters",
				Description: "Join additional master nodes to the Kubernetes cluster",
			},
		},
	}
}

func (t *JoinMastersTask) Name() string {
	return t.Meta.Name
}

func (t *JoinMastersTask) Description() string {
	return t.Meta.Description
}

func (t *JoinMastersTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return len(ctx.GetHostsByRole(common.RoleMaster)) > 1, nil
}

func (t *JoinMastersTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	masterHosts := ctx.GetHostsByRole(common.RoleMaster)
	joinMasterHosts := masterHosts[1:]

	if len(joinMasterHosts) == 0 {
		ctx.GetLogger().Info("No additional master nodes to join, skipping task.")
		return fragment, nil
	}

	generateJoinConfig := kubeadm.NewGenerateJoinMasterConfigStepBuilder(*runtimeCtx, "GenerateJoinMasterConfig").Build()
	kubeadmJoin := kubeadm.NewKubeadmJoinMasterStepBuilder(*runtimeCtx, "ExecuteKubeadmJoinMaster").Build()

	fragment.AddNode(&plan.ExecutionNode{Name: "GenerateJoinMasterConfig", Step: generateJoinConfig, Hosts: joinMasterHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "ExecuteKubeadmJoinMaster", Step: kubeadmJoin, Hosts: joinMasterHosts})

	fragment.AddDependency("GenerateJoinMasterConfig", "ExecuteKubeadmJoinMaster")

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
