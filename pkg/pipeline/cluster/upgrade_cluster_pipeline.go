package cluster

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/module"
	// TODO: Define and import actual Upgrade modules
	// "github.com/mensylisir/kubexm/pkg/module/preflight"
	// "github.com/mensylisir/kubexm/pkg/module/kubernetes"
	"github.com/mensylisir/kubexm/pkg/pipeline"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
)

// UpgradeClusterPipeline defines the pipeline for upgrading an existing Kubernetes cluster.
type UpgradeClusterPipeline struct {
	PipelineName    string
	PipelineModules []module.Module
	TargetVersion   string // Target Kubernetes version for the upgrade
	AssumeYes       bool
}

// NewUpgradeClusterPipeline creates a new UpgradeClusterPipeline.
func NewUpgradeClusterPipeline(targetVersion string, assumeYes bool) pipeline.Pipeline {
	// Define modules for upgrade in specific order
	// These are placeholders and need to be implemented.
	// Example:
	// upgradePreflightModule := preflight.NewUpgradePreflightModule(assumeYes, targetVersion)
	// upgradeControlPlaneModule := kubernetes.NewUpgradeControlPlaneModule(targetVersion)
	// upgradeWorkerModule := kubernetes.NewUpgradeWorkerModule(targetVersion)
	// upgradePostflightModule := preflight.NewUpgradePostflightModule(targetVersion)

	placeholderUpgradeModule := module.NewBaseModule("PlaceholderUpgrade", nil)

	return &UpgradeClusterPipeline{
		PipelineName:  "UpgradeCluster",
		TargetVersion: targetVersion, // Store target version for modules to use
		PipelineModules: []module.Module{
			// Define actual upgrade modules here
			placeholderUpgradeModule,
		},
		AssumeYes: assumeYes,
	}
}

func (p *UpgradeClusterPipeline) Name() string {
	return p.PipelineName
}

func (p *UpgradeClusterPipeline) Modules() []module.Module {
	if p.PipelineModules == nil {
		return []module.Module{}
	}
	modulesCopy := make([]module.Module, len(p.PipelineModules))
	copy(modulesCopy, p.PipelineModules)
	return modulesCopy
}

func (p *UpgradeClusterPipeline) Plan(ctx pipeline.PipelineContext) (*plan.ExecutionGraph, error) {
	logger := ctx.GetLogger().With("pipeline", p.Name(), "target_version", p.TargetVersion)
	logger.Info("Planning pipeline for cluster upgrade...")

	finalGraph := plan.NewExecutionGraph(p.Name())
	var previousModuleExitNodes []plan.NodeID

	moduleCtx, ok := ctx.(module.ModuleContext)
	if !ok {
		return nil, fmt.Errorf("pipeline context cannot be asserted to module.ModuleContext for pipeline %s", p.Name())
	}

	if len(p.Modules()) == 0 || (len(p.Modules()) == 1 && p.Modules()[0].Name() == "PlaceholderUpgrade") {
	    logger.Warn("UpgradeClusterPipeline has no effective modules defined. Returning empty graph.")
		emptyGraph := plan.NewExecutionGraph(p.Name())
		emptyGraph.CalculateEntryAndExitNodes()
	    return emptyGraph, nil
	}

	for i, mod := range p.Modules() {
		logger.Info("Planning module for upgrade", "module_name", mod.Name(), "module_index", i)
		// Upgrade modules might need the TargetVersion, passed via context or specific module constructor.
		// For now, assuming Plan method handles it or module was initialized with it.
		moduleFragment, err := mod.Plan(moduleCtx) // Module's Plan needs access to targetVersion
		if err != nil {
			return nil, fmt.Errorf("failed to plan module %s in pipeline %s: %w", mod.Name(), p.Name(), err)
		}
		if moduleFragment.IsEmpty() {
			logger.Info("Module returned an empty fragment, skipping.", "module_name", mod.Name())
			continue
		}
		if err := finalGraph.MergeFragment(moduleFragment); err != nil {
		    return nil, fmt.Errorf("failed to merge fragment from module %s: %w", mod.Name(), err)
		}

		if len(previousModuleExitNodes) > 0 {
			plan.LinkFragments(finalGraph, previousModuleExitNodes, moduleFragment.EntryNodes)
		}
		previousModuleExitNodes = moduleFragment.ExitNodes
	}

	finalGraph.CalculateEntryAndExitNodes()
	if err := finalGraph.Validate(); err != nil {
		return nil, fmt.Errorf("final execution graph for pipeline %s is invalid: %w", p.Name(), err)
	}
	logger.Info("Cluster upgrade pipeline planning complete.", "total_nodes", len(finalGraph.Nodes))
	return finalGraph, nil
}

func (p *UpgradeClusterPipeline) Run(ctx pipeline.PipelineContext, graph *plan.ExecutionGraph, dryRun bool) (*plan.GraphExecutionResult, error) {
	logger := ctx.GetLogger().With("pipeline", p.Name())
	logger.Info("Running cluster upgrade pipeline...", "dryRun", dryRun, "target_version", p.TargetVersion)

	engineCtx, ok := ctx.(engine.EngineExecuteContext)
	if !ok {
		err := fmt.Errorf("pipeline context cannot be asserted to engine.EngineExecuteContext for pipeline %s", p.Name())
		logger.Error(err, "Context type assertion failed")
		return nil, err
	}

	var currentGraph *plan.ExecutionGraph
	var err error
	if graph == nil {
		logger.Info("No pre-computed graph provided to UpgradeClusterPipeline.Run, planning now...")
		currentGraph, err = p.Plan(ctx)
		if err != nil {
			logger.Error(err, "Pipeline planning phase failed within Run method.")
			return nil, fmt.Errorf("planning phase for pipeline %s failed: %w", p.Name(), err)
		}
	} else {
		currentGraph = graph
	}

	if currentGraph == nil || currentGraph.IsEmpty() {
		logger.Info("Pipeline planned no executable nodes for upgrade or was given an empty graph. Nothing to run.")
		return &plan.GraphExecutionResult{GraphName: p.Name(), Status: plan.StatusSuccess}, nil
	}

	result, execErr := ctx.GetEngine().Execute(engineCtx, currentGraph, dryRun)
	if execErr != nil {
		if result == nil { result = &plan.GraphExecutionResult{GraphName: p.Name(), Status: plan.StatusFailed} }
		return result, fmt.Errorf("execution phase for pipeline %s failed: %w", p.Name(), execErr)
	}
	logger.Info("Cluster upgrade pipeline run completed.", "status", result.Status)
	return result, nil
}

var _ pipeline.Pipeline = (*UpgradeClusterPipeline)(nil)
