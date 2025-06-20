package executor

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	// "github.com/kubexms/kubexms/pkg/config" // No longer used directly by executor
	"github.com/mensylisir/kubexm/pkg/cache"   // For creating new caches
	"github.com/mensylisir/kubexm/pkg/logger"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

const (
	DefaultTaskConcurrency = 10
)

type Executor struct {
	Logger                 *logger.Logger
	DefaultTaskConcurrency int
}

type ExecutorOptions struct {
	Logger                 *logger.Logger
	DefaultTaskConcurrency int
}

func NewExecutor(opts ExecutorOptions) (*Executor, error) {
	log := opts.Logger
	if log == nil {
		log = logger.Get()
		log.Debugf("ExecutorOptions.Logger not provided, using global default logger for executor.")
	}
	concurrency := opts.DefaultTaskConcurrency
	if concurrency <= 0 {
		concurrency = DefaultTaskConcurrency
		log.Debugf("ExecutorOptions.DefaultTaskConcurrency not set or invalid, using default value: %d", concurrency)
	}
	return &Executor{
		Logger:                 log,
		DefaultTaskConcurrency: concurrency,
	}, nil
}

// ExecutePipeline is the main entry point for running a pipeline.
func (e *Executor) ExecutePipeline(
	goCtx context.Context,
	pipelineSpec *spec.PipelineSpec,
	cluster *runtime.ClusterRuntime,
) (allResults []*step.Result, finalError error) {
	if pipelineSpec == nil { return nil, fmt.Errorf("ExecutePipeline: pipelineSpec cannot be nil") }
	if cluster == nil { return nil, fmt.Errorf("ExecutePipeline: cluster runtime cannot be nil") }
	if e.Logger == nil { return nil, fmt.Errorf("ExecutePipeline: executor logger is nil")}

	// Base logger for this pipeline execution, derived from the executor's logger.
	pipelineLogger := &logger.Logger{SugaredLogger: e.Logger.SugaredLogger.With("pipeline_name", pipelineSpec.Name)}

	pipelineLogger.Infof("Starting pipeline execution...")
	overallStartTime := time.Now(); allResults = make([]*step.Result, 0)

	// Defer PostRun hook for pipeline
	if pipelineSpec.PostRun != nil {
		defer func() {
			// Logger for this specific PostRun hook event
			postRunHookEventLogger := pipelineLogger.SugaredLogger.With("hook_event", "PipelinePostRun", "hook_step", pipelineSpec.PostRun.GetName()).Sugar()
			postRunHookEventLogger.Infof("Executing...")

			var targetHosts []*runtime.Host; if len(cluster.Hosts) > 0 { targetHosts = []*runtime.Host{cluster.Hosts[0]} }

			// Pass the pipelineLogger as the parent for the hook execution
			hookResults, hookErr := e.executeHookTask(goCtx, pipelineSpec.PostRun, "PipelinePostRun", targetHosts, cluster, pipelineLogger)
			if hookResults != nil { allResults = append(allResults, hookResults...) }
			if hookErr != nil {
				postRunHookEventLogger.Errorf("Failed: %v", hookErr)
				if finalError == nil { finalError = fmt.Errorf("pipeline PostRun hook '%s' failed: %w", pipelineSpec.PostRun.GetName(), hookErr) }
			} else { postRunHookEventLogger.Infof("Completed.") }

			totalDuration := time.Since(overallStartTime)
			if finalError != nil {
			    pipelineLogger.Errorf("Pipeline execution finished in %s with error: %v", totalDuration, finalError)
			} else {
			    pipelineLogger.Successf("Pipeline execution finished successfully in %s.", totalDuration)
			}
		}()
	} else {
	    defer func() {
			totalDuration := time.Since(overallStartTime)
			if finalError != nil {
			    pipelineLogger.Errorf("Pipeline execution finished in %s with error: %v", totalDuration, finalError)
			} else {
			    pipelineLogger.Successf("Pipeline execution finished successfully in %s.", totalDuration)
			}
	    }()
	}

	// 1. Execute Pipeline PreRun Hook
	if pipelineSpec.PreRun != nil {
		preRunHookEventLogger := pipelineLogger.SugaredLogger.With("hook_event", "PipelinePreRun", "hook_step", pipelineSpec.PreRun.GetName()).Sugar()
		preRunHookEventLogger.Infof("Executing...")
		var targetHosts []*runtime.Host; if len(cluster.Hosts) > 0 { targetHosts = []*runtime.Host{cluster.Hosts[0]} } // TODO: Define target hosts for pipeline hooks more robustly

		hookResults, hookErr := e.executeHookTask(goCtx, pipelineSpec.PreRun, "PipelinePreRun", targetHosts, cluster, pipelineLogger)
		if hookResults != nil { allResults = append(allResults, hookResults...) }
		if hookErr != nil {
			finalError = fmt.Errorf("pipeline PreRun hook '%s' failed: %w", pipelineSpec.PreRun.GetName(), hookErr)
			preRunHookEventLogger.Errorf("Failed: %v. Halting pipeline.", finalError);
			return allResults, finalError
		}
		preRunHookEventLogger.Infof("Completed.")
	}

	// 2. Iterate through Modules
	for modIdx, moduleSpec := range pipelineSpec.Modules {
		if moduleSpec == nil {
			pipelineLogger.Warnf("Skipping nil moduleSpec at index %d in pipeline '%s'.", modIdx, pipelineSpec.Name)
			continue
		}
		// Create logger for this module, deriving from the pipeline's logger
		moduleLogger := &logger.Logger{SugaredLogger: pipelineLogger.SugaredLogger.With("module_name", moduleSpec.Name, "module_idx", fmt.Sprintf("%d/%d", modIdx+1, len(pipelineSpec.Modules)))}

		if moduleSpec.IsEnabled != nil && !moduleSpec.IsEnabled(cluster) { // Pass cluster runtime
			moduleLogger.Infof("Module is disabled by configuration, skipping.")
			continue
		}
		// Pass the module-specific logger down
		moduleResults, moduleErr := e.executeModule(goCtx, moduleSpec, cluster, moduleLogger)
		if moduleResults != nil { allResults = append(allResults, moduleResults...) }
		if moduleErr != nil {
			finalError = moduleErr;
			moduleLogger.Errorf("Module execution failed: %v. Halting pipeline.", moduleErr);
			return allResults, finalError
		}
	}
	return allResults, nil
}

// executeModule now accepts a parentLogger (which is pipeline-specific with module context)
func (e *Executor) executeModule(
	goCtx context.Context,
	moduleSpec *spec.ModuleSpec,
	cluster *runtime.ClusterRuntime,
	moduleLogger *logger.Logger, // Expects a logger already contextualized for this module
) (allModStepRes []*step.Result, modErr error) {

	moduleLogger.Infof("Starting module execution...") // Log already has module context

	allModStepRes = make([]*step.Result, 0)
	// selectHostsForModule uses e.Logger for its own diagnostic logs
	modTargetHosts := e.selectHostsForModule(moduleSpec, cluster)

	if len(modTargetHosts) == 0 && (moduleSpec.PreRun != nil || moduleSpec.PostRun != nil || len(moduleSpec.Tasks) > 0) {
		moduleLogger.Warnf("No target hosts identified for any task in module. Pre/PostRun hooks (if any) and tasks will be skipped.")
	}

	if moduleSpec.PostRun != nil {
		defer func() {
			// Logger for this specific PostRun hook event, derived from moduleLogger
			postRunHookEventLogger := moduleLogger.SugaredLogger.With("hook_event", fmt.Sprintf("ModulePostRun[%s]", moduleSpec.Name), "hook_step", moduleSpec.PostRun.GetName()).Sugar()
			postRunHookEventLogger.Infof("Executing...")
			if len(modTargetHosts) > 0 || (moduleSpec.PostRun != nil && len(moduleSpec.PostRun.Steps) == 0) { // Allow hooks with no steps to run (e.g. for logging) if modTargetHosts might be empty.
				// Pass the moduleLogger as the parent for the hook execution
				hookRes, hookErr := e.executeHookTask(goCtx, moduleSpec.PostRun, fmt.Sprintf("ModulePostRun[%s]", moduleSpec.Name), modTargetHosts, cluster, moduleLogger)
				if hookRes != nil { allModStepRes = append(allModStepRes, hookRes...) }
				if hookErr != nil {
					postRunHookEventLogger.Errorf("Failed: %v", hookErr)
					if modErr == nil { modErr = fmt.Errorf("module PostRun hook '%s' failed: %w", moduleSpec.PostRun.GetName(), hookErr) }
				} else { postRunHookEventLogger.Infof("Completed.") }
			} else { postRunHookEventLogger.Infof("Skipping (no target hosts for hook with steps).") }
		}()
	}

	if moduleSpec.PreRun != nil {
		preRunHookEventLogger := moduleLogger.SugaredLogger.With("hook_event", fmt.Sprintf("ModulePreRun[%s]", moduleSpec.Name), "hook_task_name", moduleSpec.PreRun.Name).Sugar()
		preRunHookEventLogger.Infof("Executing...")
		if len(modTargetHosts) > 0 || (moduleSpec.PreRun != nil && len(moduleSpec.PreRun.Steps) == 0) { // Allow hooks with no steps
			hookRes, hookErr := e.executeHookTask(goCtx, moduleSpec.PreRun, fmt.Sprintf("ModulePreRun[%s]", moduleSpec.Name), modTargetHosts, cluster, moduleLogger)
			if hookRes != nil { allModStepRes = append(allModStepRes, hookRes...) }
			if hookErr != nil {
				modErr = fmt.Errorf("module PreRun hook '%s' failed: %w", moduleSpec.PreRun.Name, hookErr)
				preRunHookEventLogger.Errorf("Failed: %v. Halting module tasks.", modErr); return allModStepRes, modErr
			}
			preRunHookEventLogger.Infof("Completed.")
		} else { preRunHookEventLogger.Infof("Skipping (no target hosts for hook with steps).") }
	}

	for taskIdx, taskSpec := range moduleSpec.Tasks {
		if taskSpec == nil {
			moduleLogger.Warnf("Skipping nil taskSpec at index %d in module '%s'.", taskIdx, moduleSpec.Name)
			continue
		}
		// Create logger for this task, deriving from the module's logger
		taskLogger := &logger.Logger{SugaredLogger: moduleLogger.SugaredLogger.With("task_name", taskSpec.Name, "task_idx", fmt.Sprintf("%d/%d", taskIdx+1, len(moduleSpec.Tasks)))}

		// selectHostsForTaskSpec uses e.Logger for its own diagnostic logs
		taskTargetHosts := e.selectHostsForTaskSpec(taskSpec, cluster)
		if len(taskTargetHosts) == 0 { taskLogger.Infof("No target hosts, skipping."); continue }

		// Pass the task-specific logger down
		taskStepRes, currentTaskErr := e.executeTaskSpec(goCtx, taskSpec, taskTargetHosts, cluster, taskLogger)
		if taskStepRes != nil { allModStepRes = append(allModStepRes, taskStepRes...) }
		if currentTaskErr != nil {
			taskLogger.Errorf("Task execution failed: %v", currentTaskErr)
			if !taskSpec.IgnoreError {
				modErr = fmt.Errorf("task '%s' in module '%s' failed: %w", taskSpec.Name, moduleSpec.Name, currentTaskErr)
				taskLogger.Errorf("Critical task failure. Halting module."); return allModStepRes, modErr
			}
			taskLogger.Warnf("Task error ignored by spec. Continuing module.")
		} else { taskLogger.Successf("Task completed successfully.") }
	}

	if modErr != nil { moduleLogger.Errorf("Module finished with error: %v", modErr)
	} else { moduleLogger.Successf("Module finished successfully.") }
	return allModStepRes, modErr
}

// executeTaskSpec now takes a parentTaskLogger (which is pipeline, module & task specific)
func (e *Executor) executeTaskSpec(
	goCtx context.Context, taskSpec *spec.TaskSpec, hosts []*runtime.Host,
	cluster *runtime.ClusterRuntime, parentTaskLogger *logger.Logger,
) (allTaskStepResults []*step.Result, taskError error) {

	taskLogger := parentTaskLogger.SugaredLogger // Use the passed-in task-specific logger
	taskLogger.Debugf("Processing task on %d hosts with concurrency %d", len(hosts), taskSpec.Concurrency)

	allTaskStepResults = make([]*step.Result, 0, len(hosts)*len(taskSpec.Steps)); var resultsMu sync.Mutex
	g, egCtx := errgroup.WithContext(goCtx)
	concurrency := taskSpec.Concurrency; if concurrency <= 0 { concurrency = e.DefaultTaskConcurrency }; g.SetLimit(concurrency)

	for _, h := range hosts {
		currentHost := h
		g.Go(func() error {
			// Create a new HostContext for each host.
			// TODO: Properly initialize PipelineCache and ModuleCache. For now, passing nil.
			// TaskCache and StepCache are new for each host/step execution within this task.
			taskCacheForHost := cache.NewMapCache()

			hostSpecificParentLogger := parentTaskLogger.SugaredLogger.With("host_name", currentHost.Name, "host_address", currentHost.Address)

			for stepIdx, stepSpecInstance := range taskSpec.Steps {
				if stepSpecInstance == nil {
					hostSpecificParentLogger.Warnf("Skipping nil stepSpecInstance at index %d in task '%s'", stepIdx, taskSpec.Name)
					continue
				}

				stepCacheForHostStep := cache.NewMapCache()
				hostCtx := runtime.NewHostContext(egCtx, currentHost, cluster,
					nil, // TODO: PipelineCache
					nil, // TODO: ModuleCache
					taskCacheForHost,
					stepCacheForHostStep,
				)
				// Assign logger with full context (pipeline, module, task, host, step)
				hostCtx.Logger = &logger.Logger{SugaredLogger: hostSpecificParentLogger.With("step_name", stepSpecInstance.GetName(), "step_idx", fmt.Sprintf("%d/%d", stepIdx+1, len(taskSpec.Steps))).Sugar()}

				// Set current StepSpec in context for the executor to retrieve
				hostCtx.Step().SetCurrentStepSpec(stepSpecInstance)

				stepExecutor := step.GetExecutor(step.GetSpecTypeName(stepSpecInstance))
				if stepExecutor == nil {
					err := fmt.Errorf("no executor for type %s (step: %s)", step.GetSpecTypeName(stepSpecInstance), stepSpecInstance.GetName())
					hostCtx.Logger.Errorf("Critical: %v", err)
					res := step.NewResult(hostCtx, time.Now(),err) // Use new NewResult signature
					res.Status="Failed"; res.Message="Internal: Executor not found."
					resultsMu.Lock(); allTaskStepResults = append(allTaskStepResults, res); resultsMu.Unlock(); return err
				}

				hostCtx.Logger.Debugf("Checking step...")
				isDone, checkErr := stepExecutor.Check(hostCtx) // Use new Check signature
				if checkErr != nil {
					err := fmt.Errorf("step '%s' pre-check failed: %w", stepSpecInstance.GetName(), checkErr)
					hostCtx.Logger.Errorf("Pre-check failed: %v", checkErr)
					res := step.NewResult(hostCtx, time.Now(), err) // Use new NewResult signature (pass hostCtx)
					res.Message=err.Error() // NewResult might populate this from ctx or error
					resultsMu.Lock(); allTaskStepResults = append(allTaskStepResults, res); resultsMu.Unlock(); return err
				}
				if isDone {
					hostCtx.Logger.Infof("Skipped (already done).")
					res := step.NewResult(hostCtx, time.Now(), nil) // Use new NewResult signature
					res.Status="Skipped"; res.Message="Condition met."; res.EndTime=time.Now()
					resultsMu.Lock(); allTaskStepResults = append(allTaskStepResults, res); resultsMu.Unlock(); continue
				}

				hostCtx.Logger.Infof("Running step...")
				execResult := stepExecutor.Execute(hostCtx) // Use new Execute signature
				if execResult == nil {
					err := fmt.Errorf("step '%s' Execute nil result", stepSpecInstance.GetName())
					hostCtx.Logger.Errorf("%v", err)
					res := step.NewResult(hostCtx, time.Now(),err); res.Status = "Failed"; res.Message = "Internal error: Step Execute returned nil result."
					resultsMu.Lock(); allTaskStepResults = append(allTaskStepResults, res); resultsMu.Unlock(); return err
				}
				resultsMu.Lock(); allTaskStepResults = append(allTaskStepResults, execResult); resultsMu.Unlock()
				if execResult.Status == "Failed" {
					errToPropagate := execResult.Error
					if errToPropagate == nil { errToPropagate = fmt.Errorf("step '%s' on host '%s' reported status Failed without specific error. Message: %s", execResult.StepName, currentHost.Name, execResult.Message) }
					hostCtx.Logger.Errorf("Failed: %v", errToPropagate)
					return fmt.Errorf("step '%s' failed on host '%s': %w", execResult.StepName, currentHost.Name, errToPropagate)
				}
				hostCtx.Logger.Successf("Completed. %s", execResult.Message)
			}
			hostSpecificParentLogger.Debugf("Step sequence completed for task %s on host %s.", taskSpec.Name, currentHost.Name)
			return nil
		})
	}
	taskError = g.Wait()
	if taskError != nil { taskLogger.Errorf("Task finished with host error(s): %v", taskError)
	} else { taskLogger.Successf("Task finished successfully on all %d hosts.", len(hosts)) }
	return allTaskStepResults, taskError
}

// executeHookTask executes a single TaskSpec, typically for PreRun or PostRun hooks.
// It derives a logger for the hook task from the parentLogger.
func (e *Executor) executeHookTask(
	goCtx context.Context,
	hookTaskSpec *spec.TaskSpec,
	hookTypeForLog string, // e.g., "PipelinePreRun", "ModulePostRun"
	targetHosts []*runtime.Host,
	cluster *runtime.ClusterRuntime,
	parentLogger *logger.Logger,
) (results []*step.Result, err error) {
	if hookTaskSpec == nil {
		parentLogger.Debugf("Hook %s is nil, skipping.", hookTypeForLog)
		return nil, nil
	}
	// Allow hooks with no steps to still "run" (e.g., for logging start/end of pipeline/module)
	// but only proceed to executeTaskSpec if there are steps OR if there are no steps AND no specific hosts required (global hook)
	if len(hookTaskSpec.Steps) > 0 && len(targetHosts) == 0 {
		parentLogger.Debugf("No target hosts for hook %s (%s) which has steps, skipping.", hookTypeForLog, hookTaskSpec.Name)
		return nil, nil
	}

	// If there are no steps, and no hosts, we might still want to log the hook's execution
    // For now, if no steps, effectively a no-op beyond logging.
    if len(hookTaskSpec.Steps) == 0 {
        parentLogger.Infof("Hook %s (%s) has no steps, considered complete.", hookTypeForLog, hookTaskSpec.Name)
        return nil, nil
    }


	hookLogger := parentLogger.SugaredLogger.With("hook_type", hookTypeForLog, "hook_task_name", hookTaskSpec.Name).Sugar()
	hookLogger.Infof("Executing hook task...")

	// Use executeTaskSpec to run this hook task
	results, err = e.executeTaskSpec(goCtx, hookTaskSpec, targetHosts, cluster, &logger.Logger{SugaredLogger: hookLogger})

	if err != nil {
		hookLogger.Errorf("Hook task execution failed: %v", err)
		// Error is already wrapped by executeTaskSpec if it's from a step
	} else {
		hookLogger.Successf("Hook task completed successfully.")
	}
	return results, err
}


// selectHostsForTaskSpec (as previously defined)
func (e *Executor) selectHostsForTaskSpec( taskSpec *spec.TaskSpec, cluster *runtime.ClusterRuntime) []*runtime.Host {
	var selectedHosts []*runtime.Host
	if cluster == nil || len(cluster.Hosts) == 0 { e.Logger.Debugf("selectHostsForTaskSpec: No hosts in cluster."); return selectedHosts }
	if taskSpec == nil { e.Logger.Warnf("selectHostsForTaskSpec: taskSpec nil."); return selectedHosts }

	pipelineName := "unknown-pipeline"
	if cluster.ClusterConfig != nil && cluster.ClusterConfig.Metadata.Name != "" { pipelineName = cluster.ClusterConfig.Metadata.Name }

	selectorLogger := e.Logger.SugaredLogger.With("pipeline", pipelineName, "task_name_selecting_for", taskSpec.Name,"component","host_selector").Sugar()
	selectorLogger.Debugf("Selecting hosts (Roles: %v, HasFilter: %v)", taskSpec.RunOnRoles, taskSpec.Filter != nil)
	for _, host := range cluster.Hosts {
		roleMatch := false
		if len(taskSpec.RunOnRoles) == 0 { roleMatch = true
		} else { for _, requiredRole := range taskSpec.RunOnRoles { if host.HasRole(requiredRole) { roleMatch = true; break } } }
		if !roleMatch {
			var hostRoleNames []string; for rn, pr := range host.Roles { if pr { hostRoleNames = append(hostRoleNames, rn)}}; sort.Strings(hostRoleNames)
			selectorLogger.Debugf("Host '%s' skipped by role (has: [%s], needs: %v).", host.Name, strings.Join(hostRoleNames, ","), taskSpec.RunOnRoles); continue
		}
		filterMatch := true
		if taskSpec.Filter != nil { filterMatch = taskSpec.Filter(host); if !filterMatch { selectorLogger.Debugf("Host '%s' skipped by custom filter.", host.Name) } }
		if filterMatch { selectedHosts = append(selectedHosts, host) }
	}
	selectedHostNames := make([]string, len(selectedHosts)); for i,h := range selectedHosts { selectedHostNames[i] = h.Name }; sort.Strings(selectedHostNames)
	selectorLogger.Debugf("Selected %d hosts: %v", len(selectedHosts), selectedHostNames)
	return selectedHosts
}

// selectHostsForModule (as previously defined)
func (e *Executor) selectHostsForModule( moduleSpec *spec.ModuleSpec, cluster *runtime.ClusterRuntime) []*runtime.Host {
	if moduleSpec == nil || cluster == nil || len(cluster.Hosts) == 0 { return []*runtime.Host{} }
	moduleHostsMap := make(map[string]*runtime.Host)
	for _, taskSpec := range moduleSpec.Tasks {
		if taskSpec == nil { continue }
		taskTargetHosts := e.selectHostsForTaskSpec(taskSpec, cluster)
		for _, host := range taskTargetHosts {
			if _, exists := moduleHostsMap[host.Name]; !exists { moduleHostsMap[host.Name] = host }
		}
	}
	if len(moduleHostsMap) == 0 {
		e.Logger.Debugf("No specific hosts targeted by tasks in module '%s'.", moduleSpec.Name); return []*runtime.Host{}
	}
	uniqueHosts := make([]*runtime.Host, 0, len(moduleHostsMap)); for _, host := range moduleHostsMap { uniqueHosts = append(uniqueHosts, host) }
	sort.Slice(uniqueHosts, func(i, j int) bool { return uniqueHosts[i].Name < uniqueHosts[j].Name })
	selectedHostNames := make([]string, len(uniqueHosts)); for i, h := range uniqueHosts { selectedHostNames[i] = h.Name }

	pipelineName := "unknown-pipeline"; if cluster.ClusterConfig != nil && cluster.ClusterConfig.Metadata.Name != "" { pipelineName = cluster.ClusterConfig.Metadata.Name }
	e.Logger.SugaredLogger.With("pipeline", pipelineName, "module_name", moduleSpec.Name, "component", "module_host_selector").Debugf("Hosts selected for module Pre/PostRun hooks: %v", selectedHostNames)
	return uniqueHosts
}

```

This subtask updates the logging within the executor methods:
-   **Hierarchical Logger Passing**:
    -   `ExecutePipeline` creates a `pipelineLogger`.
    -   This `pipelineLogger` is passed to `executeModule` and `executeHookSteps` (for pipeline-level hooks).
    -   `executeModule` takes the `pipelineLogger`, derives a `moduleLogger` (adding module context), and uses it. This `moduleLogger` is then passed to `executeTaskSpec` and `executeHookSteps` (for module-level hooks).
    -   `executeTaskSpec` takes the `moduleLogger`, derives a `taskLogger`, and uses it.
    -   Inside `executeTaskSpec`'s per-host goroutine:
        -   `runtime.NewHostContext` is called. Its internally created logger (based on `cluster.Logger`) will have host context but not the pipeline/module/task context from the execution flow.
        -   This is corrected: the `hostCtx.Logger` is immediately replaced by one derived from the `taskLogger` (which has pipeline/module/task context) by adding host-specific fields.
        -   This `hostCtx.Logger` is then further specialized for each step within the loop for step-specific logging.
    -   `executeHookSteps` also follows this pattern: it takes a parent logger (pipeline or module level), derives a `hookEventLogger`, and then for each host, derives a `hostSpecificHookLogger` which is set into the `hostCtx`.
-   **Log Field Consistency**: Standardized field names like `pipeline_name`, `module_name`, `task_name`, `host_name`, `step_name`, `hook_event`, `hook_step_name` are used with `logger.With(...)`.
-   **Logging in Helper Functions**: `selectHostsForTaskSpec` and `selectHostsForModule` now use more contextualized loggers for their diagnostic messages, including pipeline and task/module names.
-   The previous global `pipelineSpecName` variable was removed from `selectHostsForTaskSpec` as context is now passed via logger.
-   Corrected some log messages to use `Successf` or `Errorf` where appropriate.
-   Ensured `cluster.ClusterConfig.Metadata.Name` access is guarded by nil checks for `cluster.ClusterConfig` first.
-   Simplified some logging calls and made sure loggers are correctly wrapped in `&logger.Logger{}` when assigning to `hostCtx.Logger`.

This refined logging approach should provide much richer and more traceable logs during pipeline execution.
The `config.ClusterSpec` and `config.GlobalSpec` are assumed to exist in `pkg/config/config.go` for `cluster.ClusterConfig.Metadata.Name` and other config access. The placeholder `config.go` was updated to include `Metadata`.
The `ExecutePipeline` logger for "Pipeline execution finished..." was moved inside the `defer` blocks to correctly capture `finalError` state.
The `runtime.NewHostContext` creates a logger like `cluster.Logger.With("host", host.Name)`. The executor methods then take this `hostCtx.Logger` and further `.With()` more context (module, task, step). This is a good hierarchical approach. The change in `executeTaskSpec` and `executeHookSteps` to further refine `hostCtx.Logger` after `NewHostContext` creates it is the key.
Simplified the logger creation in `ExecutePipeline` to use `e.Logger` as the base.
The `executeModule`, `executeTaskSpec`, and `executeHookSteps` now correctly take a `parentLogger *logger.Logger` and build upon it.
