package cluster

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mensylisir/kubexm/pkg/common" // For default directory names
	"github.com/mensylisir/kubexm/pkg/config"
	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml" // For pretty printing the config
)

// GetOptions holds options for the get cluster command
type GetOptions struct {
	// OutputFormat string // Future: "yaml", "json", "summary"
}

var getOptions = &GetOptions{}

// clusterConfigFileName defines the conventional name for the stored cluster config.
const clusterConfigFileName = "kubexm-cluster-config.yaml"

func init() {
	ClusterCmd.AddCommand(getCmd)
	// getCmd.Flags().StringVarP(&getOptions.OutputFormat, "output", "o", "summary", "Output format. One of: summary|yaml|json.")
}

var getCmd = &cobra.Command{
	Use:   "get [CLUSTER_NAME]",
	Short: "Get detailed information about a specific Kubernetes cluster",
	Long:  `Retrieves and displays detailed information about a Kubernetes cluster managed by kubexm.`,
	Args:  cobra.ExactArgs(1), // Requires exactly one argument: cluster_name
	RunE: func(cmd *cobra.Command, args []string) error {
		clusterName := args[0]

		baseClustersDir, err := clustersBaseDir() // Using the same helper as 'list'
		if err != nil {
			return fmt.Errorf("could not determine clusters base directory: %w", err)
		}

		clusterDir := filepath.Join(baseClustersDir, clusterName)
		configFilePath := filepath.Join(clusterDir, clusterConfigFileName)

		if _, err := os.Stat(clusterDir); os.IsNotExist(err) {
			return fmt.Errorf("cluster '%s' not found. Directory %s does not exist", clusterName, clusterDir)
		}

		if _, err := os.Stat(configFilePath); os.IsNotExist(err) {
			// Config file doesn't exist, but directory does.
			// This means we know the cluster was likely created but its config isn't stored in the conventional place.
			fmt.Printf("Cluster Name: %s\n", clusterName)
			fmt.Printf("Status: Unknown (configuration file %s not found)\n", configFilePath)
			fmt.Println("Detailed configuration is unavailable.")
			// Potentially list other files found in the directory if that's useful.
			return nil
		}

		loadedConfig, err := config.LoadClusterConfigFromFile(configFilePath)
		if err != nil {
			return fmt.Errorf("failed to load configuration for cluster '%s' from %s: %w", clusterName, configFilePath, err)
		}

		fmt.Printf("Details for cluster '%s':\n\n", clusterName)

		// Pretty print the configuration as YAML
		// This provides a comprehensive view of what kubexm knows about the cluster.
		yamlData, err := yaml.Marshal(loadedConfig)
		if err != nil {
			return fmt.Errorf("failed to marshal cluster configuration to YAML: %w", err)
		}
		fmt.Println(string(yamlData))

		// TODO: Future enhancements:
		// - Display a summarized view instead of full YAML by default.
		// - Check actual cluster status by trying to connect (would require kubeconfig access).
		// - Show paths to important artifacts (kubeconfig, certs) if known.

		return nil
	},
}
