package pipeline

import (
	"github.com/mensylisir/kubexm/pkg/config" // For config.Cluster
	"github.com/mensylisir/kubexm/pkg/spec"

	// Import module factories (assuming they are now ...ModuleSpec)
	modulePreflight "github.com/mensylisir/kubexm/pkg/module/preflight"
	moduleContainerd "github.com/mensylisir/kubexm/pkg/module/containerd"
	moduleEtcd "github.com/mensylisir/kubexm/pkg/module/etcd"
	// moduleKubernetes "github.com/kubexms/kubexms/pkg/module/kubernetes" // Placeholder
	// moduleNetwork "github.com/kubexms/kubexms/pkg/module/network"    // Placeholder
	// moduleAddons "github.com/kubexms/kubexms/pkg/module/addons"      // Placeholder

	// Import step specs for potential hooks, if using simple commands for hooks
	// stepCommandSpec "github.com/kubexms/kubexms/pkg/step/command"
)

// NewCreateClusterPipelineSpec defines the pipeline specification for creating a new Kubernetes cluster.
// It assembles all necessary modules in the correct order.
// The cfg *config.Cluster parameter provides access to the cluster configuration.
func NewCreateClusterPipelineSpec(cfg *config.Cluster) *spec.PipelineSpec {
	if cfg == nil {
		// Return an empty/error pipeline spec if config is missing
		return &spec.PipelineSpec{
			Name:        "Create New Kubernetes Cluster",
			Description: "Error: Missing cluster configuration.",
			Modules:     []*spec.ModuleSpec{},
		}
	}

	modules := []*spec.ModuleSpec{
		// 1. Preflight checks and base system setup
		modulePreflight.NewPreflightModuleSpec(cfg),

		// 2. Install and configure container runtime
		moduleContainerd.NewContainerdModuleSpec(cfg),
		// TODO: Add logic or separate module factories for other runtimes like Docker,
		// and select based on cfg.Spec.ContainerRuntime.Type.

		// 3. (Optional) Setup HA components
		// Example: if cfg.Spec.HighAvailability != nil && cfg.Spec.HighAvailability.Type == "keepalived" {
		//     modules = append(modules, moduleHA.NewKeepalivedModuleSpec(cfg))
		// }

		// 4. Deploy Etcd cluster.
		moduleEtcd.NewEtcdModuleSpec(cfg),

		// 5. Deploy Kubernetes control plane components
		// Example: modules = append(modules, moduleKubernetes.NewControlPlaneModuleSpec(cfg))

		// 6. Join worker nodes to the cluster
		// Example: modules = append(modules, moduleKubernetes.NewWorkerNodeModule(cfg))

		// 7. Deploy network plugin (CNI)
		// Example: modules = append(modules, moduleNetwork.NewCNIModule(cfg)) // CNI choice from cfg

		// 8. (Optional) Deploy cluster addons (e.g., CoreDNS, Metrics Server, Ingress Controller)
		// Example: modules = append(modules, moduleAddons.NewCoreDNSModule(cfg))
		// Example: modules = append(modules, moduleAddons.NewMetricsServerModule(cfg))
	}

	// Filter out nil modules. This is defensive; factories should ideally not return nil.
	// However, if a factory could return nil (e.g., if a component is entirely optional based on config
	// and the factory decides not to add a module at all), this handles it.
	finalModules := make([]*spec.ModuleSpec, 0, len(modules))
	for _, m := range modules {
		if m != nil { // Ensure module spec itself isn't nil
			finalModules = append(finalModules, m)
		}
	}

	// Define Pipeline PreRun/PostRun Steps if needed
	// These are spec.StepSpec instances.
	// Example using a (hypothetical) CommandStepSpec from a shared step definitions package:
	// var preRunStep spec.StepSpec = &stepCommandSpec.CommandStepSpec{
	// 	 SpecName: "Pipeline PreRun: Start Create Cluster",
	// 	 Cmd:      "echo 'Starting Kubernetes cluster creation pipeline at $(date)'",
	// 	 Sudo:     false,
	// }
	// var postRunStep spec.StepSpec = &stepCommandSpec.CommandStepSpec{
	// 	 SpecName: "Pipeline PostRun: Create Cluster Finished",
	// 	 Cmd:      "echo 'Kubernetes cluster creation pipeline finished at $(date). Check logs for details.'",
	// 	 Sudo:     false,
	// }

	return &spec.PipelineSpec{
		Name:        "Create New Kubernetes Cluster",
		Description: "Pipeline to create a new Kubernetes cluster from scratch, including preflight, runtime, etcd, and control plane setup.",
		Modules:     finalModules,
		PreRunHook:  "", // Example: "create_cluster_pre_hook_generic_setup"
		PostRunHook: "", // Example: "create_cluster_post_hook_final_validation"
	}
}

// TODO: Implement other pipeline factories as needed:
// - NewScaleUpWorkerPipelineSpec(cfg *config.Cluster, newWorkerConfigs []config.HostSpec) *spec.PipelineSpec
// - NewScaleUpControlPlanePipelineSpec(cfg *config.Cluster, newCPConfigs []config.HostSpec) *spec.PipelineSpec
// - NewDeleteNodePipelineSpec(cfg *config.Cluster, nodeNameToDelete string) *spec.PipelineSpec
// - NewDeleteClusterPipelineSpec(cfg *config.Cluster) *spec.PipelineSpec
// - NewUpgradeClusterPipelineSpec(cfg *config.Cluster, targetVersion string) *spec.PipelineSpec
