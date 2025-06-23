package cluster

import (
	"fmt"
	"os" // For os.Exit
	"path/filepath" // For config file path manipulation

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	kubexmcluster "github.com/mensylisir/kubexm/pkg/pipeline/cluster" // Alias to avoid conflict
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/config" // For LoadClusterConfigFromFile
	"github.com/mensylisir/kubexm/pkg/logger" // For logger
	"github.com/mensylisir/kubexm/pkg/engine" // For NewEngine
	"github.com/mensylisir/kubexm/pkg/plan" // For plan.StatusFailed
	// "github.com/mensylisir/kubexm/pkg/connector" // For NewConnectionPool if needed directly here
	"github.com/spf13/cobra"
	"context" // For context.Background
	"time" // For timeout example
)

type CreateOptions struct {
	ClusterConfigFile string
	SkipPreflight     bool
	DryRun            bool
	Verbose           bool
	YesAssume         bool // Corresponds to --yes or -y for auto-approval
}

var createOptions = &CreateOptions{}

func init() {
	ClusterCmd.AddCommand(createCmd)
	createCmd.Flags().StringVarP(&createOptions.ClusterConfigFile, "config", "f", "", "Path to the cluster configuration YAML file (required)")
	createCmd.Flags().BoolVar(&createOptions.SkipPreflight, "skip-preflight", false, "Skip preflight checks")
	createCmd.Flags().BoolVar(&createOptions.DryRun, "dry-run", false, "Simulate the cluster creation without making any changes")
	createCmd.Flags().BoolVarP(&createOptions.Verbose, "verbose", "v", false, "Enable verbose output")
	createCmd.Flags().BoolVarP(&createOptions.YesAssume, "yes", "y", false, "Assume yes to all prompts and run non-interactively")

	// Mark flags as required if necessary
	if err := createCmd.MarkFlagRequired("config"); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to mark 'config' flag as required: %v\n", err)
		// Depending on desired strictness, could os.Exit(1) here or let Cobra handle it
	}
}

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new Kubernetes cluster",
	Long:  `Create a new Kubernetes cluster based on a provided configuration file.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Initialize logger
		log := logger.NewLogger(createOptions.Verbose, "") // Empty log file path for now
		log.Info("Starting cluster creation process...")

		if createOptions.ClusterConfigFile == "" {
			return fmt.Errorf("cluster configuration file must be provided via -f or --config flag")
		}

		absPath, err := filepath.Abs(createOptions.ClusterConfigFile)
		if err != nil {
			log.Error(err, "Failed to get absolute path for config file")
			return fmt.Errorf("failed to get absolute path for config file %s: %w", createOptions.ClusterConfigFile, err)
		}
		log.Infof("Loading cluster configuration from: %s", absPath)

		clusterConfig, err := config.LoadClusterConfigFromFile(absPath)
		if err != nil {
			log.Error(err, "Failed to load cluster configuration")
			return fmt.Errorf("failed to load cluster configuration from %s: %w", absPath, err)
		}

		// Validate the loaded configuration (basic validation for now)
		if err := config.ValidateClusterConfig(clusterConfig); err != nil {
			log.Error(err, "Cluster configuration validation failed")
			return fmt.Errorf("cluster configuration validation failed: %w", err)
		}
		log.Info("Cluster configuration loaded and validated successfully.")

		// Initialize runtime.Context
		// Most of these would come from global flags or a config file for kubexm itself
		// For now, using some defaults or options from createCmd
		// The runtime builder would normally handle this.
		// We are creating a simplified context for now.
		goCtx, cancel := context.WithTimeout(context.Background(), 30*time.Minute) // Example timeout
		defer cancel()

		// TODO: Replace this with a proper RuntimeBuilder
		eng := engine.NewEngine(log) // Create a new engine instance

		// Simplified context creation
		// In a real scenario, a RuntimeBuilder would populate HostRuntimes, ConnectionPool etc.
		// based on clusterConfig and connect to hosts.
		// For this stub, many fields will be nil or default.
		runtimeCtx := &runtime.Context{
			GoCtx:         goCtx,
			Logger:        log,
			Engine:        eng, // Pass the engine
			ClusterConfig: clusterConfig,
			// HostRuntimes:  make(map[string]*runtime.HostRuntime), // Needs population by a builder
			// ConnectionPool: connector.NewConnectionPool(log, connectionTimeout), // Needs population
			GlobalWorkDir: ".", // Default to current directory or get from a global config
			GlobalVerbose: createOptions.Verbose,
			GlobalIgnoreErr: false, // Default
			GlobalConnectionTimeout: 1 * time.Minute, // Default
			// Caches would be initialized here too
		}
		log.Info("Runtime context initialized.")

		// Instantiate the CreateClusterPipeline
		// The 'assumeYes' parameter for NewCreateClusterPipeline should come from createOptions.YesAssume
		createPipeline := kubexmcluster.NewCreateClusterPipeline(createOptions.YesAssume)
		log.Infof("Instantiated pipeline: %s", createPipeline.Name())

		// Run the pipeline
		log.Info("Executing pipeline run...")
		result, err := createPipeline.Run(runtimeCtx, createOptions.DryRun)
		if err != nil {
			log.Error(err, "Cluster creation pipeline failed")
			// result might still contain partial information
			if result != nil {
				log.Infof("Pipeline result status: %s", result.Status)
			}
			return fmt.Errorf("cluster creation pipeline execution failed: %w", err)
		}

		if result.Status == plan.StatusFailed { // Corrected to use plan.StatusFailed
			log.Error(nil, "Cluster creation pipeline reported failure.", "status", result.Status)
			return fmt.Errorf("cluster creation pipeline failed with status: %s", result.Status)
		}

		log.Info("Cluster creation pipeline completed successfully!", "status", result.Status)
		// TODO: Print summary or kubeconfig location if applicable

		return nil
	},
}
