package multus

import (
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	multusstep "github.com/mensylisir/kubexm/pkg/step/network/multus"
	"github.com/mensylisir/kubexm/pkg/task"
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

	cleanStep := multusstep.NewCleanMultusStepBuilder(*runtimeCtx, "UninstallMultusRelease").Build()
	fragment.AddNode(&plan.ExecutionNode{Name: "UninstallMultusRelease", Step: cleanStep, Hosts: []connector.Host{executionHost}})

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
