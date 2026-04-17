package config

import (
	"fmt"

	"github.com/spf13/cobra"
)

var setCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a KubeXM CLI configuration value",
	Long: `Set a specific configuration key to a new value for the KubeXM CLI.

Supported keys:
  default-package-dir  - Default directory for packages (e.g., /tmp/kubexm/packages)
  verbose              - Enable verbose output (true/false)

Examples:
  # Set default package directory
  kubexm config set default-package-dir /opt/kubexm/packages

  # Enable verbose mode
  kubexm config set verbose true`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]
		value := args[1]

		config, err := LoadLocalConfig()
		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}

		switch key {
		case "default-package-dir":
			config.DefaultPackageDir = value
			fmt.Printf("Set default-package-dir to: %s\n", value)
		case "verbose":
			if value == "true" || value == "false" {
				config.Verbose = value == "true"
				fmt.Printf("Set verbose to: %v\n", config.Verbose)
			} else {
				return fmt.Errorf("verbose must be 'true' or 'false', got: %s", value)
			}
		default:
			return fmt.Errorf("unknown configuration key: %s. Supported keys: default-package-dir, verbose", key)
		}

		if err := SaveLocalConfig(config); err != nil {
			return fmt.Errorf("failed to save configuration: %w", err)
		}

		fmt.Println("Configuration saved successfully.")
		return nil
	},
}

func init() {
	ConfigCmd.AddCommand(setCmd)
}