package cmd

import (
	"github.com/mensylisir/kubexm/cmd/kubexm/cmd/certs"
	"github.com/mensylisir/kubexm/cmd/kubexm/cmd/cluster"
	"github.com/mensylisir/kubexm/cmd/kubexm/cmd/config"
	"github.com/mensylisir/kubexm/cmd/kubexm/cmd/node"
	"github.com/mensylisir/kubexm/pkg/logger"
	"github.com/spf13/cobra"
)

var (
	// Global flags
	verboseFlag   bool
	assumeYesFlag bool
	// cfgFile string // Future use for global config file
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "kubexm",
	Short: "kubexm is a tool for managing Kubernetes clusters.",
	Long: `kubexm is a command-line interface tool that helps you
create, manage, and scale Kubernetes clusters efficiently.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Initialize global logger based on verboseFlag
		logOpts := logger.DefaultOptions()
		logOpts.ColorConsole = true // Default for CLI
		if verboseFlag {
			logOpts.ConsoleLevel = logger.DebugLevel
		}
		logger.Init(logOpts) // Initialize global logger
		// No need to defer SyncGlobal here, individual commands should do it or main.
		return nil
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// cobra.OnInitialize(initConfig) // For viper config

	// Define global persistent flags
	rootCmd.PersistentFlags().BoolVarP(&verboseFlag, "verbose", "v", false, "Enable verbose output")
	rootCmd.PersistentFlags().BoolVarP(&assumeYesFlag, "yes", "y", false, "Assume yes to all prompts and run non-interactively")
	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config-global", "", "Global config file (default is $HOME/.kubexm/config.yaml)")

	// Add cluster command group
	rootCmd.AddCommand(cluster.ClusterCmd)
	// Add node command group
	node.AddNodeCommand(rootCmd) // Using the exported function from node package
	// Add certs command group
	certs.AddCertsCommand(rootCmd) // Using the exported function from certs package
	// Add config command group
	config.AddConfigCommand(rootCmd) // Using the exported function from config package
}

/* // initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".kubexm" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigName(".kubexm")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}
*/
