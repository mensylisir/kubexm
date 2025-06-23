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
)

type DeleteOptions struct {
	ClusterConfigFile string
	YesAssume         bool
	Force             bool // May not be used initially, but common for delete commands
	Verbose           bool
}

var deleteOptions = &DeleteOptions{}

func init() {
	ClusterCmd.AddCommand(deleteCmd)
	deleteCmd.Flags().StringVarP(&deleteOptions.ClusterConfigFile, "config", "f", "", "Path to the cluster configuration YAML file (required)")
	deleteCmd.Flags().BoolVarP(&deleteOptions.YesAssume, "yes", "y", false, "Assume yes to all prompts and run non-interactively")
	deleteCmd.Flags().BoolVar(&deleteOptions.Force, "force", false, "Force deletion without some safety checks (use with caution)")
	deleteCmd.Flags().BoolVarP(&deleteOptions.Verbose, "verbose", "v", false, "Enable verbose output (already in create, but good to have consistency if root doesn't have it)")

	if err := deleteCmd.MarkFlagRequired("config"); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to mark 'config' flag as required for delete command: %v\n", err)
	}
}

var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a Kubernetes cluster",
	Long:  `Delete a Kubernetes cluster based on a provided configuration file. This operation is destructive and will remove cluster components and data.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		log := logger.NewLogger(deleteOptions.Verbose, "") // Create logger instance
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

		clusterConfig, err := config.LoadClusterConfigFromFile(absPath)
		if err != nil {
			log.Error(err, "Failed to load cluster configuration")
			return fmt.Errorf("failed to load cluster configuration from %s: %w", absPath, err)
		}
		log.Infof("Configuration loaded for cluster: %s", clusterConfig.Name)

		// Confirmation prompt
		if !deleteOptions.YesAssume {
			fmt.Printf("WARNING: This action will delete the Kubernetes cluster '%s'.\n", clusterConfig.Name)
			fmt.Printf("This is a destructive operation and cannot be undone.\n")
			fmt.Print("Are you sure you want to proceed? (yes/no): ")
			reader := bufio.NewReader(os.Stdin)
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(strings.ToLower(input))
			if input != "yes" {
				log.Info("Cluster deletion aborted by user.")
				return nil // Not an error, user chose not to proceed
			}
		}

		log.Info("Proceeding with cluster deletion...")

		goCtx, cancel := context.WithTimeout(context.Background(), 30*time.Minute) // TODO: Make timeout configurable
		defer cancel()

		// TODO: Replace with a proper RuntimeBuilder, similar to 'create' command.
		// For deletion, the runtime context might need to connect to hosts to perform cleanup.
		eng := engine.NewEngine(log)
		runtimeCtx := &runtime.Context{
			GoCtx:         goCtx,
			Logger:        log,
			Engine:        eng,
			ClusterConfig: clusterConfig,
			GlobalWorkDir: ".", // Or load from a global kubexm config
			GlobalVerbose: deleteOptions.Verbose,
			// HostRuntimes and ConnectionPool would be populated by a builder
		}
		log.Info("Runtime context initialized for deletion.")

		// Instantiate the DeleteClusterPipeline
		// The 'assumeYes' might be useful for tasks within the pipeline that could also prompt.
		deletePipeline := kubexmcluster.NewDeleteClusterPipeline(deleteOptions.YesAssume)
		log.Infof("Instantiated pipeline: %s", deletePipeline.Name())

		// Run the pipeline
		log.Info("Executing delete pipeline run...")
		result, err := deletePipeline.Run(runtimeCtx, false) // dryRun is false for actual deletion
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
