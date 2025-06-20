package task

import (
	"github.com/mensylisir/kubexm/pkg/plan"    // Updated import path
	"github.com/mensylisir/kubexm/pkg/runtime" // Updated import path
)

type Task interface {
	Name() string
	IsRequired(ctx runtime.TaskContext) (bool, error)
	Plan(ctx runtime.TaskContext) (*plan.ExecutionPlan, error)
}
