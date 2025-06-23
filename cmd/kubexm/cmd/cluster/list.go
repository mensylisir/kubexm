package cluster

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	// "time" // Potentially for last modified time

	"github.com/mensylisir/kubexm/pkg/common" // For default directory names if applicable
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	// "github.com/mensylisir/kubexm/pkg/config" // If we decide to load config for more details
)

type ListOptions struct {
	// OutputFormat string // e.g., "table", "json", "yaml" - for future enhancement
}

var listOptions = &ListOptions{}

// clustersBaseDir returns the directory where cluster artifacts are expected to be stored.
// This is a simplified approach. A more robust solution might involve a global config for kubexm.
func clustersBaseDir() (string, error) {
	// Option 1: Use a directory relative to the current user's home directory.
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	// Path: $HOME/.kubexm/clusters
	// This assumes that `kubexm cluster create` will store artifacts (like its config or a marker file)
	// in $HOME/.kubexm/clusters/<cluster_name>/
	// The runtime.Context.GetClusterArtifactsDir() uses GlobalWorkDir/.kubexm/<cluster_name>
	// For consistency, let's use a similar pattern based on a global work dir concept,
	// defaulting to home if a global work dir isn't easily accessible here or configured.
	// For now, using $HOME/.kubexm/ as the root for kubexm specific data.
	return filepath.Join(homeDir, common.DefaultKubeXMRootDir, common.DefaultClustersDir), nil
}

func init() {
	ClusterCmd.AddCommand(listCmd)
	// listCmd.Flags().StringVarP(&listOptions.OutputFormat, "output", "o", "table", "Output format. One of: table|json|yaml.")
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List created Kubernetes clusters",
	Long:  `Lists all Kubernetes clusters that have been created and are tracked by kubexm.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		baseDir, err := clustersBaseDir()
		if err != nil {
			return fmt.Errorf("could not determine clusters base directory: %w", err)
		}

		if _, err := os.Stat(baseDir); os.IsNotExist(err) {
			fmt.Printf("No clusters found (directory %s does not exist).\n", baseDir)
			// To make it behave like `kubectl get pods` when none exist, print headers then nothing.
			table := tablewriter.NewWriter(os.Stdout)
			table.SetHeader([]string{"NAME", "STATUS", "VERSION", "CONTROL-PLANE ENDPOINT"}) // Example headers
			table.Render()
			return nil
		}

		files, err := os.ReadDir(baseDir)
		if err != nil {
			return fmt.Errorf("failed to read clusters directory %s: %w", baseDir, err)
		}

		var clusterInfos [][]string
		for _, file := range files {
			if file.IsDir() {
				clusterName := file.Name()
				// For now, we don't have live status or easy access to version/endpoint without cluster config.
				// We'll display placeholders or basic info.
				// To get more details, we would need to:
				// 1. Read a specific file within <cluster_name> dir (e.g., the original config.yaml or a status file)
				// 2. Connect to the cluster (if list needed live status, which is more 'get' behavior)

				// Simple approach: list only names.
				// To make it more useful, we'd need to store metadata or the config.
				// Let's assume a copy of the config is stored, e.g., baseDir/<cluster_name>/config.yaml
				// clusterConfigFile := filepath.Join(baseDir, clusterName, "config.yaml") // Hypothetical path
				// status := "Unknown"
				// version := "Unknown"
				// endpoint := "Unknown"

				// Placeholder data
				clusterInfos = append(clusterInfos, []string{clusterName, "Unknown", "Unknown", "Unknown"})
			}
		}

		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"NAME", "STATUS", "VERSION", "CONTROL-PLANE ENDPOINT"}) // Example headers
		table.SetBorder(true)
		table.SetColumnSeparator(" ")
		table.AppendBulk(clusterInfos)

		if len(clusterInfos) == 0 {
			// If the directory exists but contains no subdirectories (clusters)
			fmt.Println("No clusters found.")
		} else {
			table.Render()
		}

		return nil
	},
}
