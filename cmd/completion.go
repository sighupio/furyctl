// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func NewCompletionCmd() *cobra.Command {
	return &cobra.Command{
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
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				if err := cmd.Root().GenBashCompletion(os.Stdout); err != nil {
					return fmt.Errorf("error generating bash completion: %w", err)
				}
			case "zsh":
				if err := cmd.Root().GenZshCompletion(os.Stdout); err != nil {
					return fmt.Errorf("error generating zsh completion: %w", err)
				}
			case "fish":
				if err := cmd.Root().GenFishCompletion(os.Stdout, true); err != nil {
					return fmt.Errorf("error generating fish completion: %w", err)
				}
			case "powershell":
				if err := cmd.Root().GenPowerShellCompletion(os.Stdout); err != nil {
					return fmt.Errorf("error generating powershell completion: %w", err)
				}
			}

			return nil
		},
	}
}
