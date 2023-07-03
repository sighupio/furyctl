// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package legacy

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sighupio/furyctl/internal/analytics"
	"github.com/sighupio/furyctl/internal/cmd/cmdutil"
	"github.com/sighupio/furyctl/internal/legacy"
	cobrax "github.com/sighupio/furyctl/internal/x/cobra"
)

var (
	ErrParsingFlag     = errors.New("error while parsing flag")
	ErrParsingFuryFile = errors.New("error while parsing furyfile")
	ErrParsingPackages = errors.New("error while parsing packages")
	ErrDownloading     = errors.New("error while downloading")
)

type VendorCmdFlags struct {
	FuryFilePath string
	Prefix       string
	HTTPS        bool
}

func NewVendorCmd(tracker *analytics.Tracker) *cobra.Command {
	var cmdEvent analytics.Event

	cmd := &cobra.Command{
		Use:   "vendor",
		Short: "Download the dependencies specified in the Furyfile.yml",
		PreRun: func(cmd *cobra.Command, _ []string) {
			cmdEvent = analytics.NewCommandEvent(cobrax.GetFullname(cmd))
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			flags, err := getLegacyVendorCmdFlags(cmd, tracker, cmdEvent)
			if err != nil {
				return err
			}

			ff, err := legacy.NewFuryFile(flags.FuryFilePath)
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("%w: %v", ErrParsingFuryFile, err)
			}

			ps, err := ff.BuildPackages(flags.Prefix)
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("%w: %v", ErrParsingPackages, err)
			}

			if token := os.Getenv("GITHUB_TOKEN"); strings.Contains(token, " ") {
				logrus.Warn("GITHUB_TOKEN contains a space character. As a result, " +
					"vendoring modules may fail. If it's intended, you can ignore this warning.\n")
			}

			for _, p := range ps {
				if p.Version == "" {
					logrus.Warnf(
						"package '%s' has no version specified. Will download the default git branch",
						p.Name,
					)
				} else {
					logrus.Infof("using version '%v' for package '%s'", p.Version, p.Name)
				}
			}

			downloader := legacy.NewDownloader(flags.HTTPS)

			err = downloader.Download(ps)
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("%w: %v", ErrDownloading, err)
			}

			cmdEvent.AddSuccessMessage("dependencies downloaded successfully")
			tracker.Track(cmdEvent)

			return nil
		},
	}

	cmd.Flags().StringP(
		"furyfile",
		"F",
		"Furyfile.yml",
		"Path to the Furyfile.yml file",
	)

	cmd.Flags().StringP(
		"prefix",
		"P",
		"",
		"download modules that start with prefix only to reduce download scope. "+
			"Example:\nfuryctl legacy vendor -P mon\nwill download all modules that start with 'mon', "+
			"like 'monitoring', and ignore the rest",
	)

	return cmd
}

func getLegacyVendorCmdFlags(cmd *cobra.Command, tracker *analytics.Tracker, cmdEvent analytics.Event) (VendorCmdFlags, error) {
	https, err := cmdutil.BoolFlag(cmd, "https", tracker, cmdEvent)
	if err != nil {
		return VendorCmdFlags{}, fmt.Errorf("%w: %s", ErrParsingFlag, "https")
	}

	prefix, err := cmdutil.StringFlag(cmd, "prefix", tracker, cmdEvent)
	if err != nil {
		return VendorCmdFlags{}, fmt.Errorf("%w: %s", ErrParsingFlag, "prefix")
	}

	furyFilePath, err := cmdutil.StringFlag(cmd, "furyfile", tracker, cmdEvent)
	if err != nil {
		return VendorCmdFlags{}, fmt.Errorf("%w: %s", ErrParsingFlag, "furyfile")
	}

	return VendorCmdFlags{
		FuryFilePath: furyFilePath,
		Prefix:       prefix,
		HTTPS:        https,
	}, nil
}
