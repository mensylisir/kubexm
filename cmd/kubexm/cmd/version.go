package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
)

// Version will be set by the build process
var Version = "dev"
var Commit = "none"
var Date = "unknown"

func init() {
	rootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of kubexm",
	Long:  `All software has versions. This is kubexm's.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("kubexm version: %s\n", Version)
		fmt.Printf("Git Commit: %s\n", Commit)
		fmt.Printf("Build Date: %s\n", Date)
	},
}
