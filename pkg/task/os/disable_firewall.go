package os

import (
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	osstep "github.com/mensylisir/kubexm/pkg/step/os"
	"github.com/mensylisir/kubexm/pkg/task"
)

type DisableFirewallTask struct {
	task.Base
}

func NewDisableFirewallTask() task.Task {
	return &DisableFirewallTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "DisableFirewall",
				Description: "Disable firewall on all nodes",
			},
		},
	}
}

func (t *DisableFirewallTask) Name() string {
	return t.Meta.Name
}

func (t *DisableFirewallTask) Description() string {
	return t.Meta.Description
}

func (t *DisableFirewallTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return *ctx.GetClusterConfig().Spec.Preflight.DisableFirewalld, nil
}

func (t *DisableFirewallTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	allHosts := ctx.GetHostsByRole("")
	if len(allHosts) == 0 {
		return fragment, nil
	}

	disableFirewallStep := osstep.NewDisableFirewallStepBuilder(*runtimeCtx, "DisableFirewall").Build()

	node := &plan.ExecutionNode{
		Name:  "DisableFirewall",
		Step:  disableFirewallStep,
		Hosts: allHosts,
	}

	fragment.AddNode(node)
	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
