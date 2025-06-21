package module

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/runtime" // For runtime.ModuleContext
	"github.com/mensylisir/kubexm/pkg/task"    // For task.Task and task.ExecutionFragment
)

// BaseModule provides a common structure for modules.
// Concrete module types should embed BaseModule and implement the Plan method
// from the Module interface.
type BaseModule struct {
	ModuleName string      // Store the name of the module
	ModuleTasks []task.Task // Store the tasks associated with this module
}

// NewBaseModule creates a new BaseModule.
func NewBaseModule(name string, tasks []task.Task) BaseModule {
	return BaseModule{
		ModuleName:  name,
		ModuleTasks: tasks,
	}
}

// Name returns the name of the module.
func (bm *BaseModule) Name() string {
	return bm.ModuleName
}

// Tasks returns the list of tasks associated with this module.
func (bm *BaseModule) Tasks() []task.Task {
	// Return a copy to prevent external modification of the slice
	if bm.ModuleTasks == nil {
		return []task.Task{}
	}
	tasksCopy := make([]task.Task, len(bm.ModuleTasks))
	copy(tasksCopy, bm.ModuleTasks)
	return tasksCopy
}

// Plan is intended to be implemented by concrete module types.
// This base implementation returns an error, signaling it needs to be overridden.
func (bm *BaseModule) Plan(ctx runtime.ModuleContext) (*task.ExecutionFragment, error) {
	return nil, fmt.Errorf("Plan() not implemented for module: %s. Concrete module types must implement this method", bm.Name())
}

// Ensure BaseModule itself doesn't fully satisfy the Module interface
// because Plan is abstract here. A concrete module will.
// var _ Module = (*SomeConcreteModule)(nil) // This would be in a concrete module's file.
