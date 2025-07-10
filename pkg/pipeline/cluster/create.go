package cluster

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/module"
	// "github.com/mensylisir/kubexm/pkg/module/etcd" // Not directly used by name, but by specific module constructors
	"github.com/mensylisir/kubexm/pkg/module/addon"
	"github.com/mensylisir/kubexm/pkg/module/infrastructure"
	"github.com/mensylisir/kubexm/pkg/module/kubernetes"
	"github.com/mensylisir/kubexm/pkg/module/network"
	"github.com/mensylisir/kubexm/pkg/module/preflight"
	"github.com/mensylisir/kubexm/pkg/pipeline" // For pipeline.Pipeline and pipeline.PipelineContext
	"github.com/mensylisir/kubexm/pkg/plan"
	// "github.com/mensylisir/kubexm/pkg/runtime" // For *runtime.Context in Run method - no, use interface
)

// CreateClusterPipeline defines the pipeline for creating a new Kubernetes cluster.
type CreateClusterPipeline struct {
	PipelineName    string
	PipelineModules []module.Module
}

// NewCreateClusterPipeline creates a new CreateClusterPipeline.
// It initializes the modules that this pipeline will orchestrate.
func NewCreateClusterPipeline(assumeYes bool) pipeline.Pipeline {
	// Instantiate modules in their logical execution order.
	preflightModule := preflight.NewPreflightModule(assumeYes)
	infrastructureModule := infrastructure.NewInfrastructureModule()
	controlPlaneModule := kubernetes.NewControlPlaneModule()
	networkModule := network.NewNetworkModule()
	workerModule := kubernetes.NewWorkerModule()
	addonsModule := addon.NewAddonsModule()
	// TODO: Add HighAvailabilityModule if separate and needed early.
	// For now, HA setup (like VIP for kubeadm init) might be part of Preflight or Infrastructure.

	return &CreateClusterPipeline{
		PipelineName: "CreateNewCluster",
		PipelineModules: []module.Module{
			preflightModule,      // System checks, initial OS setup, kernel setup, offline repo/artifacts
			infrastructureModule, // ETCD (PKI + install), Container Runtime (Docker or Containerd)
			controlPlaneModule,   // Kube binaries, image pulls, kubeadm init, join masters
			networkModule,        // CNI plugin
			workerModule,         // Join worker nodes
			addonsModule,         // Cluster addons
		},
	}
}

// Name returns the name of the pipeline.
func (p *CreateClusterPipeline) Name() string {
	return p.PipelineName
}

// Modules returns the list of modules in this pipeline.
func (p *CreateClusterPipeline) Modules() []module.Module {
	// Return a copy to prevent external modification
	if p.PipelineModules == nil {
		return []module.Module{}
	}
	modulesCopy := make([]module.Module, len(p.PipelineModules))
	copy(modulesCopy, p.PipelineModules)
	return modulesCopy
}

// Plan generates the final ExecutionGraph for the entire pipeline.
// It orchestrates module planning and links their ExecutionFragments.
func (p *CreateClusterPipeline) Plan(ctx pipeline.PipelineContext) (*plan.ExecutionGraph, error) { // Changed to pipeline.PipelineContext
	logger := ctx.GetLogger().With("pipeline", p.Name())
	logger.Info("Planning pipeline...")

	finalGraph := plan.NewExecutionGraph(p.Name()) // Initialize an empty graph
	var previousModuleExitNodes []plan.NodeID

	// TODO: This assertion will need to change when ModuleContext is refactored.
	// For now, we assume the full runtime.Context (which implements pipeline.PipelineContext)
	// also implements module.ModuleContext (the new one).
	moduleCtx, ok := ctx.(module.ModuleContext) // Changed to module.ModuleContext
	if !ok {
		// This is a critical setup issue. The context provided to Pipeline.Plan
		// must also be usable as a ModuleContext for its modules.
		// This implies the concrete runtime.Context implements all facade interfaces.
		return nil, fmt.Errorf("pipeline context cannot be asserted to module.ModuleContext for pipeline %s", p.Name())
	}

	for i, mod := range p.Modules() {
		logger.Info("Planning module", "module_name", mod.Name(), "module_index", i)
		// Modules expect ModuleContext.
		moduleFragment, err := mod.Plan(moduleCtx) // mod.Plan now expects module.ModuleContext
		if err != nil {
			logger.Error(err, "Failed to plan module", "module", mod.Name())
			return nil, fmt.Errorf("failed to plan module %s in pipeline %s: %w", mod.Name(), p.Name(), err)
		}

		if moduleFragment == nil || len(moduleFragment.Nodes) == 0 {
			logger.Info("Module returned an empty fragment, skipping merge and link.", "module", mod.Name())
			continue
		}

		// Merge nodes from moduleFragment into finalGraph.Nodes
		for nodeID, node := range moduleFragment.Nodes {
			if _, exists := finalGraph.Nodes[nodeID]; exists {
				err := fmt.Errorf("duplicate NodeID '%s' detected when merging fragment from module '%s'", nodeID, mod.Name())
				logger.Error(err, "NodeID collision")
				return nil, err
			}
			finalGraph.Nodes[nodeID] = node
		}

		// Link current module's entry nodes to previous module's exit nodes
		if len(previousModuleExitNodes) > 0 {
			for _, entryNodeID := range moduleFragment.EntryNodes {
				if node, ok := finalGraph.Nodes[entryNodeID]; ok {
					node.Dependencies = plan.UniqueNodeIDs(append(node.Dependencies, previousModuleExitNodes...)) // Use plan.UniqueNodeIDs
					logger.Debug("Linked module entry node to previous module exits", "entry_node", entryNodeID, "dependencies", node.Dependencies)
				} else {
					logger.Warn("EntryNodeID from module fragment not found in merged graph nodes map", "node_id", entryNodeID, "module", mod.Name())
				}
			}
		}
		previousModuleExitNodes = moduleFragment.ExitNodes
	}

	// Note: The final graph's overall entry/exit points are implicitly defined by the first module's entries
	// and the last module's exits that are not internal to the graph. The Engine will determine this via nodes with no incoming/outgoing dependencies.
	// Explicitly setting EntryNodes/ExitNodes on the ExecutionGraph itself is not part of plan.ExecutionGraph struct.
	// This is incorrect, ExecutionGraph *does* have EntryNodes and ExitNodes.
	// The pipeline should calculate these for the final graph.
	finalGraph.CalculateEntryAndExitNodes()


	logger.Info("Pipeline planning complete.", "total_nodes", len(finalGraph.Nodes))
	if err := finalGraph.Validate(); err != nil {
		logger.Error(err, "Final execution graph validation failed.")
		return nil, fmt.Errorf("final execution graph for pipeline %s is invalid: %w", p.Name(), err)
	}
	return finalGraph, nil
}

// Run executes the pipeline.
// It takes pipeline.PipelineContext, but the underlying concrete type is expected
// to be *runtime.Context which implements all necessary context interfaces.
func (p *CreateClusterPipeline) Run(ctx pipeline.PipelineContext, graph *plan.ExecutionGraph, dryRun bool) (*plan.GraphExecutionResult, error) {
	logger := ctx.GetLogger().With("pipeline", p.Name())
	logger.Info("Running pipeline...", "dryRun", dryRun)

	// The Engine's Execute method expects an engine.EngineExecuteContext.
	// The runtime.Context (which is the concrete type for ctx) implements this.
	engineCtx, ok := ctx.(engine.EngineExecuteContext)
	if !ok {
		// This would be a programming error if the context passed to Run isn't the full runtime.Context.
		err := fmt.Errorf("pipeline context cannot be asserted to engine.EngineExecuteContext for pipeline %s", p.Name())
		logger.Error(err, "Context type assertion failed")
		return nil, err
	}

	// Note: The Plan() method should ideally be called by the CLI/caller of Run,
	// and the resulting graph passed into this Run method, as per the Pipeline interface.
	// For this implementation, if graph is nil, we can plan it here.
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

	if currentGraph == nil || len(currentGraph.Nodes) == 0 {
		logger.Info("Pipeline planned no executable nodes or was given an empty graph. Nothing to run.")
		return &plan.GraphExecutionResult{
			GraphName:   p.Name(),
			Status:      plan.StatusSuccess,
			NodeResults: make(map[plan.NodeID]*plan.NodeResult),
		}, nil
	}

	logger.Info("Executing pipeline plan...", "num_nodes", len(currentGraph.Nodes))
	result, execErr := ctx.GetEngine().Execute(engineCtx, currentGraph, dryRun) // Pass asserted engineCtx
	if execErr != nil {
		logger.Error(execErr, "Pipeline execution failed.")
		if result == nil {
			// Create a minimal result if engine returned nil result on error
			result = &plan.GraphExecutionResult{GraphName: p.Name(), Status: plan.StatusFailed}
		}
		return result, fmt.Errorf("execution phase for pipeline %s failed: %w", p.Name(), execErr)
	}

	logger.Info("Pipeline run completed.", "status", result.Status)
	return result, nil
}

var _ pipeline.Pipeline = (*CreateClusterPipeline)(nil)
