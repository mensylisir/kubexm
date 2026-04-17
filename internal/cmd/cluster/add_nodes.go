package cluster

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/mensylisir/kubexm/internal/config"
	"github.com/mensylisir/kubexm/internal/logger"
	"github.com/mensylisir/kubexm/internal/pipeline/cluster"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
)

type AddNodesOptions struct {
	ClusterConfigFile string
	SkipPreflight     bool
	DryRun            bool
}

var addNodesOptions = &AddNodesOptions{}

func init() {
	ClusterCmd.AddCommand(addNodesCmd)
	addNodesCmd.Flags().StringVarP(&addNodesOptions.ClusterConfigFile, "config", "f", "", "Path to the cluster configuration YAML file (required)")
	addNodesCmd.Flags().BoolVar(&addNodesOptions.SkipPreflight, "skip-preflight", false, "Skip preflight checks")
	addNodesCmd.Flags().BoolVar(&addNodesOptions.DryRun, "dry-run", false, "Simulate the node addition without making any changes")

	if err := addNodesCmd.MarkFlagRequired("config"); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to mark 'config' flag as required: %v\n", err)
	}
}

var addNodesCmd = &cobra.Command{
	Use:   "add-nodes",
	Short: "Add new worker or control-plane nodes to an existing cluster",
	Long:  `Add new worker or control-plane nodes to an existing Kubernetes cluster based on a provided configuration file.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		log := logger.Get()
		defer logger.SyncGlobal()

		assumeYesGlobal, _ := cmd.Flags().GetBool("yes")

		log.Info("Starting add nodes process...")

		if addNodesOptions.ClusterConfigFile == "" {
			return fmt.Errorf("cluster configuration file must be provided via -f or --config flag")
		}

		absPath, err := filepath.Abs(addNodesOptions.ClusterConfigFile)
		if err != nil {
			log.Errorf("Failed to get absolute path for config file %s: %v", addNodesOptions.ClusterConfigFile, err)
			return fmt.Errorf("failed to get absolute path for config file %s: %w", addNodesOptions.ClusterConfigFile, err)
		}
		log.Infof("Using cluster configuration from: %s", absPath)

		clusterConfig, err := config.ParseFromFile(absPath)
		if err != nil {
			log.Errorf("Failed to parse cluster configuration: %v", err)
			return fmt.Errorf("failed to parse cluster configuration from %s: %w", absPath, err)
		}

		if addNodesOptions.SkipPreflight {
			clusterConfig.Spec.Global.SkipPreflight = true
			log.Info("Preflight checks will be skipped due to --skip-preflight flag.")
		}

		goCtx := context.Background()
		rtBuilder := runtime.NewBuilderFromConfig(clusterConfig)

		log.Info("Building runtime environment...")
		runtimeCtx, cleanupFunc, err := rtBuilder.Build(goCtx)
		if err != nil {
			log.Errorf("Failed to build runtime environment: %v", err)
			return fmt.Errorf("failed to build runtime environment: %w", err)
		}
		defer cleanupFunc()

		log.Info("Runtime environment built successfully.")

		addNodesPipeline := cluster.NewAddNodesPipeline(assumeYesGlobal)
		log.Infof("Instantiated pipeline: %s", addNodesPipeline.Name())

		log.Info("Planning pipeline execution...")
		executionGraph, err := addNodesPipeline.Plan(runtimeCtx)
		if err != nil {
			log.Errorf("Pipeline planning failed: %v", err)
			return fmt.Errorf("pipeline planning failed: %w", err)
		}

		log.Info("Executing pipeline...")
		result, err := addNodesPipeline.Run(runtimeCtx, executionGraph, addNodesOptions.DryRun)
		if err != nil {
			log.Errorf("Add nodes pipeline failed: %v", err)
			if result != nil {
				log.Infof("Pipeline final status: %s", result.Status)
				if result.Message != "" {
					log.Errorf("Pipeline error message: %s", result.Message)
				}
			}
			return fmt.Errorf("add nodes pipeline execution failed: %w", err)
		}

		if result.Status == plan.StatusFailed {
			log.Errorf("Add nodes pipeline reported failure. Status: %s. Message: %s", result.Status, result.Message)
			return fmt.Errorf("add nodes pipeline failed with status: %s. Message: %s", result.Status, result.Message)
		}

		log.Infof("Add nodes pipeline completed successfully! Status: %s", result.Status)
		return nil
	},
}
