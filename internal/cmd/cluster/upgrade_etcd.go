package cluster

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
	kubexmcluster "github.com/mensylisir/kubexm/internal/pipeline/cluster"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/spf13/cobra"
)

type UpgradeEtcdOptions struct {
	ClusterConfigFile string
	TargetVersion     string
	DryRun            bool
}

var upgradeEtcdOptions = &UpgradeEtcdOptions{}

func init() {
	ClusterCmd.AddCommand(upgradeEtcdCmd)
	upgradeEtcdCmd.Flags().StringVarP(&upgradeEtcdOptions.ClusterConfigFile, "config", "f", "", "Path to the cluster configuration YAML file (required)")
	upgradeEtcdCmd.Flags().StringVarP(&upgradeEtcdOptions.TargetVersion, "to-version", "t", "", "Target etcd version for the upgrade (e.g., v3.5.9) (required)")
	upgradeEtcdCmd.Flags().BoolVar(&upgradeEtcdOptions.DryRun, "dry-run", false, "Simulate the etcd upgrade without making changes")

	if err := upgradeEtcdCmd.MarkFlagRequired("config"); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to mark 'config' flag as required for upgrade etcd command: %v\n", err)
	}
	if err := upgradeEtcdCmd.MarkFlagRequired("to-version"); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to mark 'to-version' flag as required for upgrade etcd command: %v\n", err)
	}
}

var upgradeEtcdCmd = &cobra.Command{
	Use:   "etcd",
	Short: "Upgrade etcd in an existing Kubernetes cluster",
	Long:  `Upgrade etcd cluster to a newer version. This operation is destructive and requires cluster downtime.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		log := logger.Get()
		defer logger.SyncGlobal()

		assumeYesGlobal, _ := cmd.Flags().GetBool("yes")

		log.Info("Starting etcd upgrade process...")

		if upgradeEtcdOptions.ClusterConfigFile == "" {
			return fmt.Errorf("cluster configuration file must be provided via -f or --config flag for etcd upgrade")
		}
		if upgradeEtcdOptions.TargetVersion == "" {
			return fmt.Errorf("target etcd version must be provided via --to-version flag for etcd upgrade")
		}

		absPath, err := filepath.Abs(upgradeEtcdOptions.ClusterConfigFile)
		if err != nil {
			log.With("error", err).Error("Failed to get absolute path for config file")
			return fmt.Errorf("failed to get absolute path for config file %s: %w", upgradeEtcdOptions.ClusterConfigFile, err)
		}
		log.Infof("Loading cluster configuration from: %s for etcd upgrade to version %s.", absPath, upgradeEtcdOptions.TargetVersion)

		clusterConfig, err := config.ParseFromFile(absPath)
		if err != nil {
			log.With("error", err).Error("Failed to load cluster configuration")
			return fmt.Errorf("failed to load cluster configuration from %s: %w", absPath, err)
		}
		log.Infof("Configuration loaded for cluster: %s", clusterConfig.Name)

		if !assumeYesGlobal {
			fmt.Printf("WARNING: This action will attempt to upgrade etcd in cluster '%s' to version '%s'.\n", clusterConfig.Name, upgradeEtcdOptions.TargetVersion)
			fmt.Printf("Ensure you have backed up your cluster and reviewed upgrade compatibility. Proceed? (yes/no): ")
			reader := bufio.NewReader(os.Stdin)
			input, err := reader.ReadString('\n')
			if err != nil {
				input = "no"
			}
			input = strings.TrimSpace(strings.ToLower(input))
			if input != "yes" {
				log.Info("Etcd upgrade aborted by user.")
				return nil
			}
		}
		log.Info("Proceeding with etcd upgrade...")

		goCtx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
		defer cancel()

		rtBuilder := runtime.NewBuilderFromConfig(clusterConfig)
		log.Info("Building runtime environment for etcd upgrade...")
		runtimeCtx, cleanupFunc, err := rtBuilder.Build(goCtx)
		if err != nil {
			log.With("error", err).Error("Failed to build runtime environment for etcd upgrade")
			return fmt.Errorf("failed to build runtime environment for etcd upgrade: %w", err)
		}
		defer cleanupFunc()
		log.Info("Runtime environment built successfully for etcd upgrade.")

		upgradeEtcdPipeline := kubexmcluster.NewUpgradeEtcdPipeline(upgradeEtcdOptions.TargetVersion)
		log.Infof("Instantiated pipeline: %s", upgradeEtcdPipeline.Name())

		log.Info("Executing etcd upgrade pipeline run...")
		result, err := upgradeEtcdPipeline.Run(runtimeCtx, nil, upgradeEtcdOptions.DryRun)
		if err != nil {
			log.With("error", err).Error("Etcd upgrade pipeline failed")
			if result != nil {
				log.Infof("Pipeline result status during failure: %s", result.Status)
			}
			return fmt.Errorf("etcd upgrade pipeline execution failed: %w", err)
		}

		if result.Status == plan.StatusFailed {
			log.With("status", result.Status).Error("Etcd upgrade pipeline reported failure.")
			return fmt.Errorf("etcd upgrade pipeline failed with status: %s", result.Status)
		}

		log.Infof("Etcd upgrade pipeline completed successfully! Status: %s", result.Status)
		return nil
	},
}
