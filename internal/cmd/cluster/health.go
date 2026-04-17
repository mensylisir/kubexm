package cluster

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/mensylisir/kubexm/internal/config"
	"github.com/mensylisir/kubexm/internal/logger"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/pipeline/cluster"
)

type HealthOptions struct {
	ClusterConfigFile string
	Component         string
	WaitTimeout       time.Duration
}

var healthOptions = &HealthOptions{}

func init() {
	ClusterCmd.AddCommand(healthCmd)
	healthCmd.Flags().StringVarP(&healthOptions.ClusterConfigFile, "config", "f", "", "Path to the cluster configuration YAML file (required)")
	healthCmd.Flags().StringVarP(&healthOptions.Component, "component", "c", "all", "Component to check: all, apiserver, scheduler, controller-manager, kubelet, cluster (default: all)")
	healthCmd.Flags().DurationVar(&healthOptions.WaitTimeout, "wait", 5*time.Minute, "Timeout to wait for health check")

	if err := healthCmd.MarkFlagRequired("config"); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to mark 'config' flag as required for health command: %v\n", err)
	}
}

var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "Check cluster health status",
	Long: `Check the health status of Kubernetes cluster components.

Components:
  all                   - Check all components (default)
  apiserver            - Check kube-apiserver health on control plane nodes
  scheduler            - Check kube-scheduler health on control plane nodes
  controller-manager   - Check kube-controller-manager health on control plane nodes
  kubelet              - Check kubelet health on all nodes
  cluster              - Check overall cluster health (nodes, pods, etcd)

Examples:
  # Check all components
  kubexm cluster health -f config.yaml

  # Check only apiserver
  kubexm cluster health -f config.yaml -c apiserver

  # Check with custom timeout
  kubexm cluster health -f config.yaml --wait 10m`,
	RunE: func(cmd *cobra.Command, args []string) error {
		log := logger.Get()
		defer logger.SyncGlobal()

		if healthOptions.ClusterConfigFile == "" {
			return fmt.Errorf("cluster configuration file must be provided via -f or --config flag")
		}

		absPath, err := filepath.Abs(healthOptions.ClusterConfigFile)
		if err != nil {
			return fmt.Errorf("failed to get absolute path for config file: %w", err)
		}

		clusterConfig, err := config.ParseFromFileWithOptions(absPath, config.ParseOptions{SkipHostValidation: true})
		if err != nil {
			return fmt.Errorf("failed to load cluster configuration: %w", err)
		}

		log.Infof("Starting health check for cluster '%s' (component: %s)", clusterConfig.Name, healthOptions.Component)

		goCtx, cancel := context.WithTimeout(context.Background(), healthOptions.WaitTimeout)
		defer cancel()

		rtBuilder := runtime.NewBuilderFromConfig(clusterConfig).
			WithSkipHostConnect(true).
			WithSkipConfigValidation(true)
		runtimeCtx, cleanupFunc, err := rtBuilder.Build(goCtx)
		if err != nil {
			return fmt.Errorf("failed to build runtime environment: %w", err)
		}
		defer cleanupFunc()

		p := cluster.NewHealthPipeline(healthOptions.Component)
		graph, err := p.Plan(runtimeCtx)
		if err != nil {
			return fmt.Errorf("health pipeline planning failed: %w", err)
		}

		result, err := p.Run(runtimeCtx, graph, false)
		if err != nil {
			if result != nil && result.Status == plan.StatusFailed {
				return fmt.Errorf("health check failed: %s", result.Message)
			}
			return fmt.Errorf("health pipeline execution failed: %w", err)
		}
		if result.Status == plan.StatusFailed {
			return fmt.Errorf("health check failed with status: %s. Message: %s", result.Status, result.Message)
		}

		log.Infof("Health check completed successfully. Status: %s", result.Status)
		return nil
	},
}
