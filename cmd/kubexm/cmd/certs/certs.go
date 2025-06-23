package certs

import (
	"github.com/spf13/cobra"
)

// CertsCmd represents the certs command group
var CertsCmd = &cobra.Command{
	Use:   "certs",
	Short: "Manage cluster certificates",
	Long:  `Commands for checking expiration, rotating, and managing certificates for a Kubernetes cluster managed by kubexm.`,
}

// This function should be called by root.go to add the CertsCmd.
// Example in root.go's init():
// import "github.com/mensylisir/kubexm/cmd/kubexm/cmd/certs"
// certs.AddCertsCommand(rootCmd) // Assuming AddCertsCommand is defined in this package or rootCmd is passed
func AddCertsCommand(rootCmd *cobra.Command) {
	rootCmd.AddCommand(CertsCmd)
}
