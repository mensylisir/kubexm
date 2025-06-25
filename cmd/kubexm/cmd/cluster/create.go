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
		// Initialize global logger
		logOpts := logger.DefaultOptions()
		logOpts.ColorConsole = true // Assuming color by default for CLI
		if createOptions.Verbose {
			logOpts.ConsoleLevel = logger.DebugLevel
		}
		logger.Init(logOpts) // Initialize global logger
		log := logger.Get()   // Get the global logger instance
		defer logger.SyncGlobal()

		log.Info("Starting cluster creation process...")

		if createOptions.ClusterConfigFile == "" {
			// This check is good, but Cobra's MarkFlagRequired should also handle it.
			return fmt.Errorf("cluster configuration file must be provided via -f or --config flag")
		}

		absPath, err := filepath.Abs(createOptions.ClusterConfigFile)
		if err != nil {
			log.Errorf("Failed to get absolute path for config file %s: %v", createOptions.ClusterConfigFile, err)
			return fmt.Errorf("failed to get absolute path for config file %s: %w", createOptions.ClusterConfigFile, err)
		}
		log.Infof("Using cluster configuration from: %s", absPath)

		// Initialize services for RuntimeBuilder
		// Assuming NewDefaultFactory exists and is the intended constructor
		connectorFactory := connector.NewDefaultFactory()
		if connectorFactory == nil {
			// This would be a programming error if NewDefaultFactory is expected to always succeed.
			log.Fatalf("Failed to create connector factory.") // Use Fatalf from global logger
			return fmt.Errorf("failed to create connector factory")
		}

		connectionPool := connector.NewConnectionPool(connector.DefaultPoolConfig()) // Uses default config
		runnerSvc := runner.New()
		engineSvc := engine.NewExecutor() // Uses new DAG executor

		// Create and build runtime context
		// TODO: Pass a global timeout for the entire operation if desired via goCtx.
		// For now, using a background context, individual operations might have their own timeouts.
		goCtx := context.Background()
		// Pass CLI options like SkipPreflight to the builder or context if they affect runtime behavior.
		// For now, RuntimeBuilder primarily uses the config file.
		// Global flags like Verbose are handled by logger init. YesAssume is passed to pipeline.

		rtBuilder := runtime.NewRuntimeBuilder(absPath, runnerSvc, connectionPool, connectorFactory, engineSvc)
		// The RuntimeBuilder constructor needs engineSvc if it's to be part of the context it builds.
		// Let's adjust the builder or how engine is set.
		// The runtime.Context has an Engine field. Builder should set it.
		// For now, assuming engineSvc is passed to Build, or set on RuntimeBuilder.
		// The design has engineSvc passed to Build.

		log.Info("Building runtime environment...")
		runtimeCtx, cleanupFunc, err := rtBuilder.Build(goCtx) // engineSvc removed from Build, assuming it's part of builder's init
		if err != nil {
			log.Errorf("Failed to build runtime environment: %v", err)
			return fmt.Errorf("failed to build runtime environment: %w", err)
		}
		defer cleanupFunc() // Ensure connection pool and other resources are cleaned up

		// Override context's engine if builder doesn't set it (though it should)
		if runtimeCtx.Engine == nil {
			runtimeCtx.Engine = engineSvc
		}

		log.Info("Runtime environment built successfully.")

		// Instantiate the CreateClusterPipeline
		createPipeline := kubexmcluster.NewCreateClusterPipeline(createOptions.YesAssume)
		log.Infof("Instantiated pipeline: %s", createPipeline.Name())

		// Run the pipeline
		log.Info("Executing pipeline run...")
		// The pipeline's Run method expects the full *runtime.Context
		result, err := createPipeline.Run(runtimeCtx, createOptions.DryRun)
		if err != nil {
			log.Errorf("Cluster creation pipeline failed: %v", err)
			if result != nil {
				log.Infof("Pipeline final status: %s", result.Status)
				if result.ErrorMessage != "" {
					log.Errorf("Pipeline error message: %s", result.ErrorMessage)
				}
			}
			// Consider exiting with non-zero status for scriptability
			// os.Exit(1) // Or let Cobra handle error return
			return fmt.Errorf("cluster creation pipeline execution failed: %w", err)
		}

		if result.Status == plan.StatusFailed {
			log.Errorf("Cluster creation pipeline reported failure. Status: %s. Message: %s", result.Status, result.ErrorMessage)
			// os.Exit(1)
			return fmt.Errorf("cluster creation pipeline failed with status: %s. Message: %s", result.Status, result.ErrorMessage)
		}

		log.Infof("Cluster creation pipeline completed successfully! Status: %s", result.Status)
		// TODO: Print summary or kubeconfig location if applicable if result.Status is Success

		return nil
	},
}
