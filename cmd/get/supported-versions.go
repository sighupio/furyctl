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
	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/git"
	cobrax "github.com/sighupio/furyctl/internal/x/cobra"
)

const DateFmt = "2006-01-02"

func NewSupportedVersionsCmd() *cobra.Command {
	var cmdEvent analytics.Event

	supportedVersionCmd := &cobra.Command{
		Use:   "supported-versions",
		Short: "List of currently supported KFD versions and their compatibility with this version of furyctl for each kind.",
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
			releases, err := distribution.GetSupportedVersions(git.NewGitHubClient())
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error getting supported KFD versions: %w", err)
			}

			kind := viper.GetString("kind")
			kinds := []string{distribution.EKSClusterKind, distribution.KFDDistributionKind, distribution.OnPremisesKind}
			msg := "list of currently supported KFD versions and their compatibility with this version of furyctl for "
			if kind != "" {
				kinds = []string{kind}
				msg += kind + "\n"
			} else {
				msg += "each kind\n"
			}

			logrus.Info(msg + FormatSupportedVersions(releases, kinds))

			cmdEvent.AddSuccessMessage("supported KFD versions")
			tracker.Track(cmdEvent)

			return nil
		},
	}

	supportedVersionCmd.Flags().StringP(
		"kind",
		"k",
		"",
		"Show upgrade paths for the kind of cluster specified (eg: EKSCluster, KFDDistribution, OnPremises), when missing shows all kinds.",
	)

	return supportedVersionCmd
}

func FormatSupportedVersions(releases []distribution.KFDRelease, kinds []string) string {
	fmtSupportedVersions := "\n"
	fmtSupportedVersions += "-----------------------------------------------------------------------------------------\n"
	fmtSupportedVersions += "VERSION \t\tRELEASE DATE\t\t"

	for _, k := range kinds {
		fmtSupportedVersions += k + "\t"
	}

	fmtSupportedVersions += "\n"
	fmtSupportedVersions += "-----------------------------------------------------------------------------------------\n"

	supported := func(s bool) string {
		if s {
			return "Yes"
		}

		return "No"
	}

	showUnsupportedFuryctlMsg := false

	for _, r := range releases {
		dateStr := "-"
		if !r.Date.IsZero() {
			dateStr = r.Date.Format(DateFmt)
		}

		versionStr := r.Version.String()

		allKindsSupported := func() bool {
			for _, v := range r.Support {
				if v {
					return false
				}
			}

			return true
		}

		if allKindsSupported() {
			showUnsupportedFuryctlMsg = true
			versionStr += "*"
		} else {
			versionStr += " "
		}

		fmtSupportedVersions += fmt.Sprintf(
			"v%s\t\t%s",
			versionStr,
			dateStr,
		)

		for _, k := range kinds {
			fmtSupportedVersions += "\t\t" + supported(r.Support[k])
		}

		fmtSupportedVersions += "\n"
	}

	if showUnsupportedFuryctlMsg {
		fmtSupportedVersions += "\n* this usually indicates you are not using the latest version of furyctl, try updating or checking the online documentation.\n"
	}

	return fmtSupportedVersions
}
