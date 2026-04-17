package cmd

import (
	"github.com/spf13/cobra"
)

// CreateCmd represents the create command group
var CreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create cluster resources",
	Long:  `Commands for creating Kubernetes clusters, registries, manifests, and ISOs.`,
}

func init() {
	rootCmd.AddCommand(CreateCmd)
}
