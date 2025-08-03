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

	tasks := []task.Interface{
		preflight.NewGatherFactsTask(ctx),
		preflight.NewSystemChecksTask(ctx),
		preflight.NewPrepareOSTask(ctx),
	}

	s.SetTasks(tasks)
	return s, nil
}

func (m *PreflightModule) Plan(ctx module.ModuleContext) (*plan.ExecutionGraph, error) {
	p := plan.NewExecutionGraph(fmt.Sprintf("Module: %s", m.Name))

	var lastTaskExitNodes []plan.NodeID

	for _, t := range m.GetTasks() {
		// In a real pipeline, we might check if a task is required.
		// isRequired, err := t.IsRequired(ctx) ...

		taskGraph, err := t.Plan(ctx) // Call Plan, not Execute
		if err != nil {
			return nil, fmt.Errorf("failed to plan task %s in module %s: %w", t.GetName(), m.Name, err)
		}

		if taskGraph.IsEmpty() {
			continue
		}

		p.Merge(taskGraph)

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
