package preflight

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/connector"

	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step/preflight"
	"github.com/mensylisir/kubexm/pkg/task"
)

type PreflightCheckTask struct {
	task.Base
}

func NewPreflightCheckTask() task.Task {
	return &PreflightCheckTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "PreflightCheck",
				Description: "Performs static configuration linting and dynamic node environment checks",
			},
		},
	}
}

func (t *PreflightCheckTask) Name() string {
	return t.Meta.Name
}

func (t *PreflightCheckTask) Description() string {
	return t.Meta.Description
}

func (t *PreflightCheckTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return true, nil
}

func (t *PreflightCheckTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	allHosts := ctx.GetHostsByRole("")
	if len(allHosts) == 0 {
		return nil, fmt.Errorf("no hosts found for task %s", t.Name())
	}

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}

	lintSpecStep := preflight.NewLintClusterSpecStepBuilder(*runtimeCtx, "LintClusterSpec").Build()
	checkTimeSyncStep := preflight.NewCheckTimeSyncStepBuilder(*runtimeCtx, "CheckTimeSync").Build()
	checkDNSConfigStep := preflight.NewCheckDNSConfigStepBuilder(*runtimeCtx, "CheckDNSConfig").Build()
	checkConnectivityStep := preflight.NewCheckHostConnectivityStepBuilder(*runtimeCtx, "CheckHostConnectivity").Build()
	checkCommandsStep := preflight.NewCheckRequiredCommandsStepBuilder(*runtimeCtx, "CheckRequiredCommands").Build()

	lintSpecNode := &plan.ExecutionNode{
		Name:  "LintClusterSpec",
		Step:  lintSpecStep,
		Hosts: []connector.Host{controlNode},
	}
	checkTimeSyncNode := &plan.ExecutionNode{
		Name:  "CheckTimeSyncOnAllNodes",
		Step:  checkTimeSyncStep,
		Hosts: allHosts,
	}
	checkDNSConfigNode := &plan.ExecutionNode{
		Name:  "CheckDNSConfigOnAllNodes",
		Step:  checkDNSConfigStep,
		Hosts: allHosts,
	}
	checkConnectivityNode := &plan.ExecutionNode{
		Name:  "CheckConnectivityBetweenNodes",
		Step:  checkConnectivityStep,
		Hosts: allHosts,
	}
	checkCommandsNode := &plan.ExecutionNode{
		Name:  "CheckCommandsOnAllNodes",
		Step:  checkCommandsStep,
		Hosts: allHosts,
	}

	lintSpecNodeID, _ := fragment.AddNode(lintSpecNode)
	checkTimeSyncNodeID, _ := fragment.AddNode(checkTimeSyncNode)
	checkDNSConfigNodeID, _ := fragment.AddNode(checkDNSConfigNode)
	checkConnectivityNodeID, _ := fragment.AddNode(checkConnectivityNode)
	checkCommandsNodeID, _ := fragment.AddNode(checkCommandsNode)

	fragment.AddDependency(lintSpecNodeID, checkTimeSyncNodeID)
	fragment.AddDependency(lintSpecNodeID, checkDNSConfigNodeID)
	fragment.AddDependency(lintSpecNodeID, checkConnectivityNodeID)
	fragment.AddDependency(lintSpecNodeID, checkCommandsNodeID)
	fragment.CalculateEntryAndExitNodes()

	return fragment, nil
}

var _ task.Task = (*PreflightCheckTask)(nil)
