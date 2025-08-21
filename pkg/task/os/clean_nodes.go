package os

import (
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	osstep "github.com/mensylisir/kubexm/pkg/step/os"
	"github.com/mensylisir/kubexm/pkg/task"
)

type CleanOSNodesTask struct {
	task.Base
}

func NewCleanOSNodesTask() task.Task {
	return &CleanOSNodesTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "CleanOSNodes",
				Description: "Clean up OS settings on all nodes (remove hosts entries, re-enable services, etc.)",
			},
		},
	}
}

func (t *CleanOSNodesTask) Name() string {
	return t.Meta.Name
}

func (t *CleanOSNodesTask) Description() string {
	return t.Meta.Description
}

func (t *CleanOSNodesTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return true, nil
}

func (t *CleanOSNodesTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	allHosts := ctx.GetHostsByRole("")
	if len(allHosts) == 0 {
		return fragment, nil
	}

	enableSwap := osstep.NewEnableSwapStepBuilder(*runtimeCtx, "EnableSwap").Build()
	enableSelinux := osstep.NewEnableSelinuxStepBuilder(*runtimeCtx, "EnableSelinux").Build()
	enableFirewall := osstep.NewEnableFirewallStepBuilder(*runtimeCtx, "EnableFirewall").Build()
	removeEtcHosts := osstep.NewRemoveEtcHostsStepBuilder(*runtimeCtx, "RemoveEtcHosts").Build()
	removeKernelModules := osstep.NewRemoveKernelModulesStepBuilder(*runtimeCtx, "RemoveKernelModules").Build()
	removeSysctl := osstep.NewRemoveSysctlStepBuilder(*runtimeCtx, "RemoveSysctl").Build()
	removeSecurityLimits := osstep.NewRemoveSecurityLimitsStepBuilder(*runtimeCtx, "RemoveSecurityLimits").Build()

	fragment.AddNode(&plan.ExecutionNode{Name: "EnableSwap", Step: enableSwap, Hosts: allHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "EnableSelinux", Step: enableSelinux, Hosts: allHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "EnableFirewall", Step: enableFirewall, Hosts: allHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "RemoveEtcHosts", Step: removeEtcHosts, Hosts: allHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "RemoveKernelModules", Step: removeKernelModules, Hosts: allHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "RemoveSysctl", Step: removeSysctl, Hosts: allHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "RemoveSecurityLimits", Step: removeSecurityLimits, Hosts: allHosts})

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
