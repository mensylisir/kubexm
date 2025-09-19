package os

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	osstep "github.com/mensylisir/kubexm/pkg/step/os"
	"github.com/mensylisir/kubexm/pkg/task"
)

type SetHostnameTask struct {
	task.Base
}

func NewSetHostnameTask() task.Task {
	return &SetHostnameTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "SetHostname",
				Description: "Set the hostname on each node",
			},
		},
	}
}

func (t *SetHostnameTask) Name() string {
	return t.Meta.Name
}

func (t *SetHostnameTask) Description() string {
	return t.Meta.Description
}

func (t *SetHostnameTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return true, nil
}

func (t *SetHostnameTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	allHosts := ctx.GetHostsByRole("")
	if len(allHosts) == 0 {
		return fragment, nil
	}

	for _, host := range allHosts {
		hostname := host.GetName()
		stepName := fmt.Sprintf("SetHostnameFor_%s", hostname)
		nodeName := fmt.Sprintf("SetHostnameFor_%s_node", hostname)

		setHostnameStep := osstep.NewSetHostnameStepBuilder(*runtimeCtx, stepName, hostname).Build()
		node := &plan.ExecutionNode{Name: nodeName, Step: setHostnameStep, Hosts: []connector.Host{host}}
		fragment.AddNode(node)
	}

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
