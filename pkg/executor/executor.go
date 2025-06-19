package executor

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/kubexms/kubexms/pkg/config"
	"github.com/kubexms/kubexms/pkg/logger"
	"github.com/kubexms/kubexms/pkg/runtime"
	"github.com/kubexms/kubexms/pkg/spec"
	"github.com/kubexms/kubexms/pkg/step"
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

			var targetHosts []*runtime.Host
			allHosts := cluster.GetAllHosts()
			if len(allHosts) > 0 {
				targetHosts = []*runtime.Host{allHosts[0]}
			}

			// Pass the pipelineLogger as the parent for the hook execution
			hookResults, hookErr := e.executeHookSteps(goCtx, pipelineSpec.PostRun, "PipelinePostRun", targetHosts, cluster, pipelineLogger)
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
		var targetHosts []*runtime.Host
		allHosts := cluster.GetAllHosts()
		if len(allHosts) > 0 {
			targetHosts = []*runtime.Host{allHosts[0]}
		}

		hookResults, hookErr := e.executeHookSteps(goCtx, pipelineSpec.PreRun, "PipelinePreRun", targetHosts, cluster, pipelineLogger)
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

		if moduleSpec.IsEnabled != nil && !moduleSpec.IsEnabled(cluster.ClusterConfig) {
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
			if len(modTargetHosts) > 0 {
				// Pass the moduleLogger as the parent for the hook execution
				hookRes, hookErr := e.executeHookSteps(goCtx, moduleSpec.PostRun, fmt.Sprintf("ModulePostRun[%s]", moduleSpec.Name), modTargetHosts, cluster, moduleLogger)
				if hookRes != nil { allModStepRes = append(allModStepRes, hookRes...) }
				if hookErr != nil {
					postRunHookEventLogger.Errorf("Failed: %v", hookErr)
					if modErr == nil { modErr = fmt.Errorf("module PostRun hook '%s' failed: %w", moduleSpec.PostRun.GetName(), hookErr) }
				} else { postRunHookEventLogger.Infof("Completed.") }
			} else { postRunHookEventLogger.Infof("Skipping (no target hosts).") }
		}()
	}

	if moduleSpec.PreRun != nil {
		preRunHookEventLogger := moduleLogger.SugaredLogger.With("hook_event", fmt.Sprintf("ModulePreRun[%s]", moduleSpec.Name), "hook_step", moduleSpec.PreRun.GetName()).Sugar()
		preRunHookEventLogger.Infof("Executing...")
		if len(modTargetHosts) > 0 {
			hookRes, hookErr := e.executeHookSteps(goCtx, moduleSpec.PreRun, fmt.Sprintf("ModulePreRun[%s]", moduleSpec.Name), modTargetHosts, cluster, moduleLogger)
			if hookRes != nil { allModStepRes = append(allModStepRes, hookRes...) }
			if hookErr != nil {
				modErr = fmt.Errorf("module PreRun hook '%s' failed: %w", moduleSpec.PreRun.GetName(), hookErr)
				preRunHookEventLogger.Errorf("Failed: %v. Halting module tasks.", modErr); return allModStepRes, modErr
			}
			preRunHookEventLogger.Infof("Completed.")
		} else { preRunHookEventLogger.Infof("Skipping (no target hosts).") }
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
			// NewHostContext uses cluster.Logger as its base, then adds host fields.
			// We want the logger in HostContext to have pipeline, module, task, AND host context.
			hostCtx := runtime.NewHostContext(egCtx, currentHost, cluster)
			// So, we take the parentTaskLogger (which has pipeline, module, task context)
			// and add host context to it, then set it in hostCtx.
			hostSpecificLogger := &logger.Logger{SugaredLogger: parentTaskLogger.SugaredLogger.With("host_name", currentHost.Name)}
			hostCtx.Logger = hostSpecificLogger


			hostSpecificLogger.Debugf("Starting step sequence")
			for stepIdx, stepSpecInstance := range taskSpec.Steps {
				if stepSpecInstance == nil {
					hostSpecificLogger.Warnf("Skipping nil stepSpecInstance at index %d in task '%s'", stepIdx, taskSpec.Name)
					continue
				}
				// Create logger for this step, deriving from the host-specific task logger
				stepLoggerInLoop := &logger.Logger{SugaredLogger: hostSpecificLogger.SugaredLogger.With("step_name", stepSpecInstance.GetName(), "step_idx", fmt.Sprintf("%d/%d", stepIdx+1, len(taskSpec.Steps)))}
				hostCtx.Logger = stepLoggerInLoop // Update context's logger for this specific step's Check/Execute call

				stepExecutor := step.GetExecutor(step.GetSpecTypeName(stepSpecInstance))
				if stepExecutor == nil {
					err := fmt.Errorf("no executor for type %s (step: %s)", step.GetSpecTypeName(stepSpecInstance), stepSpecInstance.GetName())
					stepLoggerInLoop.Errorf("Critical: %v", err); res := step.NewResult(stepSpecInstance.GetName(),currentHost.Name,time.Now(),err); res.Status="Failed"; res.Message="Internal: Executor not found."
					resultsMu.Lock(); allTaskStepResults = append(allTaskStepResults, res); resultsMu.Unlock(); return err
				}
				stepLoggerInLoop.Debugf("Checking..."); isDone, checkErr := stepExecutor.Check(stepSpecInstance, hostCtx)
				if checkErr != nil {
					err := fmt.Errorf("step '%s' pre-check failed: %w", stepSpecInstance.GetName(), checkErr); stepLoggerInLoop.Errorf("Pre-check failed: %v", checkErr)
					res := step.NewResult(stepSpecInstance.GetName()+" [CheckPhase]",currentHost.Name,time.Now(),err); res.Message=err.Error()
					resultsMu.Lock(); allTaskStepResults = append(allTaskStepResults, res); resultsMu.Unlock(); return err
				}
				if isDone {
					stepLoggerInLoop.Infof("Skipped (already done)."); res := step.NewResult(stepSpecInstance.GetName(),currentHost.Name,time.Now(),nil)
					res.Status="Skipped"; res.Message="Condition met."; res.EndTime=time.Now()
					resultsMu.Lock(); allTaskStepResults = append(allTaskStepResults, res); resultsMu.Unlock(); continue
				}
				stepLoggerInLoop.Infof("Running..."); execResult := stepExecutor.Execute(stepSpecInstance, hostCtx)
				if execResult == nil {
					err := fmt.Errorf("step '%s' Execute nil result", stepSpecInstance.GetName()); stepLoggerInLoop.Errorf("%v", err)
					res := step.NewResult(stepSpecInstance.GetName(),currentHost.Name,time.Now(),err); res.Status = "Failed"; res.Message = "Internal error: Step Execute returned nil result."
					resultsMu.Lock(); allTaskStepResults = append(allTaskStepResults, res); resultsMu.Unlock(); return err
				}
				resultsMu.Lock(); allTaskStepResults = append(allTaskStepResults, execResult); resultsMu.Unlock()
				if execResult.Status == "Failed" {
					errToPropagate := execResult.Error
					if errToPropagate == nil { errToPropagate = fmt.Errorf("step '%s' on host '%s' reported status Failed without specific error. Message: %s", execResult.StepName, currentHost.Name, execResult.Message) }
					stepLoggerInLoop.Errorf("Failed: %v", errToPropagate); return fmt.Errorf("step '%s' failed on host '%s': %w", execResult.StepName, currentHost.Name, errToPropagate)
				}
				stepLoggerInLoop.Successf("Completed. %s", execResult.Message)
			}
			hostSpecificLogger.Debugf("Step sequence completed.")
			return nil
		})
	}
	taskError = g.Wait()
	if taskError != nil { taskLogger.Errorf("Task finished with host error(s): %v", taskError)
	} else { taskLogger.Successf("Task finished successfully on all %d hosts.", len(hosts)) }
	return allTaskStepResults, taskError
}

// executeHookSteps now accepts a parentLogger (pipeline or module specific)
func (e *Executor) executeHookSteps(
	goCtx context.Context, hookSpec spec.StepSpec, hookNameForLog string,
	targetHosts []*runtime.Host, cluster *runtime.ClusterRuntime, parentHookLogger *logger.Logger,
) (hookResults []*step.Result, firstError error) {
	if hookSpec == nil { parentHookLogger.SugaredLogger.Debugf("Hook '%s' is nil, skipping.", hookNameForLog); return nil, nil }
	if len(targetHosts) == 0 { parentHookLogger.SugaredLogger.Debugf("No target hosts for hook '%s' (%s), skipping.", hookNameForLog, hookSpec.GetName()); return nil, nil }

	// Create a base logger for this specific hook event, derived from the parent (pipeline/module) logger
	hookEventLogger := parentHookLogger.SugaredLogger.With("hook_event", hookNameForLog, "hook_step_name", hookSpec.GetName()).Sugar()
	hookResults = make([]*step.Result, 0, len(targetHosts))

	for _, host := range targetHosts {
		// runtime.NewHostContext uses cluster.Logger. We need to pass our enriched logger.
		// Modifying NewHostContext to accept a base logger, or set it after.
		hostCtx := runtime.NewHostContext(goCtx, host, cluster)
		// Create a logger specific to this hook AND this host, from the hookEventLogger
		hostSpecificHookLogger := &logger.Logger{SugaredLogger: hookEventLogger.With("host_name", host.Name)}
		hostCtx.Logger = hostSpecificHookLogger // Update context's logger

		hostSpecificHookLogger.Infof("Executing on host...")
		stepExecutor := step.GetExecutor(step.GetSpecTypeName(hookSpec))
		if stepExecutor == nil {
			err := fmt.Errorf("no executor for hook type %s (step: %s)", step.GetSpecTypeName(hookSpec), hookSpec.GetName())
			hostSpecificHookLogger.Errorf("Critical: %v", err); res := step.NewResult(hookSpec.GetName(), host.Name, time.Now(), err)
			res.Status = "Failed"; res.Message = "Internal: Hook executor not found."; hookResults = append(hookResults, res)
			if firstError == nil { firstError = err }; continue
		}
		hostSpecificHookLogger.Debugf("Checking hook step..."); isDone, checkErr := stepExecutor.Check(hookSpec, hostCtx)
		if checkErr != nil {
			err := fmt.Errorf("hook step '%s' pre-check failed: %w", hookSpec.GetName(), checkErr); hostSpecificHookLogger.Errorf("Pre-check failed: %v", checkErr)
			res := step.NewResult(hookSpec.GetName()+" [CheckPhase]", host.Name, time.Now(), err); res.Message = err.Error()
			hookResults = append(hookResults, res); if firstError == nil { firstError = err }; continue
		}
		if isDone {
			hostSpecificHookLogger.Infof("Skipped (already done)."); res := step.NewResult(hookSpec.GetName(), host.Name, time.Now(), nil)
			res.Status = "Skipped"; res.Message = "Hook condition met."; res.EndTime = time.Now()
			hookResults = append(hookResults, res); continue
		}
		hostSpecificHookLogger.Infof("Running hook step..."); execResult := stepExecutor.Execute(hookSpec, hostCtx)
		if execResult == nil {
			err := fmt.Errorf("hook step '%s' Execute nil result", hookSpec.GetName()); hostSpecificHookLogger.Errorf("%v", err)
			res := step.NewResult(hookSpec.GetName(),host.Name,time.Now(),err); res.Status="Failed"; res.Message="Internal: Hook Execute returned nil result."
			hookResults = append(hookResults, res); if firstError == nil { firstError = err }; continue
		}
		hookResults = append(hookResults, execResult)
		if execResult.Status == "Failed" {
			errToPropagate := execResult.Error
			if errToPropagate == nil { errToPropagate = fmt.Errorf("hook step '%s' on host '%s' reported status Failed without specific error. Message: %s", execResult.StepName, host.Name, execResult.Message)}
			hostSpecificHookLogger.Errorf("Failed: %v", errToPropagate);
			if firstError == nil { firstError = fmt.Errorf("hook step '%s' failed on host '%s': %w", execResult.StepName, host.Name, errToPropagate) }
		} else { hostSpecificHookLogger.Successf("Completed. %s", execResult.Message) }
	}
	if firstError != nil { hookEventLogger.Errorf("Hook execution completed with host error(s): %v", firstError)
	} else { hookEventLogger.Successf("Hook execution completed on all %d hosts.", len(targetHosts)) }
	return hookResults, firstError
}

// selectHostsForTaskSpec (as previously defined)
func (e *Executor) selectHostsForTaskSpec(taskSpec *spec.TaskSpec, cluster *runtime.ClusterRuntime) []*runtime.Host {
	var selectedHosts []*runtime.Host
	if cluster == nil || len(cluster.GetAllHosts()) == 0 { // Use GetAllHosts()
		e.Logger.Debugf("selectHostsForTaskSpec: No hosts in cluster.")
		return selectedHosts
	}
	if taskSpec == nil {
		e.Logger.Warnf("selectHostsForTaskSpec: taskSpec nil.")
		return selectedHosts
	}

	pipelineName := "unknown-pipeline"
	if cluster.ClusterConfig != nil && cluster.ClusterConfig.Metadata.Name != "" {
		pipelineName = cluster.ClusterConfig.Metadata.Name
	}

	selectorLogger := e.Logger.SugaredLogger.With("pipeline", pipelineName, "task_name_selecting_for", taskSpec.Name, "component", "host_selector").Sugar()
	selectorLogger.Debugf("Selecting hosts (Roles: %v, HasFilter: %v)", taskSpec.RunOnRoles, taskSpec.Filter != nil)
	for _, host := range cluster.GetAllHosts() { // Use GetAllHosts()
		roleMatch := false
		if len(taskSpec.RunOnRoles) == 0 {
			roleMatch = true
		} else {
			for _, requiredRole := range taskSpec.RunOnRoles {
				if host.HasRole(requiredRole) {
					roleMatch = true
					break
				}
			}
		}
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
func (e *Executor) selectHostsForModule(moduleSpec *spec.ModuleSpec, cluster *runtime.ClusterRuntime) []*runtime.Host {
	if moduleSpec == nil || cluster == nil || len(cluster.GetAllHosts()) == 0 { // Use GetAllHosts()
		return []*runtime.Host{}
	}
	moduleHostsMap := make(map[string]*runtime.Host)
	for _, taskSpec := range moduleSpec.Tasks {
		if taskSpec == nil {
			continue
		}
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
