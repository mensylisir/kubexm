package preflight

import (
	"github.com/kubexms/kubexms/pkg/config"
	"github.com/kubexms/kubexms/pkg/module"
	"github.com/kubexms/kubexms/pkg/runtime" // For PreRun/PostRun signature
	"github.com/kubexms/kubexms/pkg/task"
	taskPreflight "github.com/kubexms/kubexms/pkg/task/preflight" // Import the actual preflight tasks
)

// NewPreflightModule creates a new module that groups preflight check and setup tasks.
func NewPreflightModule(cfg *config.Cluster) *module.Module {
	return &module.Module{
		Name: "Preflight Checks and Setup",
		// IsEnabled is typically always true for preflight, unless explicitly configured otherwise.
		IsEnabled: func(clusterCfg *config.Cluster) bool {
			// Example: Could be disabled via a global config flag if needed
			// if cfg != nil && cfg.Spec.Global.SkipPreflight { return false }
			return true
		},
		Tasks: []*task.Task{
			taskPreflight.NewSystemChecksTask(cfg),
			taskPreflight.NewSetupKernelTask(cfg),
			// Add other preflight tasks here as they are defined
			// e.g., taskPreflight.NewSetupEtcHostsTask(cfg),
		},
		PreRun: func(cluster *runtime.ClusterRuntime) error {
			if cluster != nil && cluster.Logger != nil {
				cluster.Logger.Infof("Starting preflight checks and setup across applicable hosts...")
			}
			return nil
		},
		PostRun: func(cluster *runtime.ClusterRuntime, moduleErr error) error {
			if cluster != nil && cluster.Logger != nil {
				if moduleErr != nil {
					cluster.Logger.Errorf("Preflight module finished with error: %v", moduleErr)
				} else {
					cluster.Logger.Successf("Preflight module completed successfully.")
				}
			}
			return nil // PostRun errors usually don't override moduleErr
		},
	}
}
