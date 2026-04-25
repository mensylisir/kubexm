package cmd

import (
	"github.com/spf13/cobra"

	"github.com/mensylisir/kubexm/internal/cmd/cluster"
	"github.com/mensylisir/kubexm/internal/cmd/iso"
)

// newBuildCommand creates and returns the build command group
func newBuildCommand() *cobra.Command {
	BuildCmd := &cobra.Command{
		Use:   "build",
		Short: "Build configuration files for cluster deployment",
		Long:  `Commands for building configuration files, certificates, and manifests for offline deployment.`,
	}

	// Add cluster build subcommand: kubexm build cluster
	BuildCmd.AddCommand(cluster.BuildClusterCmd)

	// Add iso build subcommand: kubexm build iso
	BuildCmd.AddCommand(iso.BuildIsoCmd)

	return BuildCmd
}
