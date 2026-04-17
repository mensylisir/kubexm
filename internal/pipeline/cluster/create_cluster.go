package cluster

import (
	"fmt"
	"github.com/mensylisir/kubexm/internal/runtime"

	"github.com/mensylisir/kubexm/internal/engine"
	"github.com/mensylisir/kubexm/internal/module"
	"github.com/mensylisir/kubexm/internal/module/addon"
	"github.com/mensylisir/kubexm/internal/module/infrastructure"
	"github.com/mensylisir/kubexm/internal/module/kubernetes"
	"github.com/mensylisir/kubexm/internal/module/loadbalancer"
	"github.com/mensylisir/kubexm/internal/module/network"
	"github.com/mensylisir/kubexm/internal/module/preflight"
	"github.com/mensylisir/kubexm/internal/pipeline"
	"github.com/mensylisir/kubexm/internal/plan"
)

// CreateClusterPipeline defines the pipeline for creating a new Kubernetes cluster.
type CreateClusterPipeline struct {
	*pipeline.Base
	modules   []module.Module
	assumeYes bool
}

// NewCreateClusterPipeline creates a new CreateClusterPipeline.
// It initializes the modules that this pipeline will orchestrate.
func NewCreateClusterPipeline(assumeYes bool) *CreateClusterPipeline {
	// Create modules in logical execution order
	modules := []module.Module{
		preflight.NewPreflightConnectivityModule(), // SSH connectivity check (gate for all operations)
		preflight.NewPreflightModule(assumeYes),    // System checks, initial OS setup, kernel setup
		infrastructure.NewInfrastructureModule(),   // ETCD (PKI + install), Container Runtime
		loadbalancer.NewLoadBalancerModule(),       // Load balancer setup (external/internal/kube-vip)
		kubernetes.NewControlPlaneModule(),         // Kube binaries, image pulls, kubeadm init
		network.NewNetworkModule(),                 // CNI plugin
		kubernetes.NewWorkerModule(),               // Join worker nodes
		addon.NewAddonsModule(),                   // Cluster addons
	}

	return &CreateClusterPipeline{
		Base:      pipeline.NewBase("CreateNewCluster", "Creates a new Kubernetes cluster with all necessary components"),
		modules:   modules,
		assumeYes: assumeYes,
	}
}

// Name returns the designated name of the pipeline.
func (p *CreateClusterPipeline) Name() string {
	return p.Base.Meta.Name
}

// Description returns a brief description of the pipeline.
func (p *CreateClusterPipeline) Description() string {
	return p.Base.Meta.Description
}

// Modules returns a list of modules that belong to this pipeline.
func (p *CreateClusterPipeline) Modules() []module.Module {
	return p.modules
}

// Plan generates the final ExecutionGraph for the entire pipeline.
// It orchestrates module planning and links their ExecutionFragments.
func (p *CreateClusterPipeline) Plan(ctx runtime.PipelineContext) (*plan.ExecutionGraph, error) {
	return pipeline.SafePlan(ctx, p.Name(), func() (*plan.ExecutionGraph, error) {
		logger := ctx.GetLogger().With("pipeline", p.Name())
		logger.Info("Planning cluster creation pipeline...")

		finalGraph := plan.NewExecutionGraph(p.Name())
		var previousModuleExitNodes []plan.NodeID

		// Assert that the pipeline context can be used as a module context
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
						node.Dependencies = plan.UniqueNodeIDs(append(node.Dependencies, previousModuleExitNodes...))
						logger.Debug("Linked module entry node to previous module exits", "entry_node", entryNodeID, "dependencies", node.Dependencies)
					} else {
						logger.Warn("EntryNodeID from module fragment not found in merged graph nodes map", "node_id", entryNodeID, "module", mod.Name())
					}
				}
			}
			previousModuleExitNodes = moduleFragment.ExitNodes
		}

		// Calculate final graph entry and exit nodes
		finalGraph.CalculateEntryAndExitNodes()

		logger.Info("Pipeline planning complete.", "total_nodes", len(finalGraph.Nodes))
		if err := finalGraph.Validate(); err != nil {
			logger.Error(err, "Final execution graph validation failed.")
			return nil, fmt.Errorf("final execution graph for pipeline %s is invalid: %w", p.Name(), err)
		}
		return finalGraph, nil
	})
}

// Run executes the pipeline.
// It takes runtime.PipelineContext, but the underlying concrete type is expected
// to be *runtime.Context which implements all necessary context interfaces.
func (p *CreateClusterPipeline) Run(ctx runtime.PipelineContext, graph *plan.ExecutionGraph, dryRun bool) (*plan.GraphExecutionResult, error) {
	logger := ctx.GetLogger().With("pipeline", p.Name())
	logger.Info("Running cluster creation pipeline...", "dryRun", dryRun)

	engineCtx, ok := ctx.(*runtime.Context)
	if !ok {
		err := fmt.Errorf("pipeline context cannot be asserted to *runtime.Context for pipeline %s", p.Name())
		logger.Error(err, "Context type assertion failed")
		return nil, err
	}

	// Detect offline mode and ensure assets are available
	logger.Info("Detecting offline mode and ensuring assets availability...")
	if err := EnsureAssetsAvailable(engineCtx, p.assumeYes); err != nil {
		logger.Error(err, "Failed to ensure assets are available")
		return nil, fmt.Errorf("asset preparation failed: %w", err)
	}

	// Use the provided graph or plan if none was provided
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

	logger.Info("Executing cluster creation plan...", "num_nodes", len(currentGraph.Nodes))
	execEngine := engine.NewCheckpointExecutorForPipeline(engineCtx, p.Name())
	result, execErr := execEngine.Execute(engineCtx, currentGraph, dryRun)
	if execErr != nil {
		logger.Error(execErr, "Pipeline execution failed.")
		if result == nil {
			// Create a minimal result if engine returned nil result on error
			result = &plan.GraphExecutionResult{GraphName: p.Name(), Status: plan.StatusFailed}
		}
		return result, fmt.Errorf("execution phase for pipeline %s failed: %w", p.Name(), execErr)
	}

	logger.Info("Cluster creation pipeline completed.", "status", result.Status)
	return result, nil
}

// Ensure CreateClusterPipeline implements Pipeline interface
var _ pipeline.Pipeline = (*CreateClusterPipeline)(nil)
