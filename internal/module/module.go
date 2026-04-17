package module

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/task"
)

type BaseModule struct {
	Meta        spec.ModuleMeta
	Timeout     time.Duration
	IgnoreError bool
	ModuleTasks []task.Task
}

func NewBaseModule(name string, tasks []task.Task) BaseModule {
	return BaseModule{
		Meta: spec.ModuleMeta{
			Name: name,
		},
		ModuleTasks: tasks,
	}
}

func (b *BaseModule) Name() string {
	return b.Meta.Name
}

func (b *BaseModule) Description() string {
	return b.Meta.Description
}

func (b *BaseModule) Tasks() []task.Task {
	return b.ModuleTasks
}

func (b *BaseModule) GetBase() *BaseModule {
	return b
}

func (b *BaseModule) Plan(ctx runtime.ModuleContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(b.Name())
	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}

// PlanTasks iterates over all ModuleTasks, calls IsRequired() on each, then Plan()
// only for tasks that are required, and merges the results.
// This implements the documented contract: tasks are conditionally planned based on IsRequired().
func (b *BaseModule) PlanTasks(ctx runtime.ModuleContext) (*plan.ExecutionFragment, map[string]interface{}, error) {
	fragment := plan.NewExecutionFragment(b.Name() + "-Fragment")

	taskCtx, ok := ctx.(runtime.TaskContext)
	if !ok {
		return nil, nil, fmt.Errorf("module context cannot be asserted to task context for module %s", b.Name())
	}

	for _, t := range b.ModuleTasks {
		required, err := t.IsRequired(taskCtx)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to evaluate IsRequired for task %s: %w", t.Name(), err)
		}
		if !required {
			continue
		}
		taskFragment, err := t.Plan(taskCtx)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to plan task %s: %w", t.Name(), err)
		}
		if err := fragment.MergeFragment(taskFragment); err != nil {
			return nil, nil, fmt.Errorf("failed to merge fragment from task %s: %w", t.Name(), err)
		}
	}

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil, nil
}

// PlanSingleTask plans a single task and returns the execution fragment.
func (b *BaseModule) PlanSingleTask(ctx runtime.TaskContext, t task.Task) (*plan.ExecutionFragment, error) {
	fragment, err := t.Plan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to plan task %s: %w", t.Name(), err)
	}
	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
