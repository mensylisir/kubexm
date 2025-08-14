package hybridnet

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	hybridnetstep "github.com/mensylisir/kubexm/pkg/step/network/hybridnet"
	"github.com/mensylisir/kubexm/pkg/task"
)

type CleanHybridnetTask struct {
	task.Base
}

func NewCleanHybridnetTask() task.Task {
	return &CleanHybridnetTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "CleanHybridnet",
				Description: "Uninstall Hybridnet CNI and cleanup related resources",
			},
		},
	}
}

func (t *CleanHybridnetTask) Name() string {
	return t.Meta.Name
}

func (t *CleanHybridnetTask) Description() string {
	return t.Meta.Description
}

func (t *CleanHybridnetTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return ctx.GetClusterConfig().Spec.Network.Plugin == string(common.CNITypeHybridnet), nil
}

func (t *CleanHybridnetTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
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

	cleanStep := hybridnetstep.NewCleanHybridnetStepBuilder(*runtimeCtx, "UninstallHybridnetRelease").Build()
	fragment.AddNode(&plan.ExecutionNode{Name: "UninstallHybridnetRelease", Step: cleanStep, Hosts: []connector.Host{executionHost}})

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
