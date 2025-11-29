package os

import (
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	osstep "github.com/mensylisir/kubexm/pkg/step/os"
	"github.com/mensylisir/kubexm/pkg/task"
)

type DisableServicesTask struct {
	task.Base
}

func NewDisableServicesTask() task.Task {
	return &DisableServicesTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "DisableServices",
				Description: "Disable swap, firewall, and SELinux on all nodes",
			},
		},
	}
}

func (t *DisableServicesTask) Name() string {
	return t.Meta.Name
}

func (t *DisableServicesTask) Description() string {
	return t.Meta.Description
}

func (t *DisableServicesTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return true, nil
}

func (t *DisableServicesTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	allHosts := ctx.GetHostsByRole("")
	if len(allHosts) == 0 {
		return fragment, nil
	}

	disableSwapStep := osstep.NewDisableSwapStepBuilder(runtimeCtx, "DisableSwap").Build()
	disableSelinuxStep := osstep.NewDisableSelinuxStepBuilder(runtimeCtx, "DisableSelinux").Build()
	disableFirewallStep := osstep.NewDisableFirewallStepBuilder(runtimeCtx, "DisableFirewall").Build()

	// These steps can run in parallel
	fragment.AddNode(&plan.ExecutionNode{Name: "DisableSwap", Step: disableSwapStep, Hosts: allHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "DisableSelinux", Step: disableSelinuxStep, Hosts: allHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "DisableFirewall", Step: disableFirewallStep, Hosts: allHosts})

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
