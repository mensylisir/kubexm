package hybridnet

import (
	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/remotefw"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	hybridnetstep "github.com/mensylisir/kubexm/internal/step/network/hybridnet"
	"github.com/mensylisir/kubexm/internal/task"
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
	netSpec := ctx.GetClusterConfig().Spec.Network
	if netSpec == nil {
		return false, nil
	}
	return netSpec.Plugin == string(common.CNITypeHybridnet), nil
}

func (t *CleanHybridnetTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.ForTask(t.Name())

	masterHosts := ctx.GetHostsByRole(common.RoleMaster)
	if len(masterHosts) == 0 {
		return fragment, nil
	}
	executionHost := masterHosts[0]

	cleanStep, err := hybridnetstep.NewCleanHybridnetStepBuilder(runtimeCtx, "UninstallHybridnetRelease").Build()
	if err != nil {
		return nil, err
	}
	fragment.AddNode(&plan.ExecutionNode{Name: "UninstallHybridnetRelease", Step: cleanStep, Hosts: []remotefw.Host{executionHost}})

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
