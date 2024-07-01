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
	"github.com/spf13/viper"

	"github.com/sighupio/furyctl/cmd"
	"github.com/sighupio/furyctl/internal/analytics"
	"github.com/sighupio/furyctl/internal/app"
	"github.com/sighupio/furyctl/internal/legacy"
	cobrax "github.com/sighupio/furyctl/internal/x/cobra"
)

type VendorCmdFlags struct {
	FuryFilePath string
	Prefix       string
	GitProtocol  string
}

var (
	ErrParsingFlag     = errors.New("error while parsing flag")
	ErrParsingFuryFile = errors.New("error while parsing furyfile")
	ErrParsingPackages = errors.New("error while parsing packages")
	ErrDownloading     = errors.New("error while downloading")
	cmdEvent           analytics.Event   //nolint:gochecknoglobals // needed for cobra/viper compatibility.
	vendorCmd          = &cobra.Command{ //nolint:gochecknoglobals // needed for cobra/viper compatibility.
		Use:   "vendor",
		Short: "Download the dependencies specified in the Furyfile.yml",
		PreRun: func(cmd *cobra.Command, _ []string) {
			cmdEvent = analytics.NewCommandEvent(cobrax.GetFullname(cmd))
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			ctn := app.GetContainerInstance()

			tracker := ctn.Tracker()
			tracker.Flush()

			flags := getLegacyVendorCmdFlags()

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

			downloader := legacy.NewDownloader(flags.GitProtocol)

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
)

//nolint:gochecknoinits // this pattern requires init function to work.
func init() {
	vendorCmd.Flags().StringP(
		"furyfile",
		"F",
		"Furyfile.yaml",
		"Path to the Furyfile.yaml file",
	)

	vendorCmd.Flags().StringP(
		"prefix",
		"P",
		"",
		"download modules that start with prefix only to reduce download scope. "+
			"Example:\nfuryctl legacy vendor -P mon\nwill download all modules that start with 'mon', "+
			"like 'monitoring', and ignore the rest",
	)

	if err := viper.BindPFlags(vendorCmd.Flags()); err != nil {
		logrus.Fatalf("error while binding flags: %v", err)
	}

	cmd.LegacyCmd.AddCommand(vendorCmd)
}

func getLegacyVendorCmdFlags() VendorCmdFlags {
	return VendorCmdFlags{
		FuryFilePath: viper.GetString("furyfile"),
		Prefix:       viper.GetString("prefix"),
		GitProtocol:  viper.GetString("git-protocol"),
	}
}
