package registry

import (
	"github.com/spf13/cobra"
)

// RegistryCmd represents the registry command group
var RegistryCmd = &cobra.Command{
	Use:   "registry",
	Short: "Manage private image registries",
	Long:  `Commands for creating, deleting, and managing private image registries for Kubernetes clusters.`,
}

// AddRegistryCommand adds the registry command to the parent command.
func AddRegistryCommand(parentCmd *cobra.Command) {
	parentCmd.AddCommand(RegistryCmd)
}
