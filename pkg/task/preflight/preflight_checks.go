package preflight

import (
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	preflightstep "github.com/mensylisir/kubexm/pkg/step/preflight"
	"github.com/mensylisir/kubexm/pkg/task"
)

type PreflightChecksTask struct {
	task.Base
}

func NewPreflightChecksTask() task.Task {
	return &PreflightChecksTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "PreflightChecks",
				Description: "Run preflight checks (CPU, memory, etc.) on all nodes before installation",
			},
		},
	}
}

func (t *PreflightChecksTask) Name() string {
	return t.Meta.Name
}

func (t *PreflightChecksTask) Description() string {
	return t.Meta.Description
}

func (t *PreflightChecksTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	if ctx.GetClusterConfig().Spec.Preflight != nil {
		for _, check := range ctx.GetClusterConfig().Spec.Preflight.SkipChecks {
			if check == "all" {
				ctx.GetLogger().Info("Skipping all preflight checks because 'all' is in skipChecks list.")
				return false, nil
			}
		}
	}
	return true, nil
}

func (t *PreflightChecksTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	allHosts := ctx.GetHostsByRole("")
	if len(allHosts) == 0 {
		// If there are no hosts, there's nothing to check. Return an empty fragment.
		return fragment, nil
	}

	// The control node is where some cluster-wide checks might run
	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}

	// Create builders for all the preflight steps
	checkCPU := preflightstep.NewCheckCPUStepBuilder(*runtimeCtx, "CheckMinCPUCores").Build()
	checkMemory := preflightstep.NewCheckMemoryStepBuilder(*runtimeCtx, "CheckMinMemory").Build()
	checkConnectivity := preflightstep.NewCheckHostConnectivityStepBuilder(*runtimeCtx, "CheckHostConnectivity").Build()
	checkDNS := preflightstep.NewCheckDNSConfigStepBuilder(*runtimeCtx, "CheckDNSConfig").Build()
	checkCommands := preflightstep.NewCheckRequiredCommandsStepBuilder(*runtimeCtx, "CheckRequiredCommands").Build()
	checkTimeSync := preflightstep.NewCheckTimeSyncStepBuilder(*runtimeCtx, "CheckTimeSync").Build()
	lintSpec := preflightstep.NewLintClusterSpecStepBuilder(*runtimeCtx, "LintClusterSpec").Build()

	// Add nodes to the execution fragment for each check.
	// Most checks run on all hosts. Linting the spec might only need to run on the control node.
	fragment.AddNode(&plan.ExecutionNode{Name: "CheckMinCPUCores", Step: checkCPU, Hosts: allHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "CheckMinMemory", Step: checkMemory, Hosts: allHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "CheckHostConnectivity", Step: checkConnectivity, Hosts: allHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "CheckDNSConfig", Step: checkDNS, Hosts: allHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "CheckRequiredCommands", Step: checkCommands, Hosts: allHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "CheckTimeSync", Step: checkTimeSync, Hosts: allHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "LintClusterSpec", Step: lintSpec, Hosts: []connector.Host{controlNode}})

	// Since all these checks can run in parallel, we don't add any dependencies between them.
	// The CalculateEntryAndExitNodes function will correctly identify all of them as entry points.
	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
