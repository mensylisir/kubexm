package pre

import (
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/task"
)

type CreateRepositoryTask struct {
	task.Base
}

func NewCreateRepositoryTask() task.Task {
	return &CreateRepositoryTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "CreateRepository",
				Description: "Create a local package repository for offline installation (Not Yet Implemented)",
			},
		},
	}
}

func (t *CreateRepositoryTask) Name() string {
	return t.Meta.Name
}

func (t *CreateRepositoryTask) Description() string {
	return t.Meta.Description
}

func (t *CreateRepositoryTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	// This task would only be required in offline mode.
	return ctx.IsOfflineMode(), nil
}

func (t *CreateRepositoryTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	ctx.GetLogger().Info("Task 'CreateRepository' is not yet implemented. Skipping.")
	// Return an empty fragment.
	return plan.NewExecutionFragment(t.Name()), nil
}
