package preflight

import (
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	preflightstep "github.com/mensylisir/kubexm/pkg/step/preflight"
	"github.com/mensylisir/kubexm/pkg/task"
)

type CheckHostConnectivityTask struct {
	task.Base
}

func NewCheckHostConnectivityTask() task.Task {
	return &CheckHostConnectivityTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "CheckHostConnectivity",
				Description: "Check network connectivity between all nodes",
			},
		},
	}
}

func (t *CheckHostConnectivityTask) Name() string {
	return t.Meta.Name
}

func (t *CheckHostConnectivityTask) Description() string {
	return t.Meta.Description
}

func (t *CheckHostConnectivityTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return true, nil
}

func (t *CheckHostConnectivityTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	allHosts := ctx.GetHostsByRole("")
	if len(allHosts) == 0 {
		return fragment, nil
	}

	checkConnectivityStep := preflightstep.NewCheckHostConnectivityStepBuilder(*runtimeCtx, "CheckHostConnectivity").Build()

	node := &plan.ExecutionNode{
		Name:  "CheckHostConnectivity",
		Step:  checkConnectivityStep,
		Hosts: allHosts,
	}

	fragment.AddNode(node)
	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
