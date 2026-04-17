package cluster

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mensylisir/kubexm/internal/config"
	"github.com/mensylisir/kubexm/internal/logger"
	"github.com/mensylisir/kubexm/internal/pipeline/cluster"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
)

type DeleteNodesOptions struct {
	ClusterConfigFile string
	Force            bool
	DryRun           bool
}

var deleteNodesOptions = &DeleteNodesOptions{}

func init() {
	ClusterCmd.AddCommand(deleteNodesCmd)
	deleteNodesCmd.Flags().StringVarP(&deleteNodesOptions.ClusterConfigFile, "config", "f", "", "Path to the cluster configuration YAML file (required)")
	deleteNodesCmd.Flags().BoolVar(&deleteNodesOptions.Force, "force", false, "Force delete without confirmation")
	deleteNodesCmd.Flags().BoolVar(&deleteNodesOptions.DryRun, "dry-run", false, "Simulate the node deletion without making any changes")

	if err := deleteNodesCmd.MarkFlagRequired("config"); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to mark 'config' flag as required: %v\n", err)
	}
}

var deleteNodesCmd = &cobra.Command{
	Use:   "delete-nodes",
	Short: "Delete nodes from an existing cluster",
	Long:  `Delete specified worker or control-plane nodes from an existing Kubernetes cluster.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		log := logger.Get()
		defer logger.SyncGlobal()

		assumeYesGlobal, _ := cmd.Flags().GetBool("yes")

		log.Info("Starting delete nodes process...")

		if deleteNodesOptions.ClusterConfigFile == "" {
			return fmt.Errorf("cluster configuration file must be provided via -f or --config flag")
		}

		absPath, err := filepath.Abs(deleteNodesOptions.ClusterConfigFile)
		if err != nil {
			log.Errorf("Failed to get absolute path for config file %s: %v", deleteNodesOptions.ClusterConfigFile, err)
			return fmt.Errorf("failed to get absolute path for config file %s: %w", deleteNodesOptions.ClusterConfigFile, err)
		}
		log.Infof("Using cluster configuration from: %s", absPath)

		clusterConfig, err := config.ParseFromFile(absPath)
		if err != nil {
			log.Errorf("Failed to parse cluster configuration: %v", err)
			return fmt.Errorf("failed to parse cluster configuration from %s: %w", absPath, err)
		}

		if !deleteNodesOptions.Force && !assumeYesGlobal {
			fmt.Printf("WARNING: This action will delete nodes from cluster '%s'.\n", clusterConfig.Name)
			fmt.Println("Nodes will be drained and removed from the cluster.")
			fmt.Print("Are you sure you want to proceed? (yes/no): ")
			reader := bufio.NewReader(os.Stdin)
			input, err := reader.ReadString('\n')
			if err != nil {
				input = "no"
			}
			input = strings.TrimSpace(strings.ToLower(input))
			if input != "yes" {
				log.Info("Delete nodes aborted by user.")
				return nil
			}
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

		deleteNodesPipeline := cluster.NewDeleteNodesPipeline(assumeYesGlobal)
		log.Infof("Instantiated pipeline: %s", deleteNodesPipeline.Name())

		log.Info("Planning pipeline execution...")
		executionGraph, err := deleteNodesPipeline.Plan(runtimeCtx)
		if err != nil {
			log.Errorf("Pipeline planning failed: %v", err)
			return fmt.Errorf("pipeline planning failed: %w", err)
		}

		log.Info("Executing pipeline...")
		result, err := deleteNodesPipeline.Run(runtimeCtx, executionGraph, deleteNodesOptions.DryRun)
		if err != nil {
			log.Errorf("Delete nodes pipeline failed: %v", err)
			if result != nil {
				log.Infof("Pipeline final status: %s", result.Status)
				if result.Message != "" {
					log.Errorf("Pipeline error message: %s", result.Message)
				}
			}
			return fmt.Errorf("delete nodes pipeline execution failed: %w", err)
		}

		if result.Status == plan.StatusFailed {
			log.Errorf("Delete nodes pipeline reported failure. Status: %s. Message: %s", result.Status, result.Message)
			return fmt.Errorf("delete nodes pipeline failed with status: %s. Message: %s", result.Status, result.Message)
		}

		log.Infof("Delete nodes pipeline completed successfully! Status: %s", result.Status)
		return nil
	},
}
