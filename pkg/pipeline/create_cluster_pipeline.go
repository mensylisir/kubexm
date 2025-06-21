package pipeline

import (
	"fmt" // For errors
	"time" // For GraphExecutionResult EndTime

	// "github.com/mensylisir/kubexm/pkg/engine"  // Engine is obtained from runtime.Context
	"github.com/mensylisir/kubexm/pkg/module"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	// "github.com/mensylisir/kubexm/pkg/task" // For task.ExecutionFragment, used by module.Plan

	// Import actual module constructors (assuming they are updated)
	modulePreflight "github.com/mensylisir/kubexm/pkg/module/preflight"
	moduleContainerd "github.com/mensylisir/kubexm/pkg/module/containerd"
	moduleEtcd "github.com/mensylisir/kubexm/pkg/module/etcd"
	// ... other module imports like Kubernetes, Network, Addons would go here
)

// CreateClusterPipeline defines the pipeline for creating a new Kubernetes cluster.
type CreateClusterPipeline struct {
	pipelineName    string
	pipelineModules []module.Module
}

// NewCreateClusterPipeline creates a new pipeline instance for cluster creation.
// It initializes the sequence of modules to be executed.
// The runtime.Context (rtCtx) is passed to allow modules to be conditionally included
// or configured based on the overall cluster configuration, if necessary at construction time.
func NewCreateClusterPipeline(rtCtx runtime.Context) Pipeline { // Returns pipeline.Pipeline
	// Example: cfg := rtCtx.GetClusterConfig() // If modules depend on config at construction.
	// For this example, module list is static. In a real scenario, you might use rtCtx
	// to decide which modules to instantiate or how to configure them.

	mods := []module.Module{
		modulePreflight.NewPreflightModule(),     // Assuming constructor is parameterless or takes rtCtx if needed
		moduleContainerd.NewContainerdModule(), // Assuming constructor is parameterless or takes rtCtx
		moduleEtcd.NewEtcdModule(),             // Assuming constructor is parameterless or takes rtCtx
		// TODO: Instantiate and add other modules:
		// moduleKubernetes.NewControlPlaneModule(rtCtx),
		// moduleKubernetes.NewWorkerNodeModule(rtCtx),
		// moduleNetwork.NewCNIModule(rtCtx),
		// moduleAddons.NewCoreDNSModule(rtCtx),
	}

	return &CreateClusterPipeline{
		pipelineName:    "CreateNewKubernetesCluster",
		pipelineModules: mods,
	}
}

// Name returns the name of the pipeline.
func (p *CreateClusterPipeline) Name() string {
	return p.pipelineName
}

// Modules returns the list of modules in this pipeline.
func (p *CreateClusterPipeline) Modules() []module.Module {
	if p.pipelineModules == nil {
		return []module.Module{}
	}
	modsCopy := make([]module.Module, len(p.pipelineModules))
	copy(modsCopy, p.pipelineModules)
	return modsCopy
}

// Plan generates the final, complete ExecutionGraph for the entire pipeline.
func (p *CreateClusterPipeline) Plan(ctx runtime.PipelineContext) (*plan.ExecutionGraph, error) {
	logger := ctx.GetLogger().With("pipeline", p.Name())
	finalGraph := plan.NewExecutionGraph(p.Name())

	var previousModuleExitNodes []plan.NodeID
	isFirstEffectiveModule := true

	for _, mod := range p.pipelineModules {
		// The PipelineContext (ctx) should be usable as a ModuleContext if the concrete
		// runtime context object implements ModuleContext (likely via embedding PipelineContext).
		moduleCtx, ok := ctx.(runtime.ModuleContext)
		if !ok {
			return nil, fmt.Errorf("pipeline context could not be asserted to module context for module %s. Ensure main runtime context implements all facades.", mod.Name())
		}

		logger.Info("Planning module", "module", mod.Name())

		// Modules could have an IsRequired method, though not in the current module.Module interface.
		// If they did, it would be checked here:
		// if required, modIsRequiredErr := mod.IsRequired(moduleCtx); modIsRequiredErr != nil || !required { ... skip ... }

		moduleFragment, err := mod.Plan(moduleCtx)
		if err != nil {
			logger.Error(err, "Failed to plan module", "module", mod.Name())
			return nil, fmt.Errorf("failed to plan module %s: %w", mod.Name(), err)
		}

		if moduleFragment == nil || len(moduleFragment.Nodes) == 0 {
			logger.Info("Module returned an empty fragment, skipping", "module", mod.Name())
			continue
		}

		for id, node := range moduleFragment.Nodes {
			if _, exists := finalGraph.Nodes[id]; exists {
				err := fmt.Errorf("duplicate NodeID '%s' detected when merging fragments from module '%s'", id, mod.Name())
				logger.Error(err, "NodeID collision")
				return nil, err
			}
			finalGraph.Nodes[id] = node
		}

		if len(previousModuleExitNodes) > 0 {
			for _, entryNodeID := range moduleFragment.EntryNodes {
				entryNode, found := finalGraph.Nodes[entryNodeID]
				if !found {
					return nil, fmt.Errorf("internal error: entry node '%s' from module '%s' not found in final graph after merge", entryNodeID, mod.Name())
				}
				existingDeps := make(map[plan.NodeID]bool)
				for _, depID := range entryNode.Dependencies {
					existingDeps[depID] = true
				}
				for _, prevExitNodeID := range previousModuleExitNodes {
					if !existingDeps[prevExitNodeID] {
						entryNode.Dependencies = append(entryNode.Dependencies, prevExitNodeID)
						existingDeps[prevExitNodeID] = true
					}
				}
			}
		} else if isFirstEffectiveModule { // Only if no preceding module contributed exit nodes
			finalGraph.EntryNodes = append(finalGraph.EntryNodes, moduleFragment.EntryNodes...)
		}

		if len(moduleFragment.ExitNodes) > 0 {
		    previousModuleExitNodes = moduleFragment.ExitNodes
			isFirstEffectiveModule = false
		}
	}

	finalGraph.EntryNodes = uniqueNodeIDs(finalGraph.EntryNodes)
	// The exit nodes of the graph are the exit nodes of the last module that contributed nodes.
	finalGraph.ExitNodes = uniqueNodeIDs(previousModuleExitNodes)

	if len(finalGraph.Nodes) == 0 {
		logger.Info("Pipeline planned no executable nodes.")
	} else {
		logger.Info("Pipeline planning complete.", "totalNodes", len(finalGraph.Nodes), "entryNodesCount", len(finalGraph.EntryNodes), "exitNodesCount", len(finalGraph.ExitNodes))
	}

	return finalGraph, nil
}

// Run executes the pipeline.
func (p *CreateClusterPipeline) Run(rtCtx *runtime.Context, dryRun bool) (*plan.GraphExecutionResult, error) {
	logger := rtCtx.GetLogger().With("pipeline", p.Name())
	logger.Info("Starting pipeline run...", "dryRun", dryRun)

	// The full runtime.Context (rtCtx) should implement PipelineContext.
	pipelinePlanCtx, ok := rtCtx.(runtime.PipelineContext)
	if !ok {
		err := fmt.Errorf("full runtime context does not satisfy PipelineContext for planning")
		logger.Error(err, "Context type assertion failed")
		result := plan.NewGraphExecutionResult(p.Name())
		result.Status = plan.StatusFailed
		result.EndTime = time.Now()
		return result, err
	}

	executionGraph, err := p.Plan(pipelinePlanCtx)
	if err != nil {
		logger.Error(err, "Failed to generate execution plan for pipeline")
		result := plan.NewGraphExecutionResult(p.Name())
		result.Status = plan.StatusFailed
		result.EndTime = time.Now()
		return result, fmt.Errorf("pipeline plan generation failed: %w", err)
	}

	// If dryRun is true, the engine's Execute method will handle the dry run logic.
	// The plan (graph) is still generated.

	eng := rtCtx.GetEngine()
	if eng == nil {
		err := fmt.Errorf("engine not found in runtime context")
		logger.Error(err, "Cannot execute pipeline")
		result := plan.NewGraphExecutionResult(p.Name())
		result.Status = plan.StatusFailed
		result.EndTime = time.Now()
		return result, err
	}

	logger.Info("Submitting execution graph to engine.", "nodeCount", len(executionGraph.Nodes))
	graphResult, execErr := eng.Execute(rtCtx, executionGraph, dryRun)

	if execErr != nil {
		// This error is from the engine's execution logic itself (e.g., setup error),
		// not necessarily a failure of a node within the graph (which would be in graphResult.Status).
		logger.Error(execErr, "Engine execution encountered an error")
		if graphResult == nil { // Should not happen if engine.Execute respects its contract
			graphResult = plan.NewGraphExecutionResult(p.Name())
			graphResult.Status = plan.StatusFailed
		}
		// Ensure EndTime is set, even if engine failed to set it.
		if graphResult.EndTime.IsZero() {
			graphResult.EndTime = time.Now()
		}
		return graphResult, fmt.Errorf("engine execution failed: %w", execErr)
	}

	logger.Info("Pipeline run finished.", "status", graphResult.Status)
	return graphResult, nil
}

// uniqueNodeIDs helper (can be moved to a common utility package if used elsewhere)
func uniqueNodeIDs(ids []plan.NodeID) []plan.NodeID {
    if len(ids) == 0 {
        return []plan.NodeID{}
    }
    seen := make(map[plan.NodeID]bool)
    result := []plan.NodeID{}
    for _, id := range ids {
        if !seen[id] {
            seen[id] = true
            result = append(result, id)
        }
    }
    return result
}

// Ensure CreateClusterPipeline implements the pipeline.Pipeline interface.
var _ Pipeline = (*CreateClusterPipeline)(nil)
