package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate shell completion scripts",
	Long: `Generate shell completion scripts for mac-cleaner.

To load completions:

Bash:
  $ source <(mac-cleaner completion bash)

  # To install permanently:
  $ mac-cleaner completion bash > /usr/local/etc/bash_completion.d/mac-cleaner

Zsh:
  $ mac-cleaner completion zsh > "${fpath[1]}/_mac-cleaner"

Fish:
  $ mac-cleaner completion fish > ~/.config/fish/completions/mac-cleaner.fish

PowerShell:
  PS> mac-cleaner completion powershell | Out-String | Invoke-Expression
`,
}

var bashCompletionCmd = &cobra.Command{
	Use:   "bash",
	Short: "Generate bash completion script",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return rootCmd.GenBashCompletionV2(os.Stdout, true)
	},
}

var zshCompletionCmd = &cobra.Command{
	Use:   "zsh",
	Short: "Generate zsh completion script",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return rootCmd.GenZshCompletion(os.Stdout)
	},
}

var fishCompletionCmd = &cobra.Command{
	Use:   "fish",
	Short: "Generate fish completion script",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return rootCmd.GenFishCompletion(os.Stdout, true)
	},
}

var powershellCompletionCmd = &cobra.Command{
	Use:   "powershell",
	Short: "Generate powershell completion script",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return rootCmd.GenPowerShellCompletionWithDesc(os.Stdout)
	},
}

func init() {
	completionCmd.AddCommand(bashCompletionCmd)
	completionCmd.AddCommand(zshCompletionCmd)
	completionCmd.AddCommand(fishCompletionCmd)
	completionCmd.AddCommand(powershellCompletionCmd)
	rootCmd.AddCommand(completionCmd)
}
