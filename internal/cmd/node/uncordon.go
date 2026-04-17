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
)

type NodeUncordonOptions struct {
	ClusterName    string
	KubeconfigPath string
}

var nodeUncordonOptions = &NodeUncordonOptions{}

func init() {
	NodeCmd.AddCommand(uncordonNodeCmd)
	uncordonNodeCmd.Flags().StringVarP(&nodeUncordonOptions.ClusterName, "cluster", "c", "", "Name of the cluster where the node resides (required)")
	uncordonNodeCmd.Flags().StringVar(&nodeUncordonOptions.KubeconfigPath, "kubeconfig", "", "Path to the kubeconfig file. If empty, uses the default for the specified cluster.")

	if err := uncordonNodeCmd.MarkFlagRequired("cluster"); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to mark 'cluster' flag as required for 'node uncordon': %v\n", err)
	}
}

var uncordonNodeCmd = &cobra.Command{
	Use:   "uncordon [NODE_NAME]",
	Short: "Mark node as schedulable (uncordon)",
	Long: `Marks a node as schedulable, allowing new pods to be scheduled on it again.
This reverses the effect of 'node cordon'.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		nodeName := args[0]

		kubeconfigPath := nodeUncordonOptions.KubeconfigPath
		if kubeconfigPath == "" {
			if nodeUncordonOptions.ClusterName == "" {
				return fmt.Errorf("cluster name must be specified via --cluster or -c flag")
			}
			baseDir, err := clustersBaseDirForNodeCmd()
			if err != nil {
				return fmt.Errorf("could not determine clusters base directory: %w", err)
			}
			kubeconfigPath = filepath.Join(baseDir, nodeUncordonOptions.ClusterName, defaultKubeconfigFileNameForNodeCmd)
		}

		if _, err := os.Stat(kubeconfigPath); os.IsNotExist(err) {
			return fmt.Errorf("kubeconfig file not found at %s. Ensure the cluster is created and kubeconfig is available", kubeconfigPath)
		}

		config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
		if err != nil {
			return fmt.Errorf("failed to build config from kubeconfig at %s: %w", kubeconfigPath, err)
		}

		clientset, err := kubernetes.NewForConfig(config)
		if err != nil {
			return fmt.Errorf("failed to create Kubernetes clientset: %w", err)
		}

		node, err := clientset.CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get node '%s' in cluster '%s': %w", nodeName, nodeUncordonOptions.ClusterName, err)
		}

		if !node.Spec.Unschedulable {
			fmt.Printf("node/%s already schedulable\n", nodeName)
			return nil
		}

		node.Spec.Unschedulable = false
		_, err = clientset.CoreV1().Nodes().Update(context.Background(), node, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to uncordon node '%s': %w", nodeName, err)
		}

		fmt.Printf("node/%s uncordoned\n", nodeName)
		return nil
	},
}
