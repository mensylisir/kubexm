package preflight

import (
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	preflightstep "github.com/mensylisir/kubexm/pkg/step/preflight"
	"github.com/mensylisir/kubexm/pkg/task"
)

type CheckDNSTask struct {
	task.Base
}

func NewCheckDNSTask() task.Task {
	return &CheckDNSTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "CheckDNS",
				Description: "Check for a valid DNS configuration on the host",
			},
		},
	}
}

func (t *CheckDNSTask) Name() string {
	return t.Meta.Name
}

func (t *CheckDNSTask) Description() string {
	return t.Meta.Description
}

func (t *CheckDNSTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return true, nil
}

func (t *CheckDNSTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	allHosts := ctx.GetHostsByRole("")
	if len(allHosts) == 0 {
		return fragment, nil
	}

	checkDNSStep := preflightstep.NewCheckDNSConfigStepBuilder(*runtimeCtx, "CheckDNSConfig").Build()

	node := &plan.ExecutionNode{
		Name:  "CheckDNSConfig",
		Step:  checkDNSStep,
		Hosts: allHosts,
	}

	fragment.AddNode(node)
	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
