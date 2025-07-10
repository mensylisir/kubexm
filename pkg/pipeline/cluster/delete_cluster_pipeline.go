package cluster

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/module"
	// TODO: Define and import actual Teardown/Cleanup modules
	// "github.com/mensylisir/kubexm/pkg/module/cleanup"
	// "github.com/mensylisir/kubexm/pkg/module/kubernetes" // For NodeResetModule potentially
	"github.com/mensylisir/kubexm/pkg/pipeline"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
)

// DeleteClusterPipeline defines the pipeline for deleting an existing Kubernetes cluster.
type DeleteClusterPipeline struct {
	PipelineName    string
	PipelineModules []module.Module
	AssumeYes       bool // Added to potentially pass to confirmation tasks within modules
}

// NewDeleteClusterPipeline creates a new DeleteClusterPipeline.
func NewDeleteClusterPipeline(assumeYes bool) pipeline.Pipeline {
	// Define modules for deletion in reverse order of creation or specific teardown logic
	// These are placeholders and need to be implemented.
	// Example:
	// teardownAddonsModule := addon.NewTeardownAddonsModule(assumeYes)
	// drainNodesModule := kubernetes.NewDrainNodesModule(assumeYes)
	// deleteNodesModule := kubernetes.NewDeleteKubeNodesModule(assumeYes) // kubeadm reset on nodes
	// teardownControlPlaneModule := kubernetes.NewTeardownControlPlaneModule(assumeYes)
	// teardownNetworkModule := network.NewTeardownNetworkModule(assumeYes)
	// cleanupInfrastructureModule := infrastructure.NewCleanupInfrastructureModule(assumeYes) // Uninstall etcd, runtime
	// finalCleanupModule := cleanup.NewFinalCleanupModule(assumeYes) // Remove work dirs, etc.

	// Placeholder modules for now
	placeholderTeardownModule := module.NewBaseModule("PlaceholderTeardown", nil)

	return &DeleteClusterPipeline{
		PipelineName: "DeleteCluster",
		PipelineModules: []module.Module{
			// Define actual teardown modules here in correct order
			placeholderTeardownModule,
		},
		AssumeYes: assumeYes,
	}
}

func (p *DeleteClusterPipeline) Name() string {
	return p.PipelineName
}

func (p *DeleteClusterPipeline) Modules() []module.Module {
	if p.PipelineModules == nil {
		return []module.Module{}
	}
	modulesCopy := make([]module.Module, len(p.PipelineModules))
	copy(modulesCopy, p.PipelineModules)
	return modulesCopy
}

func (p *DeleteClusterPipeline) Plan(ctx pipeline.PipelineContext) (*plan.ExecutionGraph, error) {
	logger := ctx.GetLogger().With("pipeline", p.Name())
	logger.Info("Planning pipeline for cluster deletion...")

	finalGraph := plan.NewExecutionGraph(p.Name())
	var previousModuleExitNodes []plan.NodeID

	moduleCtx, ok := ctx.(module.ModuleContext)
	if !ok {
		return nil, fmt.Errorf("pipeline context cannot be asserted to module.ModuleContext for pipeline %s", p.Name())
	}

	if len(p.Modules()) == 0 || (len(p.Modules()) == 1 && p.Modules()[0].Name() == "PlaceholderTeardown") {
	    logger.Warn("DeleteClusterPipeline has no effective modules defined. Returning empty graph.")
	    // Return an empty plan.ExecutionGraph
		emptyGraph := plan.NewExecutionGraph(p.Name())
		emptyGraph.CalculateEntryAndExitNodes() // Should result in empty entry/exit
	    return emptyGraph, nil
	}


	for i, mod := range p.Modules() {
		logger.Info("Planning module for deletion", "module_name", mod.Name(), "module_index", i)

		// Modules for deletion should have their own Plan/PlanDelete methods or be specific teardown modules.
		// For now, assuming they have a standard Plan method.
		moduleFragment, err := mod.Plan(moduleCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to plan module %s in pipeline %s: %w", mod.Name(), p.Name(), err)
		}
		if moduleFragment.IsEmpty() {
			logger.Info("Module returned an empty fragment, skipping.", "module_name", mod.Name())
			continue
		}
		if err := finalGraph.MergeFragment(moduleFragment); err != nil { // Assuming ExecutionGraph has MergeFragment
		    return nil, fmt.Errorf("failed to merge fragment from module %s: %w", mod.Name(), err)
		}

		if len(previousModuleExitNodes) > 0 {
			plan.LinkFragments(finalGraph, previousModuleExitNodes, moduleFragment.EntryNodes) // Assuming LinkFragments works on ExecutionGraph
		}
		previousModuleExitNodes = moduleFragment.ExitNodes
	}

	finalGraph.CalculateEntryAndExitNodes()
	if err := finalGraph.Validate(); err != nil {
		return nil, fmt.Errorf("final execution graph for pipeline %s is invalid: %w", p.Name(), err)
	}
	logger.Info("Cluster deletion pipeline planning complete.", "total_nodes", len(finalGraph.Nodes))
	return finalGraph, nil
}

func (p *DeleteClusterPipeline) Run(ctx pipeline.PipelineContext, graph *plan.ExecutionGraph, dryRun bool) (*plan.GraphExecutionResult, error) {
	logger := ctx.GetLogger().With("pipeline", p.Name())
	logger.Info("Running cluster deletion pipeline...", "dryRun", dryRun)

	engineCtx, ok := ctx.(engine.EngineExecuteContext)
	if !ok {
		err := fmt.Errorf("pipeline context cannot be asserted to engine.EngineExecuteContext for pipeline %s", p.Name())
		logger.Error(err, "Context type assertion failed")
		return nil, err
	}

	var currentGraph *plan.ExecutionGraph
	var err error
	if graph == nil {
		logger.Info("No pre-computed graph provided to DeleteClusterPipeline.Run, planning now...")
		currentGraph, err = p.Plan(ctx)
		if err != nil {
			logger.Error(err, "Pipeline planning phase failed within Run method.")
			return nil, fmt.Errorf("planning phase for pipeline %s failed: %w", p.Name(), err)
		}
	} else {
		currentGraph = graph
	}

	if currentGraph == nil || currentGraph.IsEmpty() { // Assuming IsEmpty for ExecutionGraph
		logger.Info("Pipeline planned no executable nodes for deletion or was given an empty graph. Nothing to run.")
		return &plan.GraphExecutionResult{GraphName: p.Name(), Status: plan.StatusSuccess}, nil
	}

	result, execErr := ctx.GetEngine().Execute(engineCtx, currentGraph, dryRun)
	if execErr != nil {
		if result == nil { result = &plan.GraphExecutionResult{GraphName: p.Name(), Status: plan.StatusFailed} }
		return result, fmt.Errorf("execution phase for pipeline %s failed: %w", p.Name(), execErr)
	}
	logger.Info("Cluster deletion pipeline run completed.", "status", result.Status)
	return result, nil
}

var _ pipeline.Pipeline = (*DeleteClusterPipeline)(nil)
