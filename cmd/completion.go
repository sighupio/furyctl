// Copyright (c) 2021 SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate completion script",
	Long: `To load furyctl completions:

Bash:

  $ source <(furyctl completion bash)

  # To load completions for each session, execute once:
  # Linux:
  $ furyctl completion bash > /etc/bash_completion.d/furyctl
  # macOS:
  $ furyctl completion bash > /usr/local/etc/bash_completion.d/furyctl

Zsh:

  # If shell completion is not already enabled in your environment,
  # you will need to enable it.  You can execute the following once:

  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # To load completions for each session, execute once:
  $ furyctl completion zsh > "${fpath[1]}/_furyctl"

  # You will need to start a new shell for this setup to take effect.

fish:

  $ furyctl completion fish | source

  # To load completions for each session, execute once:
  $ furyctl completion fish > ~/.config/fish/completions/furyctl.fish

PowerShell:

  PS> furyctl completion powershell | Out-String | Invoke-Expression

  # To load completions for every new session, run:
  PS> furyctl completion powershell > furyctl.ps1
  # and source this file from your PowerShell profile.
`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.ExactValidArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		switch args[0] {
		case "bash":
			cmd.Root().GenBashCompletion(os.Stdout)
		case "zsh":
			cmd.Root().GenZshCompletion(os.Stdout)
		case "fish":
			cmd.Root().GenFishCompletion(os.Stdout, true)
		case "powershell":
			cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
		}
	},
}

func init() {
	rootCmd.AddCommand(completionCmd)
}
