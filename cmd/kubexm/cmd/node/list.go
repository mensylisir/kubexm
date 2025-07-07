package node

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/duration"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	// "k8s.io/client-go/util/homedir" // For default kubeconfig path if needed outside cluster context
)

// NodeListOptions holds options for the list nodes command
type NodeListOptions struct {
	ClusterName      string
	KubeconfigPath   string
	OutputFormat     string // Future: "table", "json", "yaml"
	NoHeaders        bool
	LabelSelector    string
	FieldSelector    string
	ShowLabels       bool
	AllNamespaces    bool // Typically for pods, but some node lists might use it for other reasons. Not standard for nodes.
}

var nodeListOptions = &NodeListOptions{}

// defaultKubeconfigFileName is the conventional name for the admin kubeconfig file for a cluster.
const defaultKubeconfigFileNameForNodeCmd = "admin.kubeconfig" // Copied from cluster/kubeconfig.go, consider centralizing

// clustersBaseDirForNodeCmd returns the directory where cluster artifacts are expected.
func clustersBaseDirForNodeCmd() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	return filepath.Join(homeDir, common.KubexmRootDirName, "clusters"), nil
}

func init() {
	NodeCmd.AddCommand(listNodeCmd)
	listNodeCmd.Flags().StringVarP(&nodeListOptions.ClusterName, "cluster", "c", "", "Name of the cluster to list nodes from (required)")
	listNodeCmd.Flags().StringVar(&nodeListOptions.KubeconfigPath, "kubeconfig", "", "Path to the kubeconfig file. If empty, uses the default for the specified cluster.")
	// listNodeCmd.Flags().StringVarP(&nodeListOptions.OutputFormat, "output", "o", "table", "Output format. One of: table|json|yaml.")
	// listNodeCmd.Flags().BoolVar(&nodeListOptions.NoHeaders, "no-headers", false, "When using the default output, don't print headers.")
	// listNodeCmd.Flags().StringVarP(&nodeListOptions.LabelSelector, "selector", "l", "", "Selector (label query) to filter on, supports '=', '==', and '!='.(e.g. -l key1=value1,key2=value2)")
	// listNodeCmd.Flags().StringVar(&nodeListOptions.FieldSelector, "field-selector", "", "Selector (field query) to filter on, supports '=', '==', and '!='.(e.g. --field-selector key1=value1,key2=value2). The server only supports a limited number of field queries per type.")
	// listNodeCmd.Flags().BoolVar(&nodeListOptions.ShowLabels, "show-labels", false, "When printing, show all labels as the last column (default hide).")


	if err := listNodeCmd.MarkFlagRequired("cluster"); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to mark 'cluster' flag as required for 'node list': %v\n", err)
	}
}

var listNodeCmd = &cobra.Command{
	Use:   "list",
	Short: "List nodes in a specified cluster",
	Long:  `Lists all nodes, their status, roles, and other information for a given Kubernetes cluster managed by kubexm.`,
	Aliases: []string{"ls", "get"}, // Common aliases for list
	RunE: func(cmd *cobra.Command, args []string) error {
		kubeconfigPath := nodeListOptions.KubeconfigPath
		if kubeconfigPath == "" {
			if nodeListOptions.ClusterName == "" {
				return fmt.Errorf("cluster name must be specified via --cluster or -c flag")
			}
			baseDir, err := clustersBaseDirForNodeCmd()
			if err != nil {
				return fmt.Errorf("could not determine clusters base directory: %w", err)
			}
			kubeconfigPath = filepath.Join(baseDir, nodeListOptions.ClusterName, defaultKubeconfigFileNameForNodeCmd)
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

		// List nodes
		nodes, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{
			LabelSelector: nodeListOptions.LabelSelector,
			FieldSelector: nodeListOptions.FieldSelector,
		})
		if err != nil {
			return fmt.Errorf("failed to list nodes in cluster '%s': %w", nodeListOptions.ClusterName, err)
		}

		if len(nodes.Items) == 0 {
			fmt.Printf("No nodes found in cluster '%s'.\n", nodeListOptions.ClusterName)
			return nil
		}

		// Display nodes in a table
		table := tablewriter.NewWriter(os.Stdout)
		headers := []string{"NAME", "STATUS", "ROLES", "AGE", "VERSION", "INTERNAL-IP", "EXTERNAL-IP"}
		// if nodeListOptions.ShowLabels {
		// 	headers = append(headers, "LABELS")
		// }
		if !nodeListOptions.NoHeaders {
			table.SetHeader(headers)
		}

		table.SetBorder(false)
		table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
		table.SetAlignment(tablewriter.ALIGN_LEFT)
		table.SetCenterSeparator("")
		table.SetColumnSeparator("")
		table.SetRowSeparator("")
		table.SetHeaderLine(false)
		table.SetTablePadding("\t") // pad with tabs
		table.SetNoWhiteSpace(true)


		for _, node := range nodes.Items {
			status := "Unknown"
			for _, cond := range node.Status.Conditions {
				if cond.Type == corev1.NodeReady {
					if cond.Status == corev1.ConditionTrue {
						status = "Ready"
					} else {
						status = "NotReady"
					}
					break
				}
			}

			roles := getRoles(&node)
			age := duration.HumanDuration(time.Since(node.CreationTimestamp.Time))
			version := node.Status.NodeInfo.KubeletVersion

			internalIP := "None"
			externalIP := "None"
			for _, addr := range node.Status.Addresses {
				if addr.Type == corev1.NodeInternalIP {
					internalIP = addr.Address
				}
				if addr.Type == corev1.NodeExternalIP {
					externalIP = addr.Address
				}
			}

			row := []string{node.Name, status, roles, age, version, internalIP, externalIP}
			// if nodeListOptions.ShowLabels {
			// 	row = append(row, formatLabels(node.Labels))
			// }
			table.Append(row)
		}
		table.Render()

		return nil
	},
}

// getRoles extracts and formats node roles from labels.
func getRoles(node *corev1.Node) string {
	var roles []string
	for k := range node.Labels {
		if strings.HasPrefix(k, "node-role.kubernetes.io/") {
			role := strings.TrimPrefix(k, "node-role.kubernetes.io/")
			if role != "" { // handle cases like "node-role.kubernetes.io/" which is just master
				roles = append(roles, role)
			}
		}
	}
	// Check for master role specifically, as it might not have a value (e.g., "node-role.kubernetes.io/master:")
	if _, ok := node.Labels["node-role.kubernetes.io/master"]; ok && !contains(roles, "master") {
		roles = append(roles, "master")
	}
	if _, ok := node.Labels["node-role.kubernetes.io/control-plane"]; ok && !contains(roles, "control-plane") {
		roles = append(roles, "control-plane")
	}


	if len(roles) == 0 {
		return "<none>"
	}
	return strings.Join(roles, ",")
}

// formatLabels converts a map of labels to a comma-separated string.
// func formatLabels(labels map[string]string) string {
// 	if len(labels) == 0 {
// 		return ""
// 	}
// 	var parts []string
// 	for k, v := range labels {
// 		parts = append(parts, fmt.Sprintf("%s=%s", k, v))
// 	}
// 	sort.Strings(parts) // For consistent output
// 	return strings.Join(parts, ",")
// }

// contains checks if a string slice contains a specific string.
func contains(slice []string, str string) bool {
	for _, item := range slice {
		if item == str {
			return true
		}
	}
	return false
}
