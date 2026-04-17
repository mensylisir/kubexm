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

type BackupOptions struct {
	ClusterConfigFile string
	BackupType        string
	OutputPath        string
	DryRun            bool
}

var backupOptions = &BackupOptions{}

func init() {
	ClusterCmd.AddCommand(backupCmd)
	backupCmd.Flags().StringVarP(&backupOptions.ClusterConfigFile, "config", "f", "", "Path to the cluster configuration YAML file (required)")
	backupCmd.Flags().StringVarP(&backupOptions.BackupType, "type", "t", "all", "Backup type: all, pki, etcd, kubernetes (default: all)")
	backupCmd.Flags().StringVarP(&backupOptions.OutputPath, "output", "o", "", "Output path for backup (default: uses cluster work dir)")
	backupCmd.Flags().BoolVar(&backupOptions.DryRun, "dry-run", false, "Simulate the backup without making changes")

	if err := backupCmd.MarkFlagRequired("config"); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to mark 'config' flag as required for backup command: %v\n", err)
	}
}

var backupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Backup cluster data and certificates",
	Long: `Backup Kubernetes cluster data and certificates.

Backup types:
  all         - Backup all (PKI, Etcd, Kubernetes configs) (default)
  pki         - Backup only PKI certificates and keys
  etcd        - Backup only etcd data
  kubernetes  - Backup only Kubernetes configurations

Examples:
  # Backup all cluster data
  kubexm cluster backup -f config.yaml

  # Backup only PKI
  kubexm cluster backup -f config.yaml --type pki

  # Dry run to preview
  kubexm cluster backup -f config.yaml --dry-run`,
	RunE: func(cmd *cobra.Command, args []string) error {
		log := logger.Get()
		defer logger.SyncGlobal()

		assumeYesGlobal, _ := cmd.Flags().GetBool("yes")

		if backupOptions.ClusterConfigFile == "" {
			return fmt.Errorf("cluster configuration file must be provided via -f or --config flag")
		}

		absPath, err := filepath.Abs(backupOptions.ClusterConfigFile)
		if err != nil {
			return fmt.Errorf("failed to get absolute path for config file: %w", err)
		}

		clusterConfig, err := config.ParseFromFileWithOptions(absPath, config.ParseOptions{SkipHostValidation: true})
		if err != nil {
			return fmt.Errorf("failed to load cluster configuration: %w", err)
		}

		// Set output path as work dir if specified
		if backupOptions.OutputPath != "" {
			clusterConfig.Spec.Global.WorkDir = backupOptions.OutputPath
		}

		log.Infof("Starting backup for cluster '%s' (type: %s)", clusterConfig.Name, backupOptions.BackupType)

		if !assumeYesGlobal && !backupOptions.DryRun {
			fmt.Printf("WARNING: This will create a backup of cluster '%s' data.\n", clusterConfig.Name)
			fmt.Print("Proceed? (yes/no): ")
			reader := bufio.NewReader(os.Stdin)
			input, err := reader.ReadString('\n')
			if err != nil {
				input = "no"
			}
			input = strings.TrimSpace(strings.ToLower(input))
			if input != "yes" {
				log.Info("Backup aborted by user.")
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

		p := pipelinecluster.NewBackupPipeline(backupOptions.BackupType)
		graph, err := p.Plan(runtimeCtx)
		if err != nil {
			return fmt.Errorf("backup pipeline planning failed: %w", err)
		}

		result, err := p.Run(runtimeCtx, graph, backupOptions.DryRun)
		if err != nil {
			if result != nil && result.Status == plan.StatusFailed {
				return fmt.Errorf("backup failed: %s", result.Message)
			}
			return fmt.Errorf("backup pipeline execution failed: %w", err)
		}
		if result.Status == plan.StatusFailed {
			return fmt.Errorf("backup failed with status: %s. Message: %s", result.Status, result.Message)
		}

		log.Infof("Backup completed successfully. Status: %s", result.Status)
		return nil
	},
}
