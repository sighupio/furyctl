// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package get

import (
	"fmt"
	"strings"

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

	kinds := []string{distribution.EKSClusterKind, distribution.KFDDistributionKind, distribution.OnPremisesKind}

	supportedVersionCmd := &cobra.Command{
		Use:   "supported-versions",
		Short: "List of currently supported SD versions and their compatibility with this version of furyctl for each kind.",
		Long: `List of currently supported SD versions and their compatibility with this version of furyctl for each kind. If the "--kind" parameter is specified, the command will only provide information about the selected provider.
        Examples:
 - furyctl get supported-versions                  	will list the currently supported SD versions and their compatibility with this version of furyctl for each kind.
 - furyctl get supported-versions --kind OnPremises	will list the currently supported SD versions and their compatibility with this version of furyctl but for the OnPremises kind.
        `,
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

				return fmt.Errorf("error getting supported SD versions: %w", err)
			}

			kind := viper.GetString("kind")
			validKind, err := validateKind(kind, kinds)
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return err
			}

			kindsToPrint := kinds
			msg := "list of currently supported SD versions and their compatibility with this version of furyctl for "

			if validKind != "" {
				kindsToPrint = []string{validKind}
				msg += validKind + "\n"
			} else {
				msg += "each kind\n"
			}

			logrus.Info(msg + FormatSupportedVersions(releases, kindsToPrint))
			cmdEvent.AddSuccessMessage("supported SD versions")
			tracker.Track(cmdEvent)

			return nil
		},
	}

	supportedVersionCmd.Flags().StringP(
		"kind",
		"k",
		"",
		fmt.Sprintf("Show supported SD versions for the kind of cluster specified. Valid values: %s",
			strings.Join(kinds, ", ")),
	)

	return supportedVersionCmd
}

func validateKind(kind string, validKinds []string) (string, error) {
	if kind == "" {
		return "", nil
	}

	kindMap := make(map[string]string)
	for _, k := range validKinds {
		kindMap[strings.ToLower(k)] = k
	}

	matchedKind, ok := kindMap[strings.ToLower(kind)]
	if !ok {
		return "", fmt.Errorf("invalid kind: %s. Valid values are: %s",
			kind, strings.Join(validKinds, ", "))
	}

	return matchedKind, nil
}

func FormatSupportedVersions(releases []distribution.KFDRelease, kinds []string) string {
	distribution.SetRecommendedVersions(releases)

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
	showRecommendedMsg := false

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
			versionStr += " *"
		} else {
			versionStr += " "
		}

		if r.Recommended {
			versionStr += "**"
			showRecommendedMsg = true
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

	if showRecommendedMsg {
		fmtSupportedVersions += "\n** this indicates the recommended SD versions.\n"
	}

	return fmtSupportedVersions
}
