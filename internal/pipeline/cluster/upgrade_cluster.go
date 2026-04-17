package cluster

import (
	"fmt"

	"github.com/mensylisir/kubexm/internal/engine"
	"github.com/mensylisir/kubexm/internal/module"
	"github.com/mensylisir/kubexm/internal/module/kubernetes"
	"github.com/mensylisir/kubexm/internal/module/preflight"
	"github.com/mensylisir/kubexm/internal/pipeline"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
)

// UpgradeClusterPipeline defines the pipeline for upgrading an existing Kubernetes cluster.
type UpgradeClusterPipeline struct {
	*pipeline.Base
	PipelineModules []module.Module
	TargetVersion   string
	AssumeYes       bool
}

// NewUpgradeClusterPipeline creates a new UpgradeClusterPipeline.
// Modules are ordered to follow safe upgrade procedures:
// 1. Preflight (connectivity + user confirmation) - CRITICAL: requires user confirmation unless --yes is set
// 2. ControlPlaneUpgrade - upgrade control plane nodes first (one by one, maxUnavailable=1)
// 3. WorkerUpgrade - upgrade worker nodes (cordon, drain, upgrade, uncordon)
// 4. NetworkUpgrade - upgrade CNI plugin (helm upgrade --install)
func NewUpgradeClusterPipeline(targetVersion string, assumeYes bool) pipeline.Pipeline {
	// CRITICAL: targetVersion must be explicitly provided - no silent defaults for upgrades
	if targetVersion == "" {
		targetVersion = "unknown" // Will fail validation later, never silently default to "latest"
	}

	modules := []module.Module{
		preflight.NewPreflightConnectivityModule(), // SSH connectivity check
		preflight.NewPreflightModule(assumeYes),                 // Connectivity + user confirmation (CRITICAL)
		kubernetes.NewControlPlaneUpgradeModule(targetVersion),  // Upgrade control plane first
		kubernetes.NewWorkerUpgradeModule(targetVersion),        // Upgrade worker nodes
		kubernetes.NewNetworkUpgradeModule(),                    // Upgrade CNI plugin
	}

	return &UpgradeClusterPipeline{
		Base:            pipeline.NewBase("UpgradeCluster", "Upgrades an existing Kubernetes cluster to a target version"),
		PipelineModules: modules,
		TargetVersion:   targetVersion,
		AssumeYes:       assumeYes,
	}
}

func (p *UpgradeClusterPipeline) Name() string {
	return p.Base.Meta.Name
}

func (p *UpgradeClusterPipeline) Description() string {
	return p.Base.Meta.Description
}

func (p *UpgradeClusterPipeline) Modules() []module.Module {
	if p.PipelineModules == nil {
		return []module.Module{}
	}
	modulesCopy := make([]module.Module, len(p.PipelineModules))
	copy(modulesCopy, p.PipelineModules)
	return modulesCopy
}

func (p *UpgradeClusterPipeline) Plan(ctx runtime.PipelineContext) (*plan.ExecutionGraph, error) {
	return pipeline.SafePlan(ctx, p.Name(), func() (*plan.ExecutionGraph, error) {
		logger := ctx.GetLogger().With("pipeline", p.Name(), "target_version", p.TargetVersion)

		// Validation: targetVersion must not be empty or "unknown" (which indicates it was missing)
		if p.TargetVersion == "" || p.TargetVersion == "unknown" {
			return nil, fmt.Errorf("target version is required for cluster upgrade (use --to-version flag)")
		}

		logger.Info("Planning cluster upgrade pipeline...")

		finalGraph := plan.NewExecutionGraph(p.Name())
		var previousModuleExitNodes []plan.NodeID

		moduleCtx, ok := ctx.(runtime.ModuleContext)
		if !ok {
			return nil, fmt.Errorf("pipeline context cannot be asserted to module.ModuleContext for pipeline %s", p.Name())
		}

		for i, mod := range p.Modules() {
			logger.Info("Planning module", "module_name", mod.Name(), "module_index", i)

			moduleFragment, err := pipeline.SafeModulePlan(moduleCtx, p.Name(), mod)
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
						logger.Debug("Linked module entry node to previous module exits", "entry_node", entryNodeID, "dependencies", node.Dependencies)
					} else {
						logger.Warn("EntryNodeID from module fragment not found in merged graph nodes map", "node_id", entryNodeID, "module", mod.Name())
					}
				}
			}
			previousModuleExitNodes = moduleFragment.ExitNodes
		}

		finalGraph.CalculateEntryAndExitNodes()

		logger.Info("Pipeline planning complete.", "total_nodes", len(finalGraph.Nodes))
		if err := finalGraph.Validate(); err != nil {
			logger.Error(err, "Final execution graph validation failed.")
			return nil, fmt.Errorf("final execution graph for pipeline %s is invalid: %w", p.Name(), err)
		}
		return finalGraph, nil
	})
}

func (p *UpgradeClusterPipeline) Run(ctx runtime.PipelineContext, graph *plan.ExecutionGraph, dryRun bool) (*plan.GraphExecutionResult, error) {
	logger := ctx.GetLogger().With("pipeline", p.Name())
	logger.Info("Running cluster upgrade pipeline...", "dryRun", dryRun, "target_version", p.TargetVersion)

	engineCtx, ok := ctx.(*runtime.Context)
	if !ok {
		err := fmt.Errorf("pipeline context cannot be asserted to *runtime.Context for pipeline %s", p.Name())
		logger.Error(err, "Context type assertion failed")
		return nil, err
	}

	var currentGraph *plan.ExecutionGraph
	var err error
	if graph == nil {
		logger.Info("No pre-computed graph provided to Run, planning now...")
		currentGraph, err = p.Plan(ctx)
		if err != nil {
			logger.Error(err, "Pipeline planning phase failed within Run method.")
			return nil, fmt.Errorf("planning phase for pipeline %s failed: %w", p.Name(), err)
		}
	} else {
		currentGraph = graph
	}

	if currentGraph == nil || currentGraph.IsEmpty() {
		logger.Info("Pipeline planned no executable nodes or was given an empty graph. Nothing to run.")
		return &plan.GraphExecutionResult{
			GraphName:   p.Name(),
			Status:      plan.StatusSuccess,
			NodeResults: make(map[plan.NodeID]*plan.NodeResult),
		}, nil
	}

	logger.Info("Executing cluster upgrade plan...", "num_nodes", len(currentGraph.Nodes))
	execEngine := engine.NewCheckpointExecutorForPipeline(engineCtx, p.Name())
	result, execErr := execEngine.Execute(engineCtx, currentGraph, dryRun)
	if execErr != nil {
		logger.Error(execErr, "Pipeline execution failed.")
		if result == nil {
			result = &plan.GraphExecutionResult{GraphName: p.Name(), Status: plan.StatusFailed}
		}
		return result, fmt.Errorf("execution phase for pipeline %s failed: %w", p.Name(), execErr)
	}

	logger.Info("Cluster upgrade pipeline completed.", "status", result.Status)
	return result, nil
}

var _ pipeline.Pipeline = (*UpgradeClusterPipeline)(nil)
