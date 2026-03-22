package assets

import (
	"fmt"

	"github.com/mensylisir/kubexm/internal/engine"
	"github.com/mensylisir/kubexm/internal/module"
	assetsmodule "github.com/mensylisir/kubexm/internal/module/assets"
	"github.com/mensylisir/kubexm/internal/pipeline"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
)

// DownloadAssetsPipeline downloads and packages artifacts for offline use.
type DownloadAssetsPipeline struct {
	name       string
	desc       string
	modules    []module.Module
	outputPath string
}

func NewDownloadAssetsPipeline(outputPath string) *DownloadAssetsPipeline {
	modules := []module.Module{
		assetsmodule.NewAssetsDownloadModule(outputPath),
	}
	return &DownloadAssetsPipeline{
		name:       "DownloadAssets",
		desc:       "Download and package all assets for offline installation",
		modules:    modules,
		outputPath: outputPath,
	}
}

func (p *DownloadAssetsPipeline) Name() string {
	return p.name
}

func (p *DownloadAssetsPipeline) Description() string {
	return p.desc
}

func (p *DownloadAssetsPipeline) Modules() []module.Module {
	return p.modules
}

func (p *DownloadAssetsPipeline) Plan(ctx runtime.PipelineContext) (*plan.ExecutionGraph, error) {
	logger := ctx.GetLogger().With("pipeline", p.Name())
	logger.Info("Planning asset download pipeline...")

	finalGraph := plan.NewExecutionGraph(p.Name())
	var previousModuleExitNodes []plan.NodeID

	moduleCtx, ok := ctx.(runtime.ModuleContext)
	if !ok {
		return nil, fmt.Errorf("pipeline context cannot be asserted to module context for pipeline %s", p.Name())
	}

	for i, mod := range p.Modules() {
		logger.Info("Planning module", "module_name", mod.Name(), "module_index", i)

		moduleFragment, err := mod.Plan(moduleCtx)
		if err != nil {
			logger.Error(err, "Failed to plan module", "module", mod.Name())
			return nil, fmt.Errorf("failed to plan module %s in pipeline %s: %w", mod.Name(), p.Name(), err)
		}

		if moduleFragment == nil || len(moduleFragment.Nodes) == 0 {
			logger.Info("Module returned an empty fragment, skipping merge and link.", "module", mod.Name())
			continue
		}

		for nodeID, node := range moduleFragment.Nodes {
			if _, exists := finalGraph.Nodes[nodeID]; exists {
				err := fmt.Errorf("duplicate NodeID '%s' detected when merging fragment from module '%s'", nodeID, mod.Name())
				logger.Error(err, "NodeID collision")
				return nil, err
			}
			finalGraph.Nodes[nodeID] = node
		}

		if len(previousModuleExitNodes) > 0 {
			for _, entryNodeID := range moduleFragment.EntryNodes {
				if node, ok := finalGraph.Nodes[entryNodeID]; ok {
					node.Dependencies = plan.UniqueNodeIDs(append(node.Dependencies, previousModuleExitNodes...))
				}
			}
		}
		previousModuleExitNodes = moduleFragment.ExitNodes
	}

	finalGraph.CalculateEntryAndExitNodes()
	if err := finalGraph.Validate(); err != nil {
		return nil, fmt.Errorf("final execution graph for pipeline %s is invalid: %w", p.Name(), err)
	}
	return finalGraph, nil
}

func (p *DownloadAssetsPipeline) Run(ctx runtime.PipelineContext, graph *plan.ExecutionGraph, dryRun bool) (*plan.GraphExecutionResult, error) {
	logger := ctx.GetLogger().With("pipeline", p.Name())
	engineCtx, ok := ctx.(*runtime.Context)
	if !ok {
		return nil, fmt.Errorf("pipeline context cannot be asserted to *runtime.Context for pipeline %s", p.Name())
	}

	var currentGraph *plan.ExecutionGraph
	var err error
	if graph == nil {
		currentGraph, err = p.Plan(ctx)
		if err != nil {
			return nil, fmt.Errorf("planning phase for pipeline %s failed: %w", p.Name(), err)
		}
	} else {
		currentGraph = graph
	}

	if currentGraph == nil || len(currentGraph.Nodes) == 0 {
		logger.Info("Pipeline planned no executable nodes. Nothing to run.")
		return &plan.GraphExecutionResult{GraphName: p.Name(), Status: plan.StatusSuccess}, nil
	}

	execEngine := engine.NewExecutor()
	result, execErr := execEngine.Execute(engineCtx, currentGraph, dryRun)
	if execErr != nil {
		if result == nil {
			result = &plan.GraphExecutionResult{GraphName: p.Name(), Status: plan.StatusFailed}
		}
		return result, fmt.Errorf("execution phase for pipeline %s failed: %w", p.Name(), execErr)
	}
	return result, nil
}

// GetBase returns the base pipeline context.
func (p *DownloadAssetsPipeline) GetBase() *pipeline.Base {
	return nil
}

var _ pipeline.Pipeline = (*DownloadAssetsPipeline)(nil)
