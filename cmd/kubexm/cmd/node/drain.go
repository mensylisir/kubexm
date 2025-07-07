package node

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// NodeDrainOptions holds options for the drain node command
type NodeDrainOptions struct {
	ClusterName         string
	KubeconfigPath      string
	GracePeriodSeconds  int
	Force               bool
	IgnoreDaemonSets    bool
	DeleteEmptyDirData  bool // Not implemented in this simplified version
	Timeout             time.Duration
	SkipWaitForDeleteTimeout time.Duration // Timeout for waiting for pods to delete.
}

var nodeDrainOptions = &NodeDrainOptions{}

func init() {
	NodeCmd.AddCommand(drainNodeCmd)
	drainNodeCmd.Flags().StringVarP(&nodeDrainOptions.ClusterName, "cluster", "c", "", "Name of the cluster where the node resides (required)")
	drainNodeCmd.Flags().StringVar(&nodeDrainOptions.KubeconfigPath, "kubeconfig", "", "Path to the kubeconfig file. If empty, uses the default for the specified cluster.")
	drainNodeCmd.Flags().IntVar(&nodeDrainOptions.GracePeriodSeconds, "grace-period", -1, "Period of time in seconds given to each pod to terminate gracefully. If negative, the default value specified in the pod will be used.")
	drainNodeCmd.Flags().BoolVar(&nodeDrainOptions.Force, "force", false, "Continue even if there are pods not managed by a ReplicationController, ReplicaSet, Job, DaemonSet, StatefulSet or CronJob.")
	drainNodeCmd.Flags().BoolVar(&nodeDrainOptions.IgnoreDaemonSets, "ignore-daemonsets", false, "Ignore DaemonSet-managed pods.")
	// drainNodeCmd.Flags().BoolVar(&nodeDrainOptions.DeleteEmptyDirData, "delete-emptydir-data", false, "Continue even if there are pods using emptyDir (local data that will be deleted when the node is drained).") // More complex
	drainNodeCmd.Flags().DurationVar(&nodeDrainOptions.Timeout, "timeout", 1*time.Minute, "Timeout for the drain operation itself.")
	drainNodeCmd.Flags().DurationVar(&nodeDrainOptions.SkipWaitForDeleteTimeout, "wait-for-delete-timeout", 30*time.Second, "Timeout for waiting for individual pods to be deleted.")


	if err := drainNodeCmd.MarkFlagRequired("cluster"); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to mark 'cluster' flag as required for 'node drain': %v\n", err)
	}
}

var drainNodeCmd = &cobra.Command{
	Use:   "drain [NODE_NAME]",
	Short: "Drain node in preparation for maintenance",
	Long: `Drains a node by marking it unschedulable and evicting/deleting pods.
This simplified version cordons the node and then attempts to delete pods.
It does not fully replicate kubectl drain's PDB handling or complex eviction logic.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		nodeName := args[0]
		ctx, cancel := context.WithTimeout(context.Background(), nodeDrainOptions.Timeout)
		defer cancel()

		kubeconfigPath := nodeDrainOptions.KubeconfigPath
		if kubeconfigPath == "" {
			if nodeDrainOptions.ClusterName == "" {
				return fmt.Errorf("cluster name must be specified")
			}
			baseDir, err := clustersBaseDirForNodeCmd()
			if err != nil {
				return err
			}
			kubeconfigPath = filepath.Join(baseDir, nodeDrainOptions.ClusterName, defaultKubeconfigFileNameForNodeCmd)
		}

		if _, err := os.Stat(kubeconfigPath); os.IsNotExist(err) {
			return fmt.Errorf("kubeconfig not found at %s", kubeconfigPath)
		}

		config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
		if err != nil {
			return fmt.Errorf("failed to build config: %w", err)
		}
		clientset, err := kubernetes.NewForConfig(config)
		if err != nil {
			return fmt.Errorf("failed to create clientset: %w", err)
		}

		// 1. Cordon the node
		node, err := clientset.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get node '%s': %w", nodeName, err)
		}
		if !node.Spec.Unschedulable {
			node.Spec.Unschedulable = true
			_, err = clientset.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
			if err != nil {
				return fmt.Errorf("failed to cordon node '%s': %w", nodeName, err)
			}
			fmt.Printf("node/%s cordoned\n", nodeName)
		} else {
			fmt.Printf("node/%s already cordoned\n", nodeName)
		}

		// 2. List pods on the node
		fmt.Printf("Listing pods on node/%s...\n", nodeName)
		pods, err := clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{
			FieldSelector: "spec.nodeName=" + nodeName,
		})
		if err != nil {
			return fmt.Errorf("failed to list pods on node '%s': %w", nodeName, err)
		}

		if len(pods.Items) == 0 {
			fmt.Printf("No pods found on node/%s to drain.\n", nodeName)
			return nil
		}

		// 3. Evict/Delete pods
		fmt.Printf("Evicting pods from node/%s...\n", nodeName)
		for _, pod := range pods.Items {
			// Check for DaemonSet pods
			isDaemonSet := false
			for _, ownerRef := range pod.OwnerReferences {
				if ownerRef.Kind == "DaemonSet" {
					isDaemonSet = true
					break
				}
			}
			if isDaemonSet && nodeDrainOptions.IgnoreDaemonSets {
				fmt.Printf("pod/%s in namespace %s is managed by a DaemonSet, ignoring.\n", pod.Name, pod.Namespace)
				continue
			}
			if isDaemonSet && !nodeDrainOptions.IgnoreDaemonSets {
				fmt.Printf("WARNING: pod/%s in namespace %s is managed by a DaemonSet and --ignore-daemonsets is false. Deleting anyway (simplified drain).\n", pod.Name, pod.Namespace)
				// In real kubectl, daemonset pods are not deleted unless --ignore-daemonsets is true and a few other conditions
			}


			// Simplified check for "unmanaged" pods for --force
			// Real kubectl has more sophisticated checks (ReplicaSet, Job, etc.)
			isManaged := len(pod.OwnerReferences) > 0
			if !isManaged && !nodeDrainOptions.Force {
				return fmt.Errorf("pod/%s in namespace %s is not managed by a controller and --force is not set. Aborting drain", pod.Name, pod.Namespace)
			}


			gracePeriod := int64(nodeDrainOptions.GracePeriodSeconds)
			deletePolicy := metav1.DeletePropagationBackground // Or Foreground, depending on desired behavior

			fmt.Printf("Deleting pod/%s in namespace %s...\n", pod.Name, pod.Namespace)
			err := clientset.CoreV1().Pods(pod.Namespace).Delete(ctx, pod.Name, metav1.DeleteOptions{
				GracePeriodSeconds: &gracePeriod,
				PropagationPolicy:  &deletePolicy,
			})
			if err != nil {
				if apierrors.IsNotFound(err) {
					fmt.Printf("pod/%s in namespace %s already deleted.\n", pod.Name, pod.Namespace)
					continue
				}
				// For this simplified version, we're not implementing full eviction API.
				// If --force, we could try a delete with gracePeriod=0 here if the initial delete fails.
				fmt.Printf("WARNING: Failed to delete pod/%s in namespace %s: %v. Continuing (simplified drain).\n", pod.Name, pod.Namespace, err)
				if nodeDrainOptions.Force {
					fmt.Printf("Attempting forceful delete for pod/%s in namespace %s...\n", pod.Name, pod.Namespace)
					gracePeriodZero := int64(0)
					_ = clientset.CoreV1().Pods(pod.Namespace).Delete(ctx, pod.Name, metav1.DeleteOptions{
						GracePeriodSeconds: &gracePeriodZero,
						PropagationPolicy:  &deletePolicy,
					}) // Ignoring error on force delete for simplicity
				}
				continue // Move to next pod
			}
		}

		// 4. Wait for pods to be deleted (simplified wait)
		fmt.Printf("Waiting for pods to delete on node/%s...\n", nodeName)
		waitCtx, waitCancel := context.WithTimeout(ctx, nodeDrainOptions.SkipWaitForDeleteTimeout)
		defer waitCancel()

		for {
			select {
			case <-waitCtx.Done():
				fmt.Printf("Timed out waiting for all pods to be deleted from node/%s.\n", nodeName)
				return fmt.Errorf("timed out waiting for pods to delete from node %s", nodeName)
			default:
				remainingPods, err := clientset.CoreV1().Pods("").List(waitCtx, metav1.ListOptions{
					FieldSelector: "spec.nodeName=" + nodeName,
				})
				if err != nil {
					return fmt.Errorf("failed to list remaining pods on node '%s': %w", nodeName, err)
				}

				nonDaemonSetPods := 0
				for _, pod := range remainingPods.Items {
					isDaemonSet := false
					for _, ownerRef := range pod.OwnerReferences {
						if ownerRef.Kind == "DaemonSet" {isDaemonSet = true; break}
					}
					if !(isDaemonSet && nodeDrainOptions.IgnoreDaemonSets) {
						nonDaemonSetPods++
					}
				}

				if nonDaemonSetPods == 0 {
					fmt.Printf("All deletable pods removed from node/%s.\n", nodeName)
					fmt.Printf("node/%s drained\n", nodeName)
					return nil
				}
				fmt.Printf("Waiting for %d pods to be deleted from node/%s...\n", nonDaemonSetPods, nodeName)
				time.Sleep(5 * time.Second)
			}
		}
	},
}
