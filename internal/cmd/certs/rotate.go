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

// RotateOptions holds options for the rotate certificates command
type RotateOptions struct {
	ClusterConfigFile string
	CertType          string
	ServiceName       string
	Force             bool
	DryRun            bool
}

var rotateOptions = &RotateOptions{}

func init() {
	CertsCmd.AddCommand(rotateCmd)
	rotateCmd.Flags().StringVarP(&rotateOptions.ClusterConfigFile, "config", "f", "", "Path to the cluster configuration YAML file (required)")
	rotateCmd.Flags().StringVarP(&rotateOptions.CertType, "type", "t", "all", "Certificate type to rotate: kubernetes-ca, etcd-ca, kubernetes-certs, etcd-certs, all (default: all)")
	rotateCmd.Flags().StringVar(&rotateOptions.ServiceName, "service", "", "Service name for targeted certificate rotation (e.g., 'apiserver', 'etcd', 'kubelet')")
	rotateCmd.Flags().BoolVar(&rotateOptions.Force, "force", false, "Force rotation without confirmation")
	rotateCmd.Flags().BoolVar(&rotateOptions.DryRun, "dry-run", false, "Simulate the certificate rotation without making changes")

	if err := rotateCmd.MarkFlagRequired("config"); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to mark 'config' flag as required: %v\n", err)
	}
}

var rotateCmd = &cobra.Command{
	Use:   "rotate",
	Short: "Rotate certificates for a service or all services in a cluster",
	Long: `Rotate PKI certificates for specified services or all components within a Kubernetes cluster.

Certificate rotation involves generating new certificates, distributing them to relevant nodes,
updating component configurations, and restarting components in the correct order to maintain
cluster availability.

Certificate types:
  kubernetes-ca    - Rotate Kubernetes CA certificate and key
  etcd-ca         - Rotate etcd CA certificate and key
  kubernetes-certs - Rotate all Kubernetes component certificates
  etcd-certs      - Rotate all etcd certificates
  all             - Rotate all certificates (default)

Service names (for targeted rotation):
  apiserver       - Rotate kube-apiserver certificates
  etcd            - Rotate etcd certificates
  kubelet         - Rotate kubelet certificates on worker nodes
  controller      - Rotate kube-controller-manager certificates
  scheduler       - Rotate kube-scheduler certificates
  front-proxy     - Rotate front-proxy certificates

Examples:
  # Rotate all certificates
  kubexm certs rotate --config cluster-config.yaml --type all

  # Rotate only etcd certificates
  kubexm certs rotate --config cluster-config.yaml --type etcd-certs

  # Rotate apiserver certificates
  kubexm certs rotate --config cluster-config.yaml --service apiserver

  # Dry run to see what would be rotated
  kubexm certs rotate --config cluster-config.yaml --type all --dry-run`,
	RunE: func(cmd *cobra.Command, args []string) error {
		log := logger.Get()
		defer logger.SyncGlobal()

		assumeYesGlobal, _ := cmd.Flags().GetBool("yes")

		log.Info("Starting certificate rotation process...")

		if rotateOptions.ClusterConfigFile == "" {
			return fmt.Errorf("cluster configuration file must be provided via -f or --config flag")
		}

		certType := rotateOptions.CertType

		absPath, err := filepath.Abs(rotateOptions.ClusterConfigFile)
		if err != nil {
			log.With("error", err).Error("Failed to get absolute path for config file")
			return fmt.Errorf("failed to get absolute path for config file %s: %w", rotateOptions.ClusterConfigFile, err)
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

		if !rotateOptions.Force && !assumeYesGlobal {
			fmt.Printf("WARNING: This action will rotate certificate(s) of type '%s' in cluster '%s'.\n", certType, clusterConfig.Name)
			if rotateOptions.ServiceName != "" {
				fmt.Printf("Target service: %s\n", rotateOptions.ServiceName)
			}
			fmt.Printf("This operation may require cluster restart and could cause temporary disruption.\n")
			fmt.Print("Are you sure you want to proceed? (yes/no): ")
			reader := bufio.NewReader(os.Stdin)
			input, err := reader.ReadString('\n')
			if err != nil {
				input = "no"
			}
			input = strings.TrimSpace(strings.ToLower(input))
			if input != "yes" {
				log.Info("Certificate rotation aborted by user.")
				return nil
			}
		}
		log.Info("Proceeding with certificate rotation...")

		goCtx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
		defer cancel()

		rtBuilder := runtime.NewBuilderFromConfig(clusterConfig)
		log.Info("Building runtime environment for certificate rotation...")
		runtimeCtx, cleanupFunc, err := rtBuilder.Build(goCtx)
		if err != nil {
			log.With("error", err).Error("Failed to build runtime environment")
			return fmt.Errorf("failed to build runtime environment for certificate rotation: %w", err)
		}
		defer cleanupFunc()
		log.Info("Runtime environment built successfully for certificate rotation.")

		// Select the appropriate pipeline based on certificate type
		var rotatePipeline pipeline.Pipeline
		switch certType {
		case "all":
			rotatePipeline = cluster.NewRenewAllPipeline(assumeYesGlobal)
		case "kubernetes-ca":
			rotatePipeline = cluster.NewRenewKubernetesCAPipeline(assumeYesGlobal)
		case "kubernetes-certs":
			rotatePipeline = cluster.NewRenewKubernetesLeafPipeline(assumeYesGlobal)
		case "etcd-ca":
			rotatePipeline = cluster.NewRenewEtcdCAPipeline(assumeYesGlobal)
		case "etcd-certs":
			rotatePipeline = cluster.NewRenewEtcdLeafPipeline(assumeYesGlobal)
		default:
			rotatePipeline = cluster.NewRenewAllPipeline(assumeYesGlobal)
		}
		log.Infof("Instantiated pipeline: %s", rotatePipeline.Name())

		log.Info("Executing certificate rotation pipeline run...")
		result, err := rotatePipeline.Run(runtimeCtx, nil, rotateOptions.DryRun)
		if err != nil {
			log.With("error", err).Error("Certificate rotation pipeline failed")
			if result != nil {
				log.Infof("Pipeline result status during failure: %s", result.Status)
			}
			return fmt.Errorf("certificate rotation pipeline execution failed: %w", err)
		}

		if result.Status == plan.StatusFailed {
			log.With("status", result.Status).Error("Certificate rotation pipeline reported failure.")
			return fmt.Errorf("certificate rotation pipeline failed with status: %s", result.Status)
		}

		log.Infof("Certificate rotation pipeline completed successfully! Status: %s", result.Status)
		return nil
	},
}