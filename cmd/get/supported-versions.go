// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package get

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sighupio/furyctl/internal/analytics"
	"github.com/sighupio/furyctl/internal/app"
	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/git"
	cobrax "github.com/sighupio/furyctl/internal/x/cobra"
)

const DateFmt = "2006-01-02"

func NewSupportedVersionsCmd() *cobra.Command {
	var cmdEvent analytics.Event

	distroVersionCmd := &cobra.Command{
		Use:   "supported-versions",
		Short: "List the currently supported KFD versions and compatibilities with the different distribution's kind.",
		PreRun: func(cmd *cobra.Command, _ []string) {
			cmdEvent = analytics.NewCommandEvent(cobrax.GetFullname(cmd))
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			ctn := app.GetContainerInstance()

			tracker := ctn.Tracker()
			tracker.Flush()
			releases, err := distribution.GetSupportedVersions(git.NewGitHubClient())
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error getting supported KFD versions: %w", err)
			}

			logrus.Info(FormatDistroVersions(releases))

			cmdEvent.AddSuccessMessage("supported KFD versions")
			tracker.Track(cmdEvent)

			return nil
		},
	}

	return distroVersionCmd
}

func FormatDistroVersions(releases []distribution.DistroRelease) string {
	fmtDistroVersions := "\n"
	fmtDistroVersions += "------------------------------------------------------------------------------------\n"
	fmtDistroVersions += "VERSION\t\tRELEASE DATE\t\tEKSCluster\tKFDDistribution\tOnPremises\n"
	fmtDistroVersions += "------------------------------------------------------------------------------------\n"

	for _, r := range releases {
		supported := func(s bool) string {
			if s {
				return "Yes"
			}

			return "No"
		}

		dateStr := "-"
		if !r.Date.IsZero() {
			dateStr = r.Date.Format(DateFmt)
		}

		fmtDistroVersions += fmt.Sprintf(
			"v%s\t\t%s\t\t%s\t\t%s\t\t%s\n",
			r.Version.String(),
			dateStr,
			supported(r.FuryctlSupport.EKSCluster),
			supported(r.FuryctlSupport.KFDDistribution),
			supported(r.FuryctlSupport.OnPremises),
		)
	}

	return fmtDistroVersions
}
