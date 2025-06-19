package task

import (
	"context"
	"fmt"
	"sync" // Required for mu in Task.Run if collecting results with a mutex
	"time" // Required for time.Now() in NewResult

	"golang.org/x/sync/errgroup"

	"github.com/kubexms/kubexms/pkg/logger" // For logger.Logger type in runtime.Context
	"github.com/kubexms/kubexms/pkg/runtime"
	"github.com/kubexms/kubexms/pkg/step"
	// "github.com/kubexms/kubexms/pkg/config" // Not directly used in this file
)

// Task represents a collection of ordered Steps designed to achieve a specific,
// small, and independent functional goal (e.g., "Install Containerd", "Perform System Preflight Checks").
// Tasks are orchestrated by Modules.
type Task struct {
	// Name is a descriptive name for the task, used for logging and identification.
	Name string

	// Steps is an ordered slice of step.Step interfaces to be executed by this task.
	Steps []step.Step

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
//   - hosts: A slice of *runtime.Host pointers on which this task should be executed.
//     This list is typically pre-filtered by a Module based on Task.RunOnRoles and Task.Filter.
//
// Returns:
//   - []*step.Result: A slice containing the results of all steps executed on all target hosts.
//   - error: The first critical error encountered during the execution on any host. If all steps
//     succeed or if failing steps are ignored (e.g., step-level ignore, not Task.IgnoreError),
//     this will be nil. Task.IgnoreError influences how the *calling Module* treats this error.
func (t *Task) Run(taskParentCtx runtime.Context, hosts []*runtime.Host) ([]*step.Result, error) {
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
	// taskParentCtx.Logger should already be contextualized by the calling module.
	taskLogger := taskParentCtx.Logger.SugaredLogger.With("task", t.Name).Sugar()
	taskLogger.Infof("Starting task...")

	if len(hosts) == 0 {
		taskLogger.Infof("No target hosts for task, skipping.")
		return nil, nil
	}
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
	taskLogger.Debugf("Running task with concurrency limit of %d on %d hosts", concurrency, len(hosts))

	var allResults []*step.Result
	var resultsMu sync.Mutex

	for _, h := range hosts {
		currentHost := h

		g.Go(func() error {
			// Create a new HostContext for each host.
			// Pass caches from the parent context. Steps are expected to use these scopes.
			// TODO: Review if task-specific or step-specific sub-caches are needed here.
			// For now, module-level caches are propagated.
			hostCtx := runtime.NewHostContext(egCtx, currentHost, cluster,
				taskParentCtx.Pipeline(),
				taskParentCtx.Module(),
				taskParentCtx.Task(), // Propagates the module's task cache
				taskParentCtx.Step(), // Propagates the module's step cache (or nil)
			)

			// Further specialize logger for this task on this host.
			// NewHostContext already adds host.name and host.address using the base logger from `cluster`.
			// The logger in hostCtx is now derived from `cluster.Logger`.
			// We should ensure taskParentCtx.Logger (which includes module context) is used as base in NewHostContext.
			// This requires NewHostContext to potentially take a baseLogger argument.
			// For now, let's assume NewHostContext's current logger derivation is sufficient,
			// or specialize it here if taskParentCtx.Logger is preferred over cluster.Logger.
			// Current NewHostContext uses cluster.Logger.
			// Let's re-assign logger in hostCtx to be derived from taskParentCtx.Logger for more specific context.
			// This assumes taskParentCtx.Logger is already contextualized (e.g., with module name).
			hostCtx.Logger = &logger.Logger{SugaredLogger: taskParentCtx.Logger.SugaredLogger.With(
				"host_name", currentHost.Name,
				"host_address", currentHost.Address,
			).Sugar()}

			// Further specialize logger for this task on this host within this task's scope.
			// We add the task name here.
			// The logger in hostCtx is now derived from taskParentCtx.Logger and enriched with host info.
			hostTaskLogger := hostCtx.Logger.SugaredLogger.With("task_on_host", t.Name).Sugar()
			// No need to re-assign hostCtx.Logger here if NewHostContext correctly uses/derives from taskParentCtx.Logger
			// For now, assuming hostCtx.Logger from NewHostContext is the one to use for steps.
			// If NewHostContext needs taskParentCtx.Logger, that's a change in NewHostContext.
			// The current NewHostContext uses cluster.Logger (i.e. taskParentCtx.Cluster.Logger()).
			// This is a subtle point: should step logs inherit module context or just cluster context?
			// Let's assume for now module context is desired for steps.
			// This means NewHostContext should ideally take taskParentCtx.Logger as its base.
			// Modifying NewHostContext is outside this diff. For now, steps will log with cluster + host context.
			// To ensure module context is included, we can overwrite hostCtx.Logger here:
			hostCtx.Logger = &logger.Logger{SugaredLogger: hostTaskLogger}


			hostTaskLogger.Infof("Starting task execution on host")

			for i, s := range t.Steps {
				stepSpecificLogger := hostTaskLogger.With("step", s.Name(), "step_index", fmt.Sprintf("%d/%d", i+1, len(t.Steps)))
				originalStepHostCtxLogger := hostCtx.Logger // Save it
				hostCtx.Logger = &logger.Logger{SugaredLogger: stepSpecificLogger.Sugar()} // Set step-specific logger for the step's execution

				stepSpecificLogger.Debugf("Checking step...")
				isDone, checkErr := s.Check(hostCtx)
				hostCtx.Logger = originalStepHostCtxLogger // Restore previous logger context for the task loop

				if checkErr != nil {
					err := fmt.Errorf("step '%s' (on host '%s') pre-check failed: %w", s.Name(), currentHost.Name, checkErr)
					stepSpecificLogger.Errorf("Step pre-check failed: %v", checkErr) // Use step-specific logger

					checkRes := step.NewResult(s.Name()+" [CheckPhase]", currentHost.Name, time.Now(), err)
					checkRes.Message = err.Error()
					resultsMu.Lock()
					allResults = append(allResults, checkRes)
					resultsMu.Unlock()
					return err // Critical pre-check failure
				}

				if isDone {
					stepSpecificLogger.Infof("Step is already done, skipping.")
					skipRes := step.NewResult(s.Name(), currentHost.Name, time.Now(), nil)
					skipRes.Status = "Skipped"
					skipRes.Message = "Condition already met or task already completed."
					skipRes.EndTime = time.Now()
					resultsMu.Lock()
					allResults = append(allResults, skipRes)
					resultsMu.Unlock()
					continue
				}

				originalStepHostCtxLogger = hostCtx.Logger // Save again before Run
				hostCtx.Logger = &logger.Logger{SugaredLogger: stepSpecificLogger.Sugar()} // Set for Run
				stepSpecificLogger.Infof("Running step...")
				stepResult := s.Run(hostCtx)
				hostCtx.Logger = originalStepHostCtxLogger // Restore

				if stepResult == nil {
				    nilResultErr := fmt.Errorf("step '%s' (on host '%s') Run method returned a nil result", s.Name(), currentHost.Name)
					stepSpecificLogger.Errorf("%v", nilResultErr)
					failedRes := step.NewResult(s.Name(), currentHost.Name, time.Now(), nilResultErr)
					failedRes.Status = "Failed"
					failedRes.Message = "Step implementation returned nil result."
                    resultsMu.Lock()
					allResults = append(allResults, failedRes)
                    resultsMu.Unlock()
                    return nilResultErr // Critical failure
				}

				resultsMu.Lock()
				allResults = append(allResults, stepResult)
				resultsMu.Unlock()

				if stepResult.Status == "Failed" {
					errToPropagate := stepResult.Error
					if errToPropagate == nil {
						errToPropagate = fmt.Errorf("step '%s' on host '%s' reported status Failed without a specific error. Message: %s", s.Name(), currentHost.Name, stepResult.Message)
					}
					// Step's Run should have logged its failure with stepSpecificLogger.
					return fmt.Errorf("step '%s' failed on host '%s': %w", s.Name(), currentHost.Name, errToPropagate) // Critical step failure
				}
			}
			hostTaskLogger.Infof("Task execution completed successfully on host.")
			return nil
		})
	}

	err := g.Wait()

	if err != nil {
		taskLogger.Errorf("Task finished with errors. First critical error: %v", err)
		return allResults, err
	}

	taskLogger.Successf("Task finished successfully on all targeted hosts.")
	return allResults, nil
}
