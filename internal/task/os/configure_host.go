package os

import (
	"fmt"

	"github.com/mensylisir/kubexm/internal/remotefw"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	osstep "github.com/mensylisir/kubexm/internal/step/os"
	"github.com/mensylisir/kubexm/internal/task"
)

type ConfigureHostTask struct {
	task.Base
}

func NewConfigureHostTask() task.Task {
	return &ConfigureHostTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "ConfigureHost",
				Description: "Set hostname and update /etc/hosts on all nodes",
			},
		},
	}
}

func (t *ConfigureHostTask) Name() string {
	return t.Meta.Name
}

func (t *ConfigureHostTask) Description() string {
	return t.Meta.Description
}

func (t *ConfigureHostTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return true, nil
}

func (t *ConfigureHostTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	runtimeCtx := ctx.ForTask(t.Name())

	allHosts := ctx.GetHostsByRole("")
	if len(allHosts) == 0 {
		return fragment, nil
	}

	// Step 1: Set Hostname for each host individually
	var setHostnameExitNodes []plan.NodeID
	for _, host := range allHosts {
		hostname := host.GetName()
		stepName := fmt.Sprintf("SetHostnameFor_%s", hostname)
		nodeName := fmt.Sprintf("SetHostnameFor_%s_node", hostname)

		setHostnameStep, err := osstep.NewSetHostnameStepBuilder(runtimeCtx, stepName, hostname).Build()
		if err != nil {
			return nil, err
		}
		node := &plan.ExecutionNode{Name: nodeName, Step: setHostnameStep, Hosts: []remotefw.Host{host}}
		nodeID, _ := fragment.AddNode(node)
		setHostnameExitNodes = append(setHostnameExitNodes, nodeID)
	}

	// Step 2: Update /etc/hosts on all nodes, depending on all hostnames being set
	updateEtcHostsStep, err := osstep.NewUpdateEtcHostsStepBuilder(runtimeCtx, "UpdateEtcHosts").Build()
	if err != nil {
		return nil, err
	}
	updateEtcHostsNode := &plan.ExecutionNode{Name: "UpdateEtcHosts", Step: updateEtcHostsStep, Hosts: allHosts}
	updateEtcHostsNodeID, _ := fragment.AddNode(updateEtcHostsNode)

	for _, exitNodeID := range setHostnameExitNodes {
		fragment.AddDependency(exitNodeID, updateEtcHostsNodeID)
	}

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
