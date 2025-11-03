// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package renew

import (
	"errors"
	"fmt"
	"path"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/sighupio/furyctl/internal/analytics"
	"github.com/sighupio/furyctl/internal/app"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/config"
	"github.com/sighupio/furyctl/internal/flags"
	"github.com/sighupio/furyctl/internal/git"
	cobrax "github.com/sighupio/furyctl/internal/x/cobra"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	"github.com/sighupio/furyctl/pkg/dependencies"
	dist "github.com/sighupio/furyctl/pkg/distribution"
	netx "github.com/sighupio/furyctl/pkg/x/net"
)

var ErrDownloadDependenciesFailed = errors.New("dependencies download failed")

func NewCertificatesCmd() *cobra.Command {
	var cmdEvent analytics.Event

	certificatesCmd := &cobra.Command{
		Use:   "certificates",
		Short: "Renew certificates of the cluster's PKI used for componenents authentication",
		Long:  "Renew certificates of the cluster's PKI used for componenents authentication. Note that this not renews other certificates like Ingress certificates or certificates managed by cert-manager.",
		PreRun: func(cmd *cobra.Command, _ []string) {
			cmdEvent = analytics.NewCommandEvent(cobrax.GetFullname(cmd))

			// Load and validate flags from configuration FIRST.
			if err := flags.LoadAndMergeCommandFlags("renew"); err != nil {
				logrus.Fatalf("failed to load flags from configuration: %v", err)
			}

			if err := viper.BindPFlags(cmd.Flags()); err != nil {
				logrus.Fatalf("error while binding flags: %v", err)
			}
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			ctn := app.GetContainerInstance()

			tracker := ctn.Tracker()
			tracker.Flush()

			// Get flags.
			debug := viper.GetBool("debug")
			binPath := viper.GetString("bin-path")
			furyctlPath := viper.GetString("config")
			outDir := viper.GetString("outdir")
			distroLocation := viper.GetString("distro-location")
			gitProtocol := viper.GetString("git-protocol")
			skipDepsDownload := viper.GetBool("skip-deps-download")
			skipDepsValidation := viper.GetBool("skip-deps-validation")

			// Get absolute path to the config file.
			var err error
			furyctlPath, err = filepath.Abs(furyctlPath)
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while getting config directory: %w", err)
			}

			if binPath == "" {
				binPath = path.Join(outDir, ".furyctl", "bin")
			} else {
				binPath, err = filepath.Abs(binPath)
				if err != nil {
					cmdEvent.AddErrorMessage(err)
					tracker.Track(cmdEvent)

					return fmt.Errorf("error while getting absolute path for bin folder: %w", err)
				}
			}

			typedGitProtocol, err := git.NewProtocol(gitProtocol)
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("%w", err)
			}

			// Init packages.
			execx.Debug = debug

			executor := execx.NewStdExecutor()

			distrodl := &dist.Downloader{}
			depsvl := dependencies.NewValidator(executor, binPath, furyctlPath, false)

			// Init first half of collaborators.
			client := netx.NewGoGetterClient()

			if distroLocation == "" {
				distrodl = dist.NewCachingDownloader(client, outDir, typedGitProtocol, "")
			} else {
				distrodl = dist.NewDownloader(client, typedGitProtocol, "")
			}

			// Validate base requirements.
			if err := depsvl.ValidateBaseReqs(); err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while validating requirements: %w", err)
			}

			// Download the distribution.
			logrus.Info("Downloading distribution...")

			res, err := distrodl.Download(distroLocation, furyctlPath)
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while downloading distribution: %w", err)
			}

			basePath := path.Join(outDir, ".furyctl", res.MinimalConf.Metadata.Name)

			// Init second half of collaborators.
			depsdl := dependencies.NewCachingDownloader(client, outDir, basePath, binPath, typedGitProtocol)

			// Validate the furyctl.yaml file.
			logrus.Info("Validating configuration file...")
			if err := config.Validate(furyctlPath, res.RepoPath); err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while validating configuration file: %w", err)
			}

			// Download the dependencies.
			if !skipDepsDownload {
				logrus.Info("Downloading dependencies...")
				if _, err := depsdl.DownloadTools(res.DistroManifest); err != nil {
					cmdEvent.AddErrorMessage(ErrDownloadDependenciesFailed)
					tracker.Track(cmdEvent)

					return fmt.Errorf("%w: %v", ErrDownloadDependenciesFailed, err)
				}
			} else {
				logrus.Info("Dependencies download skipped")
			}

			// Validate the dependencies, unless explicitly told to skip it.
			if !skipDepsValidation {
				logrus.Info("Validating dependencies...")
				if err := depsvl.Validate(res); err != nil {
					cmdEvent.AddErrorMessage(err)
					tracker.Track(cmdEvent)

					return fmt.Errorf("error while validating dependencies: %w", err)
				}
			} else {
				logrus.Info("Dependencies validation skipped")
			}

			renewer, err := cluster.NewCertificatesRenewer(res.MinimalConf, res.DistroManifest, res.RepoPath, furyctlPath)
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while creating the certificates renewer: %w", err)
			}

			if err := renewer.Renew(); err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while renewing certificates: %w", err)
			}

			logrus.Infof("Certificates successfully renewed")

			cmdEvent.AddSuccessMessage("certificates successfully renewed")
			tracker.Track(cmdEvent)

			return nil
		},
	}

	certificatesCmd.Flags().StringP(
		"bin-path",
		"b",
		"",
		"Path to the folder where all the dependencies' binaries are downloaded",
	)

	certificatesCmd.Flags().StringP(
		"config",
		"c",
		"furyctl.yaml",
		"Path to the configuration file",
	)

	certificatesCmd.Flags().StringP(
		"distro-location",
		"",
		"",
		"Location where to download schemas, defaults and the distribution manifests from. "+
			"It can either be a local path (eg: /path/to/distribution) or "+
			"a remote URL (eg: git::git@github.com:sighupio/distribution?depth=1&ref=BRANCH_NAME). "+
			"Any format supported by hashicorp/go-getter can be used",
	)

	certificatesCmd.Flags().Bool(
		"skip-deps-download",
		false,
		"Skip downloading the distribution modules, installers and binaries",
	)

	certificatesCmd.Flags().Bool(
		"skip-deps-validation",
		false,
		"Skip validating dependencies",
	)

	return certificatesCmd
}
