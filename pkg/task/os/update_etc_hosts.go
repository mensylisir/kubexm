package os

import (
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	osstep "github.com/mensylisir/kubexm/pkg/step/os"
	"github.com/mensylisir/kubexm/pkg/task"
)

type UpdateEtcHostsTask struct {
	task.Base
}

func NewUpdateEtcHostsTask() task.Task {
	return &UpdateEtcHostsTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "UpdateEtcHosts",
				Description: "Update /etc/hosts file on all nodes",
			},
		},
	}
}

func (t *UpdateEtcHostsTask) Name() string {
	return t.Meta.Name
}

func (t *UpdateEtcHostsTask) Description() string {
	return t.Meta.Description
}

func (t *UpdateEtcHostsTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return true, nil
}

func (t *UpdateEtcHostsTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	allHosts := ctx.GetHostsByRole("")
	if len(allHosts) == 0 {
		return fragment, nil
	}

	updateEtcHostsStep := osstep.NewUpdateEtcHostsStepBuilder(*runtimeCtx, "UpdateEtcHosts").Build()

	node := &plan.ExecutionNode{
		Name:  "UpdateEtcHosts",
		Step:  updateEtcHostsStep,
		Hosts: allHosts,
	}

	fragment.AddNode(node)
	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
