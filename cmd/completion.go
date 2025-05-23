// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sighupio/furyctl/internal/analytics"
	"github.com/sighupio/furyctl/internal/app"
	cobrax "github.com/sighupio/furyctl/internal/x/cobra"
)

var (
	ErrBashCompletion       = errors.New("error generating bash completion")
	ErrZshCompletion        = errors.New("error generating zsh completion")
	ErrFishCompletion       = errors.New("error generating fish completion")
	ErrPowershellCompletion = errors.New("error generating powershell completion")
)

func NewCompletionCmd(rootCmd *cobra.Command) *cobra.Command {
	var cmdEvent analytics.Event

	completionCmd := &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate completion script for your shell",
		Long: fmt.Sprintf(`To load completions:

Bash:

  $ source <(%[1]s completion bash)

  To load completions for each session, execute once:

  Linux:
  $ %[1]s completion bash > /etc/bash_completion.d/%[1]s

  macOS:
  $ %[1]s completion bash > $(brew --prefix)/etc/bash_completion.d/%[1]s

Zsh:

  If shell completion is not already enabled in your environment,
  you will need to enable it.  You can execute the following once:

  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  To load completions for each session, execute once:
  $ %[1]s completion zsh > "${fpath[1]}/_%[1]s"

  You will need to start a new shell for this setup to take effect.

fish:

  $ %[1]s completion fish | source

  To load completions for each session, execute once:
  $ %[1]s completion fish > ~/.config/fish/completions/%[1]s.fish

PowerShell:

  PS> %[1]s completion powershell | Out-String | Invoke-Expression

  To load completions for every new session, run:
  PS> %[1]s completion powershell > %[1]s.ps1
  and source this file from your PowerShell profile.
`, rootCmd.Name()),
		DisableFlagsInUseLine: true,
		ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
		Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		PreRun: func(cmd *cobra.Command, _ []string) {
			cmdEvent = analytics.NewCommandEvent(cobrax.GetFullname(cmd))
			logrus.SetLevel(logrus.FatalLevel)
		},

		RunE: func(cmd *cobra.Command, args []string) error {
			ctn := app.GetContainerInstance()

			tracker := ctn.Tracker()
			defer tracker.Flush()

			switch args[0] {
			case "bash":
				if err := cmd.Root().GenBashCompletion(os.Stdout); err != nil {
					cmdEvent.AddErrorMessage(ErrBashCompletion)
					tracker.Track(cmdEvent)

					return fmt.Errorf("error generating bash completion: %w", err)
				}
			case "zsh":
				if err := cmd.Root().GenZshCompletion(os.Stdout); err != nil {
					cmdEvent.AddErrorMessage(ErrZshCompletion)
					tracker.Track(cmdEvent)

					return fmt.Errorf("error generating zsh completion: %w", err)
				}
			case "fish":
				if err := cmd.Root().GenFishCompletion(os.Stdout, true); err != nil {
					cmdEvent.AddErrorMessage(ErrFishCompletion)
					tracker.Track(cmdEvent)

					return fmt.Errorf("error generating fish completion: %w", err)
				}
			case "powershell":
				if err := cmd.Root().GenPowerShellCompletion(os.Stdout); err != nil {
					cmdEvent.AddErrorMessage(ErrPowershellCompletion)
					tracker.Track(cmdEvent)

					return fmt.Errorf("error generating powershell completion: %w", err)
				}
			}

			cmdEvent.AddSuccessMessage("completion generated for " + args[0])
			tracker.Track(cmdEvent)

			return nil
		},
	}

	return completionCmd
}
