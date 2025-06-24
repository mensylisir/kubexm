package cmd

import (
	"github.com/mensylisir/kubexm/cmd/kubexm/cmd/certs"   // Import the certs package
	"github.com/mensylisir/kubexm/cmd/kubexm/cmd/cluster" // Import the cluster package
	"github.com/mensylisir/kubexm/cmd/kubexm/cmd/config"  // Import the config package
	"github.com/mensylisir/kubexm/cmd/kubexm/cmd/node"    // Import the node package
	"github.com/spf13/cobra"
	// Homedir will be needed if we add config file handling later
	// homedir "github.com/mitchellh/go-homedir"
	// Viper will be needed for config file handling
	// "github.com/spf13/viper"
)

var (
// cfgFile string // For config file, if used
// userLicense string // Example global flag
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "kubexm",
	Short: "kubexm is a tool for managing Kubernetes clusters.",
	Long: `kubexm is a command-line interface tool that helps you
create, manage, and scale Kubernetes clusters efficiently.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// cobra.OnInitialize(initConfig) // For viper config

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.kubexm.yaml)")
	// rootCmd.PersistentFlags().StringP("author", "a", "YOUR NAME", "author name for copyright attribution")
	// rootCmd.PersistentFlags().StringVarP(&userLicense, "license", "l", "", "name of license for the project")
	// rootCmd.PersistentFlags().Bool("viper", true, "use Viper for configuration")
	// viper.BindPFlag("author", rootCmd.PersistentFlags().Lookup("author"))
	// viper.BindPFlag("useViper", rootCmd.PersistentFlags().Lookup("viper"))

	// Add cluster command group
	rootCmd.AddCommand(cluster.ClusterCmd)
	// Add node command group
	node.AddNodeCommand(rootCmd) // Using the exported function from node package
	// Add certs command group
	certs.AddCertsCommand(rootCmd) // Using the exported function from certs package
	// Add config command group
	config.AddConfigCommand(rootCmd) // Using the exported function from config package
	// viper.SetDefault("author", "NAME HERE <EMAIL ADDRESS>")
	// viper.SetDefault("license", "apache")

	// Example of adding a global flag:
	// rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Enable verbose output")
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
