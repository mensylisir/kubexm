package registry

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/internal/config"
	"github.com/mensylisir/kubexm/internal/logger"
	"github.com/mensylisir/kubexm/internal/plan"
	pipelinecluster "github.com/mensylisir/kubexm/internal/pipeline/cluster"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/spf13/cobra"
)

type CreateOptions struct {
	ClusterConfigFile string
	RegistryType     string
	RegistryPort     int
	DryRun          bool
}

var createOptions = &CreateOptions{}

func init() {
	CreateRegistryCmd.Flags().StringVarP(&createOptions.ClusterConfigFile, "config", "f", "", "Path to the cluster configuration YAML file (required)")
	CreateRegistryCmd.Flags().StringVar(&createOptions.RegistryType, "type", "registry", "Registry type: registry (default: registry)")
	CreateRegistryCmd.Flags().IntVar(&createOptions.RegistryPort, "port", 5000, "Registry port (default: 5000)")
	CreateRegistryCmd.Flags().BoolVar(&createOptions.DryRun, "dry-run", false, "Simulate the registry creation without making changes")

	if err := CreateRegistryCmd.MarkFlagRequired("config"); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to mark 'config' flag as required: %v\n", err)
	}
}

// CreateRegistryCmd is the registry create command
var CreateRegistryCmd = &cobra.Command{
	Use:   "registry",
	Short: "Create a private image registry",
	Long:  `Create a private Docker Registry on registry-role hosts.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		log := logger.Get()
		defer logger.SyncGlobal()

		assumeYesGlobal, _ := cmd.Flags().GetBool("yes")

		if createOptions.ClusterConfigFile == "" {
			return fmt.Errorf("cluster configuration file must be provided via -f or --config flag")
		}

		absPath, err := filepath.Abs(createOptions.ClusterConfigFile)
		if err != nil {
			return fmt.Errorf("failed to get absolute path for config file %s: %w", createOptions.ClusterConfigFile, err)
		}

		clusterConfig, err := config.ParseFromFileWithOptions(absPath, config.ParseOptions{SkipHostValidation: true})
		if err != nil {
			return fmt.Errorf("failed to load cluster configuration from %s: %w", absPath, err)
		}

		log.Infof("Starting registry creation for cluster '%s' (type: %s)", clusterConfig.Name, createOptions.RegistryType)

		goCtx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()

		rtBuilder := runtime.NewBuilderFromConfig(clusterConfig).
			WithSkipHostConnect(true).
			WithSkipConfigValidation(true)
		runtimeCtx, cleanupFunc, err := rtBuilder.Build(goCtx)
		if err != nil {
			return fmt.Errorf("failed to build runtime environment for registry creation: %w", err)
		}
		defer cleanupFunc()

		p := pipelinecluster.NewCreateRegistryPipeline(assumeYesGlobal)
		graph, err := p.Plan(runtimeCtx)
		if err != nil {
			return fmt.Errorf("registry pipeline planning failed: %w", err)
		}

		result, err := p.Run(runtimeCtx, graph, createOptions.DryRun)
		if err != nil {
			if result != nil && result.Status == plan.StatusFailed {
				return fmt.Errorf("registry creation failed: %s", result.Message)
			}
			return fmt.Errorf("registry pipeline execution failed: %w", err)
		}
		if result.Status == plan.StatusFailed {
			return fmt.Errorf("registry creation failed with status: %s. Message: %s", result.Status, result.Message)
		}

		log.Infof("Registry creation completed successfully. Status: %s", result.Status)
		return nil
	},
}
