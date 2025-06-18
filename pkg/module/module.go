package module

import (
	"context"
	"fmt"
	// "golang.org/x/sync/errgroup" // Not used for sequential task execution within a module

	"github.com/kubexms/kubexms/pkg/config"
	"github.com/kubexms/kubexms/pkg/logger" // For logger.Logger type
	"github.com/kubexms/kubexms/pkg/runtime"
	"github.com/kubexms/kubexms/pkg/step"
	"github.com/kubexms/kubexms/pkg/task"
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
	// typically based on the provided cluster configuration.
	// If nil, the module is considered always enabled.
	IsEnabled func(clusterCfg *config.Cluster) bool

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

// selectHostsForTask filters the full list of hosts in the ClusterRuntime
// based on the rules defined in the given Task (RunOnRoles and Filter).
func selectHostsForTask(cluster *runtime.ClusterRuntime, t *task.Task) []*runtime.Host {
	var selectedHosts []*runtime.Host

	if cluster == nil || len(cluster.Hosts) == 0 {
		// Attempt to get a logger even if cluster is nil, for this specific message
		var log *logger.Logger
		if cluster != nil && cluster.Logger != nil { log = cluster.Logger } else { log = logger.Get() }
		log.Debugf("selectHostsForTask: No hosts in cluster to filter from.")
		return selectedHosts
	}
	if t == nil {
		cluster.Logger.Warnf("selectHostsForTask: Task is nil, cannot select hosts.")
		return selectedHosts
	}

	// Use a logger that includes task context if possible
	taskSelectionLogger := cluster.Logger.SugaredLogger
	if t.Name != "" {
		taskSelectionLogger = taskSelectionLogger.With("task_for_host_selection", t.Name)
	}
	taskSelectionLogger.Debugf("Selecting hosts (Roles: %v, HasFilter: %v)", t.RunOnRoles, t.Filter != nil)

	for _, host := range cluster.Hosts {
		roleMatch := false
		if len(t.RunOnRoles) == 0 {
			roleMatch = true
		} else {
			for _, requiredRole := range t.RunOnRoles {
				if host.HasRole(requiredRole) {
					roleMatch = true
					break
				}
			}
		}

		if !roleMatch {
			taskSelectionLogger.Debugf("Host '%s' skipped: role mismatch (host roles: %v, task needs: %v)", host.Name, host.Roles, t.RunOnRoles)
			continue
		}

		filterMatch := true
		if t.Filter != nil {
			filterMatch = t.Filter(host)
			if !filterMatch {
				taskSelectionLogger.Debugf("Host '%s' skipped: custom filter returned false", host.Name, t.Name)
			}
		}

		if filterMatch {
			selectedHosts = append(selectedHosts, host)
		}
	}
	taskSelectionLogger.Debugf("Selected %d hosts: %v", len(selectedHosts), func() []string {
		names := make([]string, len(selectedHosts))
		for i, h := range selectedHosts { names[i] = h.Name }
		return names
	}())
	return selectedHosts
}


// Run executes all tasks defined in the Module sequentially.
// It first checks IsEnabled (if defined). If not enabled, it skips execution.
// It runs PreRun and PostRun hooks if they are defined.
// If a task fails (and its IgnoreError is false), subsequent tasks in this module are skipped.
//
// Parameters:
//   - goCtx: The parent Go context, for cancellation and deadlines.
//   - cluster: A pointer to the global *runtime.ClusterRuntime.
//
// Returns:
//   - []*step.Result: An aggregated slice of results from all steps in all tasks executed by this module.
//   - error: The first critical error encountered (from PreRun or a non-ignored Task failure).
func (m *Module) Run(goCtx context.Context, cluster *runtime.ClusterRuntime) ([]*step.Result, error) {
	if cluster == nil || cluster.Logger == nil {
		// Fallback to global logger if cluster or its logger is nil. This is defensive.
		tempLogger := logger.Get().SugaredLogger.With("module", m.Name).Sugar()
		tempLogger.Errorf("Module.Run called with nil cluster or cluster.Logger. This is unexpected.")
		// Proceeding with a temporary logger, but this indicates an issue in the calling code.
		if cluster == nil {
		    return nil, fmt.Errorf("module '%s' Run called with nil ClusterRuntime", m.Name)
		}
		// If only cluster.Logger was nil, we can assign the tempLogger for the module's operations.
		// However, this suggests an incomplete ClusterRuntime setup.
		// For now, let's assume cluster.Logger is always initialized by NewRuntime.
	}
	moduleLogger := cluster.Logger.SugaredLogger.With("module", m.Name).Sugar()
	moduleLogger.Infof("Starting module execution...")

	if m.IsEnabled != nil && !m.IsEnabled(cluster.ClusterConfig) {
		moduleLogger.Infof("Module is disabled by configuration, skipping.")
		return nil, nil
	}

	var allStepResults []*step.Result
	var moduleExecError error

	// Execute PreRun hook
	if m.PreRun != nil {
		moduleLogger.Debugf("Executing PreRun hook...")
		if err := m.PreRun(cluster); err != nil {
			moduleLogger.Errorf("PreRun hook failed: %v. Halting module.", err)
			moduleExecError = fmt.Errorf("module '%s' PreRun hook failed: %w", m.Name, err)
			// Call PostRun even if PreRun fails
			if m.PostRun != nil {
				moduleLogger.Debugf("Executing PostRun hook after PreRun failure...")
				if postRunErr := m.PostRun(cluster, moduleExecError); postRunErr != nil {
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
		taskLogger := moduleLogger.With("task_name", currentTask.Name, "task_index", fmt.Sprintf("%d/%d", i+1, len(m.Tasks))).Sugar()

		targetHosts := selectHostsForTask(cluster, currentTask)
		if len(targetHosts) == 0 {
			taskLogger.Infof("No target hosts for task, skipping.")
			continue
		}
		taskLogger.Infof("Running task on %d hosts...", len(targetHosts))

		taskStepResults, taskErr := currentTask.Run(goCtx, targetHosts, cluster)
		if taskStepResults != nil {
			allStepResults = append(allStepResults, taskStepResults...)
		}

		if taskErr != nil {
			taskLogger.Errorf("Task execution failed: %v", taskErr)
			if !currentTask.IgnoreError {
				moduleLogger.Errorf("Task '%s' failed critically. Halting module.", currentTask.Name)
				moduleExecError = fmt.Errorf("module '%s' task '%s' failed: %w", m.Name, currentTask.Name, taskErr)
				break
			} else {
				taskLogger.Warnf("Task '%s' failed, but error is ignored. Continuing module.", currentTask.Name)
			}
		} else {
			taskLogger.Successf("Task completed successfully.")
		}
	}

	// Execute PostRun hook
	if m.PostRun != nil {
		moduleLogger.Debugf("Executing PostRun hook...")
		if err := m.PostRun(cluster, moduleExecError); err != nil {
			moduleLogger.Errorf("PostRun hook failed: %v", err)
			if moduleExecError == nil { // Only set PostRun error if no prior critical error
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
