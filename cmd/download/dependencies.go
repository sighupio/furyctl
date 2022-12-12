// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package download

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sighupio/furyctl/internal/dependencies"
	"github.com/sighupio/furyctl/internal/distribution"
	cobrax "github.com/sighupio/furyctl/internal/x/cobra"
	netx "github.com/sighupio/furyctl/internal/x/net"
)

var (
	ErrParsingFlag    = errors.New("error while parsing flag")
	ErrDownloadFailed = errors.New("dependencies download failed")
)

func NewDependenciesCmd(furyctlBinVersion string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dependencies",
		Short: "Download dependencies",
		RunE: func(cmd *cobra.Command, _ []string) error {
			furyctlPath, ok := cobrax.Flag[string](cmd, "config").(string)
			if !ok {
				return fmt.Errorf("%w: config", ErrParsingFlag)
			}
			distroLocation, ok := cobrax.Flag[string](cmd, "distro-location").(string)
			if !ok {
				return fmt.Errorf("%w: distro-location", ErrParsingFlag)
			}

			binPath := cobrax.Flag[string](cmd, "bin-path").(string) //nolint:errcheck,forcetypeassert // optional flag

			homeDir, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("failed to get current user home directory: %w", err)
			}

			if binPath == "" {
				binPath = filepath.Join(homeDir, ".furyctl", "bin")
			}

			client := netx.NewGoGetterClient()

			distrodl := distribution.NewDownloader(client)

			dres, err := distrodl.Download(furyctlBinVersion, distroLocation, furyctlPath)
			if err != nil {
				return fmt.Errorf("failed to download distribution: %w", err)
			}

			basePath := filepath.Join(homeDir, ".furyctl", dres.MinimalConf.Metadata.Name)

			depsdl := dependencies.NewDownloader(client, basePath, binPath)

			errs, uts := depsdl.DownloadAll(dres.DistroManifest)

			for _, ut := range uts {
				logrus.Warn(fmt.Sprintf("'%s' download is not supported", ut))
			}

			if len(errs) > 0 {
				logrus.Debugf("Repository path: %s", dres.RepoPath)

				for _, err := range errs {
					logrus.Error(err)
				}

				return ErrDownloadFailed
			}

			logrus.Info("Dependencies download succeeded")

			return nil
		},
	}

	cmd.Flags().StringP(
		"bin-path",
		"b",
		"",
		"Path to the bin folder where all dependencies are installed",
	)

	cmd.Flags().StringP(
		"config",
		"c",
		"furyctl.yaml",
		"Path to the furyctl.yaml file",
	)

	cmd.Flags().StringP(
		"distro-location",
		"",
		"",
		"Base URL used to download schemas, defaults and the distribution manifest. "+
			"It can either be a local path(eg: /path/to/fury/distribution) or "+
			"a remote URL(eg: https://git@github.com/sighupio/fury-distribution?ref=BRANCH_NAME)."+
			"Any format supported by hashicorp/go-getter can be used.",
	)

	return cmd
}
