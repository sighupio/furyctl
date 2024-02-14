// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package diff

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sighupio/furyctl/internal/analytics"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/cmd/cmdutil"
	"github.com/sighupio/furyctl/internal/dependencies"
	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/git"
	"github.com/sighupio/furyctl/internal/tool"
	"github.com/sighupio/furyctl/internal/tool/kubectl"
	"github.com/sighupio/furyctl/internal/tool/shell"
	cobrax "github.com/sighupio/furyctl/internal/x/cobra"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	kubex "github.com/sighupio/furyctl/internal/x/kube"
	netx "github.com/sighupio/furyctl/internal/x/net"
)

const (
	defaultCreatorTimeout = 300
)

var (
	ErrCannotProduceDiff          = errors.New("cannot produce diff")
	ErrDownloadDependenciesFailed = errors.New("dependencies download failed")
	ErrUnknownPhase               = errors.New("unknown phase")
)

type ManifestsCommandFlags struct {
	Debug          bool
	FuryctlPath    string
	DistroLocation string
	Phase          string
	GitProtocol    git.Protocol
	BinPath        string
	Outdir         string
}

func NewManifestsCommand(
	tracker *analytics.Tracker,
) *cobra.Command {
	var cmdEvent analytics.Event

	cmd := &cobra.Command{
		Use:   "manifests",
		Short: "Diff the current manifests with the ones in the cluster",
		PreRun: func(cmd *cobra.Command, _ []string) {
			cmdEvent = analytics.NewCommandEvent(cobrax.GetFullname(cmd))
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			flags, err := getManifestsCommandFlags(cmd, tracker, cmdEvent)
			if err != nil {
				return fmt.Errorf("%w: %w", ErrCannotProduceDiff, err)
			}

			execx.Debug = flags.Debug

			if err := setupKubeconfig(); err != nil {
				return fmt.Errorf("%w: %w", ErrCannotProduceDiff, err)
			}

			client := netx.NewGoGetterClient()
			distrodl := distribution.NewCachingDownloader(client, flags.GitProtocol)

			logrus.Info("Downloading distribution...")
			res, err := distrodl.Download(flags.DistroLocation, flags.FuryctlPath)
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while downloading distribution: %w", err)
			}

			workDir, err := os.MkdirTemp("", "furyctl-")
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while creating temporary directory: %w", err)
			}

			logrus.Infof("Working directory: %s", workDir)

			depsdl := dependencies.NewCachingDownloader(client, workDir, flags.BinPath, flags.GitProtocol)
			if errs, _ := depsdl.DownloadAll(res.DistroManifest); len(errs) > 0 {
				cmdEvent.AddErrorMessage(ErrDownloadDependenciesFailed)
				tracker.Track(cmdEvent)

				return fmt.Errorf("%w: %v", ErrDownloadDependenciesFailed, errs)
			}

			absFuryctlPath, err := filepath.Abs(flags.FuryctlPath)
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while initializing cluster creation: %w", err)
			}

			clusterCreator, err := cluster.NewCreator(
				res.MinimalConf,
				res.DistroManifest,
				cluster.CreatorPaths{
					ConfigPath: absFuryctlPath,
					WorkDir:    workDir,
					DistroPath: res.RepoPath,
					BinPath:    flags.BinPath,
				},
				cluster.OperationPhaseAll,
				true,
				false,
				true,
				true,
				true,
				false,
				"",
				"",
			)
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while initializing cluster creation: %w", err)
			}

			if err := clusterCreator.Create("", defaultCreatorTimeout, defaultCreatorTimeout); err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while creating cluster: %w", err)
			}

			differ, err := createDiffer(res, flags, workDir)
			if err != nil {
				return fmt.Errorf("%w: %w", ErrCannotProduceDiff, err)
			}

			switch flags.Phase {
			case cluster.OperationPhaseAll, "all":
				logrus.Info("Diffing all phases (to be implemented)")

				return nil

			case cluster.OperationPhaseInfrastructure:
				logrus.Info("Diffing infrastructure phase (to be implemented)")

				return nil

			case cluster.OperationPhaseKubernetes:
				logrus.Info("Diffing kubernetes phase (to be implemented)")

				return nil

			case cluster.OperationPhaseDistribution:
				logrus.Info("Diffing distribution phase")

				diff, err := differ.DiffDistributionPhase()
				if err != nil {
					return fmt.Errorf("%w: %w", ErrCannotProduceDiff, err)
				}

				if len(diff) == 0 {
					fmt.Println("No differences were found!")

					return nil
				}

				fmt.Println("Differences were found:")

				fmt.Println(string(diff))

			default:
				return fmt.Errorf("%w: %s", ErrUnknownPhase, flags.Phase)
			}

			return nil
		},
	}

	setupManifestsCommandFlags(cmd)

	return cmd
}

func setupKubeconfig() error {
	kubeconfigPath := os.Getenv("KUBECONFIG")
	if kubeconfigPath == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("cannot determine current working directory: %w", err)
		}

		kubeconfigPath = filepath.Join(cwd, "kubeconfig")
	}

	if _, err := os.Stat(kubeconfigPath); err != nil {
		return fmt.Errorf("cannot find kubeconfig: %w", err)
	}

	if err := kubex.SetConfigEnv(kubeconfigPath); err != nil {
		return fmt.Errorf("error setting kubeconfig env: %w", err)
	}

	return nil
}

func createDiffer(
	res distribution.DownloadResult,
	flags ManifestsCommandFlags,
	workDir string,
) (cluster.Differ, error) {
	executor := execx.NewStdExecutor()
	trf := tool.NewRunnerFactory(executor, tool.RunnerFactoryPaths{
		Bin: flags.BinPath,
	})

	basePath := path.Join(workDir, "distribution", "manifests")

	shellRunner, ok := trf.Create(tool.Shell, "", basePath).(*shell.Runner)
	if !ok {
		return nil, fmt.Errorf("%w: cannot initialize shell runner", ErrCannotProduceDiff)
	}

	kubectlVersion := res.DistroManifest.Tools.Common.Kubectl.Version

	kubectlRunner, ok := trf.Create(tool.Kubectl, kubectlVersion, basePath).(*kubectl.Runner)
	if !ok {
		return nil, fmt.Errorf("%w: cannot initialize kubectl runner", ErrCannotProduceDiff)
	}

	return cluster.NewDiffer(
		res.MinimalConf.APIVersion,
		res.MinimalConf.Kind,
		shellRunner,
		kubectlRunner,
	), nil
}

func getManifestsCommandFlags(
	cmd *cobra.Command,
	tracker *analytics.Tracker,
	cmdEvent analytics.Event,
) (ManifestsCommandFlags, error) {
	furyctlPath, err := cmdutil.StringFlag(cmd, "config", tracker, cmdEvent)
	if err != nil {
		return ManifestsCommandFlags{}, fmt.Errorf("%w: %s", ErrParsingFlag, "config")
	}

	distroLocation, err := cmdutil.StringFlag(cmd, "distro-location", tracker, cmdEvent)
	if err != nil {
		return ManifestsCommandFlags{}, fmt.Errorf("%w: %s", ErrParsingFlag, "distro-location")
	}

	outDir, err := cmdutil.StringFlag(cmd, "outdir", tracker, cmdEvent)
	if err != nil {
		return ManifestsCommandFlags{}, fmt.Errorf("%w: %s", ErrParsingFlag, "outdir")
	}

	if outDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			cmdEvent.AddErrorMessage(err)
			tracker.Track(cmdEvent)

			return ManifestsCommandFlags{}, fmt.Errorf("error while getting user home directory: %w", err)
		}

		outDir = homeDir
	}

	binPath := cmdutil.StringFlagOptional(cmd, "bin-path")
	if binPath == "" {
		binPath = filepath.Join(outDir, ".furyctl", "bin")
	}

	gitProtocol, err := cmdutil.StringFlag(cmd, "git-protocol", tracker, cmdEvent)
	if err != nil {
		return ManifestsCommandFlags{}, fmt.Errorf("%w: %s", ErrParsingFlag, "git-protocol")
	}

	typedGitProtocol, err := git.NewProtocol(gitProtocol)
	if err != nil {
		return ManifestsCommandFlags{}, fmt.Errorf("%w: %w", ErrParsingFlag, err)
	}

	debug, err := cmdutil.BoolFlag(cmd, "debug", tracker, cmdEvent)
	if err != nil {
		return ManifestsCommandFlags{}, fmt.Errorf("%w: %s", ErrParsingFlag, "debug")
	}

	phase, err := cmdutil.StringFlag(cmd, "phase", tracker, cmdEvent)
	if err != nil {
		return ManifestsCommandFlags{}, fmt.Errorf("%w: %s", ErrParsingFlag, "phase")
	}

	if err := cluster.CheckPhase(phase); err != nil {
		return ManifestsCommandFlags{}, fmt.Errorf("%w: %s: %s", ErrParsingFlag, "phase", err.Error())
	}

	return ManifestsCommandFlags{
		DistroLocation: distroLocation,
		FuryctlPath:    furyctlPath,
		GitProtocol:    typedGitProtocol,
		BinPath:        binPath,
		Debug:          debug,
		Outdir:         outDir,
		Phase:          phase,
	}, nil
}

func setupManifestsCommandFlags(cmd *cobra.Command) {
	cmd.Flags().StringP(
		"config",
		"c",
		"furyctl.yaml",
		"Path to the configuration file",
	)

	cmd.Flags().StringP(
		"phase",
		"p",
		"",
		"Limit the execution to a specific phase. Options are: infrastructure, kubernetes, distribution",
	)

	cmd.Flags().StringP(
		"distro-location",
		"",
		"",
		"Location where to download schemas, defaults and the distribution manifests from. "+
			"It can either be a local path (eg: /path/to/fury/distribution) or "+
			"a remote URL (eg: git::git@github.com:sighupio/fury-distribution?depth=1&ref=BRANCH_NAME). "+
			"Any format supported by hashicorp/go-getter can be used.",
	)

	cmd.Flags().StringP(
		"bin-path",
		"b",
		"",
		"Path to the folder where all the dependencies' binaries are installed",
	)
}
