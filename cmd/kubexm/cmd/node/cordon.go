package node

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	// Ensure common constants/helpers are accessible if needed
	// "github.com/mensylisir/kubexm/pkg/common"
)

// NodeCordonOptions holds options for the cordon node command
type NodeCordonOptions struct {
	ClusterName    string
	KubeconfigPath string
}

var nodeCordonOptions = &NodeCordonOptions{}

func init() {
	NodeCmd.AddCommand(cordonNodeCmd)
	cordonNodeCmd.Flags().StringVarP(&nodeCordonOptions.ClusterName, "cluster", "c", "", "Name of the cluster where the node resides (required)")
	cordonNodeCmd.Flags().StringVar(&nodeCordonOptions.KubeconfigPath, "kubeconfig", "", "Path to the kubeconfig file. If empty, uses the default for the specified cluster.")

	if err := cordonNodeCmd.MarkFlagRequired("cluster"); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to mark 'cluster' flag as required for 'node cordon': %v\n", err)
	}
}

var cordonNodeCmd = &cobra.Command{
	Use:   "cordon [NODE_NAME]",
	Short: "Mark node as unschedulable (cordon)",
	Long: `Marks a node as unschedulable, preventing new pods from being scheduled on it.
Existing pods on the node are not affected.`,
	Args: cobra.ExactArgs(1), // Requires exactly one argument: node_name
	RunE: func(cmd *cobra.Command, args []string) error {
		nodeName := args[0]

		kubeconfigPath := nodeCordonOptions.KubeconfigPath
		if kubeconfigPath == "" {
			if nodeCordonOptions.ClusterName == "" {
				return fmt.Errorf("cluster name must be specified via --cluster or -c flag")
			}
			baseDir, err := clustersBaseDirForNodeCmd()
			if err != nil {
				return fmt.Errorf("could not determine clusters base directory: %w", err)
			}
			kubeconfigPath = filepath.Join(baseDir, nodeCordonOptions.ClusterName, defaultKubeconfigFileNameForNodeCmd)
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

		// Get the node
		node, err := clientset.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get node '%s' in cluster '%s': %w", nodeName, nodeCordonOptions.ClusterName, err)
		}

		// Check if already cordoned
		if node.Spec.Unschedulable {
			fmt.Printf("node/%s already cordoned\n", nodeName)
			return nil
		}

		// Cordon the node
		node.Spec.Unschedulable = true
		_, err = clientset.CoreV1().Nodes().Update(context.TODO(), node, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to cordon node '%s': %w", nodeName, err)
		}

		fmt.Printf("node/%s cordoned\n", nodeName)
		return nil
	},
}

// uncordonNodeCmd definition (for completeness, if we add uncordon later)
/*
var uncordonNodeCmd = &cobra.Command{
	Use:   "uncordon [NODE_NAME]",
	Short: "Mark node as schedulable (uncordon)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// ... similar logic ...
		nodeName := args[0]
		// ... get clientset ...
		node, err := clientset.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
		if err != nil { return err }

		if !node.Spec.Unschedulable {
			fmt.Printf("node/%s already schedulable\n", nodeName)
			return nil
		}
		node.Spec.Unschedulable = false
		_, err = clientset.CoreV1().Nodes().Update(context.TODO(), node, metav1.UpdateOptions{})
		if err != nil { return err }
		fmt.Printf("node/%s uncordoned\n", nodeName)
		return nil
	},
}
func init() {
	// ...
	// NodeCmd.AddCommand(uncordonNodeCmd)
}
*/
