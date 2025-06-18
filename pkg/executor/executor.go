package executor

import (
	"context"
	"fmt"
	"sort" // For stable host list if needed after filtering, and for unique hosts
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

// Executor is responsible for interpreting and running PipelineSpec objects.
// It orchestrates the execution of modules, tasks, and steps.
type Executor struct {
	Logger                 *logger.Logger
	DefaultTaskConcurrency int
}

// ExecutorOptions provides configuration options for creating a new Executor.
type ExecutorOptions struct {
	Logger                 *logger.Logger
	DefaultTaskConcurrency int
}

// NewExecutor creates a new instance of an Executor.
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

// ExecutePipeline is the main entry point for running a pipeline defined by a PipelineSpec.
func (e *Executor) ExecutePipeline(
	goCtx context.Context,
	pipelineSpec *spec.PipelineSpec,
	cluster *runtime.ClusterRuntime,
) (allResults []*step.Result, finalError error) {
	if pipelineSpec == nil { return nil, fmt.Errorf("ExecutePipeline: pipelineSpec cannot be nil") }
	if cluster == nil { return nil, fmt.Errorf("ExecutePipeline: cluster runtime cannot be nil") }
	if e.Logger == nil { return nil, fmt.Errorf("ExecutePipeline: executor logger is nil")}


	pipelineLogger := e.Logger.SugaredLogger.With("pipeline", pipelineSpec.Name).Sugar()
	pipelineLogger.Infof("Starting pipeline execution...")
	overallStartTime := time.Now()
	allResults = make([]*step.Result, 0)

	// Defer PostRun hook for pipeline
	if pipelineSpec.PostRun != nil {
		defer func() {
			pipelineLogger.Infof("Executing pipeline PostRun hook: %s", pipelineSpec.PostRun.GetName())
			var postRunTargetHosts []*runtime.Host
			if len(cluster.Hosts) > 0 {
				postRunTargetHosts = []*runtime.Host{cluster.Hosts[0]} // Default to first host for pipeline hooks
			}

			hookResults, hookErr := e.executeHookSteps(goCtx, pipelineSpec.PostRun, "PipelinePostRun", postRunTargetHosts, cluster)
			if hookResults != nil { allResults = append(allResults, hookResults...) }
			if hookErr != nil {
				pipelineLogger.Errorf("Pipeline PostRun hook '%s' failed: %v", pipelineSpec.PostRun.GetName(), hookErr)
				if finalError == nil {
					finalError = fmt.Errorf("pipeline PostRun hook '%s' failed: %w", pipelineSpec.PostRun.GetName(), hookErr)
				} else {
				    pipelineLogger.Errorf("Original pipeline error was: %v", finalError)
				}
			} else {
				pipelineLogger.Infof("Pipeline PostRun hook '%s' completed.", pipelineSpec.PostRun.GetName())
			}
			pipelineLogger.Infof("Pipeline execution finished in %s.", time.Since(overallStartTime))
			if finalError != nil {
			    pipelineLogger.Errorf("Pipeline finished with error: %v", finalError)
			} else {
			    pipelineLogger.Successf("Pipeline finished successfully.")
			}
		}()
	} else {
	    defer func() {
			pipelineLogger.Infof("Pipeline execution finished in %s.", time.Since(overallStartTime))
			if finalError != nil {
			    pipelineLogger.Errorf("Pipeline finished with error: %v", finalError)
			} else {
			    pipelineLogger.Successf("Pipeline finished successfully.")
			}
	    }()
	}

	// 1. Execute Pipeline PreRun Hook
	if pipelineSpec.PreRun != nil {
		pipelineLogger.Infof("Executing pipeline PreRun hook: %s", pipelineSpec.PreRun.GetName())
		var preRunTargetHosts []*runtime.Host
		if len(cluster.Hosts) > 0 {
			preRunTargetHosts = []*runtime.Host{cluster.Hosts[0]} // Default to first host
		}

		hookResults, hookErr := e.executeHookSteps(goCtx, pipelineSpec.PreRun, "PipelinePreRun", preRunTargetHosts, cluster)
		if hookResults != nil { allResults = append(allResults, hookResults...) }
		if hookErr != nil {
			pipelineLogger.Errorf("Pipeline PreRun hook '%s' failed: %v. Halting pipeline.", pipelineSpec.PreRun.GetName(), hookErr)
			finalError = fmt.Errorf("pipeline PreRun hook '%s' failed: %w", pipelineSpec.PreRun.GetName(), hookErr)
			return allResults, finalError
		}
		pipelineLogger.Infof("Pipeline PreRun hook '%s' completed.", pipelineSpec.PreRun.GetName())
	}

	// 2. Iterate through Modules
	for modIdx, moduleSpec := range pipelineSpec.Modules {
		if moduleSpec == nil {
			pipelineLogger.Warnf("Skipping nil moduleSpec at index %d in pipeline '%s'.", modIdx, pipelineSpec.Name)
			continue
		}
		moduleLogger := pipelineLogger.With("module", moduleSpec.Name, "module_idx", fmt.Sprintf("%d/%d", modIdx+1, len(pipelineSpec.Modules))).Sugar()

		if moduleSpec.IsEnabled != nil && !moduleSpec.IsEnabled(cluster.ClusterConfig) {
			moduleLogger.Infof("Module is disabled by configuration, skipping.")
			continue
		}
		moduleLogger.Infof("Starting module execution...")

		moduleResults, moduleErr := e.executeModule(goCtx, moduleSpec, cluster)
		if moduleResults != nil { allResults = append(allResults, moduleResults...) }
		if moduleErr != nil {
			moduleLogger.Errorf("Module execution failed: %v. Halting pipeline.", moduleErr)
			finalError = moduleErr
			return allResults, finalError
		}
		moduleLogger.Successf("Module execution completed successfully.")
	}
	return allResults, nil
}

// executeModule orchestrates the execution of a single module's PreRun, Tasks, and PostRun.
func (e *Executor) executeModule(
	goCtx context.Context,
	moduleSpec *spec.ModuleSpec,
	cluster *runtime.ClusterRuntime,
) (allModuleStepResults []*step.Result, moduleError error) {

	pipelineName := "unknown-pipeline"
	if cluster.ClusterConfig != nil && cluster.ClusterConfig.Metadata.Name != "" {
		pipelineName = cluster.ClusterConfig.Metadata.Name
	}
	moduleLogger := e.Logger.SugaredLogger.With("pipeline", pipelineName, "module", moduleSpec.Name).Sugar()
	moduleLogger.Infof("Starting module...")
	allModuleStepResults = make([]*step.Result, 0)

	moduleTargetHosts := e.selectHostsForModule(moduleSpec, cluster)
	if len(moduleTargetHosts) == 0 && (moduleSpec.PreRun != nil || moduleSpec.PostRun != nil || len(moduleSpec.Tasks) > 0) {
	    moduleLogger.Warnf("No target hosts identified for any task in module. Pre/PostRun hooks (if any) and tasks will be skipped.")
	}

	if moduleSpec.PostRun != nil {
		defer func() {
			moduleLogger.Infof("Executing module PostRun hook: %s", moduleSpec.PostRun.GetName())
			if len(moduleTargetHosts) > 0 {
				hookResults, hookErr := e.executeHookSteps(goCtx, moduleSpec.PostRun, fmt.Sprintf("ModulePostRun[%s]", moduleSpec.Name), moduleTargetHosts, cluster)
				if hookResults != nil { allModuleStepResults = append(allModuleStepResults, hookResults...) }
				if hookErr != nil {
					moduleLogger.Errorf("Module PostRun hook '%s' failed: %v", moduleSpec.PostRun.GetName(), hookErr)
					if moduleError == nil { moduleError = fmt.Errorf("module PostRun hook '%s' failed: %w", moduleSpec.PostRun.GetName(), hookErr) }
				} else { moduleLogger.Infof("Module PostRun hook '%s' completed.", moduleSpec.PostRun.GetName()) }
			} else { moduleLogger.Infof("Skipping module PostRun hook '%s' as no target hosts were identified for the module.", moduleSpec.PostRun.GetName()) }
		}()
	}

	if moduleSpec.PreRun != nil {
		moduleLogger.Infof("Executing module PreRun hook: %s", moduleSpec.PreRun.GetName())
		if len(moduleTargetHosts) > 0 {
			hookResults, hookErr := e.executeHookSteps(goCtx, moduleSpec.PreRun, fmt.Sprintf("ModulePreRun[%s]", moduleSpec.Name), moduleTargetHosts, cluster)
			if hookResults != nil { allModuleStepResults = append(allModuleStepResults, hookResults...) }
			if hookErr != nil {
				moduleLogger.Errorf("Module PreRun hook '%s' failed: %v. Halting module tasks.", moduleSpec.PreRun.GetName(), hookErr)
				moduleError = fmt.Errorf("module PreRun hook '%s' failed: %w", moduleSpec.PreRun.GetName(), hookErr)
				return allModuleStepResults, moduleError
			}
			moduleLogger.Infof("Module PreRun hook '%s' completed.", moduleSpec.PreRun.GetName())
		} else { moduleLogger.Infof("Skipping module PreRun hook '%s' as no target hosts were identified for the module.", moduleSpec.PreRun.GetName()) }
	}

	for taskIdx, taskSpec := range moduleSpec.Tasks {
		if taskSpec == nil {
			moduleLogger.Warnf("Skipping nil taskSpec at index %d in module '%s'.", taskIdx, moduleSpec.Name)
			continue
		}
		taskLogger := moduleLogger.With("task", taskSpec.Name, "task_idx", fmt.Sprintf("%d/%d", taskIdx+1, len(moduleSpec.Tasks))).Sugar()
		taskTargetHosts := e.selectHostsForTaskSpec(taskSpec, cluster)
		if len(taskTargetHosts) == 0 { taskLogger.Infof("No target hosts for task, skipping."); continue }
		taskLogger.Infof("Running task on %d hosts...", len(taskTargetHosts))
		taskStepResults, currentTaskErr := e.executeTaskSpec(goCtx, taskSpec, taskTargetHosts, cluster)
		if taskStepResults != nil { allModuleStepResults = append(allModuleStepResults, taskStepResults...) }
		if currentTaskErr != nil {
			taskLogger.Errorf("Task execution failed: %v", currentTaskErr)
			if !taskSpec.IgnoreError {
				moduleLogger.Errorf("Task '%s' failed critically. Halting module.", taskSpec.Name)
				moduleError = fmt.Errorf("task '%s' in module '%s' failed: %w", taskSpec.Name, moduleSpec.Name, currentTaskErr)
				return allModuleStepResults, moduleError
			}
			taskLogger.Warnf("Task '%s' failed, but error is ignored by task spec. Continuing module.", taskSpec.Name)
		} else { taskLogger.Successf("Task completed successfully.") }
	}
	if moduleError != nil { moduleLogger.Errorf("Module finished with error: %v", moduleError)
	} else { moduleLogger.Successf("Module finished successfully.") }
	return allModuleStepResults, moduleError
}

// executeTaskSpec orchestrates the execution of a single task's steps concurrently across multiple hosts.
func (e *Executor) executeTaskSpec(
	goCtx context.Context,
	taskSpec *spec.TaskSpec,
	hosts []*runtime.Host,
	cluster *runtime.ClusterRuntime,
) (allTaskStepResults []*step.Result, taskError error) {
	pipelineName := "unknown-pipeline"
	if cluster.ClusterConfig != nil && cluster.ClusterConfig.Metadata.Name != "" {
		pipelineName = cluster.ClusterConfig.Metadata.Name
	}
	taskLogger := e.Logger.SugaredLogger.With("pipeline", pipelineName, "task", taskSpec.Name).Sugar()
	taskLogger.Debugf("Starting task on %d hosts with concurrency %d", len(hosts), taskSpec.Concurrency)
	allTaskStepResults = make([]*step.Result, 0, len(hosts)*len(taskSpec.Steps)); var resultsMu sync.Mutex
	g, egCtx := errgroup.WithContext(goCtx)
	concurrency := taskSpec.Concurrency; if concurrency <= 0 { concurrency = e.DefaultTaskConcurrency }; g.SetLimit(concurrency)

	for _, h := range hosts {
		currentHost := h
		g.Go(func() error {
			hostCtx := runtime.NewHostContext(egCtx, currentHost, cluster)
			hostLogger := taskLogger.With("host", currentHost.Name).Sugar(); hostCtx.Logger = &logger.Logger{SugaredLogger: hostLogger}
			hostLogger.Debugf("Starting step sequence for task")
			for stepIdx, stepSpecInstance := range taskSpec.Steps {
				if stepSpecInstance == nil {
					hostLogger.Warnf("Skipping nil stepSpecInstance at index %d in task '%s'", stepIdx, taskSpec.Name)
					continue
				}
				stepLogger := hostLogger.With("step", stepSpecInstance.GetName(), "step_idx", fmt.Sprintf("%d/%d", stepIdx+1, len(taskSpec.Steps))).Sugar(); hostCtx.Logger = &logger.Logger{SugaredLogger: stepLogger}
				stepExecutor := step.GetExecutor(step.GetSpecTypeName(stepSpecInstance))
				if stepExecutor == nil {
					err := fmt.Errorf("no executor registered for step spec type: %s (step: %s)", step.GetSpecTypeName(stepSpecInstance), stepSpecInstance.GetName())
					stepLogger.Errorf("Critical error: %v", err); res := step.NewResult(stepSpecInstance.GetName(),currentHost.Name,time.Now(),err); res.Status="Failed"; res.Message="Internal error: Step executor not found."
					resultsMu.Lock(); allTaskStepResults = append(allTaskStepResults, res); resultsMu.Unlock(); return err
				}
				stepLogger.Debugf("Checking step..."); isDone, checkErr := stepExecutor.Check(stepSpecInstance, hostCtx)
				if checkErr != nil {
					err := fmt.Errorf("step '%s' pre-check failed: %w", stepSpecInstance.GetName(), checkErr); stepLogger.Errorf("Step pre-check failed: %v", checkErr)
					res := step.NewResult(stepSpecInstance.GetName()+" [CheckPhase]",currentHost.Name,time.Now(),err); res.Message=err.Error()
					resultsMu.Lock(); allTaskStepResults = append(allTaskStepResults, res); resultsMu.Unlock(); return err
				}
				if isDone {
					stepLogger.Infof("Step is already done, skipping."); res := step.NewResult(stepSpecInstance.GetName(),currentHost.Name,time.Now(),nil)
					res.Status="Skipped"; res.Message="Condition already met."; res.EndTime=time.Now()
					resultsMu.Lock(); allTaskStepResults = append(allTaskStepResults, res); resultsMu.Unlock(); continue
				}
				stepLogger.Infof("Running step..."); execResult := stepExecutor.Execute(stepSpecInstance, hostCtx)
				if execResult == nil {
					err := fmt.Errorf("step '%s' Execute method returned a nil result", stepSpecInstance.GetName()); stepLogger.Errorf("%v", err)
					res := step.NewResult(stepSpecInstance.GetName(),currentHost.Name,time.Now(),err); res.Status = "Failed"; res.Message = "Internal error: Step Execute returned nil result."
					resultsMu.Lock(); allTaskStepResults = append(allTaskStepResults, res); resultsMu.Unlock(); return err
				}
				resultsMu.Lock(); allTaskStepResults = append(allTaskStepResults, execResult); resultsMu.Unlock()
				if execResult.Status == "Failed" {
					errToPropagate := execResult.Error
					if errToPropagate == nil { errToPropagate = fmt.Errorf("step '%s' on host '%s' reported status Failed without specific error. Message: %s", execResult.StepName, currentHost.Name, execResult.Message) }
					stepLogger.Errorf("Step failed: %v", errToPropagate); return fmt.Errorf("step '%s' failed on host '%s': %w", execResult.StepName, currentHost.Name, errToPropagate)
				}
				stepLogger.Successf("Step completed successfully. %s", execResult.Message)
			}
			hostLogger.Debugf("Step sequence for task completed on host.")
			return nil
		})
	}
	taskError = g.Wait()
	if taskError != nil { taskLogger.Errorf("Task finished with at least one host error: %v", taskError)
	} else { taskLogger.Successf("Task finished successfully on all %d hosts.", len(hosts)) }
	return allTaskStepResults, taskError
}

// executeHookSteps executes a single hook StepSpec on a list of target hosts sequentially.
// If the hook step fails on any host, it records the error and continues with other hosts.
// The first error encountered is returned as the overall hookError.
func (e *Executor) executeHookSteps(
	goCtx context.Context,         // Parent context
	hookSpec spec.StepSpec,        // The single step spec for the hook
	hookNameForLog string,         // A descriptive name for logging (e.g., "PipelinePreRun", "ModulePostRun[MyModule]")
	targetHosts []*runtime.Host,   // List of hosts to run this hook step on
	cluster *runtime.ClusterRuntime,
) (hookResults []*step.Result, firstError error) {

	if hookSpec == nil {
		e.Logger.Debugf("Hook '%s' is nil, skipping.", hookNameForLog)
		return nil, nil
	}
	if len(targetHosts) == 0 {
		e.Logger.Debugf("No target hosts for hook '%s' (%s), skipping.", hookNameForLog, hookSpec.GetName())
		return nil, nil
	}

	pipelineName := "unknown-pipeline"
	if cluster.ClusterConfig != nil && cluster.ClusterConfig.Metadata.Name != "" {
		pipelineName = cluster.ClusterConfig.Metadata.Name
	}
	hookLoggerBase := e.Logger.SugaredLogger.With("pipeline", pipelineName, "hook_event", hookNameForLog, "hook_step", hookSpec.GetName()).Sugar()

	hookResults = make([]*step.Result, 0, len(targetHosts))

	for _, host := range targetHosts {
		hostCtx := runtime.NewHostContext(goCtx, host, cluster)
		hostHookLogger := hookLoggerBase.With("host", host.Name).Sugar()
		hostCtx.Logger = &logger.Logger{SugaredLogger: hostHookLogger}

		hostHookLogger.Infof("Executing on host...")

		stepExecutor := step.GetExecutor(step.GetSpecTypeName(hookSpec))
		if stepExecutor == nil {
			err := fmt.Errorf("no executor registered for hook step spec type: %s (step: %s)", step.GetSpecTypeName(hookSpec), hookSpec.GetName())
			hostHookLogger.Errorf("Critical error: %v", err)
			res := step.NewResult(hookSpec.GetName(), host.Name, time.Now(), err)
			res.Status = "Failed"; res.Message = "Internal error: Hook Step executor not found."
			hookResults = append(hookResults, res)
			if firstError == nil { firstError = err }
			continue
		}

		hostHookLogger.Debugf("Checking hook step...")
		isDone, checkErr := stepExecutor.Check(hookSpec, hostCtx)
		if checkErr != nil {
			err := fmt.Errorf("hook step '%s' pre-check failed: %w", hookSpec.GetName(), checkErr)
			hostHookLogger.Errorf("Hook step pre-check failed: %v", checkErr)
			res := step.NewResult(hookSpec.GetName()+" [CheckPhase]", host.Name, time.Now(), err)
			res.Message = err.Error()
			hookResults = append(hookResults, res)
			if firstError == nil { firstError = err }
			continue
		}

		if isDone {
			hostHookLogger.Infof("Hook step is already done (or condition met), skipping execution.")
			res := step.NewResult(hookSpec.GetName(), host.Name, time.Now(), nil)
			res.Status = "Skipped"; res.Message = "Hook condition already met or task already completed."
			res.EndTime = time.Now()
			hookResults = append(hookResults, res)
			continue
		}

		hostHookLogger.Infof("Running hook step...")
		execResult := stepExecutor.Execute(hookSpec, hostCtx)
		if execResult == nil {
			err := fmt.Errorf("hook step '%s' Execute method returned a nil result", hookSpec.GetName())
			hostHookLogger.Errorf("%v", err)
			res := step.NewResult(hookSpec.GetName(), host.Name, time.Now(), err)
			res.Status = "Failed"; res.Message = "Internal error: Hook step Execute returned nil result."
			hookResults = append(hookResults, res)
			if firstError == nil { firstError = err }
            continue
		}

		hookResults = append(hookResults, execResult)

		if execResult.Status == "Failed" {
			errToPropagate := execResult.Error
			if errToPropagate == nil {
				errToPropagate = fmt.Errorf("hook step '%s' on host '%s' reported status Failed without specific error. Message: %s", execResult.StepName, host.Name, execResult.Message)
			}
			hostHookLogger.Errorf("Hook step failed: %v", errToPropagate)
			if firstError == nil {
				firstError = fmt.Errorf("hook step '%s' failed on host '%s': %w", execResult.StepName, host.Name, errToPropagate)
			}
		} else {
			hostHookLogger.Successf("Hook step completed successfully. %s", execResult.Message)
		}
	}

	if firstError != nil {
		hookLoggerBase.Errorf("Hook execution completed with at least one host error: %v", firstError)
	} else {
		hookLoggerBase.Successf("Hook execution completed successfully on all %d target hosts.", len(targetHosts))
	}
	return hookResults, firstError
}

// selectHostsForTaskSpec filters hosts for a specific task based on its RunOnRoles and Filter.
func (e *Executor) selectHostsForTaskSpec(
	taskSpec *spec.TaskSpec,
	cluster *runtime.ClusterRuntime,
) []*runtime.Host {
	var selectedHosts []*runtime.Host

	if cluster == nil || len(cluster.Hosts) == 0 {
		e.Logger.Debugf("selectHostsForTaskSpec: No hosts in cluster runtime to filter from.")
		return selectedHosts
	}
	if taskSpec == nil {
		e.Logger.Warnf("selectHostsForTaskSpec: taskSpec is nil, cannot determine target hosts.")
		return selectedHosts
	}

	pipelineName := "unknown-pipeline"
	if cluster.ClusterConfig != nil && cluster.ClusterConfig.Metadata.Name != "" {
	    pipelineName = cluster.ClusterConfig.Metadata.Name
	}

	taskLogger := e.Logger.SugaredLogger.With(
		"pipeline", pipelineName,
		"task", taskSpec.Name,
		"component", "host_selector",
	).Sugar()

	taskLogger.Debugf("Selecting hosts (Task Roles: %v, Task HasFilter: %v)", taskSpec.RunOnRoles, taskSpec.Filter != nil)

	for _, host := range cluster.Hosts {
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
			var hostRoleNames []string
			for roleName, present := range host.Roles {
				if present { hostRoleNames = append(hostRoleNames, roleName) }
			}
			sort.Strings(hostRoleNames)

			taskLogger.Debugf("Host '%s' skipped: role mismatch (host roles: [%s], task needs one of: %v)", host.Name, strings.Join(hostRoleNames, ", "), taskSpec.RunOnRoles)
			continue
		}

		filterMatch := true
		if taskSpec.Filter != nil {
			filterMatch = taskSpec.Filter(host)
			if !filterMatch {
				taskLogger.Debugf("Host '%s' skipped: custom filter returned false", host.Name)
			}
		}

		if filterMatch {
			selectedHosts = append(selectedHosts, host)
		}
	}

	selectedHostNames := make([]string, len(selectedHosts))
	for i, h := range selectedHosts {
		selectedHostNames[i] = h.Name
	}
	sort.Strings(selectedHostNames)
	taskLogger.Debugf("Selected %d hosts: %v", len(selectedHosts), selectedHostNames)

	return selectedHosts
}

// selectHostsForModule determines the comprehensive list of unique hosts a module's
// PreRun and PostRun hooks should execute on. This is achieved by taking the union
// of all hosts targeted by any task within that module.
func (e *Executor) selectHostsForModule(
    moduleSpec *spec.ModuleSpec,
    cluster *runtime.ClusterRuntime,
) []*runtime.Host {
	if moduleSpec == nil || cluster == nil || len(cluster.Hosts) == 0 {
		e.Logger.Debugf("selectHostsForModule: ModuleSpec or ClusterRuntime is nil, or no hosts in cluster. Returning empty host list.")
		return []*runtime.Host{}
	}

	moduleHostsMap := make(map[string]*runtime.Host) // Use a map to ensure uniqueness by host name

	pipelineName := "unknown-pipeline"
	if cluster.ClusterConfig != nil && cluster.ClusterConfig.Metadata.Name != "" {
		pipelineName = cluster.ClusterConfig.Metadata.Name
	}
	moduleSelectionLogger := e.Logger.SugaredLogger.With(
		"pipeline", pipelineName,
		"module", moduleSpec.Name,
		"component", "module_host_selector",
	).Sugar()

	moduleSelectionLogger.Debugf("Determining unique hosts for module based on its tasks...")

	if len(moduleSpec.Tasks) == 0 {
		moduleSelectionLogger.Debugf("Module has no tasks. Module-level hooks will target 0 hosts by this selection logic.")
		// Depending on desired behavior, could default to all hosts, or first host, if module has no tasks but has hooks.
		// For now, hooks run on hosts targeted by tasks. If no tasks, no hosts for hooks from this logic.
		return []*runtime.Host{}
	}

	for _, taskSpec := range moduleSpec.Tasks {
		if taskSpec == nil { continue } // Skip nil task specs
		taskTargetHosts := e.selectHostsForTaskSpec(taskSpec, cluster) // This already logs verbosely
		for _, host := range taskTargetHosts {
			if _, exists := moduleHostsMap[host.Name]; !exists {
				moduleHostsMap[host.Name] = host
			}
		}
	}

	if len(moduleHostsMap) == 0 {
		moduleSelectionLogger.Warnf("No specific hosts targeted by any task in module. Module hooks will target 0 hosts by this selection.")
		return []*runtime.Host{}
	}

	uniqueHosts := make([]*runtime.Host, 0, len(moduleHostsMap))
	for _, host := range moduleHostsMap {
		uniqueHosts = append(uniqueHosts, host)
	}

	// Sort for deterministic order, although not strictly necessary for functionality here.
	sort.Slice(uniqueHosts, func(i, j int) bool {
		return uniqueHosts[i].Name < uniqueHosts[j].Name
	})

	selectedHostNames := make([]string, len(uniqueHosts))
	for i, h := range uniqueHosts {
		selectedHostNames[i] = h.Name
	}
	moduleSelectionLogger.Debugf("Unique hosts selected for module Pre/PostRun hooks: %v", selectedHostNames)

	return uniqueHosts
}
```
