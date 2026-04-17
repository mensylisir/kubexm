package cluster

import (
	"fmt"

	"github.com/mensylisir/kubexm/internal/engine"
	"github.com/mensylisir/kubexm/internal/module"
	"github.com/mensylisir/kubexm/internal/module/addon"
	"github.com/mensylisir/kubexm/internal/module/etcd"
	"github.com/mensylisir/kubexm/internal/module/kubernetes"
	"github.com/mensylisir/kubexm/internal/module/cni"
	"github.com/mensylisir/kubexm/internal/module/loadbalancer"
	"github.com/mensylisir/kubexm/internal/module/os"
	"github.com/mensylisir/kubexm/internal/module/preflight"
	modruntime "github.com/mensylisir/kubexm/internal/module/runtime"
	"github.com/mensylisir/kubexm/internal/module/storage"
	"github.com/mensylisir/kubexm/internal/pipeline"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
)

// DeleteClusterPipeline defines the pipeline for deleting an existing Kubernetes cluster.
type DeleteClusterPipeline struct {
	*pipeline.Base
	PipelineModules []module.Module
	AssumeYes       bool
}

// NewDeleteClusterPipeline creates a new DeleteClusterPipeline.
// Modules are ordered in reverse of cluster creation to ensure proper teardown:
// 1. Preflight (connectivity + user confirmation) - CRITICAL: requires user confirmation unless --yes is set
// 2. AddonCleanup - uninstall cluster addons first
// 3. WorkerCleanup - drain and reset worker nodes
// 4. ControlPlaneCleanup - remove control plane components
// 5. NetworkCleanup - remove CNI plugin
// 6. LoadBalancerCleanup - remove load balancer
// 7. EtcdCleanup - remove etcd (if managed)
// 8. RuntimeCleanup - remove container runtime
// 9. StorageCleanup - remove storage classes
// 10. OsCleanup - restore OS-level changes
func NewDeleteClusterPipeline(assumeYes bool) pipeline.Pipeline {
	modules := []module.Module{
		preflight.NewPreflightConnectivityModule(), // SSH connectivity check
		preflight.NewPreflightModule(assumeYes),           // Connectivity + user confirmation (CRITICAL)
		addon.NewAddonCleanupModule(),              // Uninstall addons from the cluster
		kubernetes.NewWorkerCleanupModule(),        // Drain and reset worker nodes
		kubernetes.NewControlPlaneCleanupModule(),  // Remove control plane components
		cni.NewNetworkCleanupModule(),              // Remove CNI plugin
		loadbalancer.NewLoadBalancerCleanupModule(), // Remove load balancer
		etcd.NewEtcdCleanupModule(),                // Remove etcd (skip if external)
		modruntime.NewRuntimeCleanupModule(),          // Remove container runtime
		storage.NewStorageCleanupModule(),           // Remove storage classes
		os.NewOsCleanupModule(),                    // Restore OS-level changes
	}

	return &DeleteClusterPipeline{
		Base:            pipeline.NewBase("DeleteCluster", "Deletes an existing Kubernetes cluster and cleans up all resources"),
		PipelineModules: modules,
		AssumeYes:       assumeYes,
	}
}

func (p *DeleteClusterPipeline) Name() string {
	return p.Base.Meta.Name
}

func (p *DeleteClusterPipeline) Description() string {
	return p.Base.Meta.Description
}

func (p *DeleteClusterPipeline) Modules() []module.Module {
	if p.PipelineModules == nil {
		return []module.Module{}
	}
	modulesCopy := make([]module.Module, len(p.PipelineModules))
	copy(modulesCopy, p.PipelineModules)
	return modulesCopy
}

func (p *DeleteClusterPipeline) Plan(ctx runtime.PipelineContext) (*plan.ExecutionGraph, error) {
	return pipeline.SafePlan(ctx, p.Name(), func() (*plan.ExecutionGraph, error) {
		logger := ctx.GetLogger().With("pipeline", p.Name())
		logger.Info("Planning cluster deletion pipeline...")

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

func (p *DeleteClusterPipeline) Run(ctx runtime.PipelineContext, graph *plan.ExecutionGraph, dryRun bool) (*plan.GraphExecutionResult, error) {
	logger := ctx.GetLogger().With("pipeline", p.Name())
	logger.Info("Running cluster deletion pipeline...", "dryRun", dryRun)

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

	logger.Info("Executing cluster deletion plan...", "num_nodes", len(currentGraph.Nodes))
	execEngine := engine.NewCheckpointExecutorForPipeline(engineCtx, p.Name())
	result, execErr := execEngine.Execute(engineCtx, currentGraph, dryRun)
	if execErr != nil {
		logger.Error(execErr, "Pipeline execution failed.")
		if result == nil {
			result = &plan.GraphExecutionResult{GraphName: p.Name(), Status: plan.StatusFailed}
		}
		return result, fmt.Errorf("execution phase for pipeline %s failed: %w", p.Name(), execErr)
	}

	logger.Info("Cluster deletion pipeline completed.", "status", result.Status)
	return result, nil
}

var _ pipeline.Pipeline = (*DeleteClusterPipeline)(nil)
