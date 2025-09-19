package preflight

import (
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	preflightstep "github.com/mensylisir/kubexm/pkg/step/preflight"
	"github.com/mensylisir/kubexm/pkg/task"
)

type CheckTimeSyncTask struct {
	task.Base
}

func NewCheckTimeSyncTask() task.Task {
	return &CheckTimeSyncTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "CheckTimeSync",
				Description: "Check if node time is synchronized via NTP",
			},
		},
	}
}

func (t *CheckTimeSyncTask) Name() string {
	return t.Meta.Name
}

func (t *CheckTimeSyncTask) Description() string {
	return t.Meta.Description
}

func (t *CheckTimeSyncTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return true, nil
}

func (t *CheckTimeSyncTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	allHosts := ctx.GetHostsByRole("")
	if len(allHosts) == 0 {
		return fragment, nil
	}

	checkTimeSyncStep := preflightstep.NewCheckTimeSyncStepBuilder(*runtimeCtx, "CheckTimeSync").Build()

	node := &plan.ExecutionNode{
		Name:  "CheckTimeSync",
		Step:  checkTimeSyncStep,
		Hosts: allHosts,
	}

	fragment.AddNode(node)
	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
