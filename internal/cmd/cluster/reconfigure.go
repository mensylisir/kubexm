package cluster

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mensylisir/kubexm/internal/config"
	"github.com/mensylisir/kubexm/internal/logger"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	pipelinecluster "github.com/mensylisir/kubexm/internal/pipeline/cluster"
)

type ReconfigureOptions struct {
	ClusterConfigFile  string
	NewConfigFile      string
	Component          string
	RestartServices    bool
	BackupBeforeChange bool
	DryRun             bool
}

var reconfigureOptions = &ReconfigureOptions{}

func init() {
	ClusterCmd.AddCommand(reconfigureCmd)
	reconfigureCmd.Flags().StringVarP(&reconfigureOptions.ClusterConfigFile, "config", "f", "", "Path to current cluster configuration YAML file (required)")
	reconfigureCmd.Flags().StringVarP(&reconfigureOptions.NewConfigFile, "new-config", "n", "", "Path to new cluster configuration YAML file (required for full reconfigure)")
	reconfigureCmd.Flags().StringVarP(&reconfigureOptions.Component, "component", "c", "all", "Component to reconfigure: all, apiserver, scheduler, controller-manager, kubelet, proxy (default: all)")
	reconfigureCmd.Flags().BoolVar(&reconfigureOptions.RestartServices, "restart", true, "Restart affected services after reconfiguration (default: true)")
	reconfigureCmd.Flags().BoolVar(&reconfigureOptions.BackupBeforeChange, "backup", true, "Create backup before making changes (default: true)")
	reconfigureCmd.Flags().BoolVar(&reconfigureOptions.DryRun, "dry-run", false, "Show what would be changed without making changes")

	if err := reconfigureCmd.MarkFlagRequired("config"); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to mark 'config' flag as required for reconfigure command: %v\n", err)
	}
}

var reconfigureCmd = &cobra.Command{
	Use:   "reconfigure",
	Short: "Reconfigure cluster components",
	Long: `Reconfigure Kubernetes cluster components.

This command updates configuration for cluster components using the current
cluster configuration. For changes between configs, compare configs first.

Components:
  all                - Reconfigure all components (default)
  apiserver         - Reconfigure kube-apiserver
  scheduler         - Reconfigure kube-scheduler
  controller-manager - Reconfigure kube-controller-manager
  kubelet           - Reconfigure kubelet on nodes
  proxy             - Reconfigure kube-proxy

Examples:
  # Reconfigure all components
  kubexm cluster reconfigure -f config.yaml

  # Reconfigure only kubelet
  kubexm cluster reconfigure -f config.yaml -c kubelet

  # Dry run to see changes
  kubexm cluster reconfigure -f config.yaml --dry-run`,
	RunE: func(cmd *cobra.Command, args []string) error {
		log := logger.Get()
		defer logger.SyncGlobal()

		assumeYesGlobal, _ := cmd.Flags().GetBool("yes")

		if reconfigureOptions.ClusterConfigFile == "" {
			return fmt.Errorf("cluster configuration file must be provided via -f or --config flag")
		}

		absPath, err := filepath.Abs(reconfigureOptions.ClusterConfigFile)
		if err != nil {
			return fmt.Errorf("failed to get absolute path for current config file: %w", err)
		}

		clusterConfig, err := config.ParseFromFileWithOptions(absPath, config.ParseOptions{SkipHostValidation: true})
		if err != nil {
			return fmt.Errorf("failed to load current cluster configuration: %w", err)
		}

		log.Infof("Starting reconfiguration for cluster '%s' (component: %s)", clusterConfig.Name, reconfigureOptions.Component)

		if !assumeYesGlobal && !reconfigureOptions.DryRun {
			fmt.Printf("WARNING: This will reconfigure components in cluster '%s'.\n", clusterConfig.Name)
			fmt.Printf("This may cause temporary service disruption.\n")
			fmt.Print("Are you sure you want to proceed? (yes/no): ")
			reader := bufio.NewReader(os.Stdin)
			input, err := reader.ReadString('\n')
			if err != nil {
				input = "no"
			}
			input = strings.TrimSpace(strings.ToLower(input))
			if input != "yes" {
				log.Info("Reconfiguration aborted by user.")
				return nil
			}
		}

		goCtx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()

		rtBuilder := runtime.NewBuilderFromConfig(clusterConfig).
			WithSkipHostConnect(true).
			WithSkipConfigValidation(true)
		runtimeCtx, cleanupFunc, err := rtBuilder.Build(goCtx)
		if err != nil {
			return fmt.Errorf("failed to build runtime environment: %w", err)
		}
		defer cleanupFunc()

		p := pipelinecluster.NewReconfigurePipeline(reconfigureOptions.Component, assumeYesGlobal)
		graph, err := p.Plan(runtimeCtx)
		if err != nil {
			return fmt.Errorf("reconfigure pipeline planning failed: %w", err)
		}

		result, err := p.Run(runtimeCtx, graph, reconfigureOptions.DryRun)
		if err != nil {
			if result != nil && result.Status == plan.StatusFailed {
				return fmt.Errorf("reconfigure failed: %s", result.Message)
			}
			return fmt.Errorf("reconfigure pipeline execution failed: %w", err)
		}
		if result.Status == plan.StatusFailed {
			return fmt.Errorf("reconfigure failed with status: %s. Message: %s", result.Status, result.Message)
		}

		log.Infof("Reconfigure completed successfully. Status: %s", result.Status)
		return nil
	},
}
