// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package create

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	distroconf "github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/furyctl/internal/analytics"
	"github.com/sighupio/furyctl/internal/app"
	"github.com/sighupio/furyctl/internal/config"
	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/git"
	"github.com/sighupio/furyctl/internal/semver"
	cobrax "github.com/sighupio/furyctl/internal/x/cobra"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	"github.com/sighupio/furyctl/pkg/dependencies"
	dist "github.com/sighupio/furyctl/pkg/distribution"
	netx "github.com/sighupio/furyctl/pkg/x/net"
)

var (
	ErrParsingFlag          = errors.New("error while parsing flag")
	ErrMandatoryFlag        = errors.New("flag must be specified")
	ErrConfigCreationFailed = errors.New("config creation failed")
)

func NewConfigCmd() *cobra.Command {
	var cmdEvent analytics.Event

	configCmd := &cobra.Command{
		Use:     "config",
		Short:   "Scaffolds a new furyctl configuration file for a specific version and kind",
		Example: "furyctl create config --kind OnPremises --version v1.30.0 --name test-cluster",
		PreRun: func(cmd *cobra.Command, _ []string) {
			cmdEvent = analytics.NewCommandEvent(cobrax.GetFullname(cmd))

			if err := viper.BindPFlags(cmd.Flags()); err != nil {
				logrus.Fatalf("error while binding flags: %v", err)
			}
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			ctn := app.GetContainerInstance()

			tracker := ctn.Tracker()
			defer tracker.Flush()

			// Get flags.
			debug := viper.GetBool("debug")
			furyctlPath := viper.GetString("config")
			distroLocation := viper.GetString("distro-location")
			apiVersion := viper.GetString("api-version")
			name := viper.GetString("name")
			distroPatchesLocation := viper.GetString("distro-patches")
			outDir := viper.GetString("outdir")

			version := viper.GetString("version")

			if version == "" {
				return fmt.Errorf("%w: version", ErrMandatoryFlag)
			}

			kind := viper.GetString("kind")

			if _, err := distribution.NewCompatibilityChecker(version, kind); err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while checking compatibility: %w", err)
			}

			gitProtocol := viper.GetString("git-protocol")

			typedGitProtocol, err := git.NewProtocol(gitProtocol)
			if err != nil {
				return fmt.Errorf("%w: %w", ErrParsingFlag, err)
			}

			minimalConf := distroconf.Furyctl{
				APIVersion: apiVersion,
				Kind:       kind,
				Metadata: distroconf.FuryctlMeta{
					Name: name,
				},
				Spec: distroconf.FuryctlSpec{
					DistributionVersion: version,
				},
			}

			absDistroPatchesLocation := distroPatchesLocation

			if absDistroPatchesLocation != "" {
				absDistroPatchesLocation, err = filepath.Abs(distroPatchesLocation)
				if err != nil {
					cmdEvent.AddErrorMessage(err)
					tracker.Track(cmdEvent)

					return fmt.Errorf("error while getting absolute path of distro patches location: %w", err)
				}
			}

			var distrodl *dist.Downloader

			// Init collaborators.
			client := netx.NewGoGetterClient()
			executor := execx.NewStdExecutor()
			depsvl := dependencies.NewValidator(executor, "", "", false)

			if distroLocation == "" {
				distrodl = dist.NewCachingDownloader(client, outDir, typedGitProtocol, absDistroPatchesLocation)
			} else {
				distrodl = dist.NewDownloader(client, typedGitProtocol, absDistroPatchesLocation)
			}

			// Init packages.
			execx.Debug = debug

			// Validate path.
			if _, err := os.Stat(furyctlPath); err == nil {
				absPath, err := filepath.Abs(furyctlPath)
				if err != nil {
					return fmt.Errorf("%w: error while getting absolute path %v", ErrConfigCreationFailed, err)
				}

				p := filepath.Dir(absPath)

				return fmt.Errorf(
					"%w: a configuration file already exists in %s, please remove it and try again",
					ErrConfigCreationFailed,
					p,
				)
			}

			// Validate base requirements.
			if err := depsvl.ValidateBaseReqs(); err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while validating requirements: %w", err)
			}

			// Download the distribution.
			logrus.Info("Downloading distribution...")
			res, err := distrodl.DoDownload(distroLocation, minimalConf)
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("failed to download distribution: %w", err)
			}

			cmdEvent.AddClusterDetails(analytics.ClusterDetails{
				Provider:   res.MinimalConf.Kind,
				KFDVersion: res.DistroManifest.Version,
			})

			data := map[string]string{
				"Kind":                kind,
				"Name":                name,
				"DistributionVersion": semver.EnsurePrefix(version),
			}

			out, err := config.Create(res, furyctlPath, cmdEvent, tracker, data)
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("failed to create configuration file: %w", err)
			}

			if err := config.Validate(furyctlPath, res.RepoPath); err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				_ = os.Remove(furyctlPath)

				return fmt.Errorf("error while validating configuration file: %w", err)
			}

			logrus.Infof("Configuration file created successfully at: %s", out.Name())

			cmdEvent.AddSuccessMessage("Configuration file created successfully at:" + out.Name())
			tracker.Track(cmdEvent)

			return nil
		},
	}

	configCmd.Flags().StringP(
		"config",
		"c",
		"furyctl.yaml",
		"Path to the configuration file",
	)

	configCmd.Flags().StringP(
		"distro-location",
		"",
		"",
		"Location where to download schemas, defaults, and the distribution manifests from. "+
			"It can either be a local path(eg: /path/to/distribution) or "+
			"a remote URL(eg: git::git@github.com:sighupio/distribution?depth=1&ref=BRANCH_NAME)."+
			"Any format supported by hashicorp/go-getter can be used",
	)

	configCmd.Flags().String(
		"distro-patches",
		"",
		"Location where the distribution's user-made patches can be downloaded from. "+
			"This can be either a local path (eg: /path/to/distro-patches) or "+
			"a remote URL (eg: git::git@github.com:your-org/distro-patches?depth=1&ref=BRANCH_NAME). "+
			"Any format supported by hashicorp/go-getter can be used."+
			" Patches within this location must be in a folder named after the distribution version (eg: v1.29.0) and "+
			"must have the same structure as the distribution's repository",
	)

	configCmd.Flags().StringP(
		"version",
		"v",
		"",
		"SIGHUP Distribution version to use (eg: v1.24.1)",
	)

	configCmd.Flags().StringP(
		"kind",
		"k",
		"",
		"Type of cluster to create. Options are "+strings.Join(distribution.ConfigKinds(), ", "),
	)

	if err := configCmd.RegisterFlagCompletionFunc("kind", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return distribution.ConfigKinds(), cobra.ShellCompDirectiveDefault
	}); err != nil {
		logrus.Fatalf("error while registering flag completion: %v", err)
	}

	configCmd.Flags().StringP(
		"api-version",
		"a",
		"kfd.sighup.io/v1alpha2",
		"Version of the API to use for the selected kind (eg: kfd.sighup.io/v1alpha2)",
	)

	configCmd.Flags().StringP(
		"name",
		"n",
		"example",
		"Name of cluster to create",
	)

	return configCmd
}
