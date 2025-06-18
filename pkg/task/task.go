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
//   - goCtx: The parent Go context, for cancellation and deadlines.
//   - hosts: A slice of *runtime.Host pointers on which this task should be executed.
//     This list is typically pre-filtered by a Module based on Task.RunOnRoles and Task.Filter.
//   - cluster: A pointer to the global *runtime.ClusterRuntime providing access to overall
//     cluster configuration and shared resources like the logger.
//
// Returns:
//   - []*step.Result: A slice containing the results of all steps executed on all target hosts.
//   - error: The first critical error encountered during the execution on any host. If all steps
//     succeed or if failing steps are ignored (e.g., step-level ignore, not Task.IgnoreError),
//     this will be nil. Task.IgnoreError influences how the *calling Module* treats this error.
func (t *Task) Run(goCtx context.Context, hosts []*runtime.Host, cluster *runtime.ClusterRuntime) ([]*step.Result, error) {
	// Ensure logger is available from cluster runtime, or use global default.
	var baseLogger *logger.Logger
	if cluster != nil && cluster.Logger != nil {
		baseLogger = cluster.Logger
	} else {
		baseLogger = logger.Get() // Fallback to global logger
		baseLogger.Warnf("Task.Run for '%s' using global logger due to nil cluster.Logger.", t.Name)
	}
	taskLogger := baseLogger.SugaredLogger.With("task", t.Name).Sugar()
	taskLogger.Infof("Starting task...")

	if len(hosts) == 0 {
		taskLogger.Infof("No target hosts for task, skipping.")
		return nil, nil
	}
	if len(t.Steps) == 0 {
		taskLogger.Infof("No steps defined for task, skipping.")
		return nil, nil
	}

	g, egCtx := errgroup.WithContext(goCtx)

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
			// The logger within this hostCtx will be derived from cluster.Logger.
			hostCtx := runtime.NewHostContext(egCtx, currentHost, cluster)

			// Further specialize logger for this task on this host.
			// NewHostContext already adds host.name and host.address.
			// We add the task name here.
			hostTaskLogger := hostCtx.Logger.SugaredLogger.With("task_on_host", t.Name).Sugar()
			hostCtx.Logger = &logger.Logger{SugaredLogger: hostTaskLogger}


			hostTaskLogger.Infof("Starting task execution on host")
			// var hostStepResults []*step.Result // Not currently used outside this goroutine

			for i, s := range t.Steps {
				// Create a step-specific logger from the host-task logger
				stepLoggerWithFields := hostTaskLogger.With("step", s.Name(), "step_index", fmt.Sprintf("%d/%d", i+1, len(t.Steps)))
				hostCtx.Logger = &logger.Logger{SugaredLogger: stepLoggerWithFields.Sugar()} // Update context's logger for this step

				stepLoggerWithFields.Debugf("Checking step...")
				isDone, checkErr := s.Check(hostCtx)
				if checkErr != nil {
					err := fmt.Errorf("step '%s' (on host '%s') pre-check failed: %w", s.Name(), currentHost.Name, checkErr)
					stepLoggerWithFields.Errorf("Step pre-check failed: %v", checkErr)

					checkRes := step.NewResult(s.Name()+" [CheckPhase]", currentHost.Name, time.Now(), err)
					checkRes.Message = err.Error() // Ensure message has the error
					resultsMu.Lock()
					allResults = append(allResults, checkRes)
					resultsMu.Unlock()
					return err
				}

				if isDone {
					stepLoggerWithFields.Infof("Step is already done, skipping.")
					skipRes := step.NewResult(s.Name(), currentHost.Name, time.Now(), nil)
					skipRes.Status = "Skipped"
					skipRes.Message = "Condition already met or task already completed."
					skipRes.EndTime = time.Now()
					resultsMu.Lock()
					allResults = append(allResults, skipRes)
					resultsMu.Unlock()
					// hostStepResults = append(hostStepResults, skipRes) // If needed per host
					continue
				}

				stepLoggerWithFields.Infof("Running step...")
				stepResult := s.Run(hostCtx)
				if stepResult == nil {
				    nilResultErr := fmt.Errorf("step '%s' (on host '%s') Run method returned a nil result", s.Name(), currentHost.Name)
					stepLoggerWithFields.Errorf("%v", nilResultErr)
					// Create a result for this failure
					failedRes := step.NewResult(s.Name(), currentHost.Name, time.Now(), nilResultErr)
					failedRes.Status = "Failed"
					failedRes.Message = "Step implementation returned nil result."
                    resultsMu.Lock()
					allResults = append(allResults, failedRes)
                    resultsMu.Unlock()
                    return nilResultErr
				}

				resultsMu.Lock()
				allResults = append(allResults, stepResult)
				resultsMu.Unlock()
				// hostStepResults = append(hostStepResults, stepResult) // If needed per host

				if stepResult.Status == "Failed" {
					// Error is already logged by the step's Run method.
					// We just need to propagate it to errgroup.
					// The stepResult.Error should contain the specific error from the step.
					// If stepResult.Error is nil but status is Failed, use a generic message.
					errToPropagate := stepResult.Error
					if errToPropagate == nil {
						errToPropagate = fmt.Errorf("step '%s' on host '%s' reported status Failed without a specific error in Result.Error. Message: %s", s.Name(), currentHost.Name, stepResult.Message)
					}
					// No need to log again here, step's Run should have logged its failure.
					return fmt.Errorf("step '%s' failed on host '%s': %w", s.Name(), currentHost.Name, errToPropagate)
				}
				// Success is logged by the step's Run method.
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
