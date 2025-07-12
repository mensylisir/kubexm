package cluster

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	
	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/config"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/engine"
	"github.com/mensylisir/kubexm/pkg/logger"
	"github.com/mensylisir/kubexm/pkg/pipeline/cluster"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runner"
	"github.com/mensylisir/kubexm/pkg/runtime"
)

type CreateOptions struct {
	ClusterConfigFile string
	SkipPreflight     bool
	DryRun            bool
	// Verbose and YesAssume will use global flags from root.go
}

var createOptions = &CreateOptions{} // Verbose and YesAssume removed

func init() {
	ClusterCmd.AddCommand(createCmd)
	createCmd.Flags().StringVarP(&createOptions.ClusterConfigFile, "config", "f", "", "Path to the cluster configuration YAML file (required)")
	createCmd.Flags().BoolVar(&createOptions.SkipPreflight, "skip-preflight", false, "Skip preflight checks")
	createCmd.Flags().BoolVar(&createOptions.DryRun, "dry-run", false, "Simulate the cluster creation without making any changes")
	// Local verbose and yes flags are removed, will use global ones from rootCmd

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
		log := logger.Get()
		defer logger.SyncGlobal()

		// Get global flags
		assumeYesGlobal, _ := cmd.Flags().GetBool("yes")

		log.Info("Starting cluster creation process...")

		// Validate config file path
		if createOptions.ClusterConfigFile == "" {
			return fmt.Errorf("cluster configuration file must be provided via -f or --config flag")
		}

		absPath, err := filepath.Abs(createOptions.ClusterConfigFile)
		if err != nil {
			log.Errorf("Failed to get absolute path for config file %s: %v", createOptions.ClusterConfigFile, err)
			return fmt.Errorf("failed to get absolute path for config file %s: %w", createOptions.ClusterConfigFile, err)
		}
		log.Infof("Using cluster configuration from: %s", absPath)

		// Load and parse configuration
		clusterConfig, err := config.ParseFromFile(absPath)
		if err != nil {
			log.Errorf("Failed to parse cluster configuration: %v", err)
			return fmt.Errorf("failed to parse cluster configuration from %s: %w", absPath, err)
		}

		// Apply CLI overrides
		if createOptions.SkipPreflight {
			if clusterConfig.Spec.Global == nil {
				clusterConfig.Spec.Global = &v1alpha1.GlobalSpec{}
			}
			clusterConfig.Spec.Global.SkipPreflight = true
			log.Info("Preflight checks will be skipped due to --skip-preflight flag.")
		}

		// Initialize services
		connectorFactory := connector.NewDefaultFactory()
		if connectorFactory == nil {
			log.Fatalf("Failed to create connector factory.")
			return fmt.Errorf("failed to create connector factory")
		}

		connectionPool := connector.NewConnectionPool(connector.DefaultPoolConfig())
		runnerSvc := runner.New()
		engineSvc := engine.NewExecutor()

		// Create runtime context
		goCtx := context.Background()
		rtBuilder := runtime.NewRuntimeBuilderFromConfig(clusterConfig, runnerSvc, connectionPool, connectorFactory)

		log.Info("Building runtime environment...")
		runtimeCtx, cleanupFunc, err := rtBuilder.Build(goCtx, engineSvc)
		if err != nil {
			log.Errorf("Failed to build runtime environment: %v", err)
			return fmt.Errorf("failed to build runtime environment: %w", err)
		}
		defer cleanupFunc()

		log.Info("Runtime environment built successfully.")

		// Create and execute pipeline
		createPipeline := cluster.NewCreateClusterPipeline(assumeYesGlobal)
		log.Infof("Instantiated pipeline: %s", createPipeline.Name())

		// Plan the pipeline
		log.Info("Planning pipeline execution...")
		executionGraph, err := createPipeline.Plan(runtimeCtx)
		if err != nil {
			log.Errorf("Pipeline planning failed: %v", err)
			return fmt.Errorf("pipeline planning failed: %w", err)
		}

		// Execute the pipeline
		log.Info("Executing pipeline...")
		result, err := createPipeline.Run(runtimeCtx, executionGraph, createOptions.DryRun)
		if err != nil {
			log.Errorf("Cluster creation pipeline failed: %v", err)
			if result != nil {
				log.Infof("Pipeline final status: %s", result.Status)
				if result.ErrorMessage != "" {
					log.Errorf("Pipeline error message: %s", result.ErrorMessage)
				}
			}
			return fmt.Errorf("cluster creation pipeline execution failed: %w", err)
		}

		if result.Status == plan.StatusFailed {
			log.Errorf("Cluster creation pipeline reported failure. Status: %s. Message: %s", result.Status, result.ErrorMessage)
			return fmt.Errorf("cluster creation pipeline failed with status: %s. Message: %s", result.Status, result.ErrorMessage)
		}

		log.Infof("Cluster creation pipeline completed successfully! Status: %s", result.Status)
		return nil
	},
}
