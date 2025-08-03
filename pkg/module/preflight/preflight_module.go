package preflight

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/module"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/task"
	"github.com/mensylisir/kubexm/pkg/task/preflight"
)

type PreflightModule struct {
	module.Base
}

func NewPreflightModule(ctx *module.ModuleContext) (module.Interface, error) {
	s := &PreflightModule{
		Base: module.Base{
			Name: "Preflight",
			Desc: "Run preflight checks and prepare OS on all nodes",
		},
	}

	// The tasks this module will orchestrate.
	tasks := []task.Interface{
		preflight.NewGatherFactsTask(ctx),
		preflight.NewSystemChecksTask(ctx),
		preflight.NewPrepareOSTask(ctx),
	}

	s.SetTasks(tasks)
	return s, nil
}

func (m *PreflightModule) Execute(ctx *module.ModuleContext) (*plan.ExecutionGraph, error) {
	p := plan.NewExecutionGraph(fmt.Sprintf("Module: %s", m.Name))

	var lastTaskExitNodes []plan.NodeID

	for _, t := range m.GetTasks() {
		// Check if the task is required for the current execution context.
		// This logic would be more complex in a real scenario.
		// For preflight, all tasks are usually required.

		taskGraph, err := t.Execute(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to execute task %s in module %s: %w", t.GetName(), m.Name, err)
		}

		if taskGraph.IsEmpty() {
			continue
		}

		// Merge the task's graph into the module's graph.
		p.Merge(taskGraph)

		// Create dependencies: the entry points of the current task graph
		// should depend on the exit points of the previous task graph.
		if len(lastTaskExitNodes) > 0 {
			for _, entryNodeID := range taskGraph.EntryNodes {
				for _, depNodeID := range lastTaskExitNodes {
					p.AddDependency(depNodeID, entryNodeID)
				}
			}
		}

		lastTaskExitNodes = taskGraph.ExitNodes
	}

	p.CalculateEntryAndExitNodes()
	return p, nil
}
