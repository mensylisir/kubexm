package module

import (
	"context"
	"fmt"
	// "golang.org/x/sync/errgroup" // Not used for sequential task execution within a module

	// "github.com/kubexms/kubexms/pkg/config" // No longer needed directly
	"github.com/mensylisir/kubexm/pkg/logger" // For logger.Logger type
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/task"
)

// Module represents a collection of related Tasks that manage the lifecycle
// or a significant functional aspect of a software component (e.g., EtcdModule, ContainerdModule).
// Modules are orchestrated by a Pipeline.
type Module struct {
	// Name is a descriptive name for the module, used for logging and identification.
	Name string

	// Tasks is an ordered slice of *task.Task pointers to be executed by this module.
	// The order defines the sequence of execution.
	Tasks []*task.Task

	// IsEnabled is a function that determines if this module should be run,
	// typically based on the provided cluster configuration (via ClusterRuntime).
	// If nil, the module is considered always enabled.
	IsEnabled func(clusterRt *runtime.ClusterRuntime) bool

	// PreRun is a hook executed once before any tasks in the module are run.
	// If it returns an error, the module's execution (including tasks and PostRun) is halted.
	PreRun func(cluster *runtime.ClusterRuntime) error

	// PostRun is a hook executed once after all tasks in the module have attempted to run,
	// or after a PreRun failure, or after a task failure that halts the module.
	// It receives the ClusterRuntime and any error that occurred during the module's
	// main execution phase (from PreRun or a critical task).
	// Errors from PostRun are typically logged but usually do not override the primary module error.
	PostRun func(cluster *runtime.ClusterRuntime, moduleExecError error) error
}

// Run executes all tasks defined in the Module sequentially.
// It first checks IsEnabled (if defined). If not enabled, it skips execution.
// It runs PreRun and PostRun hooks if they are defined.
// If a task fails (and its IgnoreError is false), subsequent tasks in this module are skipped.
//
// Parameters:
//   - moduleParentCtx: The parent runtime.Context (typically from a Pipeline or global).
//
// Returns:
//   - []*step.Result: An aggregated slice of results from all steps in all tasks executed by this module.
//   - error: The first critical error encountered (from PreRun or a non-ignored Task failure).
func (m *Module) Run(moduleParentCtx runtime.Context) ([]*step.Result, error) {
	if moduleParentCtx.Cluster == nil {
		// Use moduleParentCtx.Logger if available, otherwise global logger
		tempLogger := logger.Get().SugaredLogger.With("module", m.Name).Sugar()
		if moduleParentCtx.Logger != nil {
			tempLogger = moduleParentCtx.Logger.SugaredLogger.With("module", m.Name).Sugar()
		}
		tempLogger.Errorf("Module.Run called with a Context that has a nil ClusterRuntime. This is unexpected.")
		return nil, fmt.Errorf("module '%s' Run called with a Context that has a nil ClusterRuntime", m.Name)
	}
	if moduleParentCtx.Logger == nil {
		// This case should ideally not happen if moduleParentCtx.Cluster is not nil,
		// as ClusterRuntime should have a logger. Defensive check.
		moduleParentCtx.Logger = logger.Get() // Fallback to global logger
		moduleParentCtx.Logger.Warnf("Module.Run for '%s' received a Context with nil Logger, falling back to global logger.", m.Name)
	}

	cluster := moduleParentCtx.Cluster
	goCtx := moduleParentCtx.GoContext // Use GoContext from moduleParentCtx

	moduleLogger := moduleParentCtx.Logger.SugaredLogger.With("module", m.Name).Sugar()
	moduleLogger.Infof("Starting module execution...")

	if m.IsEnabled != nil && !m.IsEnabled(cluster) { // Pass cluster (from moduleParentCtx.Cluster)
		moduleLogger.Infof("Module is disabled by configuration, skipping.")
		return nil, nil
	}

	var allStepResults []*step.Result
	var moduleExecError error

	// Execute PreRun hook
	if m.PreRun != nil {
		moduleLogger.Debugf("Executing PreRun hook...")
		if err := m.PreRun(cluster); err != nil { // Pass cluster
			moduleLogger.Errorf("PreRun hook failed: %v. Halting module.", err)
			moduleExecError = fmt.Errorf("module '%s' PreRun hook failed: %w", m.Name, err)
			if m.PostRun != nil {
				moduleLogger.Debugf("Executing PostRun hook after PreRun failure...")
				if postRunErr := m.PostRun(cluster, moduleExecError); postRunErr != nil { // Pass cluster
					moduleLogger.Errorf("PostRun hook also failed: %v (this error does not override PreRun error)", postRunErr)
				}
			}
			return nil, moduleExecError
		}
		moduleLogger.Debugf("PreRun hook completed successfully.")
	}

	// Execute Tasks sequentially
	for i, currentTask := range m.Tasks {
		if currentTask == nil {
			moduleLogger.Warnf("Skipping nil task at index %d in module '%s'.", i, m.Name)
			continue
		}

		// Create a new context for the task. Host is nil here; task.Run will create per-host contexts.
		taskCtx := runtime.NewHostContext(goCtx, nil, cluster) // Use goCtx from moduleParentCtx

		// Re-scope the logger for this specific task, inheriting from the module's logger.
		taskCtx.Logger = &logger.Logger{SugaredLogger: moduleParentCtx.Logger.SugaredLogger.With("task_name", currentTask.Name).Sugar()}

		// This task-scoped logger is for messages from the module about the task, not for the task itself.
		moduleTaskOpLogger := moduleLogger.With("task_name", currentTask.Name, "task_index", fmt.Sprintf("%d/%d", i+1, len(m.Tasks))).Sugar()

		// Host selection is now done within Task.Run.
		// The module no longer needs to know the exact number of hosts upfront.
		moduleTaskOpLogger.Infof("Attempting to run task...")

		// Pass the newly created taskCtx to task.Run. Task.Run will handle host selection.
		taskStepResults, taskErr := currentTask.Run(taskCtx)
		if taskStepResults != nil {
			allStepResults = append(allStepResults, taskStepResults...)
		}

		if taskErr != nil {
			moduleTaskOpLogger.Errorf("Task execution failed: %v", taskErr)
			if !currentTask.IgnoreError {
				moduleLogger.Errorf("Task '%s' failed critically. Halting module.", currentTask.Name) // Use moduleLogger for module-level decision
				moduleExecError = fmt.Errorf("module '%s' task '%s' failed: %w", m.Name, currentTask.Name, taskErr)
				break
			} else {
				moduleTaskOpLogger.Warnf("Task '%s' failed, but error is ignored. Continuing module.", currentTask.Name)
			}
		} else {
			moduleTaskOpLogger.Successf("Task completed successfully.")
		}
	}

	// Execute PostRun hook
	if m.PostRun != nil {
		moduleLogger.Debugf("Executing PostRun hook...")
		if err := m.PostRun(cluster, moduleExecError); err != nil { // Pass cluster
			moduleLogger.Errorf("PostRun hook failed: %v", err)
			if moduleExecError == nil {
				moduleExecError = fmt.Errorf("module '%s' PostRun hook failed: %w", m.Name, err)
			}
		} else {
			moduleLogger.Debugf("PostRun hook completed successfully.")
		}
	}

	if moduleExecError != nil {
		moduleLogger.Errorf("Module execution finished with error: %v", moduleExecError)
	} else {
		moduleLogger.Successf("Module execution finished successfully.")
	}

	return allStepResults, moduleExecError
}
