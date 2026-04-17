package config

import (
	"fmt"

	"github.com/spf13/cobra"
)

var useContextCmd = &cobra.Command{
	Use:   "use-context <context-name>",
	Short: "Switch the current KubeXM CLI context",
	Long: `Sets the current KubeXM CLI context to the specified context name.
Contexts are used to store cluster connection information.

Examples:
  # Switch to a previously created context
  kubexm config use-context my-cluster

  # After switching context, you can use other commands that
  # reference the current context without specifying --cluster.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		contextName := args[0]

		config, err := LoadLocalConfig()
		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}

		// Check if context exists
		if _, ok := config.Contexts[contextName]; !ok {
			return fmt.Errorf("context '%s' does not exist. Use 'kubexm config add-context' to create it first", contextName)
		}

		config.CurrentContext = contextName

		if err := SaveLocalConfig(config); err != nil {
			return fmt.Errorf("failed to save configuration: %w", err)
		}

		fmt.Printf("Switched to context: %s\n", contextName)
		return nil
	},
}

func init() {
	ConfigCmd.AddCommand(useContextCmd)
}