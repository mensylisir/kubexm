// Package task provides task scheduling and execution utilities
package task

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
)

// TaskScheduler is responsible for scheduling and executing tasks
type TaskScheduler struct {
	executor *TaskExecutor
}

// NewTaskScheduler creates a new TaskScheduler
func NewTaskScheduler() *TaskScheduler {
	return &TaskScheduler{
		executor: NewTaskExecutor(),
	}
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
			if ts.executor.AreDependenciesMet(task, executionState) {
				// Execute the task
				fragment, err := ts.executor.ExecuteTask(task, ctx)
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

// ValidateTaskDependencies validates that all task dependencies are resolvable
func (ts *TaskScheduler) ValidateTaskDependencies(tasks []Task) error {
	// Create a map of task names for quick lookup
	taskNames := make(map[string]bool)
	for _, task := range tasks {
		taskNames[task.Name()] = true
	}

	// Check each task's dependencies
	for _, task := range tasks {
		if enhancedTask, ok := task.(EnhancedTask); ok {
			for _, dep := range enhancedTask.GetDependencies() {
				if !taskNames[dep] {
					return fmt.Errorf("task %s has unresolved dependency: %s", task.Name(), dep)
				}
			}
		}
	}

	return nil
}

// GetDataFlowAnalysis analyzes the data flow between tasks
func (ts *TaskScheduler) GetDataFlowAnalysis(tasks []Task) (map[string][]string, map[string][]string) {
	produces := make(map[string][]string) // task -> data it produces
	consumes := make(map[string][]string) // task -> data it consumes

	for _, task := range tasks {
		taskName := task.Name()
		
		// Get data produced by this task
		if enhancedTask, ok := task.(EnhancedTask); ok {
			produces[taskName] = enhancedTask.DeclareOutputData()
			consumes[taskName] = enhancedTask.DeclareInputData()
		}
	}

	return produces, consumes
}