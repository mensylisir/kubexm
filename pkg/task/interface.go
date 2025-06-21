package task

import (
	// Adjust these import paths based on your actual project structure
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
)

type Task interface {
	Name() string
	IsRequired(ctx runtime.TaskContext) (bool, error)
	Plan(ctx runtime.TaskContext) (*plan.ExecutionPlan, error)
}
