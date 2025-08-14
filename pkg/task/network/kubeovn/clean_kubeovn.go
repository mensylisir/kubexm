package kubeovn

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	kubeovnstep "github.com/mensylisir/kubexm/pkg/step/network/kubeovn"
	"github.com/mensylisir/kubexm/pkg/task"
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

	runtimeCtx, ok := ctx.(*runtime.Context)
	if !ok {
		return nil, fmt.Errorf("internal error: TaskContext is not of type *runtime.Context")
	}

	masterHosts := ctx.GetHostsByRole(common.RoleMaster)
	if len(masterHosts) == 0 {
		return fragment, nil
	}
	executionHost := masterHosts[0]

	cleanStep := kubeovnstep.NewCleanKubeovnStepBuilder(*runtimeCtx, "UninstallKubeovnRelease").Build()
	fragment.AddNode(&plan.ExecutionNode{Name: "UninstallKubeovnRelease", Step: cleanStep, Hosts: []connector.Host{executionHost}})

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
