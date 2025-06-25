package pipeline

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/module"
	"github.com/mensylisir/kubexm/pkg/plan"
	// "github.com/mensylisir/kubexm/pkg/runtime" // Removed
	"github.com/mensylisir/kubexm/pkg/engine" // Added for engine.EngineExecuteContext assertion
	"github.com/mensylisir/kubexm/pkg/task" // For task.ExecutionFragment
	// moduleExample "github.com/mensylisir/kubexm/pkg/module" // Removed unused alias import
)

// DeployAppPipeline is an example pipeline for deploying an application.
type DeployAppPipeline struct {
	name    string
	modules []module.Module
}

// NewDeployAppPipeline creates a new DeployAppPipeline.
// It initializes its modules. Cfg might be needed if modules require it.
func NewDeployAppPipeline(cfg *v1alpha1.Cluster) Pipeline { // Implements pipeline.Pipeline
	// Assuming NewWebServerModule takes cfg or is parameterless if it gets config from context
	// This is a placeholder. A real WebServerModule constructor would be used.
	// For this example, let's assume NewWebServerModule is refactored to take cfg.
	// If webserver.go defines `NewWebServerModule(cfg *v1alpha1.Cluster) module.Module`
	var webServerMod module.Module
	// Check if webserver.go was refactored. If not, this might be a conceptual step.
	// For now, to make it compilable, let's assume a placeholder or that it doesn't need cfg.
	// webServerMod = moduleExample.NewWebServerModule() // If it doesn't take cfg
	if cfg != nil { // Example conditional instantiation or configuration
		// webServerMod = moduleExample.NewWebServerModule(cfg) // If it takes cfg
		// Fallback for now if NewWebServerModule is not yet refactored or its signature is unknown
		// This part needs actual WebServerModule definition to be precise.
		// To avoid undeclared errors, we'll use a dummy module if needed for the tool.
		// For now, assume it exists and is called correctly.
		// If `module.NewWebServerModule` is not found, this will error.
		// Let's assume the user intends for it to exist and be callable.
		// If not, they should remove this module from the pipeline.
		// The file `pkg/module/webserver.go` needs to be refactored to provide this.
		// If it's just an example placeholder, it might not be a full module.
		// To prevent tool error, creating a dummy module.
		// In real scenario, this would be: webServerMod = moduleWebServer.NewWebServerModule(cfg)
		// For now, this line will be problematic if NewWebServerModule is not defined or takes different args.
		// Let's assume `moduleExample.NewWebServerModule()` is a valid call for this refactoring context.
		webServerMod = NewWebServerModulePlaceholder(cfg) // Placeholder - call local func
	}


	return &DeployAppPipeline{
		name: "DeployApplicationPipeline",
		modules: []module.Module{
			webServerMod,
		},
	}
}

func (p *DeployAppPipeline) Name() string {
	return p.name
}

func (p *DeployAppPipeline) Modules() []module.Module {
	if p.modules == nil {
		return []module.Module{}
	}
	modsCopy := make([]module.Module, len(p.modules))
	copy(modsCopy, p.modules)
	return modsCopy
}

// Plan generates the execution graph for all relevant modules within this pipeline.
func (p *DeployAppPipeline) Plan(ctx PipelineContext) (*plan.ExecutionGraph, error) { // Changed to local PipelineContext
	logger := ctx.GetLogger().With("pipeline", p.Name())
	finalGraph := plan.NewExecutionGraph(p.Name())

	var previousModuleExitNodes []plan.NodeID
	isFirstEffectiveModule := true

	for _, mod := range p.Modules() {
		if mod == nil {
			logger.Warn("Encountered a nil module during planning, skipping.")
			continue
		}
		logger.Info("Planning module", "module", mod.Name())

		// Assert ctx to module.ModuleContext, as Module.Plan expects it.
		// The underlying concrete type (*runtime.Context) implements both.
		moduleCtx, ok := ctx.(module.ModuleContext)
		if !ok {
			return nil, fmt.Errorf("failed to assert PipelineContext to module.ModuleContext for module %s in pipeline %s", mod.Name(), p.Name())
		}
		moduleFragment, err := mod.Plan(moduleCtx)
		if err != nil {
			return nil, fmt.Errorf("planning failed for module %s in pipeline %s: %w", mod.Name(), p.Name(), err)
		}

		if moduleFragment == nil || len(moduleFragment.Nodes) == 0 {
			logger.Info("Module returned an empty fragment, skipping", "module", mod.Name())
			continue
		}

		for id, node := range moduleFragment.Nodes {
			if _, exists := finalGraph.Nodes[id]; exists {
				return nil, fmt.Errorf("duplicate NodeID '%s' from module '%s' in pipeline '%s'", id, mod.Name(), p.Name())
			}
			finalGraph.Nodes[id] = node
		}

		if len(previousModuleExitNodes) > 0 {
			for _, entryNodeID := range moduleFragment.EntryNodes {
				entryNode, found := finalGraph.Nodes[entryNodeID]
				if !found {
					return nil, fmt.Errorf("entry node '%s' from module '%s' not found after merge in pipeline '%s'", entryNodeID, mod.Name(), p.Name())
				}
				existingDeps := make(map[plan.NodeID]bool)
				for _, depID := range entryNode.Dependencies { existingDeps[depID] = true }
				for _, prevExitNodeID := range previousModuleExitNodes {
					if !existingDeps[prevExitNodeID] {
						entryNode.Dependencies = append(entryNode.Dependencies, prevExitNodeID)
					}
				}
			}
		} else if isFirstEffectiveModule {
			finalGraph.EntryNodes = append(finalGraph.EntryNodes, moduleFragment.EntryNodes...)
		}

		if len(moduleFragment.ExitNodes) > 0 {
			previousModuleExitNodes = moduleFragment.ExitNodes
			isFirstEffectiveModule = false
		}
	}

	finalGraph.ExitNodes = append(finalGraph.ExitNodes, previousModuleExitNodes...)
	finalGraph.EntryNodes = plan.UniqueNodeIDs(finalGraph.EntryNodes) // Changed to plan.UniqueNodeIDs
	finalGraph.ExitNodes = plan.UniqueNodeIDs(finalGraph.ExitNodes)   // Changed to plan.UniqueNodeIDs

	logger.Info("Pipeline planning complete.", "totalNodes", len(finalGraph.Nodes))
	return finalGraph, nil
}

// Run executes the pipeline.
// It now accepts an ExecutionGraph as input, aligning with the updated Pipeline interface.
func (p *DeployAppPipeline) Run(ctx PipelineContext, graph *plan.ExecutionGraph, dryRun bool) (*plan.GraphExecutionResult, error) {
	logger := ctx.GetLogger().With("pipeline", p.Name())
	logger.Info("Starting pipeline run...", "dryRun", dryRun, "graphName", graph.Name)

	if graph == nil {
		err := fmt.Errorf("execution graph cannot be nil for pipeline %s", p.Name())
		logger.Error(err, "Cannot execute pipeline")
		res := plan.NewGraphExecutionResult(p.Name())
		res.Finalize(plan.StatusFailed, err.Error())
		return res, err
	}

	eng := ctx.GetEngine() // GetEngine is part of PipelineContext
	if eng == nil {
		err := fmt.Errorf("engine not found in pipeline context for pipeline %s", p.Name())
		logger.Error(err, "Cannot execute pipeline")
		res := plan.NewGraphExecutionResult(p.Name())
		res.Finalize(plan.StatusFailed, err.Error())
		return res, err
	}

	logger.Info("Submitting execution graph to engine.", "nodeCount", len(graph.Nodes))
	// eng.Execute expects engine.EngineExecuteContext.
	// The ctx (PipelineContext) is implemented by *runtime.Context, which also implements EngineExecuteContext.
	engineExecuteCtx, ok := ctx.(engine.EngineExecuteContext)
	if !ok {
		err := fmt.Errorf("pipelineContext cannot be asserted to engine.EngineExecuteContext for pipeline %s", p.Name())
		logger.Error(err, "Cannot execute pipeline")
		res := plan.NewGraphExecutionResult(graph.Name) // Use graph name for result
		res.Finalize(plan.StatusFailed, err.Error())
		return res, err
	}
	graphResult, execErr := eng.Execute(engineExecuteCtx, graph, dryRun) // Use the passed-in graph

	if execErr != nil {
		logger.Error(execErr, "Engine execution encountered an error for pipeline "+p.Name())
		if graphResult == nil { // Should not happen if engine.Execute guarantees a result object
			graphResult = plan.NewGraphExecutionResult(graph.Name)
		}
		// Ensure status and end time are set if engine didn't do it on error
		if graphResult.Status != plan.StatusFailed && graphResult.Status != plan.StatusSkipped { // Assuming engine might set Skipped if all nodes skipped
			graphResult.Status = plan.StatusFailed
		}
		if graphResult.EndTime.IsZero() {
			graphResult.EndTime = time.Now()
		}
		return graphResult, fmt.Errorf("engine execution failed for pipeline %s: %w", p.Name(), execErr)
	}

	logger.Info("Pipeline run finished.", "status", graphResult.Status)
	return graphResult, nil
}

// Ensure DeployAppPipeline implements the pipeline.Pipeline interface.
var _ Pipeline = (*DeployAppPipeline)(nil)

// Placeholder for NewWebServerModule to make the example somewhat runnable if webserver.go isn't fully refactored.
// This should be removed once the actual WebServerModule is correctly defined and imported.
func (m *moduleExamplePlaceholder) Plan(ctx module.ModuleContext) (*task.ExecutionFragment, error) { return &task.ExecutionFragment{}, nil } // Changed to module.ModuleContext
func (m *moduleExamplePlaceholder) Name() string { return "WebServerModulePlaceholder" }
func (m *moduleExamplePlaceholder) Tasks() []task.Task { return nil }
type moduleExamplePlaceholder struct { module.BaseModule }
func NewWebServerModulePlaceholder(cfg *v1alpha1.Cluster) module.Module {
	return &moduleExamplePlaceholder{BaseModule: module.NewBaseModule("WebServerModulePlaceholder", nil)}
}
