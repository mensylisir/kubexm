package registry

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/internal/config"
	"github.com/mensylisir/kubexm/internal/logger"
	"github.com/mensylisir/kubexm/internal/plan"
	pipelinecluster "github.com/mensylisir/kubexm/internal/pipeline/cluster"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/spf13/cobra"
)

type DeleteOptions struct {
	ClusterConfigFile string
	Force            bool
	DeleteImages     bool
	DryRun           bool
}

var deleteOptions = &DeleteOptions{}

func init() {
	DeleteRegistryCmd.Flags().StringVarP(&deleteOptions.ClusterConfigFile, "config", "f", "", "Path to the cluster configuration YAML file (required)")
	DeleteRegistryCmd.Flags().BoolVar(&deleteOptions.Force, "force", false, "Force deletion without confirmation")
	DeleteRegistryCmd.Flags().BoolVar(&deleteOptions.DeleteImages, "delete-images", false, "Also delete stored images")
	DeleteRegistryCmd.Flags().BoolVar(&deleteOptions.DryRun, "dry-run", false, "Simulate the registry deletion without making changes")

	if err := DeleteRegistryCmd.MarkFlagRequired("config"); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to mark 'config' flag as required: %v\n", err)
	}
}

// DeleteRegistryCmd is the registry delete command
var DeleteRegistryCmd = &cobra.Command{
	Use:   "registry",
	Short: "Delete a private image registry",
	Long:  `Delete the private Docker Registry from registry-role hosts.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		log := logger.Get()
		defer logger.SyncGlobal()

		assumeYesGlobal, _ := cmd.Flags().GetBool("yes")

		if deleteOptions.ClusterConfigFile == "" {
			return fmt.Errorf("cluster configuration file must be provided via -f or --config flag")
		}

		absPath, err := filepath.Abs(deleteOptions.ClusterConfigFile)
		if err != nil {
			return fmt.Errorf("failed to get absolute path for config file %s: %w", deleteOptions.ClusterConfigFile, err)
		}

		clusterConfig, err := config.ParseFromFileWithOptions(absPath, config.ParseOptions{SkipHostValidation: true})
		if err != nil {
			return fmt.Errorf("failed to load cluster configuration from %s: %w", absPath, err)
		}

		if !deleteOptions.Force && !assumeYesGlobal {
			fmt.Printf("WARNING: This action will delete the image registry for cluster '%s'.\n", clusterConfig.Name)
			if deleteOptions.DeleteImages {
				fmt.Println("All stored images will also be deleted.")
			}
			fmt.Print("Are you sure you want to proceed? (yes/no): ")
			reader := bufio.NewReader(os.Stdin)
			input, err := reader.ReadString('\n')
			if err != nil {
				input = "no"
			}
			input = strings.TrimSpace(strings.ToLower(input))
			if input != "yes" {
				log.Info("Registry deletion aborted by user.")
				return nil
			}
		}

		log.Infof("Starting registry deletion for cluster '%s'", clusterConfig.Name)

		goCtx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()

		rtBuilder := runtime.NewBuilderFromConfig(clusterConfig).
			WithSkipHostConnect(true).
			WithSkipConfigValidation(true)
		runtimeCtx, cleanupFunc, err := rtBuilder.Build(goCtx)
		if err != nil {
			return fmt.Errorf("failed to build runtime environment for registry deletion: %w", err)
		}
		defer cleanupFunc()

		p := pipelinecluster.NewDeleteRegistryPipeline(assumeYesGlobal)
		graph, err := p.Plan(runtimeCtx)
		if err != nil {
			return fmt.Errorf("registry pipeline planning failed: %w", err)
		}

		result, err := p.Run(runtimeCtx, graph, deleteOptions.DryRun)
		if err != nil {
			if result != nil && result.Status == plan.StatusFailed {
				return fmt.Errorf("registry deletion failed: %s", result.Message)
			}
			return fmt.Errorf("registry pipeline execution failed: %w", err)
		}
		if result.Status == plan.StatusFailed {
			return fmt.Errorf("registry deletion failed with status: %s. Message: %s", result.Status, result.Message)
		}

		log.Infof("Registry deletion completed successfully. Status: %s", result.Status)
		return nil
	},
}
