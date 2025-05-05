// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package get

import (
	"errors"
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

var ErrInvalidKind = errors.New("invalid value for kind flag")

func NewSupportedVersionsCmd() *cobra.Command {
	var cmdEvent analytics.Event

	kinds := distribution.GetConfigKinds()

	supportedVersionCmd := &cobra.Command{
		Use:   "supported-versions",
		Short: "List of currently supported SD versions and their compatibility with this version of furyctl for each kind.",
		Long:  "List of currently supported SD versions and their compatibility with this version of furyctl for each kind. If the `--kind` parameter is specified, the command will only provide information about the selected provider.",
		Example: `  - furyctl get supported-versions                  	will list the currently supported SD versions and their compatibility with this version of furyctl for each kind.
  - furyctl get supported-versions --kind OnPremises	will list the currently supported SD versions and their compatibility with this version of furyctl but for the OnPremises kind.
`,
		PreRun: func(cmd *cobra.Command, _ []string) {
			cmdEvent = analytics.NewCommandEvent(cobrax.GetFullname(cmd))
			if err := viper.BindPFlags(cmd.Flags()); err != nil {
				logrus.Fatalf("error while binding flags: %v", err)
			}
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctn := app.GetContainerInstance()
			tracker := ctn.Tracker()
			tracker.Flush()

			releases, err := distribution.GetSupportedVersions(git.NewGitHubClient())
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error getting supported SD versions: %w", err)
			}

			kindsToPrint := kinds
			msg := "list of currently supported SD versions and their compatibility with this version of furyctl for "

			// Check if the kind flag is set, if it is not set we will print all kinds.
			if cmd.Flags().Changed("kind") {
				kind := viper.GetString("kind")
				validKind, err := distribution.ValidateConfigKind(kind)
				if err != nil {
					cmdEvent.AddErrorMessage(err)
					tracker.Track(cmdEvent)

					return fmt.Errorf("error while validating kind: %w", err)
				}

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
		"Show supported SD versions for the kind of cluster specified. Valid values: "+strings.Join(kinds, ", "),
	)

	if err := supportedVersionCmd.RegisterFlagCompletionFunc("kind", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return distribution.GetConfigKinds(), cobra.ShellCompDirectiveDefault
	}); err != nil {
		logrus.Fatalf("error while registering flag completion: %v", err)
	}

	return supportedVersionCmd
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
