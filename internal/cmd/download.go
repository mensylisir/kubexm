package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mensylisir/kubexm/internal/config"
	"github.com/mensylisir/kubexm/internal/logger"
	"github.com/mensylisir/kubexm/internal/pipeline/assets"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/spf13/cobra"
)

type DownloadOptions struct {
	ClusterConfigFile string
	OutputPath        string
	DryRun            bool
}

var downloadOptions = &DownloadOptions{}

var downloadCmd = &cobra.Command{
	Use:   "download",
	Short: "Download all required assets for offline installation",
	Long:  "Download binaries, charts, and images on the control node and package them for offline use.",
	RunE: func(cmd *cobra.Command, args []string) error {
		log := logger.Get()
		defer logger.SyncGlobal()

		if downloadOptions.ClusterConfigFile == "" {
			return fmt.Errorf("cluster configuration file must be provided via -f or --config flag")
		}

		absPath, err := filepath.Abs(downloadOptions.ClusterConfigFile)
		if err != nil {
			return fmt.Errorf("failed to get absolute path for config file %s: %w", downloadOptions.ClusterConfigFile, err)
		}

		clusterConfig, err := config.ParseFromFileWithOptions(absPath, config.ParseOptions{SkipHostValidation: true})
		if err != nil {
			return fmt.Errorf("failed to load cluster configuration from %s: %w", absPath, err)
		}
		clusterConfig.Spec.Global.OfflineMode = false

		builder := runtime.NewBuilderFromConfig(clusterConfig).
			WithSkipHostConnect(true).
			WithSkipConfigValidation(true)
		runtimeCtx, cleanupFunc, err := builder.Build(context.Background())
		if err != nil {
			return fmt.Errorf("failed to build runtime environment for download: %w", err)
		}
		defer cleanupFunc()

		p := assets.NewDownloadAssetsPipeline(downloadOptions.OutputPath)
		graph, err := p.Plan(runtimeCtx)
		if err != nil {
			return fmt.Errorf("pipeline planning failed: %w", err)
		}

		result, err := p.Run(runtimeCtx, graph, downloadOptions.DryRun)
		if err != nil {
			if result != nil && result.Status == plan.StatusFailed {
				return fmt.Errorf("download pipeline failed: %s", result.Message)
			}
			return fmt.Errorf("download pipeline execution failed: %w", err)
		}
		if result.Status == plan.StatusFailed {
			return fmt.Errorf("download pipeline failed with status: %s. Message: %s", result.Status, result.Message)
		}
		log.Info("Download pipeline completed successfully.", "status", result.Status)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(downloadCmd)
	downloadCmd.Flags().StringVarP(&downloadOptions.ClusterConfigFile, "config", "f", "", "Path to the cluster configuration YAML/JSON file (required)")
	downloadCmd.Flags().StringVarP(&downloadOptions.OutputPath, "output", "o", "", "Output path for offline bundle (.tar.gz)")
	downloadCmd.Flags().BoolVar(&downloadOptions.DryRun, "dry-run", false, "Simulate the download without making any changes")

	if err := downloadCmd.MarkFlagRequired("config"); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to mark 'config' flag as required: %v\n", err)
	}
}
