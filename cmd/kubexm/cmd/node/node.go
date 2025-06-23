package node

import (
	// "github.com/mensylisir/kubexm/cmd/kubexm/cmd" // To add NodeCmd to rootCmd
	"github.com/spf13/cobra"
)

// NodeCmd represents the node command group
var NodeCmd = &cobra.Command{
	Use:   "node",
	Short: "Manage cluster nodes",
	Long:  `Commands for listing, describing, and managing nodes within a Kubernetes cluster.`,
	// PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
	// 	// This function can be used to ensure that a cluster context is set
	// 	// or that necessary global flags for node commands (like --cluster) are handled.
	// 	// For example, if most node commands require --cluster, it could be made persistent here.
	// 	return nil
	// },
}

// This function should be called by root.go to add the NodeCmd.
// Example in root.go's init():
// import "github.com/mensylisir/kubexm/cmd/kubexm/cmd/node"
// rootCmd.AddCommand(node.NodeCmd)
func AddNodeCommand(rootCmd *cobra.Command) {
	rootCmd.AddCommand(NodeCmd)
}
