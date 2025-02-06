// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package get

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/sighupio/furyctl/internal/analytics"
	"github.com/sighupio/furyctl/internal/app"
	"github.com/sighupio/furyctl/internal/git"
	cobrax "github.com/sighupio/furyctl/internal/x/cobra"
)

const DateFmt = "2006-01-02"

func NewDistroVersionCmd() *cobra.Command {
	var cmdEvent analytics.Event

	distroVersionCmd := &cobra.Command{
		Use:   "distro-versions",
		Short: "Get the supported distro versions and shows compatibilities with the current furyctl version used to invoke this command with the different distribution's kind.",
		PreRun: func(cmd *cobra.Command, _ []string) {
			cmdEvent = analytics.NewCommandEvent(cobrax.GetFullname(cmd))

			if err := viper.BindPFlags(cmd.Flags()); err != nil {
				logrus.Fatalf("error while binding flags: %v", err)
			}
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			ctn := app.GetContainerInstance()

			tracker := ctn.Tracker()
			tracker.Flush()
			releases, err := app.GetSupportedDistroVersions(git.NewGitHubClient())
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error getting supported distro versions: %w", err)
			}
			logrus.Info(FormatDistroVersions(releases))

			cmdEvent.AddSuccessMessage("upgrade paths successfully retrieved")
			tracker.Track(cmdEvent)

			return nil
		},
	}

	return distroVersionCmd
}

func FormatDistroVersions(releases []app.DistroRelease) string {
	fmtDistroVersions := ""
	fmtDistroVersions += "AVAILABLE KUBERNETES FURY DISTRIBUTION VERSIONS\n"
	fmtDistroVersions += "-----------------------------------------------\n"
	fmtDistroVersions += "VERSION\tRELEASE DATE\tEKS\tKFD\tON PREMISE\n"

	for _, r := range releases {
		supported := func(s bool) string {
			if s {
				return "X"
			}

			return ""
		}

		fmtDistroVersions += fmt.Sprintf(
			"v%s\t%s\t%s\t%s\t%s\n",
			r.Version.String(),
			r.Date.Format(DateFmt),
			supported(r.FuryctlSupport.EKSCluster),
			supported(r.FuryctlSupport.KFDDistribution),
			supported(r.FuryctlSupport.OnPremises),
		)
	}

	return fmtDistroVersions
}
