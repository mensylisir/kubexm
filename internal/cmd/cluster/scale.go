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
	"github.com/mensylisir/kubexm/internal/pipeline"
	"github.com/mensylisir/kubexm/internal/pipeline/cluster"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
)

type ScaleOptions struct {
	ClusterConfigFile string
	Direction         string // "in" or "out"
	SkipPreflight     bool
	DryRun            bool
}

var scaleOptions = &ScaleOptions{}

func init() {
	ClusterCmd.AddCommand(scaleCmd)
	scaleCmd.Flags().StringVarP(&scaleOptions.ClusterConfigFile, "config", "f", "", "Path to the cluster configuration YAML file (required)")
	scaleCmd.Flags().StringVar(&scaleOptions.Direction, "direction", "", "Scale direction: 'in' (remove nodes) or 'out' (add nodes) (required)")
	scaleCmd.Flags().BoolVar(&scaleOptions.SkipPreflight, "skip-preflight", false, "Skip preflight checks")
	scaleCmd.Flags().BoolVar(&scaleOptions.DryRun, "dry-run", false, "Simulate the scaling operation without making any changes")

	if err := scaleCmd.MarkFlagRequired("config"); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to mark 'config' flag as required: %v\n", err)
	}
	if err := scaleCmd.MarkFlagRequired("direction"); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to mark 'direction' flag as required: %v\n", err)
	}
}

var scaleCmd = &cobra.Command{
	Use:   "scale",
	Short: "Scale a Kubernetes cluster by adding or removing nodes",
	Long: `Scale a Kubernetes cluster by adjusting the number of nodes.

Direction:
  out  - Add new nodes to the cluster (equivalent to add-nodes)
  in   - Remove nodes from the cluster (equivalent to delete-nodes)

Examples:
  # Scale out (add nodes) using a config file
  kubexm cluster scale --config cluster-config.yaml --direction out

  # Scale in (remove nodes) using a config file
  kubexm cluster scale --config cluster-config.yaml --direction in

  # Dry run to see what would happen
  kubexm cluster scale --config cluster-config.yaml --direction out --dry-run`,
	RunE: func(cmd *cobra.Command, args []string) error {
		log := logger.Get()
		defer logger.SyncGlobal()

		assumeYesGlobal, _ := cmd.Flags().GetBool("yes")

		// Validate direction
		direction := strings.ToLower(scaleOptions.Direction)
		if direction != "in" && direction != "out" {
			return fmt.Errorf("invalid direction: %s. Must be 'in' or 'out'", scaleOptions.Direction)
		}

		log.Infof("Starting scale %s process...", direction)

		if scaleOptions.ClusterConfigFile == "" {
			return fmt.Errorf("cluster configuration file must be provided via -f or --config flag")
		}

		absPath, err := filepath.Abs(scaleOptions.ClusterConfigFile)
		if err != nil {
			log.Errorf("Failed to get absolute path for config file %s: %v", scaleOptions.ClusterConfigFile, err)
			return fmt.Errorf("failed to get absolute path for config file %s: %w", scaleOptions.ClusterConfigFile, err)
		}
		log.Infof("Using cluster configuration from: %s", absPath)

		clusterConfig, err := config.ParseFromFile(absPath)
		if err != nil {
			log.Errorf("Failed to parse cluster configuration: %v", err)
			return fmt.Errorf("failed to parse cluster configuration from %s: %w", absPath, err)
		}

		if !assumeYesGlobal {
			action := "add nodes to"
			if direction == "in" {
				action = "remove nodes from"
			}
			fmt.Printf("WARNING: This action will %s cluster '%s'.\n", action, clusterConfig.Name)
			fmt.Println("Nodes will be added/removed according to the configuration.")
			fmt.Print("Are you sure you want to proceed? (yes/no): ")
			reader := bufio.NewReader(os.Stdin)
			input, err := reader.ReadString('\n')
			if err != nil {
				input = "no"
			}
			input = strings.TrimSpace(strings.ToLower(input))
			if input != "yes" {
				log.Info("Scale operation aborted by user.")
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

		// Select pipeline based on direction
		var scalePipeline pipeline.Pipeline
		var pipelineName string
		if direction == "out" {
			scalePipeline = cluster.NewAddNodesPipeline(assumeYesGlobal)
			pipelineName = "AddNodes"
		} else {
			scalePipeline = cluster.NewDeleteNodesPipeline(assumeYesGlobal)
			pipelineName = "DeleteNodes"
		}
		log.Infof("Instantiated pipeline: %s", pipelineName)

		log.Info("Planning pipeline execution...")
		executionGraph, err := scalePipeline.Plan(runtimeCtx)
		if err != nil {
			log.Errorf("Pipeline planning failed: %v", err)
			return fmt.Errorf("pipeline planning failed: %w", err)
		}

		log.Info("Executing pipeline...")
		result, err := scalePipeline.Run(runtimeCtx, executionGraph, scaleOptions.DryRun)
		if err != nil {
			log.Errorf("Scale pipeline failed: %v", err)
			if result != nil {
				log.Infof("Pipeline final status: %s", result.Status)
				if result.Message != "" {
					log.Errorf("Pipeline error message: %s", result.Message)
				}
			}
			return fmt.Errorf("scale pipeline execution failed: %w", err)
		}

		if result.Status == plan.StatusFailed {
			log.Errorf("Scale pipeline reported failure. Status: %s. Message: %s", result.Status, result.Message)
			return fmt.Errorf("scale pipeline failed with status: %s. Message: %s", result.Status, result.Message)
		}

		log.Infof("Scale pipeline completed successfully! Status: %s", result.Status)
		return nil
	},
}