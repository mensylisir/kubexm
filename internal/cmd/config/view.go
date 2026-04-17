package config

import (
	"fmt"
	"os"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

var viewCmd = &cobra.Command{
	Use:   "view",
	Short: "View current KubeXM CLI configuration settings",
	Long:  `Displays the current configuration settings for the KubeXM CLI.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		config, err := LoadLocalConfig()
		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}

		fmt.Println("KubeXM CLI Configuration")
		fmt.Println("========================")
		fmt.Printf("Config File: %s\n\n", mustGetConfigPath())

		if config.CurrentContext != "" {
			fmt.Printf("Current Context: %s\n", config.CurrentContext)
			if ctx, ok := config.Contexts[config.CurrentContext]; ok {
				fmt.Printf("  Cluster Name: %s\n", ctx.ClusterName)
				fmt.Printf("  Kubeconfig:   %s\n", ctx.Kubeconfig)
			}
		} else {
			fmt.Println("Current Context: (none)")
		}

		fmt.Printf("\nDefault Package Directory: %s\n", config.DefaultPackageDir)
		fmt.Printf("Verbose: %v\n", config.Verbose)

		if len(config.Contexts) > 0 {
			fmt.Println("\nConfigured Contexts:")
			table := tablewriter.NewWriter(os.Stdout)
			table.SetHeader([]string{"CONTEXT NAME", "CLUSTER NAME", "KUBECONFIG"})
			table.SetBorder(true)

			for name, ctx := range config.Contexts {
				marker := " "
				if name == config.CurrentContext {
					marker = "*"
				}
				table.Append([]string{
					marker + name,
					ctx.ClusterName,
					ctx.Kubeconfig,
				})
			}
			table.Render()
		} else {
			fmt.Println("\nNo contexts configured.")
		}

		return nil
	},
}

func init() {
	ConfigCmd.AddCommand(viewCmd)
	viewCmd.Flags().StringP("output", "o", "yaml", "Output format (currently only yaml supported)")
}

func mustGetConfigPath() string {
	path, err := GetConfigPath()
	if err != nil {
		return "(unknown)"
	}
	return path
}