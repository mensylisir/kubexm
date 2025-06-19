package preflight

import (
	"github.com/kubexms/kubexms/pkg/config"
	"github.com/kubexms/kubexms/pkg/spec"
	// Step specs are now encapsulated within task constructors from taskPreflight package
	// "github.com/kubexms/kubexms/pkg/step/preflight"
	taskPreflight "github.com/kubexms/kubexms/pkg/task/preflight"
)

// NewPreflightModule creates a new module specification for preflight checks and setup.
func NewPreflightModule(cfg *config.Cluster) *spec.ModuleSpec {

	// Start with existing tasks (if any, like system checks from task factories)
	tasks := []*spec.TaskSpec{
		// These factory functions might create tasks with steps that are now covered by
		// the SetupKubernetesPrerequisitesTask. This might lead to redundancy.
		// For this refactoring, we are primarily focused on integrating the new task.
		// A later review could consolidate steps from these factory-generated tasks
		// if they overlap with SetupKubernetesPrerequisitesTask.
		taskPreflight.NewSystemChecksTask(cfg), // Example: checks CPU, memory
		taskPreflight.NewSetupKernelTask(cfg),   // Example: might do some kernel setup, potentially overlapping module loading
	}

	// Conditionally add the new comprehensive prerequisites task.
	// TODO: Implement cfg.Spec.Preflight.EnableKubernetesPrerequisites (or similar) in config.ClusterSpec
	//       and use it here to make this task's inclusion truly conditional.
	//       For example:
	//       enableK8sPrerequisites := false
	//       if cfg.Spec.Preflight != nil && cfg.Spec.Preflight.EnableKubernetesPrerequisites {
	//           enableK8sPrerequisites = true
	//       }
	//       For now, unconditionally adding the task for testing/development.
	enableK8sPrerequisites := true // Placeholder: Default to true for now
	// if cfg.Spec.Preflight != nil && cfg.Spec.Preflight.EnableKubernetesPrerequisites { // Example of actual check
	// 	enableK8sPrerequisites = true
	// } else if cfg.Spec.Preflight == nil { // if Preflight spec part is missing, maybe default to true or false based on desired behavior
	//    enableK8sPrerequisites = true // Defaulting to true if spec section is absent
	// }

	if enableK8sPrerequisites {
		// The NewSetupKubernetesPrerequisitesTask constructor now encapsulates the logic
		// for defining kernel module lists and all prerequisite steps.
		kubePrereqTask := taskPreflight.NewSetupKubernetesPrerequisitesTask(cfg)
		if kubePrereqTask != nil { // Constructor might return nil if task is not applicable based on cfg
			tasks = append(tasks, kubePrereqTask)
		}
	}

	return &spec.ModuleSpec{
		Name: "Preflight Checks and Setup",
		IsEnabled: func(clusterCfg *config.Cluster) bool {
			// Module is enabled by default.
			// It's disabled if explicitly told to skip preflight checks in global config.
			if clusterCfg != nil && clusterCfg.Spec.Global != nil && clusterCfg.Spec.Global.SkipPreflight {
				return false // SkipPreflight is true, so module is disabled.
			}
			return true // Enabled by default or if SkipPreflight is false.
		},
		Tasks:   tasks,
		PreRun:  nil,
		PostRun: nil,
	}
}
