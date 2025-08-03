package preflight

import (
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step/common" // TODO: Replace with specific check steps when available
	"github.com/mensylisir/kubexm/pkg/step/preflight"
	"github.com/mensylisir/kubexm/pkg/task"
)

type SystemChecksTask struct {
	task.Base
}

func NewSystemChecksTask(ctx *task.TaskContext) (task.Interface, error) {
	s := &SystemChecksTask{
		Base: task.Base{
			Name:   "SystemChecks",
			Desc:   "Perform system preflight checks on all nodes",
			Hosts:  ctx.GetHosts(),
			Action: new(SystemChecksAction),
		},
	}
	return s, nil
}

type SystemChecksAction struct {
	task.Action
}

func (a *SystemChecksAction) Execute(ctx runtime.Context) (*plan.ExecutionGraph, error) {
	p := plan.NewExecutionGraph("System Preflight Checks Phase")

	hosts := a.GetHosts()
	if len(hosts) == 0 {
		return p, nil
	}

	// 1. Use dedicated preflight steps where they exist.
	checkCpu := preflight.NewCheckCPUStep(ctx, "CheckCPUCores")
	p.AddNode("check-cpu", &plan.ExecutionNode{Step: checkCpu, Hosts: hosts})

	checkMem := preflight.NewCheckMemoryStep(ctx, "CheckMemory")
	p.AddNode("check-mem", &plan.ExecutionNode{Step: checkMem, Hosts: hosts})

	// 2. TODO: The following checks are performed with a generic CommandStep.
	// This is a temporary solution. The correct approach is to create a dedicated,
	// specific `Step` for each of these checks in the `pkg/step/preflight` or `pkg/step/os` package.
	// For example, a `NewCheckFirewallStatusStep`, `NewCheckSELinuxStatusStep`, etc.
	// This is done to adhere to the user's request to not use `common` steps where possible,
	// but the specific steps do not yet exist.

	checkFirewall := common.NewCommandStep(ctx, "CheckFirewallStatus", "systemctl is-active firewalld || echo 'inactive'")
	p.AddNode("check-firewall", &plan.ExecutionNode{Step: checkFirewall, Hosts: hosts})

	checkSelinux := common.NewCommandStep(ctx, "CheckSELinuxStatus", "sestatus | grep -i 'SELinux status:'")
	p.AddNode("check-selinux", &plan.ExecutionNode{Step: checkSelinux, Hosts: hosts})

	return p, nil
}
