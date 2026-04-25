package cluster

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mensylisir/kubexm/internal/logger"
	"github.com/mensylisir/kubexm/internal/plan"
	kubexmcluster "github.com/mensylisir/kubexm/internal/pipeline/cluster"
	"github.com/mensylisir/kubexm/internal/runtime"
)

type DeleteClusterOptions struct {
	ClusterName string
	Force     bool
	DryRun    bool
}

var deleteClusterOpts = &DeleteClusterOptions{}

func init() {
	DeleteClusterCmd.Flags().StringVarP(&deleteClusterOpts.ClusterName, "name", "n", "", "Cluster name (required)")
	DeleteClusterCmd.Flags().BoolVar(&deleteClusterOpts.Force, "force", false, "Force deletion without confirmation")
	DeleteClusterCmd.Flags().BoolVar(&deleteClusterOpts.DryRun, "dry-run", false, "Simulate without making changes")
	DeleteClusterCmd.MarkFlagRequired("name")
}

// DeleteClusterCmd - kubexm delete cluster --name=xxx
var DeleteClusterCmd = &cobra.Command{
	Use:   "cluster",
	Short: "Delete a Kubernetes cluster",
	Long:  `Delete a Kubernetes cluster. This operation is destructive and will remove cluster components and data.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		log := logger.Get()
		defer logger.SyncGlobal()

		assumeYesGlobal, _ := cmd.Flags().GetBool("yes")

		log.Info("Starting cluster deletion process...")

		if deleteClusterOpts.ClusterName == "" {
			return fmt.Errorf("cluster name must be provided via --name or -n flag")
		}

		clusterConfig, err := LoadClusterConfig(deleteClusterOpts.ClusterName)
		if err != nil {
			return fmt.Errorf("failed to load cluster configuration: %w", err)
		}
		log.Infof("Configuration loaded for cluster: %s", clusterConfig.Name)

		if !assumeYesGlobal && !deleteClusterOpts.Force {
			fmt.Printf("WARNING: This will delete the Kubernetes cluster '%s'.\n", clusterConfig.Name)
			fmt.Print("Are you sure? (yes/no): ")
			reader := bufio.NewReader(os.Stdin)
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(strings.ToLower(input))
			if input != "yes" {
				log.Info("Cluster deletion aborted.")
				return nil
			}
		}

		log.Info("Proceeding with cluster deletion...")

		goCtx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()

		rtBuilder := runtime.NewBuilderFromConfig(clusterConfig)
		runtimeCtx, cleanupFunc, err := rtBuilder.Build(goCtx)
		if err != nil {
			return fmt.Errorf("failed to build runtime environment: %w", err)
		}
		defer cleanupFunc()

		deletePipeline := kubexmcluster.NewDeleteClusterPipeline(assumeYesGlobal)
		result, err := deletePipeline.Run(runtimeCtx, nil, deleteClusterOpts.DryRun)
		if err != nil {
			return fmt.Errorf("cluster deletion failed: %w", err)
		}

		if result.Status == plan.StatusFailed {
			return fmt.Errorf("cluster deletion failed: %s", result.Status)
		}

		log.Infof("Cluster deletion completed successfully! Status: %s", result.Status)
		return nil
	},
}
