package cmd

import (
	"github.com/spf13/cobra"

	"github.com/mensylisir/kubexm/internal/cmd/cluster"
	"github.com/mensylisir/kubexm/internal/cmd/registry"
)

// newDeleteCommand creates and returns the delete command group
func newDeleteCommand() *cobra.Command {
	DeleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete cluster resources",
		Long:  `Commands for deleting Kubernetes clusters, nodes, and registries.`,
	}

	DeleteCmd.AddCommand(cluster.DeleteClusterCmd)
	DeleteCmd.AddCommand(cluster.DeleteNodesCmd)

	// Add registry delete subcommand: kubexm delete registry
	DeleteCmd.AddCommand(registry.DeleteRegistryCmd)

	return DeleteCmd
}
