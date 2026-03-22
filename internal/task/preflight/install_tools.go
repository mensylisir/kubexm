package preflight

import (
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	commonstep "github.com/mensylisir/kubexm/internal/step/common"
	"github.com/mensylisir/kubexm/internal/task"
)

type InstallToolBinariesTask struct {
	task.Base
}

func NewInstallToolBinariesTask() task.Task {
	return &InstallToolBinariesTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "InstallToolBinaries",
				Description: "Install jq/yq binaries on all nodes from downloaded artifacts",
			},
		},
	}
}

func (t *InstallToolBinariesTask) Name() string {
	return t.Meta.Name
}

func (t *InstallToolBinariesTask) Description() string {
	return t.Meta.Description
}

func (t *InstallToolBinariesTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return true, nil
}

func (t *InstallToolBinariesTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	allHosts := ctx.GetHostsByRole("")
	if len(allHosts) == 0 {
		return fragment, nil
	}

	installStep, err := commonstep.NewInstallToolBinariesStepBuilder(runtimeCtx, "InstallToolBinaries").Build()
	if err != nil {
		return nil, err
	}
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallToolBinaries", Step: installStep, Hosts: allHosts})

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}

var _ task.Task = (*InstallToolBinariesTask)(nil)
