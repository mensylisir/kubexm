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

type RestoreOptions struct {
	ClusterConfigFile string
	BackupPath       string
	RestoreType      string
	SnapshotPath     string
	DryRun           bool
}

var restoreOptions = &RestoreOptions{}

func init() {
	ClusterCmd.AddCommand(restoreCmd)
	restoreCmd.Flags().StringVarP(&restoreOptions.ClusterConfigFile, "config", "f", "", "Path to the cluster configuration YAML file (required)")
	restoreCmd.Flags().StringVarP(&restoreOptions.BackupPath, "backup", "b", "", "Path to backup archive to restore from (required)")
	restoreCmd.Flags().StringVarP(&restoreOptions.RestoreType, "type", "t", "all", "Restore type: all, pki, etcd, kubernetes (default: all)")
	restoreCmd.Flags().StringVarP(&restoreOptions.SnapshotPath, "snapshot", "s", "", "Path to etcd snapshot file (required for etcd restore type)")
	restoreCmd.Flags().BoolVar(&restoreOptions.DryRun, "dry-run", false, "Simulate the restore without making changes")

	if err := restoreCmd.MarkFlagRequired("config"); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to mark 'config' flag as required for restore command: %v\n", err)
	}
	if err := restoreCmd.MarkFlagRequired("backup"); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to mark 'backup' flag as required for restore command: %v\n", err)
	}
}

var restoreCmd = &cobra.Command{
	Use:   "restore",
	Short: "Restore cluster from backup",
	Long: `Restore Kubernetes cluster data from a backup.

WARNING: This operation may overwrite existing data and could cause service disruption.
It is recommended to stop the cluster workloads before restoring.

Restore types:
  all         - Restore all (PKI, Etcd, Kubernetes configs) (default)
  pki         - Restore only PKI certificates and keys
  etcd        - Restore only etcd data (requires --snapshot flag)
  kubernetes  - Restore only Kubernetes configurations

Examples:
  # Restore all from backup
  kubexm cluster restore -f config.yaml -b /path/to/backup

  # Restore only PKI
  kubexm cluster restore -f config.yaml -b /path/to/backup --type pki

  # Restore etcd from a specific snapshot
  kubexm cluster restore -f config.yaml -b /path/to/backup --type etcd -s /path/to/snapshot.db`,
	RunE: func(cmd *cobra.Command, args []string) error {
		log := logger.Get()
		defer logger.SyncGlobal()

		assumeYesGlobal, _ := cmd.Flags().GetBool("yes")

		if restoreOptions.ClusterConfigFile == "" {
			return fmt.Errorf("cluster configuration file must be provided via -f or --config flag")
		}
		if restoreOptions.BackupPath == "" {
			return fmt.Errorf("backup path must be provided via -b or --backup flag")
		}

		absConfigPath, err := filepath.Abs(restoreOptions.ClusterConfigFile)
		if err != nil {
			return fmt.Errorf("failed to get absolute path for config file: %w", err)
		}

		absBackupPath, err := filepath.Abs(restoreOptions.BackupPath)
		if err != nil {
			return fmt.Errorf("failed to get absolute path for backup file: %w", err)
		}

		if _, err := os.Stat(absBackupPath); os.IsNotExist(err) {
			return fmt.Errorf("backup file does not exist: %s", absBackupPath)
		}

		clusterConfig, err := config.ParseFromFileWithOptions(absConfigPath, config.ParseOptions{SkipHostValidation: true})
		if err != nil {
			return fmt.Errorf("failed to load cluster configuration: %w", err)
		}

		log.Infof("Starting restore for cluster '%s' from backup '%s' (type: %s)", clusterConfig.Name, absBackupPath, restoreOptions.RestoreType)

		if !assumeYesGlobal && !restoreOptions.DryRun {
			fmt.Printf("WARNING: This will RESTORE cluster '%s' from backup '%s'.\n", clusterConfig.Name, absBackupPath)
			fmt.Printf("This operation may overwrite existing data and could cause service disruption.\n")
			fmt.Print("Are you sure you want to proceed? (yes/no): ")
			reader := bufio.NewReader(os.Stdin)
			input, err := reader.ReadString('\n')
			if err != nil {
				input = "no"
			}
			input = strings.TrimSpace(strings.ToLower(input))
			if input != "yes" {
				log.Info("Restore aborted by user.")
				return nil
			}
		}

		// Set backup path as work dir
		clusterConfig.Spec.Global.WorkDir = absBackupPath

		goCtx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
		defer cancel()

		rtBuilder := runtime.NewBuilderFromConfig(clusterConfig).
			WithSkipHostConnect(true).
			WithSkipConfigValidation(true)
		runtimeCtx, cleanupFunc, err := rtBuilder.Build(goCtx)
		if err != nil {
			return fmt.Errorf("failed to build runtime environment: %w", err)
		}
		defer cleanupFunc()

		p := pipelinecluster.NewRestorePipeline(restoreOptions.RestoreType, restoreOptions.SnapshotPath, assumeYesGlobal)
		graph, err := p.Plan(runtimeCtx)
		if err != nil {
			return fmt.Errorf("restore pipeline planning failed: %w", err)
		}

		result, err := p.Run(runtimeCtx, graph, restoreOptions.DryRun)
		if err != nil {
			if result != nil && result.Status == plan.StatusFailed {
				return fmt.Errorf("restore failed: %s", result.Message)
			}
			return fmt.Errorf("restore pipeline execution failed: %w", err)
		}
		if result.Status == plan.StatusFailed {
			return fmt.Errorf("restore failed with status: %s. Message: %s", result.Status, result.Message)
		}

		log.Infof("Restore completed successfully. Status: %s", result.Status)
		return nil
	},
}
