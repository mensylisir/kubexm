package task

import (
	"context"
	"fmt"
	"sync" // Required for mu in Task.Run if collecting results with a mutex
	"time" // Required for time.Now() in NewResult

	"golang.org/x/sync/errgroup"

	"github.com/kubexms/kubexms/pkg/logger" // For logger.Logger type in runtime.Context
	"github.com/kubexms/kubexms/pkg/runtime"
	"github.com/kubexms/kubexms/pkg/spec" // Added import
	"github.com/kubexms/kubexms/pkg/step"
	// "github.com/kubexms/kubexms/pkg/config" // Not directly used in this file
)

// Task represents a collection of ordered Steps designed to achieve a specific,
// small, and independent functional goal (e.g., "Install Containerd", "Perform System Preflight Checks").
// Tasks are orchestrated by Modules.
type Task struct {
	// Name is a descriptive name for the task, used for logging and identification.
	Name string

	// Steps is an ordered slice of spec.StepSpec interfaces to be executed by this task.
	Steps []spec.StepSpec

	// RunOnRoles specifies which host roles this task should target.
	// The actual filtering based on these roles is typically done by the Module or Pipeline
	// before invoking Task.Run with a pre-filtered list of hosts.
	// This field serves as metadata for those higher layers.
	RunOnRoles []string

	// Filter provides a more granular, dynamic way to select target hosts for this task.
	// Similar to RunOnRoles, this is usually applied by higher layers to generate the
	// list of hosts passed to the Run method.
	Filter func(host *runtime.Host) bool

	// IgnoreError, if true, means that if this Task's Run method encounters an error
	// (e.g., one of its critical steps fails on a host), the error will be logged,
	// but it will not halt the execution of subsequent tasks in a Module.
	// The Task's Run method itself will still return the error for informational purposes.
	IgnoreError bool

	// Concurrency specifies the maximum number of hosts on which this task will be
	// executed concurrently. If zero or negative, a sensible default (e.g., 10) is used.
	Concurrency int
}

// Run executes all steps in the Task on the provided list of target hosts.
// It handles step idempotency checks (Step.Check) and concurrent execution across hosts.
//
// Parameters:
//   - taskParentCtx: The parent runtime.Context (typically from a Module).
//     This context provides access to ClusterRuntime, Logger, GoContext, and caches.
//
// Returns:
//   - []*step.Result: A slice containing the results of all steps executed on all target hosts.
//   - error: The first critical error encountered during the execution on any host. If all steps
//     succeed or if failing steps are ignored (e.g., step-level ignore, not Task.IgnoreError),
//     this will be nil. Task.IgnoreError influences how the *calling Module* treats this error.
func (t *Task) Run(taskParentCtx runtime.Context) ([]*step.Result, error) {
	if taskParentCtx.Cluster == nil {
		// This should ideally be caught before calling Task.Run
		initialLogger := taskParentCtx.Logger
		if initialLogger == nil { initialLogger = logger.Get() } // Defensive
		initialLogger.Errorf("Task.Run for '%s' called with a Context that has a nil ClusterRuntime.", t.Name)
		return nil, fmt.Errorf("task '%s' Run called with a Context that has a nil ClusterRuntime", t.Name)
	}
	cluster := taskParentCtx.Cluster
	goCtx := taskParentCtx.GoContext // Use GoContext from taskParentCtx

	// Use logger from taskParentCtx, adding task-specific field.
	taskLogger := taskParentCtx.Logger.SugaredLogger.With("task", t.Name).Sugar()
	taskLogger.Infof("Starting task...")

	// --- Host Selection Logic ---
	if cluster.Hosts == nil || len(cluster.Hosts) == 0 {
		taskLogger.Warnf("No hosts defined in the cluster runtime. Skipping task.")
		return nil, nil
	}

	var targetHosts []*runtime.Host
	taskLogger.Debugf("Selecting hosts for task (Roles: %v, HasFilter: %v) from %d total hosts...", t.RunOnRoles, t.Filter != nil, len(cluster.Hosts))
	for _, host := range cluster.Hosts {
		roleMatch := false
		if len(t.RunOnRoles) == 0 {
			roleMatch = true // No specific roles required, matches all by role
		} else {
			for _, requiredRole := range t.RunOnRoles {
				if host.HasRole(requiredRole) {
					roleMatch = true
					break
				}
			}
		}

		if !roleMatch {
			taskLogger.Debugf("Host '%s' skipped: role mismatch (host roles: %v, task needs: %v)", host.Name, host.Roles, t.RunOnRoles)
			continue
		}

		filterMatch := true
		if t.Filter != nil {
			filterMatch = t.Filter(host)
			if !filterMatch {
				taskLogger.Debugf("Host '%s' skipped: custom filter returned false", host.Name)
			}
		}

		if filterMatch {
			targetHosts = append(targetHosts, host)
		}
	}

	if len(targetHosts) == 0 {
		taskLogger.Infof("No target hosts selected for task after filtering, skipping.")
		return nil, nil
	}
	taskLogger.Infof("Selected %d target hosts for task: %v", len(targetHosts), func() []string {
		names := make([]string, len(targetHosts))
		for i, h := range targetHosts { names[i] = h.Name }
		return names
	}())
	// --- End Host Selection Logic ---

	if len(t.Steps) == 0 {
		taskLogger.Infof("No steps defined for task, skipping.")
		return nil, nil
	}

	g, egCtx := errgroup.WithContext(goCtx) // Use goCtx from taskParentCtx

	concurrency := t.Concurrency
	if concurrency <= 0 {
		concurrency = 10
		taskLogger.Debugf("Task concurrency not set or invalid, defaulting to %d", concurrency)
	}
	g.SetLimit(concurrency)
	taskLogger.Debugf("Running task with concurrency limit of %d on %d hosts", concurrency, len(targetHosts))

	var allResults []*step.Result
	var resultsMu sync.Mutex

	for _, h := range targetHosts { // Iterate over newly selected targetHosts
		currentHost := h

		g.Go(func() error {
			// Create a new HostContext for each host.
			hostCtx := runtime.NewHostContext(egCtx, currentHost, cluster) // Updated call

			// As per instruction, re-assign hostCtx.Logger to ensure it's derived from taskParentCtx.Logger (module context)
			// and then specialized with host information.
			hostCtx.Logger = &logger.Logger{SugaredLogger: taskParentCtx.Logger.SugaredLogger.With(
				"host_name", currentHost.Name,
				"host_address", currentHost.Address,
			).Sugar()}
			// This hostCtx.Logger will be the base for step-specific loggers.

			// Task-specific logger for operations within this host's goroutine but outside a specific step.
			taskHostLogger := hostCtx.Logger.SugaredLogger.With("task_on_host", t.Name).Sugar()
			taskHostLogger.Infof("Starting task execution on host")

			for i, stepSpec := range t.Steps {
				// Set the current step spec in the host context for this iteration
				hostCtx.Step().SetCurrentStepSpec(stepSpec)

				// Create a more specific logger for this step
				stepSpecificLogger := hostCtx.Logger.SugaredLogger.With(
					"step", stepSpec.GetName(),
					"step_index", fmt.Sprintf("%d/%d", i+1, len(t.Steps)),
				).Sugar()

				// Temporarily set this highly specific logger in hostCtx for the duration of executor calls
				originalHostCtxLoggerForStep := hostCtx.Logger
				hostCtx.Logger = &logger.Logger{SugaredLogger: stepSpecificLogger}


				executor := step.GetExecutor(step.GetSpecTypeName(stepSpec))
				if executor == nil {
					errMsg := fmt.Errorf("no executor found for step spec %s on host %s", stepSpec.GetName(), currentHost.Name)
					stepSpecificLogger.Error(errMsg.Error()) // Use stepSpecificLogger for this error

					// Create and store a failed result
					// SetCurrentStepSpec already called above
					res := step.NewResult(hostCtx, time.Now(), errMsg)
					// res.Message is auto-set by NewResult if err is not nil
					resultsMu.Lock()
					allResults = append(allResults, res)
					resultsMu.Unlock()
					hostCtx.Logger = originalHostCtxLoggerForStep // Restore logger
					return errMsg // Critical failure for this host's task execution
				}

				stepSpecificLogger.Debugf("Checking step...")
				isDone, checkErr := executor.Check(hostCtx)


				if checkErr != nil {
					err := fmt.Errorf("step '%s' (on host '%s') pre-check failed: %w", stepSpec.GetName(), currentHost.Name, checkErr)
					stepSpecificLogger.Errorf("Step pre-check failed: %v", checkErr)

					// SetCurrentStepSpec already called
					checkRes := step.NewResult(hostCtx, time.Now(), err)
					// Message is auto-set by NewResult
					resultsMu.Lock()
					allResults = append(allResults, checkRes)
					resultsMu.Unlock()
					hostCtx.Logger = originalHostCtxLoggerForStep // Restore logger
					return err // Critical pre-check failure
				}

				if isDone {
					stepSpecificLogger.Infof("Step is already done, skipping.")
					// SetCurrentStepSpec already called
					skipRes := step.NewResult(hostCtx, time.Now(), nil) // No error for skipped
					skipRes.Status = step.StatusSkipped
					skipRes.Message = "Condition already met or task already completed."
					// EndTime is set by NewResult
					resultsMu.Lock()
					allResults = append(allResults, skipRes)
					resultsMu.Unlock()
					hostCtx.Logger = originalHostCtxLoggerForStep // Restore logger
					continue
				}

				stepSpecificLogger.Infof("Running step...")
				stepResult := executor.Execute(hostCtx) // Changed from Run to Execute


				if stepResult == nil {
					nilResultErr := fmt.Errorf("step '%s' (on host '%s') Execute method returned a nil result", stepSpec.GetName(), currentHost.Name)
					stepSpecificLogger.Error(nilResultErr.Error())
					// SetCurrentStepSpec already called
					failedRes := step.NewResult(hostCtx, time.Now(), nilResultErr)
					// Message is auto-set by NewResult
					resultsMu.Lock()
					allResults = append(allResults, failedRes)
					resultsMu.Unlock()
					hostCtx.Logger = originalHostCtxLoggerForStep // Restore logger
					return nilResultErr // Critical failure
				}

				// Restore logger before processing result, so any task-level logging uses the taskHostLogger
				hostCtx.Logger = originalHostCtxLoggerForStep


				resultsMu.Lock()
				allResults = append(allResults, stepResult)
				resultsMu.Unlock()

				if stepResult.Status == step.StatusFailed {
					errToPropagate := stepResult.Error
					if errToPropagate == nil {
						errToPropagate = fmt.Errorf("step '%s' on host '%s' reported status Failed without a specific error. Message: %s", stepSpec.GetName(), currentHost.Name, stepResult.Message)
					}
					// The step itself should have logged its errors using the stepSpecificLogger.
					// TaskHostLogger logs the fact that a step failed within its execution.
					taskHostLogger.Errorf("Step '%s' failed on host '%s': %v", stepSpec.GetName(), currentHost.Name, errToPropagate)
					return errToPropagate // Critical step failure for this host
				}
			}
			taskHostLogger.Infof("Task execution completed successfully on host.")
			return nil
		})
	}

	err := g.Wait() // This error is the first non-nil error returned by a g.Go() func

	if err != nil {
		// This log message uses taskLogger, which has the module and task context.
		// The error `err` itself will contain host and step specific details from the failing goroutine.
		taskLogger.Errorf("Task finished with errors. First critical error from a host: %v", err)
		return allResults, err
	}

	taskLogger.Successf("Task finished successfully on all targeted hosts.")
	return allResults, nil
}
