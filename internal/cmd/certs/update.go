package certs

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
	"github.com/mensylisir/kubexm/internal/pipeline"
	"github.com/mensylisir/kubexm/internal/pipeline/cluster"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
)

// UpdateCertOptions holds options for the update certificates command
type UpdateCertOptions struct {
	ClusterConfigFile string
	CertType         string
	Force            bool
	DryRun           bool
}

var updateCertOptions = &UpdateCertOptions{}

func init() {
	CertsCmd.AddCommand(updateCertCmd)
	updateCertCmd.Flags().StringVarP(&updateCertOptions.ClusterConfigFile, "config", "f", "", "Path to the cluster configuration YAML file (required)")
	updateCertCmd.Flags().StringVarP(&updateCertOptions.CertType, "type", "t", "all", "Certificate type to update: kubernetes-ca, etcd-ca, kubernetes-certs, etcd-certs, all (default: all)")
	updateCertCmd.Flags().BoolVar(&updateCertOptions.Force, "force", false, "Force update without confirmation")
	updateCertCmd.Flags().BoolVar(&updateCertOptions.DryRun, "dry-run", false, "Simulate the certificate update without making changes")

	if err := updateCertCmd.MarkFlagRequired("config"); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to mark 'config' flag as required: %v\n", err)
	}
}

var updateCertCmd = &cobra.Command{
	Use:   "update [certificate-name]",
	Short: "Update or rotate certificates in the cluster",
	Long: `Update or rotate certificates within the KubeXM-managed PKI.

This command is similar to 'renew' but is designed for more aggressive updates
such as changing certificate parameters or force-rotating certificates.

Certificate types:
  kubernetes-ca    - Update Kubernetes CA certificate and key
  etcd-ca         - Update etcd CA certificate and key
  kubernetes-certs - Update all Kubernetes component certificates
  etcd-certs      - Update all etcd certificates
  all             - Update all certificates (default)

Examples:
  # Update all certificates
  kubexm certs update --cluster my-cluster --type all

  # Update only Kubernetes CA
  kubexm certs update --cluster my-cluster --type kubernetes-ca

  # Dry run to see what would be updated
  kubexm certs update --cluster my-cluster --type all --dry-run`,
	RunE: func(cmd *cobra.Command, args []string) error {
		log := logger.Get()
		defer logger.SyncGlobal()

		assumeYesGlobal, _ := cmd.Flags().GetBool("yes")

		log.Info("Starting certificate update process...")

		if updateCertOptions.ClusterConfigFile == "" {
			return fmt.Errorf("cluster configuration file must be provided via -f or --config flag")
		}

		// Allow overriding cert type via positional arg
		certType := updateCertOptions.CertType
		if len(args) > 0 {
			certType = args[0]
		}

		absPath, err := filepath.Abs(updateCertOptions.ClusterConfigFile)
		if err != nil {
			log.With("error", err).Error("Failed to get absolute path for config file")
			return fmt.Errorf("failed to get absolute path for config file %s: %w", updateCertOptions.ClusterConfigFile, err)
		}

		clusterConfig, err := config.ParseFromFile(absPath)
		if err != nil {
			log.With("error", err).Error("Failed to load cluster configuration")
			return fmt.Errorf("failed to load cluster configuration from %s: %w", absPath, err)
		}
		log.Infof("Configuration loaded for cluster: %s", clusterConfig.Name)

		validCertTypes := []string{"kubernetes-ca", "etcd-ca", "kubernetes-certs", "etcd-certs", "all"}
		if !isValidCertType(certType, validCertTypes) {
			return fmt.Errorf("invalid certificate type: %s. Valid types are: %s", certType, strings.Join(validCertTypes, ", "))
		}

		if !updateCertOptions.Force && !assumeYesGlobal {
			fmt.Printf("WARNING: This action will update certificate(s) of type '%s' in cluster '%s'.\n", certType, clusterConfig.Name)
			fmt.Printf("This operation may require cluster restart and could cause temporary disruption.\n")
			fmt.Print("Are you sure you want to proceed? (yes/no): ")
			reader := bufio.NewReader(os.Stdin)
			input, err := reader.ReadString('\n')
			if err != nil {
				input = "no"
			}
			input = strings.TrimSpace(strings.ToLower(input))
			if input != "yes" {
				log.Info("Certificate update aborted by user.")
				return nil
			}
		}
		log.Info("Proceeding with certificate update...")

		goCtx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
		defer cancel()

		rtBuilder := runtime.NewBuilderFromConfig(clusterConfig)
		log.Info("Building runtime environment for certificate update...")
		runtimeCtx, cleanupFunc, err := rtBuilder.Build(goCtx)
		if err != nil {
			log.With("error", err).Error("Failed to build runtime environment")
			return fmt.Errorf("failed to build runtime environment for certificate update: %w", err)
		}
		defer cleanupFunc()
		log.Info("Runtime environment built successfully for certificate update.")

		// Select the appropriate pipeline based on certificate type
		var updatePipeline pipeline.Pipeline
		switch certType {
		case "all":
			updatePipeline = cluster.NewRenewAllPipeline(assumeYesGlobal)
		case "kubernetes-ca":
			updatePipeline = cluster.NewRenewKubernetesCAPipeline(assumeYesGlobal)
		case "kubernetes-certs":
			updatePipeline = cluster.NewRenewKubernetesLeafPipeline(assumeYesGlobal)
		case "etcd-ca":
			updatePipeline = cluster.NewRenewEtcdCAPipeline(assumeYesGlobal)
		case "etcd-certs":
			updatePipeline = cluster.NewRenewEtcdLeafPipeline(assumeYesGlobal)
		default:
			updatePipeline = cluster.NewRenewAllPipeline(assumeYesGlobal)
		}
		log.Infof("Instantiated pipeline: %s", updatePipeline.Name())

		log.Info("Executing certificate update pipeline run...")
		result, err := updatePipeline.Run(runtimeCtx, nil, updateCertOptions.DryRun)
		if err != nil {
			log.With("error", err).Error("Certificate update pipeline failed")
			if result != nil {
				log.Infof("Pipeline result status during failure: %s", result.Status)
			}
			return fmt.Errorf("certificate update pipeline execution failed: %w", err)
		}

		if result.Status == plan.StatusFailed {
			log.With("status", result.Status).Error("Certificate update pipeline reported failure.")
			return fmt.Errorf("certificate update pipeline failed with status: %s", result.Status)
		}

		log.Infof("Certificate update pipeline completed successfully! Status: %s", result.Status)
		return nil
	},
}