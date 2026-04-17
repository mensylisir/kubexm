package certs

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
	"github.com/mensylisir/kubexm/internal/pipeline"
	"github.com/mensylisir/kubexm/internal/pipeline/cluster"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/spf13/cobra"
)

// RenewOptions holds options for certificate renewal
type RenewOptions struct {
	ClusterConfigFile string
	CertType          string // kubernetes-ca, etcd-ca, kubernetes-certs, etcd-certs, all
	Force             bool
	DryRun            bool
}

var renewOptions = &RenewOptions{}

func init() {
	CertsCmd.AddCommand(renewCmd)
	renewCmd.Flags().StringVarP(&renewOptions.ClusterConfigFile, "config", "f", "", "Path to the cluster configuration YAML file (required)")
	renewCmd.Flags().StringVarP(&renewOptions.CertType, "type", "t", "all", "Certificate type to renew: kubernetes-ca, etcd-ca, kubernetes-certs, etcd-certs, all (default: all)")
	renewCmd.Flags().BoolVar(&renewOptions.Force, "force", false, "Force renewal without confirmation")
	renewCmd.Flags().BoolVar(&renewOptions.DryRun, "dry-run", false, "Simulate the certificate renewal without making changes")

	if err := renewCmd.MarkFlagRequired("config"); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to mark 'config' flag as required: %v\n", err)
	}
}

var renewCmd = &cobra.Command{
	Use:   "renew",
	Short: "Renew cluster certificates",
	Long: `Renew certificates for Kubernetes cluster components.

Certificate types:
  kubernetes-ca    - Renew Kubernetes CA certificate and key
  etcd-ca         - Renew etcd CA certificate and key
  kubernetes-certs - Renew all Kubernetes component certificates (apiserver, controller-manager, scheduler, kubelet)
  etcd-certs      - Renew all etcd certificates
  all             - Renew all certificates (default)

Examples:
  # Renew all certificates
  kubexm certs renew --cluster my-cluster --type all

  # Renew only Kubernetes CA
  kubexm certs renew --cluster my-cluster --type kubernetes-ca

  # Dry run to see what would be renewed
  kubexm certs renew --cluster my-cluster --type all --dry-run`,
	RunE: func(cmd *cobra.Command, args []string) error {
		log := logger.Get()
		defer logger.SyncGlobal()

		assumeYesGlobal, _ := cmd.Flags().GetBool("yes")

		log.Info("Starting certificate renewal process...")

		if renewOptions.ClusterConfigFile == "" {
			return fmt.Errorf("cluster configuration file must be provided via -f or --config flag")
		}

		absPath, err := filepath.Abs(renewOptions.ClusterConfigFile)
		if err != nil {
			log.With("error", err).Error("Failed to get absolute path for config file")
			return fmt.Errorf("failed to get absolute path for config file %s: %w", renewOptions.ClusterConfigFile, err)
		}

		clusterConfig, err := config.ParseFromFile(absPath)
		if err != nil {
			log.With("error", err).Error("Failed to load cluster configuration")
			return fmt.Errorf("failed to load cluster configuration from %s: %w", absPath, err)
		}
		log.Infof("Configuration loaded for cluster: %s", clusterConfig.Name)

		validCertTypes := []string{"kubernetes-ca", "etcd-ca", "kubernetes-certs", "etcd-certs", "all"}
		if !isValidCertType(renewOptions.CertType, validCertTypes) {
			return fmt.Errorf("invalid certificate type: %s. Valid types are: %s", renewOptions.CertType, strings.Join(validCertTypes, ", "))
		}

		if !renewOptions.Force && !assumeYesGlobal {
			fmt.Printf("WARNING: This action will renew certificate(s) of type '%s' in cluster '%s'.\n", renewOptions.CertType, clusterConfig.Name)
			fmt.Printf("This operation may require cluster restart and could cause temporary disruption.\n")
			fmt.Print("Are you sure you want to proceed? (yes/no): ")
			reader := bufio.NewReader(os.Stdin)
			input, err := reader.ReadString('\n')
			if err != nil {
				input = "no"
			}
			input = strings.TrimSpace(strings.ToLower(input))
			if input != "yes" {
				log.Info("Certificate renewal aborted by user.")
				return nil
			}
		}
		log.Info("Proceeding with certificate renewal...")

		goCtx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
		defer cancel()

		rtBuilder := runtime.NewBuilderFromConfig(clusterConfig)
		log.Info("Building runtime environment for certificate renewal...")
		runtimeCtx, cleanupFunc, err := rtBuilder.Build(goCtx)
		if err != nil {
			log.With("error", err).Error("Failed to build runtime environment for certificate renewal")
			return fmt.Errorf("failed to build runtime environment for certificate renewal: %w", err)
		}
		defer cleanupFunc()
		log.Info("Runtime environment built successfully for certificate renewal.")

		// Select the appropriate pipeline based on certificate type
		var renewPipeline pipeline.Pipeline
		switch renewOptions.CertType {
		case "all":
			renewPipeline = cluster.NewRenewAllPipeline(assumeYesGlobal)
		case "kubernetes-ca":
			renewPipeline = cluster.NewRenewKubernetesCAPipeline(assumeYesGlobal)
		case "kubernetes-certs":
			renewPipeline = cluster.NewRenewKubernetesLeafPipeline(assumeYesGlobal)
		case "etcd-ca":
			renewPipeline = cluster.NewRenewEtcdCAPipeline(assumeYesGlobal)
		case "etcd-certs":
			renewPipeline = cluster.NewRenewEtcdLeafPipeline(assumeYesGlobal)
		default:
			renewPipeline = cluster.NewRenewAllPipeline(assumeYesGlobal)
		}
		log.Infof("Instantiated pipeline: %s", renewPipeline.Name())

		log.Info("Executing certificate renewal pipeline run...")
		result, err := renewPipeline.Run(runtimeCtx, nil, renewOptions.DryRun)
		if err != nil {
			log.With("error", err).Error("Certificate renewal pipeline failed")
			if result != nil {
				log.Infof("Pipeline result status during failure: %s", result.Status)
			}
			return fmt.Errorf("certificate renewal pipeline execution failed: %w", err)
		}

		if result.Status == plan.StatusFailed {
			log.With("status", result.Status).Error("Certificate renewal pipeline reported failure.")
			return fmt.Errorf("certificate renewal pipeline failed with status: %s", result.Status)
		}

		log.Infof("Certificate renewal pipeline completed successfully! Status: %s", result.Status)
		return nil
	},
}

func isValidCertType(certType string, validTypes []string) bool {
	for _, vt := range validTypes {
		if certType == vt {
			return true
		}
	}
	return false
}
