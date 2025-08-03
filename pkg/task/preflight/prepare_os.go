package preflight

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step/os"
	"github.com/mensylisir/kubexm/pkg/step/preflight"
	"github.com/mensylisir/kubexm/pkg/task"
)

// PrepareOSTask performs essential OS-level configurations on target nodes.
type PrepareOSTask struct {
	task.Base
}

func NewPrepareOSTask(ctx *task.TaskContext) (task.Interface, error) {
	s := &PrepareOSTask{
		Base: task.Base{
			Name:   "PrepareOS",
			Desc:   "Prepare operating system for all nodes",
			Hosts:  ctx.GetHosts(),
			Action: new(PrepareOSAction),
		},
	}
	return s, nil
}

type PrepareOSAction struct {
	task.Action
}

func (a *PrepareOSAction) Execute(ctx runtime.Context) (*plan.ExecutionGraph, error) {
	p := plan.NewExecutionGraph("Prepare OS Phase")

	hosts := a.GetHosts()
	if len(hosts) == 0 {
		return p, nil
	}

	// Step 1: Gather facts first.
	gatherFacts := preflight.NewGatherFactsStep(ctx, "GatherFacts")
	p.AddNode("gather-facts", &plan.ExecutionNode{Step: gatherFacts, Hosts: hosts})

	// Step 2: A series of OS configuration steps that can run in parallel.
	osConfigDependencies := []plan.NodeID{"gather-facts"}
	var osConfigNodeIDs []plan.NodeID

	steps := map[string]plan.Step{
		"disable-selinux": os.NewDisableSelinuxStep(ctx, "DisableSelinux"),
		"disable-swap":    os.NewDisableSwapStep(ctx, "DisableSwap"),
		"disable-firewall": os.NewDisableFirewallStep(ctx, "DisableFirewall"),
		"add-modules":     os.NewAddModulesStep(ctx, "AddModules"),
		"add-sysctl":      os.NewAddSysctlStep(ctx, "AddSysctl"),
	}

	for name, s := range steps {
		nodeID := plan.NodeID(name)
		p.AddNode(nodeID, &plan.ExecutionNode{
			Step:         s,
			Hosts:        hosts,
			Dependencies: osConfigDependencies,
		})
		osConfigNodeIDs = append(osConfigNodeIDs, nodeID)
	}

	// Step 3: Hostname and /etc/hosts modification should happen after the main OS config.
	setHostname := os.NewSetHostnameStep(ctx, "SetHostname")
	p.AddNode("set-hostname", &plan.ExecutionNode{
		Step:         setHostname,
		Hosts:        hosts,
		Dependencies: osConfigNodeIDs,
	})

	addHosts := os.NewAddHostsStep(ctx, "AddHosts")
	p.AddNode("add-hosts", &plan.ExecutionNode{
		Step:         addHosts,
		Hosts:        hosts,
		Dependencies: []plan.NodeID{"set-hostname"}, // Depends on hostname being set
	})

	return p, nil
}
