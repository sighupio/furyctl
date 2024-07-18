// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sighupio/furyctl/internal/analytics"
	"github.com/sighupio/furyctl/internal/app"
	cobrax "github.com/sighupio/furyctl/internal/x/cobra"
)

func NewVersionCmd() *cobra.Command {
	var cmdEvent analytics.Event

	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print the version number and build information of furyctl",
		PreRun: func(cmd *cobra.Command, _ []string) {
			cmdEvent = analytics.NewCommandEvent(cobrax.GetFullname(cmd))
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			ctn := app.GetContainerInstance()

			tracker := ctn.Tracker()
			defer tracker.Flush()

			versions := ctn.Versions()

			if _, err := fmt.Println("buildTime:", versions.BuildTime); err != nil {
				return fmt.Errorf("error while printing buildTime: %w", err)
			}

			if _, err := fmt.Println("gitCommit:", versions.GitCommit); err != nil {
				return fmt.Errorf("error while printing gitCommit: %w", err)
			}

			if _, err := fmt.Println("goVersion:", versions.GoVersion); err != nil {
				return fmt.Errorf("error while printing goVersion: %w", err)
			}

			if _, err := fmt.Println("osArch:", versions.OSArch); err != nil {
				return fmt.Errorf("error while printing osArch: %w", err)
			}

			if _, err := fmt.Println("version:", versions.Version); err != nil {
				return fmt.Errorf("error while printing version: %w", err)
			}

			cmdEvent.AddSuccessMessage("version command executed successfully")
			tracker.Track(cmdEvent)

			return nil
		},
	}

	return versionCmd
}
