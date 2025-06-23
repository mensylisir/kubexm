package cluster

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mensylisir/kubexm/pkg/common" // For default directory names
	"github.com/spf13/cobra"
)

// KubeconfigOptions holds options for the kubeconfig command
type KubeconfigOptions struct {
	OutputPath string
	Raw        bool // If true, prints raw content without extra messages
}

var kubeconfigOptions = &KubeconfigOptions{}

// defaultKubeconfigFileName is the conventional name for the admin kubeconfig file.
const defaultKubeconfigFileName = "admin.kubeconfig"

func init() {
	ClusterCmd.AddCommand(kubeconfigCmd)
	kubeconfigCmd.Flags().StringVarP(&kubeconfigOptions.OutputPath, "output", "o", "", "Path to save the kubeconfig file. If empty, prints to stdout.")
	kubeconfigCmd.Flags().BoolVar(&kubeconfigOptions.Raw, "raw", false, "Print only raw kubeconfig content to stdout (effective if --output is not specified).")
}

var kubeconfigCmd = &cobra.Command{
	Use:   "kubeconfig [CLUSTER_NAME]",
	Short: "Get the kubeconfig file for a cluster",
	Long: `Retrieves the administrative kubeconfig file for a specified Kubernetes cluster.
This file is typically generated during the cluster creation process.`,
	Args: cobra.ExactArgs(1), // Requires exactly one argument: cluster_name
	RunE: func(cmd *cobra.Command, args []string) error {
		clusterName := args[0]

		baseClustersDir, err := clustersBaseDir() // Using the same helper as 'list' and 'get'
		if err != nil {
			return fmt.Errorf("could not determine clusters base directory: %w", err)
		}

		// Path to the kubeconfig file within the specific cluster's artifact directory
		kubeconfigPath := filepath.Join(baseClustersDir, clusterName, defaultKubeconfigFileName)

		if _, err := os.Stat(kubeconfigPath); os.IsNotExist(err) {
			return fmt.Errorf("kubeconfig file not found for cluster '%s' at %s. Was the cluster created successfully?", clusterName, kubeconfigPath)
		}

		kubeconfigBytes, err := os.ReadFile(kubeconfigPath)
		if err != nil {
			return fmt.Errorf("failed to read kubeconfig file %s: %w", kubeconfigPath, err)
		}

		if kubeconfigOptions.OutputPath != "" {
			// Ensure the output directory exists if a full path is given
			outputDir := filepath.Dir(kubeconfigOptions.OutputPath)
			if _, err := os.Stat(outputDir); os.IsNotExist(err) {
				if err := os.MkdirAll(outputDir, 0755); err != nil {
					return fmt.Errorf("failed to create output directory %s: %w", outputDir, err)
				}
			}

			err = os.WriteFile(kubeconfigOptions.OutputPath, kubeconfigBytes, 0600) // Secure permissions for kubeconfig
			if err != nil {
				return fmt.Errorf("failed to write kubeconfig to %s: %w", kubeconfigOptions.OutputPath, err)
			}
			if !kubeconfigOptions.Raw { // Avoid double printing if raw is also somehow true
				fmt.Printf("Kubeconfig for cluster '%s' saved to %s\n", clusterName, kubeconfigOptions.OutputPath)
			}
		} else {
			// Print to stdout
			if !kubeconfigOptions.Raw {
				fmt.Printf("# Kubeconfig for cluster '%s'\n# Path: %s\n\n", clusterName, kubeconfigPath)
			}
			fmt.Print(string(kubeconfigBytes))
		}

		return nil
	},
}
