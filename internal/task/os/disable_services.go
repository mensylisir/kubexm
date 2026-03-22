package os

import (
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	osstep "github.com/mensylisir/kubexm/internal/step/os"
	"github.com/mensylisir/kubexm/internal/task"
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

	disableSwapStep, err := osstep.NewDisableSwapStepBuilder(runtimeCtx, "DisableSwap").Build()
	if err != nil {
		return nil, err
	}
	disableSelinuxStep, err := osstep.NewDisableSelinuxStepBuilder(runtimeCtx, "DisableSelinux").Build()
	if err != nil {
		return nil, err
	}
	disableFirewallStep, err := osstep.NewDisableFirewallStepBuilder(runtimeCtx, "DisableFirewall").Build()
	if err != nil {
		return nil, err
	}

	// These steps can run in parallel
	fragment.AddNode(&plan.ExecutionNode{Name: "DisableSwap", Step: disableSwapStep, Hosts: allHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "DisableSelinux", Step: disableSelinuxStep, Hosts: allHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "DisableFirewall", Step: disableFirewallStep, Hosts: allHosts})

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
