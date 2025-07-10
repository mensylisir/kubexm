package cluster

import (
	"fmt"
	"github.com/spf13/cobra"
)

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrade an existing Kubernetes cluster",
	Long:  `Upgrade an existing Kubernetes cluster to a newer version or apply updates.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		log := logger.Get() // Use global logger
		defer logger.SyncGlobal()

		assumeYesGlobal, _ := cmd.Flags().GetBool("yes")
		dryRunGlobal, _ := cmd.Flags().GetBool("dry-run") // Assuming a global dry-run or add local one

		log.Info("Starting cluster upgrade process...")

		if upgradeOptions.ClusterConfigFile == "" {
			return fmt.Errorf("cluster configuration file must be provided via -f or --config flag for upgrade")
		}
		if upgradeOptions.TargetVersion == "" {
			return fmt.Errorf("target Kubernetes version must be provided via --version flag for upgrade")
		}

		absPath, err := filepath.Abs(upgradeOptions.ClusterConfigFile)
		if err != nil {
			log.Error(err, "Failed to get absolute path for config file")
			return fmt.Errorf("failed to get absolute path for config file %s: %w", upgradeOptions.ClusterConfigFile, err)
		}
		log.Infof("Loading cluster configuration from: %s for upgrade to version %s.", absPath, upgradeOptions.TargetVersion)

		clusterConfig, err := config.ParseFromFile(absPath)
		if err != nil {
			log.Error(err, "Failed to load cluster configuration")
			return fmt.Errorf("failed to load cluster configuration from %s: %w", absPath, err)
		}
		log.Infof("Configuration loaded for cluster: %s", clusterConfig.Name)

		// Apply CLI overrides if any (e.g., target version if it can also be in config)
		// For now, TargetVersion from flag is primary.

		if !assumeYesGlobal {
			fmt.Printf("WARNING: This action will attempt to upgrade the Kubernetes cluster '%s' to version '%s'.\n", clusterConfig.Name, upgradeOptions.TargetVersion)
			fmt.Print("Ensure you have backed up your cluster and reviewed upgrade compatibility. Proceed? (yes/no): ")
			reader := bufio.NewReader(os.Stdin)
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(strings.ToLower(input))
			if input != "yes" {
				log.Info("Cluster upgrade aborted by user.")
				return nil
			}
		}
		log.Info("Proceeding with cluster upgrade...")

		goCtx, cancel := context.WithTimeout(context.Background(), 60*time.Minute) // Longer timeout for upgrade
		defer cancel()

		connectorFactory := connector.NewDefaultFactory()
		connectionPool := connector.NewConnectionPool(connector.DefaultPoolConfig())
		runnerSvc := runner.New()
		engineSvc := engine.NewExecutor()

		rtBuilder := runtime.NewRuntimeBuilderFromConfig(clusterConfig, runnerSvc, connectionPool, connectorFactory)
		log.Info("Building runtime environment for upgrade...")
		runtimeCtx, cleanupFunc, err := rtBuilder.Build(goCtx, engineSvc)
		if err != nil {
			log.Error(err, "Failed to build runtime environment for upgrade")
			return fmt.Errorf("failed to build runtime environment for upgrade: %w", err)
		}
		defer cleanupFunc()
		log.Info("Runtime environment built successfully for upgrade.")

		// Pass TargetVersion and assumeYesGlobal to the pipeline constructor
		upgradePipeline := kubexmcluster.NewUpgradeClusterPipeline(upgradeOptions.TargetVersion, assumeYesGlobal)
		log.Infof("Instantiated pipeline: %s", upgradePipeline.Name())

		log.Info("Executing upgrade pipeline run...")
		result, err := upgradePipeline.Run(runtimeCtx, nil, dryRunGlobal) // Pass dryRunGlobal
		if err != nil {
			log.Error(err, "Cluster upgrade pipeline failed")
			if result != nil {
				log.Infof("Pipeline result status during failure: %s", result.Status)
			}
			return fmt.Errorf("cluster upgrade pipeline execution failed: %w", err)
		}

		if result.Status == plan.StatusFailed {
			log.Error(nil, "Cluster upgrade pipeline reported failure.", "status", result.Status)
			return fmt.Errorf("cluster upgrade pipeline failed with status: %s", result.Status)
		}

		log.Infof("Cluster upgrade pipeline completed successfully! Status: %s", result.Status)
		return nil
	},
}

type UpgradeOptions struct {
	ClusterConfigFile string
	TargetVersion     string
	// DryRun can be a global flag
}

var upgradeOptions = &UpgradeOptions{}

func init() {
	ClusterCmd.AddCommand(upgradeCmd)
	upgradeCmd.Flags().StringVarP(&upgradeOptions.ClusterConfigFile, "config", "f", "", "Path to the cluster configuration YAML file (required for context, may not be modified)")
	upgradeCmd.Flags().StringVarP(&upgradeOptions.TargetVersion, "version", "v", "", "Target Kubernetes version for the upgrade (e.g., v1.24.3) (required)")
	// Add --dry-run as a local flag if not relying on a global one for specific command behavior
	// upgradeCmd.Flags().Bool("dry-run", false, "Simulate the cluster upgrade")


	if err := upgradeCmd.MarkFlagRequired("config"); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to mark 'config' flag as required for upgrade command: %v\n", err)
	}
	if err := upgradeCmd.MarkFlagRequired("version"); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to mark 'version' flag as required for upgrade command: %v\n", err)
	}
}
