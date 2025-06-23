package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(completionCmd)
}

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate shell completion scripts",
	Long: `To load completions:

Bash:
  $ source <(kubexm completion bash)

  # To load completions for each session, execute once:
  # Linux:
  $ kubexm completion bash > /etc/bash_completion.d/kubexm
  # macOS:
  $ kubexm completion bash > /usr/local/etc/bash_completion.d/kubexm

Zsh:
  # If shell completion is not already enabled in your environment,
  # you will need to enable it.  You can execute the following once:
  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # To load completions for each session, execute once:
  $ kubexm completion zsh > "${fpath[1]}/_kubexm"

  # You will need to start a new shell for this setup to take effect.

Fish:
  $ kubexm completion fish | source

  # To load completions for each session, execute once:
  $ kubexm completion fish > ~/.config/fish/completions/kubexm.fish

PowerShell:
  PS> kubexm completion powershell | Out-String | Invoke-Expression

  # To load completions for every new session, run:
  PS> kubexm completion powershell > kubexm.ps1
  # and source this file from your PowerShell profile.
`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return rootCmd.GenBashCompletion(os.Stdout)
		case "zsh":
			return rootCmd.GenZshCompletion(os.Stdout)
		case "fish":
			return rootCmd.GenFishCompletion(os.Stdout, true) // true for include descriptions
		case "powershell":
			return rootCmd.GenPowerShellCompletionWithDesc(os.Stdout)
		}
		// Should not happen due to Args validation, but as a fallback
		return cmd.Help()
	},
}
