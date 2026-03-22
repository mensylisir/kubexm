package kubeovn

import (
	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/connector"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	kubeovnstep "github.com/mensylisir/kubexm/internal/step/network/kubeovn"
	"github.com/mensylisir/kubexm/internal/task"
)

type CleanKubeovnTask struct {
	task.Base
}

func NewCleanKubeovnTask() task.Task {
	return &CleanKubeovnTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "CleanKubeovn",
				Description: "Uninstall Kube-OVN CNI and cleanup related resources",
			},
		},
	}
}

func (t *CleanKubeovnTask) Name() string {
	return t.Meta.Name
}

func (t *CleanKubeovnTask) Description() string {
	return t.Meta.Description
}

func (t *CleanKubeovnTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return ctx.GetClusterConfig().Spec.Network.Plugin == string(common.CNITypeKubeOvn), nil
}

func (t *CleanKubeovnTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	masterHosts := ctx.GetHostsByRole(common.RoleMaster)
	if len(masterHosts) == 0 {
		return fragment, nil
	}
	executionHost := masterHosts[0]

	cleanStep, err := kubeovnstep.NewCleanKubeovnStepBuilder(runtimeCtx, "UninstallKubeovnRelease").Build()
	if err != nil {
		return nil, err
	}
	fragment.AddNode(&plan.ExecutionNode{Name: "UninstallKubeovnRelease", Step: cleanStep, Hosts: []connector.Host{executionHost}})

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
