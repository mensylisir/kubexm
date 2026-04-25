package cmd

import (
	"github.com/spf13/cobra"

	"github.com/mensylisir/kubexm/internal/cmd/cluster"
	"github.com/mensylisir/kubexm/internal/cmd/iso"
	"github.com/mensylisir/kubexm/internal/cmd/registry"
)

// newCreateCommand creates and returns the create command group
func newCreateCommand() *cobra.Command {
	CreateCmd := &cobra.Command{
		Use:   "create",
		Short: "Create cluster resources",
		Long:  `Commands for creating Kubernetes clusters, registries, manifests, and ISOs.`,
	}

	// Add cluster create subcommand: kubexm create cluster
	CreateCmd.AddCommand(cluster.CreateClusterCmd)

	// Add iso create subcommand: kubexm create iso
	CreateCmd.AddCommand(iso.CreateIsoCmd)

	// Add registry create subcommand: kubexm create registry
	CreateCmd.AddCommand(registry.CreateRegistryCmd)

	return CreateCmd
}
