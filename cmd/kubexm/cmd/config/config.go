package config

import (
	"github.com/spf13/cobra"
)

// ConfigCmd represents the config command group
var ConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage KubeXM CLI configuration",
	Long:  `View or set KubeXM CLI configuration options.`,
	// RunE: func(cmd *cobra.Command, args []string) error {
	// Persist Use: if you want the group command to do something if called directly.
	// Typically, group commands just show help if called without a subcommand.
	// return cmd.Help()
	// },
}

// AddConfigCommand adds the config command to the parent command.
// This function will be called by root.go to register the config command group.
func AddConfigCommand(parentCmd *cobra.Command) {
	parentCmd.AddCommand(ConfigCmd)
	// Child commands (setCmd, viewCmd, useContextCmd) will be added to ConfigCmd
	// by their respective init() functions in their own files.
}
