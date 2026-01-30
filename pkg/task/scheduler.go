// Package task provides task scheduling and execution utilities
package task

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
)

// TaskScheduler is responsible for scheduling tasks
type TaskScheduler struct{}

// NewTaskScheduler creates a new TaskScheduler
func NewTaskScheduler() *TaskScheduler {
	return &TaskScheduler{}
}

// ScheduleTasks schedules and executes a list of tasks considering their dependencies
func (ts *TaskScheduler) ScheduleTasks(tasks []Task, ctx runtime.TaskContext) ([]*plan.ExecutionFragment, error) {
	// Track execution state
	executionState := make(map[string]bool)
	fragments := make([]*plan.ExecutionFragment, 0)

	// Keep track of tasks that still need to be executed
	pendingTasks := make([]Task, len(tasks))
	copy(pendingTasks, tasks)

	// Process tasks until all are executed or we detect a deadlock
	for len(pendingTasks) > 0 {
		executedInThisRound := false

		// Iterate through pending tasks
		for i := 0; i < len(pendingTasks); {
			task := pendingTasks[i]

			// Check if dependencies are met
			if ts.AreDependenciesMet(task, executionState) {
				// Execute task
				fragment, err := task.Plan(ctx)
				if err != nil {
					return nil, fmt.Errorf("failed to execute task %s: %w", task.Name(), err)
				}

				// Mark task as completed
				executionState[task.Name()] = true
				fragments = append(fragments, fragment)

				// Remove task from pending list
				pendingTasks = append(pendingTasks[:i], pendingTasks[i+1:]...)
				executedInThisRound = true
			} else {
				// Move to next task
				i++
			}
		}

		// If no tasks were executed in this round, we have a deadlock
		if !executedInThisRound {
			unresolvedTasks := make([]string, 0)
			for _, task := range pendingTasks {
				unresolvedTasks = append(unresolvedTasks, task.Name())
			}
			return nil, fmt.Errorf("deadlock detected: unable to resolve dependencies for tasks: %v", unresolvedTasks)
		}
	}

	return fragments, nil
}

// AreDependenciesMet checks if all task dependencies are satisfied
func (ts *TaskScheduler) AreDependenciesMet(task Task, executionState map[string]bool) bool {
	if extendedTask, ok := task.(ExtendedTask); ok {
		for _, dep := range extendedTask.GetDependencies() {
			if !executionState[dep] {
				return false
			}
		}
	}
	return true
}

// ValidateTaskDependencies validates that all task dependencies are resolvable
func (ts *TaskScheduler) ValidateTaskDependencies(tasks []Task) error {
	// Create a map of task names for quick lookup
	taskNames := make(map[string]bool)
	for _, task := range tasks {
		taskNames[task.Name()] = true
	}

	// Check each task's dependencies
	for _, task := range tasks {
		if extendedTask, ok := task.(ExtendedTask); ok {
			for _, dep := range extendedTask.GetDependencies() {
				if !taskNames[dep] {
					return fmt.Errorf("task %s has unresolved dependency: %s", task.Name(), dep)
				}
			}
		}
	}

	return nil
}

// GetDataFlowAnalysis analyzes data flow between tasks
func (ts *TaskScheduler) GetDataFlowAnalysis(tasks []Task) (map[string][]string, map[string][]string) {
	produces := make(map[string][]string) // task -> data it produces
	consumes := make(map[string][]string) // task -> data it consumes

	for _, task := range tasks {
		_ = task.Name()

		// Get data produced by this task
		if _, ok := task.(ExtendedTask); ok {
			// ExtendedTask doesn't have DeclareOutputData or DeclareInputData methods
			// These methods don't exist, so we skip this for now
		}
	}

	return produces, consumes
}
