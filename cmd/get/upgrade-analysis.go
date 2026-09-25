// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package get

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.yaml.in/yaml/v3"

	"github.com/sighupio/furyctl/configs"
	"github.com/sighupio/furyctl/internal/analytics"
	"github.com/sighupio/furyctl/internal/app"
	"github.com/sighupio/furyctl/internal/clusterinfo"
	"github.com/sighupio/furyctl/internal/flags"
	"github.com/sighupio/furyctl/internal/git"
	"github.com/sighupio/furyctl/internal/upgradeanalysis"
	cobrax "github.com/sighupio/furyctl/internal/x/cobra"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	dist "github.com/sighupio/furyctl/pkg/distribution"
	netx "github.com/sighupio/furyctl/pkg/x/net"
)

var errMissingTarget = errors.New("the target version is required, pass it with --to")

func NewUpgradeAnalysisCmd() *cobra.Command {
	var cmdEvent analytics.Event

	upgradeAnalysisCmd := &cobra.Command{
		Args:  cobra.NoArgs,
		Use:   "upgrade-analysis",
		Short: "Display what an upgrade to a given SD version would change.",
		Long: `Display what upgrading the cluster to a given SD version would change.

The command reads the cluster's current state from the cluster itself, resolves the
sequence of supported upgrade hops needed to reach the target version, and reports what
each hop changes: the Kubernetes and installer versions, and the versions of the modules
the cluster actually deploys. Modules that are not deployed are left out.

It only reads. Nothing in the cluster is modified.

Note that reaching the target may need more than one hop, since furyctl only supports the
upgrade paths it ships. The distribution manifest of every version along the way is
downloaded, so the command needs access to the distribution repository.`,
		Example: `  furyctl get upgrade-analysis --to v1.35.1                 report the upgrade to v1.35.1
  furyctl get upgrade-analysis --to v1.35.1 --format json   the same report as JSON
 `,
		PreRun: func(cmd *cobra.Command, _ []string) {
			cmdEvent = analytics.NewCommandEvent(cobrax.GetFullname(cmd))

			// Bind the flags first: a flag on the command line has precedence over the configuration file.
			if err := viper.BindPFlags(cmd.Flags()); err != nil {
				logrus.Fatalf("error while binding flags: %v", err)
			}

			if err := flags.LoadAndMergeCommandFlags("get"); err != nil {
				logrus.Fatalf("failed to load flags from configuration: %v", err)
			}
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			ctn := app.GetContainerInstance()

			tracker := ctn.Tracker()
			tracker.Flush()

			binPath := viper.GetString("bin-path")
			currentDir := viper.GetString("workdir")
			debug := viper.GetBool("debug")
			format := viper.GetString("format")
			outDir := viper.GetString("outdir")
			to := viper.GetString("to")
			gitProtocol := viper.GetString("git-protocol")

			execx.Debug = debug

			if to == "" {
				cmdEvent.AddErrorMessage(errMissingTarget)
				tracker.Track(cmdEvent)

				return errMissingTarget
			}

			if format != outputFormatText && format != outputFormatJSON && format != outputFormatYAML {
				cmdEvent.AddErrorMessage(errInvalidOutputFormat)
				tracker.Track(cmdEvent)

				return errInvalidOutputFormat
			}

			typedGitProtocol, err := git.ParseProtocol(gitProtocol)
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("%w: %w", ErrParsingFlag, err)
			}

			kubectlBin := resolveKubectlBin(binPath, outDir)

			info, err := clusterinfo.NewCollector(kubectlBin, currentDir).Collect()
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while collecting cluster information: %w", err)
			}

			logrus.Info("Resolving the upgrade path and downloading the distribution manifests...")

			fetcher := upgradeanalysis.NewDownloadFetcher(
				dist.NewCachingDownloader(netx.NewGoGetterClient(), outDir, typedGitProtocol, ""),
			)

			analysis, err := upgradeanalysis.Build(configs.Tpl, fetcher, info, to)
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while building the upgrade analysis: %w", err)
			}

			if err := printAnalysis(analysis, format); err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while printing the upgrade analysis: %w", err)
			}

			cmdEvent.AddSuccessMessage("upgrade analysis successfully built")
			tracker.Track(cmdEvent)

			return nil
		},
	}

	upgradeAnalysisCmd.Flags().String(
		"to",
		"",
		"Target SD version of the upgrade, for example v1.35.1. Required.",
	)

	upgradeAnalysisCmd.Flags().StringP(
		"bin-path",
		"b",
		"",
		"Path to the folder where all the dependencies' binaries are installed. "+
			"When set, furyctl looks for kubectl inside this folder. "+
			"If not set, kubectl is resolved from PATH.",
	)

	upgradeAnalysisCmd.Flags().StringP(
		"format",
		"f",
		outputFormatText,
		"Output format. Supported values: text, json, yaml",
	)

	if err := upgradeAnalysisCmd.RegisterFlagCompletionFunc("format", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return []string{outputFormatText, outputFormatJSON, outputFormatYAML}, cobra.ShellCompDirectiveDefault
	}); err != nil {
		logrus.Fatalf("error while registering flag completion: %v", err)
	}

	return upgradeAnalysisCmd
}

func printAnalysis(analysis *upgradeanalysis.Analysis, format string) error {
	switch format {
	case outputFormatJSON:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")

		if err := enc.Encode(analysis); err != nil {
			return fmt.Errorf("error encoding JSON: %w", err)
		}

		return nil

	case outputFormatYAML:
		enc := yaml.NewEncoder(os.Stdout)
		defer enc.Close()

		if err := enc.Encode(analysis); err != nil {
			return fmt.Errorf("error encoding YAML: %w", err)
		}

		return nil

	default:
		if _, err := fmt.Fprint(os.Stdout, upgradeanalysis.Text(analysis)); err != nil {
			return fmt.Errorf("error writing output: %w", err)
		}

		return nil
	}
}
