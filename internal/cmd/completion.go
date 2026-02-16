package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newCompletionCmd creates the completion subcommand.
func newCompletionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion <shell>",
		Short: "Generate shell completion scripts",
		Long: `Generate shell completion scripts for envref.

Supported shells: bash, zsh, fish, powershell

To load completions:

Bash:
  $ source <(envref completion bash)

  # To load completions for each session, execute once:
  # Linux:
  $ envref completion bash > /etc/bash_completion.d/envref
  # macOS:
  $ envref completion bash > $(brew --prefix)/etc/bash_completion.d/envref

Zsh:
  # If shell completion is not already enabled in your environment,
  # you will need to enable it. You can execute the following once:
  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # To load completions for each session, execute once:
  $ envref completion zsh > "${fpath[1]}/_envref"

  # You will need to start a new shell for this setup to take effect.

Fish:
  $ envref completion fish | source

  # To load completions for each session, execute once:
  $ envref completion fish > ~/.config/fish/completions/envref.fish

PowerShell:
  PS> envref completion powershell | Out-String | Invoke-Expression

  # To load completions for every new session, add the output to your profile:
  PS> envref completion powershell >> $PROFILE`,
		Args:  cobra.ExactArgs(1),
		ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCompletion(cmd, args[0])
		},
	}

	return cmd
}

// runCompletion generates the completion script for the specified shell.
func runCompletion(cmd *cobra.Command, shell string) error {
	rootCmd := cmd.Root()
	out := cmd.OutOrStdout()

	switch shell {
	case "bash":
		return rootCmd.GenBashCompletionV2(out, true)
	case "zsh":
		return rootCmd.GenZshCompletion(out)
	case "fish":
		return rootCmd.GenFishCompletion(out, true)
	case "powershell":
		return rootCmd.GenPowerShellCompletionWithDesc(out)
	default:
		return fmt.Errorf("unsupported shell %q: valid shells are bash, zsh, fish, powershell", shell)
	}
}
