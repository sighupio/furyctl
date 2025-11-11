// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package validate

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	furyctconfig "github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/sighupio/furyctl/internal/analytics"
	"github.com/sighupio/furyctl/internal/app"
	"github.com/sighupio/furyctl/internal/config"
	"github.com/sighupio/furyctl/internal/flags"
	"github.com/sighupio/furyctl/internal/git"
	"github.com/sighupio/furyctl/internal/installation"
	cobrax "github.com/sighupio/furyctl/internal/x/cobra"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	iox "github.com/sighupio/furyctl/internal/x/io"
	"github.com/sighupio/furyctl/pkg/dependencies"
	dist "github.com/sighupio/furyctl/pkg/distribution"
	netx "github.com/sighupio/furyctl/pkg/x/net"
	yamlx "github.com/sighupio/furyctl/pkg/x/yaml"
)

var (
	ErrInstallationValidationFailed      = errors.New("cluster installation validation failed")
	ErrParsingValidationInstallationFlag = errors.New("error while parsing flag")
)

func NewInstallationCmd() *cobra.Command {
	var cmdEvent analytics.Event

	configCmd := &cobra.Command{
		Use:   "installation",
		Short: "Validate an existing installation",
		PreRun: func(cmd *cobra.Command, _ []string) {
			cmdEvent = analytics.NewCommandEvent(cobrax.GetFullname(cmd))

			// Load and validate flags from configuration FIRST.
			if err := flags.LoadAndMergeCommandFlags("installation"); err != nil {
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

			kubeconfig := viper.GetString("kubeconfig")
			distroLocation := viper.GetString("distro-location")
			gitProtocol := viper.GetString("git-protocol")
			outDir := viper.GetString("outdir")
			distroPatchesLocation := viper.GetString("distro-patches")

			typedGitProtocol, err := git.NewProtocol(gitProtocol)
			if err != nil {
				return fmt.Errorf("%w: %w", ErrParsingFlag, err)
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

			client := netx.NewGoGetterClient()
			executor := execx.NewStdExecutor()

			cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while creating kubernetes client: %w", err)
			}

			clientset, err := kubernetes.NewForConfig(cfg)
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while preparing kubernetes configuration: %w", err)
			}

			ctx := context.Background()
			ns := "kube-system"
			secretName := "furyctl-config"
			secret, err := clientset.CoreV1().
				Secrets(ns).
				Get(ctx, secretName, metav1.GetOptions{})
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while getting existing furyctl from cluster: %w", err)
			}
			key := "config"
			furyctlDecoded, ok := secret.Data[key]
			if !ok {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while getting existing furyctl config from cluster: %w", err)
			}

			furyctl := furyctconfig.Furyctl{}
			if err := yamlx.UnmarshalV3(furyctlDecoded, &furyctl); err != nil {

				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while unmarshalling config file: %w", err)
			}
			destDir := filepath.Join(outDir, ".furyctl", furyctl.Metadata.Name, "validate")
			furyctlPath := filepath.Join(destDir, "furyctl.yaml")

			if err := os.MkdirAll(destDir, iox.FullPermAccess); err != nil {

				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while creating validation directory %s: %w", destDir, err)
			}

			if err := os.WriteFile(furyctlPath, furyctlDecoded, iox.FullPermAccess); err != nil {

				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while writing cluster's furyctl to %s: %w", furyctlPath, err)
			}

			depsvl := dependencies.NewValidator(executor, "", furyctlPath, false)

			if distroLocation == "" {
				distrodl = dist.NewCachingDownloader(client, outDir, typedGitProtocol, absDistroPatchesLocation)
			} else {
				distrodl = dist.NewDownloader(client, typedGitProtocol, absDistroPatchesLocation)
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

				return fmt.Errorf("failed to download distribution: %w", err)
			}

			cmdEvent.AddClusterDetails(analytics.ClusterDetails{
				Provider:   res.MinimalConf.Kind,
				KFDVersion: res.DistroManifest.Version,
			})

			if err := config.Validate(furyctlPath, res.RepoPath); err != nil {
				logrus.Debugf("Repository path: %s", res.RepoPath)

				logrus.Error(err)

				cmdEvent.AddErrorMessage(ErrValidationFailed)
				tracker.Track(cmdEvent)

				return ErrValidationFailed
			}

			if err := installation.Validate(string(furyctlDecoded), furyctl.Kind, filepath.Join(outDir, ".furyctl")); err != nil {

				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return err

			}

			logrus.Info("furyctl installation validated succeesfully")

			cmdEvent.AddSuccessMessage("furyctl installation validated succeesfully")
			tracker.Track(cmdEvent)

			return nil
		},
	}

	configCmd.Flags().StringP(
		"kubeconfig",
		"k",
		"",
		"Path to the cluster's kubeconfig",
	)

	configCmd.Flags().StringP(
		"distro-location",
		"",
		"",
		"Location where to download schemas, defaults and the distribution manifests from. "+
			"It can either be a local path (eg: /path/to/distribution) or "+
			"a remote URL (eg: git::git@github.com:sighupio/distribution?depth=1&ref=BRANCH_NAME). "+
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

	return configCmd
}
