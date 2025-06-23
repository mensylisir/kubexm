package cluster

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/module"
	"github.com/mensylisir/kubexm/pkg/module/cni" // Placeholder for CNI cleanup module
	"github.com/mensylisir/kubexm/pkg/module/containerd" // Placeholder for Containerd cleanup
	"github.com/mensylisir/kubexm/pkg/module/etcd" // Placeholder for Etcd cleanup
	k8sModule "github.com/mensylisir/kubexm/pkg/module/kubernetes" // Placeholder for K8s components cleanup
	// "github.com/mensylisir/kubexm/pkg/module/preflight" // May not need a dedicated preflight cleanup module
	"github.com/mensylisir/kubexm/pkg/pipeline"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
)

// DeleteClusterPipeline defines the pipeline for deleting a Kubernetes cluster.
type DeleteClusterPipeline struct {
	PipelineName    string
	PipelineModules []module.Module
	AssumeYes       bool // Store assumeYes for potential use in module/task planning
}

// NewDeleteClusterPipeline creates a new DeleteClusterPipeline.
func NewDeleteClusterPipeline(assumeYes bool) pipeline.Pipeline {
	// Define modules for the deletion process.
	// These would be specialized cleanup/uninstall modules.
	// Order is generally reverse of creation, but with dependencies in mind.
	// For example, CNI and Kubelets should be cleaned from worker nodes before control plane.
	// Nodes should be reset before etcd data is wiped from former etcd nodes.

	// TODO: Create these actual modules with cleanup logic. Using existing module packages
	// as placeholders for where such cleanup modules might reside or be named.
	// For example, cni.NewCalicoCleanupModule(), k8sModule.NewNodeResetModule(), etc.

	// Example stubs (assuming constructors exist or will be created):
	// 1. Drain nodes (part of NodeReset or specific K8s module)
	// 2. Remove CNI (e.g., cni.NewCalicoCleanupModule())
	// 3. Reset nodes / Uninstall Kubelet (e.g., k8sModule.NewNodeResetModule())
	// 4. Uninstall Control Plane components (e.g., k8sModule.NewControlPlaneCleanupModule())
	// 5. Uninstall Container Runtime (e.g., containerd.NewContainerdCleanupModule()) - Optional, or part of NodeReset
	// 6. Cleanup Etcd (e.g., etcd.NewEtcdCleanupModule())

	// For P0, let's assume some placeholder modules. Real ones need to be created.
	// These names are illustrative.
	nodeResetModule := k8sModule.NewNodeResetModule()                 // Placeholder
	cniCleanupModule := cni.NewCNICleanupModule()                     // Placeholder
	controlPlaneCleanupModule := k8sModule.NewControlPlaneCleanupModule() // Placeholder
	etcdCleanupModule := etcd.NewEtcdCleanupModule()                  // Placeholder
	containerdCleanupModule := containerd.NewContainerdCleanupModule() // Placeholder

	return &DeleteClusterPipeline{
		PipelineName: "DeleteCluster",
		PipelineModules: []module.Module{
			// Order:
			// 1. Gracefully remove workloads & CNI from nodes that will be reset.
			//    NodeResetModule might handle draining. CNICleanup might run on nodes or from control-plane.
			cniCleanupModule, // Remove CNI manifests, cleanup CNI IPAM, etc.
			nodeResetModule,  // Stop kubelet, remove binaries, reset kubeadm, cleanup CRI data.
			// 2. Teardown control plane components
			controlPlaneCleanupModule, // Stop/remove apiserver, scheduler, controller-manager.
			// 3. Teardown etcd
			etcdCleanupModule, // Stop etcd, remove data.
			// 4. Containerd cleanup (if not part of node reset)
			containerdCleanupModule, // Optional: uninstall containerd if desired.
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

	for i, mod := range p.Modules() {
		logger.Info("Planning module for deletion", "module_name", mod.Name(), "module_index", i)

		// Modules expect ModuleContext.
		// TODO: Pass p.AssumeYes to modules if they need it for their own planning or task generation.
		// This might involve adding it to the ModuleContext or passing it to mod.Plan if the interface allows.
		moduleFragment, err := mod.Plan(moduleCtx) // Module's Plan might use AssumeYes from its own config or context
		if err != nil {
			logger.Error(err, "Failed to plan module for deletion", "module", mod.Name())
			return nil, fmt.Errorf("failed to plan module %s in delete pipeline %s: %w", mod.Name(), p.Name(), err)
		}

		if moduleFragment == nil || len(moduleFragment.Nodes) == 0 {
			logger.Info("Module returned an empty fragment during deletion planning, skipping.", "module", mod.Name())
			continue
		}

		for nodeID, node := range moduleFragment.Nodes {
			if _, exists := finalGraph.Nodes[nodeID]; exists {
				return nil, fmt.Errorf("duplicate NodeID '%s' detected from module '%s' during deletion planning", nodeID, mod.Name())
			}
			finalGraph.Nodes[nodeID] = node
		}

		if len(previousModuleExitNodes) > 0 {
			for _, entryNodeID := range moduleFragment.EntryNodes {
				if node, ok := finalGraph.Nodes[entryNodeID]; ok {
					node.Dependencies = plan.UniqueNodeIDs(append(node.Dependencies, previousModuleExitNodes...))
				} else {
					logger.Warn("EntryNodeID from module fragment not found in merged graph (delete pipeline)", "node_id", entryNodeID, "module", mod.Name())
				}
			}
		}
		previousModuleExitNodes = moduleFragment.ExitNodes
	}

	logger.Info("Cluster deletion pipeline planning complete.", "total_nodes", len(finalGraph.Nodes))
	if err := finalGraph.Validate(); err != nil {
		logger.Error(err, "Final execution graph validation failed for deletion pipeline.")
		return nil, fmt.Errorf("final execution graph for delete pipeline %s is invalid: %w", p.Name(), err)
	}
	return finalGraph, nil
}

func (p *DeleteClusterPipeline) Run(ctx *runtime.Context, dryRun bool) (*plan.GraphExecutionResult, error) {
	logger := ctx.GetLogger().With("pipeline", p.Name())
	logger.Info("Running cluster deletion pipeline...", "dryRun", dryRun)

	pipelineCtx, ok := ctx.AsPipelineContext()
	if !ok {
		return nil, fmt.Errorf("full runtime context cannot be asserted to PipelineContext for delete pipeline %s run", p.Name())
	}

	executionGraph, err := p.Plan(pipelineCtx)
	if err != nil {
		logger.Error(err, "Cluster deletion pipeline planning phase failed.")
		return nil, fmt.Errorf("planning phase for delete pipeline %s failed: %w", p.Name(), err)
	}

	if executionGraph == nil || len(executionGraph.Nodes) == 0 {
		logger.Info("Delete pipeline planned no executable nodes. Nothing to run.")
		return &plan.GraphExecutionResult{
			GraphName: p.Name(),
			Status:    plan.StatusSuccess, // Or a specific "NoOp" status
			NodeResults: make(map[plan.NodeID]*plan.NodeResult),
		}, nil
	}

	logger.Info("Executing delete pipeline plan...", "num_nodes", len(executionGraph.Nodes))
	result, execErr := ctx.GetEngine().Execute(ctx, executionGraph, dryRun)
	if execErr != nil {
		logger.Error(execErr, "Cluster deletion pipeline execution failed.")
		if result == nil {
			result = &plan.GraphExecutionResult{GraphName: p.Name(), Status: plan.StatusFailed}
		}
		return result, fmt.Errorf("execution phase for delete pipeline %s failed: %w", p.Name(), execErr)
	}

	logger.Info("Cluster deletion pipeline run completed.", "status", result.Status)
	return result, nil
}

var _ pipeline.Pipeline = (*DeleteClusterPipeline)(nil)
