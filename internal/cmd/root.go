package cmd

import (
	"github.com/mensylisir/kubexm/internal/cmd/config"
	"github.com/mensylisir/kubexm/internal/logger"

	"github.com/spf13/cobra"
)

var (
	// Commands - all verb-first
	CreateCmd   *cobra.Command // kubexm create
	BuildCmd    *cobra.Command // kubexm build
	DeleteCmd   *cobra.Command // kubexm delete
	InstallCmd  *cobra.Command // kubexm install
	UpdateCmd   *cobra.Command // kubexm update
	UpgradeCmd  *cobra.Command // kubexm upgrade
	DrainCmd    *cobra.Command // kubexm drain
	CordonCmd   *cobra.Command // kubexm cordon
	UncordonCmd *cobra.Command // kubexm uncordon
	ListCmd     *cobra.Command // kubexm list
	GetCmd      *cobra.Command // kubexm get
	CheckCmd    *cobra.Command // kubexm check
	RenewCmd    *cobra.Command // kubexm renew
	RotateCmd   *cobra.Command // kubexm rotate
	PushCmd     *cobra.Command // kubexm push
)

var (
	// Global flags
	verboseFlag   bool
	assumeYesFlag bool
)

var rootCmd = &cobra.Command{
	Use:   "kubexm",
	Short: "kubexm is a tool for managing Kubernetes clusters.",
	Long: `kubexm is a command-line interface tool that helps you
create, manage, and scale Kubernetes clusters efficiently.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		logOpts := logger.DefaultOptions()
		logOpts.ColorConsole = true

		if localCfg, err := config.LoadLocalConfig(); err == nil && localCfg.LogLevel != "" {
			switch localCfg.LogLevel {
			case "debug":
				logOpts.ConsoleLevel = logger.DebugLevel
			case "info":
				logOpts.ConsoleLevel = logger.InfoLevel
			case "warn":
				logOpts.ConsoleLevel = logger.WarnLevel
			case "error":
				logOpts.ConsoleLevel = logger.ErrorLevel
			}
		}

		if verboseFlag {
			logOpts.ConsoleLevel = logger.DebugLevel
		}
		logger.Init(logOpts)
		return nil
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func NewRootCmd() *cobra.Command {
	return rootCmd
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verboseFlag, "verbose", "v", false, "Enable verbose output")
	rootCmd.PersistentFlags().BoolVarP(&assumeYesFlag, "yes", "y", false, "Assume yes to all prompts and run non-interactively")

	// Verb-first commands
	CreateCmd = newCreateCommand()
	rootCmd.AddCommand(CreateCmd)

	BuildCmd = newBuildCommand()
	rootCmd.AddCommand(BuildCmd)

	DeleteCmd = newDeleteCommand()
	rootCmd.AddCommand(DeleteCmd)

	DownloadCmd = DownloadCmdVar()
	rootCmd.AddCommand(DownloadCmd)

	InstallCmd = newInstallCommand()
	rootCmd.AddCommand(InstallCmd)

	UpdateCmd = newUpdateCommand()
	rootCmd.AddCommand(UpdateCmd)

	UpgradeCmd = newUpgradeCommand()
	rootCmd.AddCommand(UpgradeCmd)

	DrainCmd = newDrainCommand()
	rootCmd.AddCommand(DrainCmd)

	CordonCmd = newCordonCommand()
	rootCmd.AddCommand(CordonCmd)

	UncordonCmd = newUncordonCommand()
	rootCmd.AddCommand(UncordonCmd)

	ListCmd = newListNodesCommand()
	rootCmd.AddCommand(ListCmd)

	GetCmd = newGetNodeCommand()
	rootCmd.AddCommand(GetCmd)

	CheckCmd = newCheckCommand()
	rootCmd.AddCommand(CheckCmd)

	RenewCmd = newRenewCommand()
	rootCmd.AddCommand(RenewCmd)

	RotateCmd = newRotateCommand()
	rootCmd.AddCommand(RotateCmd)

	PushCmd = newPushCommand()
	rootCmd.AddCommand(PushCmd)

	// Noun commands
	config.AddConfigCommand(rootCmd)
}

func EnsureInitialized() {
}
