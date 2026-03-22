package multus

import (
	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/connector"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	multusstep "github.com/mensylisir/kubexm/internal/step/network/multus"
	"github.com/mensylisir/kubexm/internal/task"
)

type CleanMultusTask struct {
	task.Base
}

func NewCleanMultusTask() task.Task {
	return &CleanMultusTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "CleanMultus",
				Description: "Uninstall Multus CNI and cleanup related resources",
			},
		},
	}
}

func (t *CleanMultusTask) Name() string {
	return t.Meta.Name
}

func (t *CleanMultusTask) Description() string {
	return t.Meta.Description
}

func (t *CleanMultusTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	cfg := ctx.GetClusterConfig()
	if cfg.Spec.Network.Multus != nil &&
		cfg.Spec.Network.Multus.Installation != nil &&
		cfg.Spec.Network.Multus.Installation.Enabled != nil {
		return *cfg.Spec.Network.Multus.Installation.Enabled, nil
	}
	return false, nil
}

func (t *CleanMultusTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	masterHosts := ctx.GetHostsByRole(common.RoleMaster)
	if len(masterHosts) == 0 {
		return fragment, nil
	}
	executionHost := masterHosts[0]

	cleanStep, err := multusstep.NewCleanMultusStepBuilder(runtimeCtx, "UninstallMultusRelease").Build()
	if err != nil {
		return nil, err
	}
	fragment.AddNode(&plan.ExecutionNode{Name: "UninstallMultusRelease", Step: cleanStep, Hosts: []connector.Host{executionHost}})

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
