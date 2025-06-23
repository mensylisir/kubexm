package cluster

import (
	"github.com/spf13/cobra"
	// Ensure the root command is accessible for adding this command group.
	// This might require a way to get the rootCmd instance or use an init pattern.
	// For now, we'll assume an AddCommands function in the parent `cmd` package or similar.
	// Let's adjust to use a public function in `cmd` package to add command.
	// For simplicity here, we'll use init() in this package to add to a package-level var in `cmd`.
	// This is a common pattern with Cobra.
	// In root.go, we would have:
	// func AddCommand(cmd *cobra.Command) {
	//	 rootCmd.AddCommand(cmd)
	// }
	// Then in this file:
	// parentCmd "github.com/mensylisir/kubexm/cmd/kubexm/cmd"
)

// ClusterCmd represents the cluster command group
var ClusterCmd = &cobra.Command{
	Use:   "cluster",
	Short: "Manage Kubernetes clusters",
	Long:  `Commands for creating, deleting, listing, and managing Kubernetes clusters.`,
}

// This init function will be called when this package is imported,
// allowing it to register itself with the root command.
// This requires `rootCmd` in `cmd/root.go` to be accessible or to have an `AddCommand` helper.
// For now, let's assume we'll call `cmd.AddCommand(ClusterCmd)` from `cmd/root.go`'s init or main.
// A cleaner way is often to have root.go's init explicitly import and add commands.
// Let's modify root.go to add this.
// No, the standard way is for root.go's init() to call AddCommand(cluster.ClusterCmd)
// So, cluster.go's init() is not needed for this purpose.
// We will modify root.go to add this command.
