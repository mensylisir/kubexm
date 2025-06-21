package cluster // Changed package name

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/module"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/pipeline" // Import for pipeline.Pipeline interface
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"


	// Import actual module constructors
	modulePreflight "github.com/mensylisir/kubexm/pkg/module/preflight"
	moduleContainerd "github.com/mensylisir/kubexm/pkg/module/containerd"
	moduleEtcd "github.com/mensylisir/kubexm/pkg/module/etcd"
	// TODO: Add other necessary module imports:
	// moduleKubeComponents "github.com/mensylisir/kubexm/pkg/module/kube_components"
	// moduleKubernetes "github.com/mensylisir/kubexm/pkg/module/kubernetes"
	// moduleNetwork "github.com/mensylisir/kubexm/pkg/module/network"
	// moduleAddons "github.com/mensylisir/kubexm/pkg/module/addons"
)

// CreateClusterPipeline defines the pipeline for creating a new Kubernetes cluster.
type CreateClusterPipeline struct {
	// Consider embedding a BasePipeline if common methods like Name() and Modules() are extracted.
	// For now, direct implementation.
	name    string
	modules []module.Module
}

// NewCreateClusterPipeline creates a new pipeline instance for cluster creation.
// It initializes the sequence of modules to be executed.
// rtCtx is the full runtime context, from which cluster configuration can be obtained.
func NewCreateClusterPipeline(rtCtx runtime.Context) pipeline.Pipeline { // Returns pipeline.Pipeline
	clusterCfg := rtCtx.GetClusterConfig()
	if clusterCfg == nil {
		// This should ideally not happen if runtime context is built correctly.
		// Handle error: log and return a nil pipeline or a pipeline that will error out.
		// For now, let's assume GetClusterConfig always returns a valid config or panics if not set.
		// A robust app might:
		// rtCtx.GetLogger().Error("ClusterConfig not found in runtime context during pipeline creation")
		// return &CreateClusterPipeline{name: "ErrorPipeline", modules: []module.Module{}}
	}

	// Instantiate modules. Module constructors are now simplified and do not take clusterCfg.
	// Modules will fetch config from the context passed to their Plan method.
	mods := []module.Module{
		modulePreflight.NewPreflightModule(),    // cfg removed
		moduleContainerd.NewContainerdModule(), // Assuming NewContainerdModule() exists and is simplified
		moduleEtcd.NewEtcdModule(),             // Assuming NewEtcdModule() exists and is simplified
		// TODO: Instantiate and add other simplified module constructors:
		// moduleKubeComponents.NewKubeComponentsModule(),
		// moduleKubernetes.NewControlPlaneModule(),
		// moduleKubernetes.NewWorkerNodeModule(),
		// moduleNetwork.NewNetworkModule(),
		// moduleAddons.NewAddonsModule(),
	}

	pipelineName := "CreateNewKubernetesCluster"
	if clusterCfg != nil && clusterCfg.Name != "" {
		pipelineName = fmt.Sprintf("CreateCluster-%s", clusterCfg.Name)
	}


	return &CreateClusterPipeline{
		name:    pipelineName,
		modules: mods,
	}
}

// Name returns the name of the pipeline.
func (p *CreateClusterPipeline) Name() string {
	return p.name
}

// Modules returns the list of modules in this pipeline.
func (p *CreateClusterPipeline) Modules() []module.Module {
	if p.modules == nil {
		return []module.Module{}
	}
	// Return a copy to prevent external modification
	modsCopy := make([]module.Module, len(p.modules))
	copy(modsCopy, p.modules)
	return modsCopy
}

// Plan generates the final, complete ExecutionGraph for the entire pipeline.
func (p *CreateClusterPipeline) Plan(ctx runtime.PipelineContext) (*plan.ExecutionGraph, error) {
	logger := ctx.GetLogger().With("pipeline", p.Name())
	finalGraph := plan.NewExecutionGraph(p.Name()) // plan.NewExecutionGraph expects a name

	var previousModuleExitNodes []plan.NodeID
	isFirstEffectiveModule := true

	// Use p.modules instead of p.pipelineModules
	for _, mod := range p.modules {
		// Assuming ctx (PipelineContext) is satisfied by *runtime.Context which also satisfies ModuleContext.
		// ModuleContext embeds PipelineContext, so this direct pass is fine if mod.Plan expects ModuleContext.
		logger.Info("Planning module", "module", mod.Name())

		// Modules could have an IsRequired method, though not in the current module.Module interface.
		// If they did, it would be checked here:
		// if required, modIsRequiredErr := mod.IsRequired(ctx); modIsRequiredErr != nil || !required { ... skip ... }

		moduleFragment, err := mod.Plan(ctx) // Pass ctx directly
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
