package node

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	// "github.com/mensylisir/kubexm/pkg/common" // Already available via list.go's import if needed for shared consts
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/yaml"
)

// NodeGetOptions holds options for the get node command
type NodeGetOptions struct {
	ClusterName    string
	KubeconfigPath string
	OutputFormat   string // "yaml", "json", "summary"
}

var nodeGetOptions = &NodeGetOptions{}

func init() {
	NodeCmd.AddCommand(getNodeCmd)
	getNodeCmd.Flags().StringVarP(&nodeGetOptions.ClusterName, "cluster", "c", "", "Name of the cluster to get the node from (required)")
	getNodeCmd.Flags().StringVar(&nodeGetOptions.KubeconfigPath, "kubeconfig", "", "Path to the kubeconfig file. If empty, uses the default for the specified cluster.")
	getNodeCmd.Flags().StringVarP(&nodeGetOptions.OutputFormat, "output", "o", "yaml", "Output format. One of: yaml|json|summary.") // Default to YAML for full details

	if err := getNodeCmd.MarkFlagRequired("cluster"); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to mark 'cluster' flag as required for 'node get': %v\n", err)
	}
}

var getNodeCmd = &cobra.Command{
	Use:   "get [NODE_NAME]",
	Short: "Get detailed information about a specific node in a cluster",
	Long:  `Retrieves and displays detailed information about a specific node within a given Kubernetes cluster.`,
	Args:  cobra.ExactArgs(1), // Requires exactly one argument: node_name
	RunE: func(cmd *cobra.Command, args []string) error {
		nodeName := args[0]

		kubeconfigPath := nodeGetOptions.KubeconfigPath
		if kubeconfigPath == "" {
			if nodeGetOptions.ClusterName == "" {
				// This check is technically redundant due to MarkFlagRequired, but good for safety.
				return fmt.Errorf("cluster name must be specified via --cluster or -c flag")
			}
			baseDir, err := clustersBaseDirForNodeCmd() // Using helper from list.go (or make it common)
			if err != nil {
				return fmt.Errorf("could not determine clusters base directory: %w", err)
			}
			kubeconfigPath = filepath.Join(baseDir, nodeGetOptions.ClusterName, defaultKubeconfigFileNameForNodeCmd)
		}

		if _, err := os.Stat(kubeconfigPath); os.IsNotExist(err) {
			return fmt.Errorf("kubeconfig file not found at %s. Ensure the cluster is created and kubeconfig is available", kubeconfigPath)
		}

		// Load kubeconfig and create clientset
		config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
		if err != nil {
			return fmt.Errorf("failed to build config from kubeconfig at %s: %w", kubeconfigPath, err)
		}

		clientset, err := kubernetes.NewForConfig(config)
		if err != nil {
			return fmt.Errorf("failed to create Kubernetes clientset: %w", err)
		}

		// Get the specific node
		node, err := clientset.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get node '%s' in cluster '%s': %w", nodeName, nodeGetOptions.ClusterName, err)
		}

		// Display node information
		switch nodeGetOptions.OutputFormat {
		case "yaml":
			yamlData, err := yaml.Marshal(node)
			if err != nil {
				return fmt.Errorf("failed to marshal node data to YAML: %w", err)
			}
			fmt.Print(string(yamlData))
		case "json":
			// TODO: Implement JSON output (e.g., using json.MarshalIndent)
			return fmt.Errorf("output format 'json' not yet implemented")
		case "summary":
			// TODO: Implement a summarized output, perhaps similar to `kubectl describe node` but briefer
			// For now, can just print some key fields.
			fmt.Printf("Name:          %s\n", node.Name)
			fmt.Printf("Status:        %s\n", getNodeStatus(node)) // Helper to get ready status
			fmt.Printf("Roles:         %s\n", getRoles(node))      // Using helper from list.go
			fmt.Printf("Kubelet Ver:   %s\n", node.Status.NodeInfo.KubeletVersion)
			fmt.Printf("OS Image:      %s\n", node.Status.NodeInfo.OSImage)
			fmt.Printf("Kernel Ver:    %s\n", node.Status.NodeInfo.KernelVersion)
			// Add more fields as desired for summary
		default:
			return fmt.Errorf("invalid output format: %s. Supported formats are: yaml, json, summary", nodeGetOptions.OutputFormat)
		}

		return nil
	},
}

// getNodeStatus is a helper to get a simple Ready/NotReady status string.
func getNodeStatus(node *corev1.Node) string {
	for _, cond := range node.Status.Conditions {
		if cond.Type == corev1.NodeReady {
			if cond.Status == corev1.ConditionTrue {
				return "Ready"
			}
			return "NotReady"
		}
	}
	return "Unknown"
}
