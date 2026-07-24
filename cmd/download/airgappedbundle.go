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
	"github.com/spf13/viper"

	"github.com/sighupio/furyctl/internal/airgap"
	"github.com/sighupio/furyctl/internal/analytics"
	"github.com/sighupio/furyctl/internal/apis/config"
	"github.com/sighupio/furyctl/internal/app"
	"github.com/sighupio/furyctl/internal/flags"
	"github.com/sighupio/furyctl/internal/git"
	"github.com/sighupio/furyctl/internal/tool/helmfile"
	cobrax "github.com/sighupio/furyctl/internal/x/cobra"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	iox "github.com/sighupio/furyctl/internal/x/io"
	"github.com/sighupio/furyctl/pkg/dependencies"
	dist "github.com/sighupio/furyctl/pkg/distribution"
	netx "github.com/sighupio/furyctl/pkg/x/net"
)

var (
	ErrBundleOutputRequired = errors.New("--bundle-output is required")
	ErrHelmDiffMissing      = errors.New("helm-diff plugin was not installed correctly (incomplete download?)")
)

// Runs `helmfile init` so the helm plugins (helm-diff and friends) are downloaded into
// binPath/helm/plugins and bundled. It is a no-op for distributions or kinds that do not use helmfile.
func preinstallHelmPlugins(kfd config.KFD, binPath string) error {
	helmfileVersion := kfd.Tools.Common.Helmfile.Version
	if helmfileVersion == "" {
		return nil
	}

	helmfilePath := filepath.Join(binPath, "helmfile", helmfileVersion, "helmfile")
	if _, err := os.Stat(helmfilePath); err != nil {
		return nil
	}

	workDir, err := os.MkdirTemp("", "furyctl-airgap-plugins-")
	if err != nil {
		return fmt.Errorf("error creating plugins workdir: %w", err)
	}

	defer os.RemoveAll(workDir)

	runner := helmfile.NewRunner(execx.NewStdExecutor(), helmfile.Paths{
		Helmfile:   helmfilePath,
		WorkDir:    workDir,
		PluginsDir: filepath.Join(binPath, "helm", "plugins"),
	})

	helmPath := filepath.Join(binPath, "helm", kfd.Tools.Common.Helm.Version, "helm")

	logrus.Info("Pre-installing helm plugins (helm-diff, ...) for the bundle...")

	if err := runner.Init(helmPath); err != nil {
		return fmt.Errorf("error pre-installing helm plugins: %w", err)
	}

	// A helmfile init run can exit 0 even when a plugin's binary download hook fails, leaving an
	// unusable plugin. Verify the helm-diff binary is actually there so we never ship a broken bundle.
	diffBin := filepath.Join(binPath, "helm", "plugins", "helm-diff", "bin", "diff")
	if _, err := os.Stat(diffBin); err != nil {
		return fmt.Errorf("%w: %s", ErrHelmDiffMissing, diffBin)
	}

	return nil
}

func NewAirGappedBundleCmd() *cobra.Command {
	var cmdEvent analytics.Event

	airGappedBundleCmd := &cobra.Command{
		Args:  cobra.NoArgs,
		Use:   "air-gapped-bundle",
		Short: "Build a self-contained tarball with the distribution, modules, installers and tools to run furyctl offline",
		Long: "Build a self-contained tarball with everything needed to run furyctl on an air-gapped machine: " +
			"the distribution manifests, modules, installers and all the tools installed via the bundled mise. " +
			"The bundle is built for the host platform only. On the target machine, extract it in the working " +
			"directory and run 'furyctl apply --skip-deps-download --distro-location ./distro'.",
		PreRun: func(cmd *cobra.Command, _ []string) {
			cmdEvent = analytics.NewCommandEvent(cobrax.GetFullname(cmd))

			if err := flags.LoadAndMergeCommandFlags("download"); err != nil {
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

			furyctlPath := viper.GetString("config")
			distroLocation := viper.GetString("distro-location")
			gitProtocol := viper.GetString("git-protocol")
			outDir := viper.GetString("outdir")
			distroPatchesLocation := viper.GetString("distro-patches")
			bundleOutput := viper.GetString("bundle-output")

			if bundleOutput == "" {
				return ErrBundleOutputRequired
			}

			bundleOutput, err := filepath.Abs(bundleOutput)
			if err != nil {
				return fmt.Errorf("error while getting absolute path for bundle output: %w", err)
			}

			typedGitProtocol, err := git.ParseProtocol(gitProtocol)
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("%w: %w", ErrParsingFlag, err)
			}

			binPath := filepath.Join(outDir, ".furyctl", "bin")

			absDistroPatchesLocation := distroPatchesLocation

			if absDistroPatchesLocation != "" {
				absDistroPatchesLocation, err = filepath.Abs(distroPatchesLocation)
				if err != nil {
					cmdEvent.AddErrorMessage(err)
					tracker.Track(cmdEvent)

					return fmt.Errorf("error while getting absolute path of distro patches location: %w", err)
				}
			}

			client := netx.NewGoGetterClient()
			executor := execx.NewStdExecutor()
			depsvl := dependencies.NewValidator(executor, binPath, furyctlPath)

			var distrodl *dist.Downloader
			if distroLocation == "" {
				distrodl = dist.NewCachingDownloader(client, outDir, typedGitProtocol, absDistroPatchesLocation)
			} else {
				distrodl = dist.NewDownloader(client, typedGitProtocol, absDistroPatchesLocation)
			}

			if err := depsvl.ValidateBaseReqs(); err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while validating requirements: %w", err)
			}

			dres, err := distrodl.Download(distroLocation, furyctlPath)
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("failed to download distribution: %w", err)
			}

			cmdEvent.AddClusterDetails(analytics.ClusterDetails{
				Provider:   dres.MinimalConf.Kind,
				KFDVersion: dres.DistroManifest.Version,
			})

			clusterName := dres.MinimalConf.Metadata.Name
			basePath := filepath.Join(outDir, ".furyctl", clusterName)

			depsdl := dependencies.NewCachingDownloader(client, outDir, basePath, binPath, typedGitProtocol)

			logrus.Info("Downloading dependencies...")

			errs, uts := depsdl.DownloadAll(dres.DistroManifest, dres.MinimalConf.Kind)

			for _, ut := range uts {
				logrus.Warnf("'%s' is a host dependency and is NOT bundled, install it on the target machine", ut)
			}

			if len(errs) > 0 {
				for _, err := range errs {
					logrus.Error(err)
				}

				cmdEvent.AddErrorMessage(ErrDownloadFailed)
				tracker.Track(cmdEvent)

				return ErrDownloadFailed
			}

			// Pre-install the helm plugins into the bundle's bin dir. They are normally fetched from the
			// network by helmfile init during the distribution and plugins phases; doing it here, on the
			// connected build host, is what makes those phases work on the target offline.
			if err := preinstallHelmPlugins(dres.DistroManifest, binPath); err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return err
			}

			logrus.Infof("Packaging air-gapped bundle into %s ...", bundleOutput)

			// The bundle carries the tool layout (.furyctl/bin, including the mise binary + installed
			// tool data), the git-vendored modules and installers (.furyctl/<cluster>/vendor) and the
			// distribution manifests (distro/, used as --distro-location). The user brings their own
			// furyctl.yaml on the target, so it is intentionally not bundled.
			entries := []iox.TarGzEntry{
				{Src: binPath, Prefix: filepath.Join(".furyctl", "bin")},
				{Src: filepath.Join(basePath, "vendor"), Prefix: filepath.Join(".furyctl", clusterName, "vendor")},
				{Src: dres.RepoPath, Prefix: airgap.DistroSubdir},
			}

			if err := iox.CreateTarGz(bundleOutput, entries); err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error creating air-gapped bundle: %w", err)
			}

			logrus.Infof("Air-gapped bundle ready: %s", bundleOutput)
			logrus.Info("On the target machine: extract it in the working directory, then run " +
				"'furyctl apply --skip-deps-download --distro-location ./distro'")

			cmdEvent.AddSuccessMessage("Air-gapped bundle created successfully")
			tracker.Track(cmdEvent)

			return nil
		},
	}

	airGappedBundleCmd.Flags().StringP(
		"config",
		"c",
		"furyctl.yaml",
		"Path to the configuration file",
	)

	airGappedBundleCmd.Flags().String(
		"bundle-output",
		"",
		"Path of the air-gapped bundle tarball to create (eg: ./cluster-airgap.tar.gz)",
	)

	airGappedBundleCmd.Flags().StringP(
		"distro-location",
		"",
		"",
		"Location where to download schemas, defaults and the distribution manifests from. "+
			"It can either be a local path (eg: /path/to/distribution) or "+
			"a remote URL (eg: git::git@github.com:sighupio/distribution?depth=1&ref=BRANCH_NAME). "+
			"Any format supported by hashicorp/go-getter can be used",
	)

	airGappedBundleCmd.Flags().String(
		"distro-patches",
		"",
		"Location where the distribution's user-made patches can be downloaded from. "+
			"This can be either a local path (eg: /path/to/distro-patches) or "+
			"a remote URL (eg: git::git@github.com:your-org/distro-patches?depth=1&ref=BRANCH_NAME). "+
			"Any format supported by hashicorp/go-getter can be used."+
			" Patches within this location must be in a folder named after the distribution version (eg: v1.29.0) and "+
			"must have the same structure as the distribution's repository",
	)

	return airGappedBundleCmd
}
