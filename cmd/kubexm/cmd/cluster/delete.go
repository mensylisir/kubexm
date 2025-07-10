package cluster

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/config"
	"github.com/mensylisir/kubexm/pkg/engine"
	"github.com/mensylisir/kubexm/pkg/logger"
	"github.com/mensylisir/kubexm/pkg/plan"
	kubexmcluster "github.com/mensylisir/kubexm/pkg/pipeline/cluster" // Alias for pipeline cluster package
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/spf13/cobra"
	"github.com/mensylisir/kubexm/pkg/connector" // Added
)

type DeleteOptions struct {
	ClusterConfigFile string
	// YesAssume is now handled by global flag
	Force             bool // May not be used initially, but common for delete commands
	// Verbose is now handled by global flag
}

var deleteOptions = &DeleteOptions{}

func init() {
	ClusterCmd.AddCommand(deleteCmd)
	deleteCmd.Flags().StringVarP(&deleteOptions.ClusterConfigFile, "config", "f", "", "Path to the cluster configuration YAML file (required)")
	// deleteCmd.Flags().BoolVarP(&deleteOptions.YesAssume, "yes", "y", false, "Assume yes to all prompts and run non-interactively") // Uses global
	deleteCmd.Flags().BoolVar(&deleteOptions.Force, "force", false, "Force deletion without some safety checks (use with caution)")
	// deleteCmd.Flags().BoolVarP(&deleteOptions.Verbose, "verbose", "v", false, "Enable verbose output (already in create, but good to have consistency if root doesn't have it)") // Uses global

	if err := deleteCmd.MarkFlagRequired("config"); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to mark 'config' flag as required for delete command: %v\n", err)
	}
}

var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a Kubernetes cluster",
	Long:  `Delete a Kubernetes cluster based on a provided configuration file. This operation is destructive and will remove cluster components and data.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		log := logger.Get() // Use global logger
		defer logger.SyncGlobal()

		assumeYesGlobal, _ := cmd.Flags().GetBool("yes") // Get global assumeYes

		log.Info("Starting cluster deletion process...")

		if deleteOptions.ClusterConfigFile == "" {
			return fmt.Errorf("cluster configuration file must be provided via -f or --config flag")
		}

		absPath, err := filepath.Abs(deleteOptions.ClusterConfigFile)
		if err != nil {
			log.Error(err, "Failed to get absolute path for config file")
			return fmt.Errorf("failed to get absolute path for config file %s: %w", deleteOptions.ClusterConfigFile, err)
		}
		log.Infof("Loading cluster configuration from: %s for deletion.", absPath)

		clusterConfig, err := config.ParseFromFile(absPath) // Use ParseFromFile
		if err != nil {
			log.Error(err, "Failed to load cluster configuration")
			return fmt.Errorf("failed to load cluster configuration from %s: %w", absPath, err)
		}
		log.Infof("Configuration loaded for cluster: %s", clusterConfig.Name)

		// Confirmation prompt
		if !assumeYesGlobal { // Use global assumeYes
			fmt.Printf("WARNING: This action will delete the Kubernetes cluster '%s'.\n", clusterConfig.Name)
			fmt.Printf("This is a destructive operation and cannot be undone.\n")
			fmt.Print("Are you sure you want to proceed? (yes/no): ")
			reader := bufio.NewReader(os.Stdin)
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(strings.ToLower(input))
			if input != "yes" {
				log.Info("Cluster deletion aborted by user.")
				return nil
			}
		}

		log.Info("Proceeding with cluster deletion...")

		goCtx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()

		// Initialize services for RuntimeBuilder
		connectorFactory := connector.NewDefaultFactory()
		connectionPool := connector.NewConnectionPool(connector.DefaultPoolConfig())
		runnerSvc := runner.New()      // Assuming runner.New() exists and returns runner.Runner
		engineSvc := engine.NewExecutor()

		rtBuilder := runtime.NewRuntimeBuilderFromConfig(clusterConfig, runnerSvc, connectionPool, connectorFactory)
		log.Info("Building runtime environment for deletion...")
		runtimeCtx, cleanupFunc, err := rtBuilder.Build(goCtx, engineSvc)
		if err != nil {
			log.Error(err, "Failed to build runtime environment for deletion")
			return fmt.Errorf("failed to build runtime environment for deletion: %w", err)
		}
		defer cleanupFunc()
		log.Info("Runtime environment built successfully for deletion.")

		deletePipeline := kubexmcluster.NewDeleteClusterPipeline(assumeYesGlobal) // Pass global assumeYes
		log.Infof("Instantiated pipeline: %s", deletePipeline.Name())

		log.Info("Executing delete pipeline run...")
		// The pipeline's Run method expects the full *runtime.Context, which runtimeCtx is.
		// The interface pipeline.PipelineContext is satisfied by *runtime.Context.
		// Pass nil for graph, as DeleteClusterPipeline's Run method will call Plan.
		result, err := deletePipeline.Run(runtimeCtx, nil, false) // dryRun is false
		if err != nil {
			log.Error(err, "Cluster deletion pipeline failed")
			if result != nil {
				log.Infof("Pipeline result status during failure: %s", result.Status)
			}
			return fmt.Errorf("cluster deletion pipeline execution failed: %w", err)
		}

		if result.Status == plan.StatusFailed {
			log.Error(nil, "Cluster deletion pipeline reported failure.", "status", result.Status)
			return fmt.Errorf("cluster deletion pipeline failed with status: %s", result.Status)
		}

		log.Info("Cluster deletion pipeline completed successfully!", "status", result.Status)

		// TODO: Add optional cleanup of local artifact directory for the cluster
		// e.g., os.RemoveAll(runtimeCtx.GetClusterArtifactsDir())
		// This should be done carefully.

		return nil
	},
}
