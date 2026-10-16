// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/r3labs/diff/v3"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/sighupio/furyctl/internal/airgap"
	"github.com/sighupio/furyctl/internal/analytics"
	"github.com/sighupio/furyctl/internal/apis/config"
	commcreate "github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/common/create"
	"github.com/sighupio/furyctl/internal/app"
	"github.com/sighupio/furyctl/internal/cluster"
	icfg "github.com/sighupio/furyctl/internal/config"
	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/flags"
	"github.com/sighupio/furyctl/internal/git"
	"github.com/sighupio/furyctl/internal/state"
	cobrax "github.com/sighupio/furyctl/internal/x/cobra"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	"github.com/sighupio/furyctl/pkg/dependencies"
	"github.com/sighupio/furyctl/pkg/diffs"
	dist "github.com/sighupio/furyctl/pkg/distribution"
	netx "github.com/sighupio/furyctl/pkg/x/net"
	yamlx "github.com/sighupio/furyctl/pkg/x/yaml"
)

// reasonEKSNotSupported is why the resource sections of an EKSCluster are not compared.
const reasonEKSNotSupported = "EKSCluster is not supported by furyctl diff yet"

// ErrDiffNotCompared is returned, after the report was printed, when a section could not be compared. The program
// exits with 1 without logging it, so that the final line of the report stays the last one.
var ErrDiffNotCompared = errors.New("some sections were not compared")

// Differ is implemented by the creators that compare the resources of their cluster with the configuration.
// Diff calls afterPreflight once, right after its preflight attempt whatever its outcome, before any render.
type Differ interface {
	Diff(afterPreflight func()) ([]commcreate.Section, error)
}

type DiffCommandFlags struct {
	ConfigOnly            bool
	SkipDepsDownload      bool
	SkipDepsValidation    bool
	Debug                 bool
	FuryctlPath           string
	DistroLocation        string
	Phase                 string
	NoTTY                 bool
	GitProtocol           git.Protocol
	BinPath               string
	Outdir                string
	UpgradePathLocation   string
	DistroPatchesLocation string
}

func NewDiffCmd() *cobra.Command {
	var cmdEvent analytics.Event

	diffCmd := &cobra.Command{
		Args:  cobra.NoArgs,
		Use:   "diff",
		Short: "Diff the current configuration with the one in the cluster",
		Long: `Compare the furyctl configuration with the one stored in the cluster and show the Kubernetes resources that apply would change.

Without --config-only, diff runs the preflight, which can connect to the nodes and write admin.conf, and renders into the same run dir as apply, so it must not run at the same time as apply. It writes nothing to the cluster. It exits with 1 when a section could not be compared.

With --config-only, diff only compares the configuration file with the stored one, as furyctl did before it compared resources.`,
		PreRun: func(cmd *cobra.Command, _ []string) {
			cmdEvent = analytics.NewCommandEvent(cobrax.GetFullname(cmd))

			// Bind the flags first: a flag on the command line has precedence over the configuration file.
			if err := viper.BindPFlags(cmd.Flags()); err != nil {
				logrus.Fatalf("error while binding flags: %v", err)
			}

			if err := flags.LoadAndMergeCommandFlags("diff"); err != nil {
				logrus.Fatalf("failed to load flags from configuration: %v", err)
			}
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			ctn := app.GetContainerInstance()

			tracker := ctn.Tracker()
			defer tracker.Flush()

			// Air-gapped: extract --airgap-bundle (if set) and rewire to run offline before reading flags.
			if err := airgap.MaybePrepare(); err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error preparing air-gapped bundle: %w", err)
			}

			flags, err := getDiffCommandFlags()
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return err
			}

			execx.Debug = flags.Debug

			if flags.ConfigOnly {
				return runConfigOnlyDiff(flags, cmdEvent, tracker)
			}

			if err := runFullDiff(newDiffSteps(flags, cmdEvent)); err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return err
			}

			cmdEvent.AddSuccessMessage("diff command executed successfully")
			tracker.Track(cmdEvent)

			return nil
		},
	}

	diffCmd.Flags().StringP(
		"config",
		"c",
		"furyctl.yaml",
		"Path to the configuration file",
	)

	diffCmd.Flags().StringP(
		"phase",
		"p",
		"",
		"Limit the execution to a specific phase. Options are: "+strings.Join(cluster.MainPhases(), ", "),
	)

	// Add completion for the phase flag.
	if err := diffCmd.RegisterFlagCompletionFunc("phase", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return cluster.MainPhases(), cobra.ShellCompDirectiveDefault
	}); err != nil {
		logrus.Fatalf("error while registering flag completion: %v", err)
	}

	diffCmd.Flags().StringP(
		"distro-location",
		"",
		"",
		"Location where to download schemas, defaults and the distribution manifests from. "+
			"It can either be a local path (eg: /path/to/distribution) or "+
			"a remote URL (eg: git::git@github.com:sighupio/distribution?depth=1&ref=BRANCH_NAME). "+
			"Any format supported by hashicorp/go-getter can be used",
	)

	diffCmd.Flags().String(
		"distro-patches",
		"",
		"Location where the distribution's user-made patches can be downloaded from. "+
			"This can be either a local path (eg: /path/to/distro-patches) or "+
			"a remote URL (eg: git::git@github.com:your-org/distro-patches?depth=1&ref=BRANCH_NAME). "+
			"Any format supported by hashicorp/go-getter can be used."+
			" Patches within this location must be in a folder named after the distribution version (eg: v1.29.0) and "+
			"must have the same structure as the distribution's repository",
	)

	diffCmd.Flags().StringP(
		"bin-path",
		"b",
		"",
		"Path to the folder where all the dependencies' binaries are downloaded",
	)

	diffCmd.Flags().StringP(
		"upgrade-path-location",
		"",
		"",
		"Location where the upgrade scripts are located, if not set the embedded ones will be used",
	)

	diffCmd.Flags().Bool(
		"config-only",
		false,
		"Only compare the configuration file with the one stored in the cluster, as furyctl diff did before it compared "+
			"resources. Without it, diff runs the preflight, renders into the same run dir as apply and must not run at the "+
			"same time as apply",
	)

	airgap.RegisterFlags(diffCmd)

	return diffCmd
}

// runConfigOnlyDiff is the diff command of the version that only compares the configuration, unchanged.
func runConfigOnlyDiff(cmdFlags DiffCommandFlags, cmdEvent analytics.Event, tracker *analytics.Tracker) error {
	res, err := downloadDistribution(cmdFlags)
	if err != nil {
		cmdEvent.AddErrorMessage(err)
		tracker.Track(cmdEvent)

		return err
	}

	diffChecker, d, err := compareConfiguration(cmdFlags, res)
	if err != nil {
		cmdEvent.AddErrorMessage(err)
		tracker.Track(cmdEvent)

		return err
	}

	if len(d) > 0 {
		fmt.Printf(
			"Differences found from previous cluster configuration:\n%s",
			diffChecker.DiffToString(d),
		)
	} else {
		logrus.Info("No differences found from previous cluster configuration")
	}

	cmdEvent.AddSuccessMessage("diff command executed successfully")
	tracker.Track(cmdEvent)

	return nil
}

func newDistributionDownloader(cmdFlags DiffCommandFlags) *dist.Downloader {
	client := netx.NewGoGetterClient()

	if cmdFlags.DistroLocation == "" {
		return dist.NewCachingDownloader(client, cmdFlags.Outdir, cmdFlags.GitProtocol, cmdFlags.DistroPatchesLocation)
	}

	return dist.NewDownloader(client, cmdFlags.GitProtocol, cmdFlags.DistroPatchesLocation)
}

func downloadDistribution(cmdFlags DiffCommandFlags) (dist.DownloadResult, error) {
	logrus.Info("Downloading distribution...")

	res, err := newDistributionDownloader(cmdFlags).Download(cmdFlags.DistroLocation, cmdFlags.FuryctlPath)
	if err != nil {
		return dist.DownloadResult{}, fmt.Errorf("error while downloading distribution: %w", err)
	}

	return res, nil
}

// compareConfiguration compares the configuration file with the one stored in the cluster, for the phase.
func compareConfiguration(cmdFlags DiffCommandFlags, res dist.DownloadResult) (diffs.Checker, diff.Changelog, error) {
	basePath := filepath.Join(cmdFlags.Outdir, ".furyctl", res.MinimalConf.Metadata.Name)

	stateStore := state.NewStore(
		res.RepoPath,
		cmdFlags.FuryctlPath,
		basePath,
		res.DistroManifest.Tools.Common.Kubectl.Version,
		cmdFlags.BinPath,
	)

	diffChecker, err := createDiffChecker(stateStore, cmdFlags.FuryctlPath)
	if err != nil {
		return nil, nil, fmt.Errorf("error while creating configuration diff checker: %w", err)
	}

	phasePath, err := getPhasePath(
		cmdFlags.FuryctlPath,
		basePath,
		res.RepoPath,
		cmdFlags.BinPath,
		cmdFlags.Phase,
		res.MinimalConf,
		res.DistroManifest,
		cmdFlags.UpgradePathLocation,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("error while getting phase path: %w", err)
	}

	d, err := getDiffs(diffChecker, phasePath)
	if err != nil {
		return nil, nil, fmt.Errorf("error while getting diffs: %w", err)
	}

	return diffChecker, d, nil
}

// diffSteps are the steps of the full comparison, so that the tests can replace them.
type diffSteps struct {
	downloadDistribution func() (dist.DownloadResult, error)
	downloadDependencies func(res dist.DownloadResult) error
	newCreator           func(res dist.DownloadResult) (cluster.Creator, error)
	compareConfiguration func(res dist.DownloadResult) configurationResult
	outOfScope           func(res dist.DownloadResult) []commcreate.Section
	print                func(report string)
}

func newDiffSteps(cmdFlags DiffCommandFlags, cmdEvent analytics.Event) diffSteps {
	return diffSteps{
		downloadDistribution: func() (dist.DownloadResult, error) {
			res, err := downloadDistribution(cmdFlags)
			if err != nil {
				return res, err
			}

			cmdEvent.AddClusterDetails(analytics.ClusterDetails{
				Provider:   res.MinimalConf.Kind,
				KFDVersion: res.DistroManifest.Version,
				Phase:      cmdFlags.Phase,
				DryRun:     false,
			})

			return res, nil
		},
		downloadDependencies: func(res dist.DownloadResult) error { return downloadDiffDependencies(cmdFlags, res) },
		newCreator: func(res dist.DownloadResult) (cluster.Creator, error) {
			return newApplyDefaultsCreator(cluster.NewCreator, cmdFlags, res)
		},
		compareConfiguration: func(res dist.DownloadResult) configurationResult {
			diffChecker, d, err := compareConfiguration(cmdFlags, res)
			if err != nil {
				return configurationResult{Err: err}
			}

			return configurationResult{Lines: diffLines(diffChecker.DiffToString(d))}
		},
		outOfScope: func(res dist.DownloadResult) []commcreate.Section {
			pluginsFeature := distribution.HasFeature(res.DistroManifest, distribution.FeaturePlugins)

			return eksOutOfScopeSections(cmdFlags.FuryctlPath, pluginsFeature)
		},
		print: func(report string) { fmt.Printf("%s", report) },
	}
}

// runFullDiff compares the configuration and the resources of the cluster and prints the report. It returns
// ErrDiffNotCompared, after the report, when a section failed.
func runFullDiff(steps diffSteps) error {
	res, err := steps.downloadDistribution()
	if err != nil {
		return err
	}

	if err := steps.downloadDependencies(res); err != nil {
		return err
	}

	creator, err := steps.newCreator(res)
	if err != nil {
		return err
	}

	var (
		sections      []commcreate.Section
		configuration configurationResult
		compared      bool
	)

	// The configuration is compared once, right after the preflight attempt, so it uses the kubeconfig the
	// preflight set, when it got that far.
	compare := func() {
		if !compared {
			configuration = steps.compareConfiguration(res)
			compared = true
		}
	}

	if differ, ok := creator.(Differ); ok {
		sections, err = differ.Diff(compare)
		if err != nil {
			if compared {
				steps.print(buildDiffReport(configuration, nil).Text)
			}

			return fmt.Errorf("error while comparing the resources: %w", err)
		}
	} else {
		sections = steps.outOfScope(res)
	}

	compare()

	report := buildDiffReport(configuration, sections)

	steps.print(report.Text)

	if report.Failed {
		return ErrDiffNotCompared
	}

	return nil
}

// downloadDiffDependencies downloads and validates the dependencies as apply does.
func downloadDiffDependencies(cmdFlags DiffCommandFlags, res dist.DownloadResult) error {
	executor := execx.NewStdExecutor()
	depsvl := dependencies.NewValidator(executor, cmdFlags.BinPath, cmdFlags.FuryctlPath)

	if err := validateDiffInputs(depsvl, cmdFlags, res); err != nil {
		return err
	}

	basePath := filepath.Join(cmdFlags.Outdir, ".furyctl", res.MinimalConf.Metadata.Name)
	depsdl := dependencies.NewCachingDownloader(
		netx.NewGoGetterClient(), cmdFlags.Outdir, basePath, cmdFlags.BinPath, cmdFlags.GitProtocol,
	)

	err := skippableStep{
		skip: cmdFlags.SkipDepsDownload, doing: "Downloading dependencies...", skipped: "Dependencies download skipped",
		run: func() error {
			if errs, _ := depsdl.DownloadAll(res.DistroManifest, res.MinimalConf.Kind); len(errs) > 0 {
				return fmt.Errorf("%w: %v", ErrDownloadDependenciesFailed, errs)
			}

			return nil
		},
	}.exec()
	if err != nil {
		return err
	}

	return skippableStep{
		skip: cmdFlags.SkipDepsValidation, doing: "Validating dependencies...", skipped: "Dependencies validation skipped",
		run: func() error {
			if err := depsvl.Validate(res); err != nil {
				return fmt.Errorf("error while validating dependencies: %w", err)
			}

			return nil
		},
	}.exec()
}

// validateDiffInputs validates the base requirements and the configuration file.
func validateDiffInputs(depsvl *dependencies.Validator, cmdFlags DiffCommandFlags, res dist.DownloadResult) error {
	if err := depsvl.ValidateBaseReqs(); err != nil {
		return fmt.Errorf("error while validating requirements: %w", err)
	}

	if err := icfg.Validate(cmdFlags.FuryctlPath, res.RepoPath); err != nil {
		return fmt.Errorf("error while validating configuration file: %w", err)
	}

	return nil
}

// skippableStep is a dependencies step that a skip flag turns off, with the log lines apply prints for it.
type skippableStep struct {
	skip    bool
	doing   string
	skipped string
	run     func() error
}

// exec logs what the step is about to do and runs it, or logs that the step is skipped.
func (s skippableStep) exec() error {
	if s.skip {
		logrus.Info(s.skipped)

		return nil
	}

	logrus.Info(s.doing)

	return s.run()
}

type creatorFactory func(
	minimalConf config.Furyctl,
	kfdManifest config.KFD,
	paths cluster.CreatorPaths,
	phase string,
	skipVpn,
	vpnAutoConnect,
	skipNodesUpgrade,
	dryRun bool,
	force []string,
	upgrade bool,
	externalUpgradesPath,
	upgradeNode string,
	postApplyPhases []string,
) (cluster.Creator, error)

// newApplyDefaultsCreator builds the creator of the cluster as apply does with no flag set, so that the
// preflight of the kind runs with its checks.
func newApplyDefaultsCreator(
	factory creatorFactory,
	cmdFlags DiffCommandFlags,
	res dist.DownloadResult,
) (cluster.Creator, error) {
	absFuryctlPath, err := filepath.Abs(cmdFlags.FuryctlPath)
	if err != nil {
		return nil, fmt.Errorf("error while getting absolute path of the configuration file: %w", err)
	}

	paths := cluster.CreatorPaths{
		ConfigPath: absFuryctlPath,
		WorkDir:    filepath.Join(cmdFlags.Outdir, ".furyctl", res.MinimalConf.Metadata.Name),
		DistroPath: res.RepoPath,
		BinPath:    cmdFlags.BinPath,
	}

	creator, err := factory(
		res.MinimalConf,
		res.DistroManifest,
		paths,
		cmdFlags.Phase,
		false,
		false,
		false,
		false,
		[]string{},
		false,
		cmdFlags.UpgradePathLocation,
		"",
		[]string{},
	)
	if err != nil {
		return nil, fmt.Errorf("error while initializing cluster creator: %w", err)
	}

	return creator, nil
}

// eksOutOfScopeSections are the resource sections of an EKSCluster, which furyctl diff does not compare yet.
func eksOutOfScopeSections(furyctlPath string, pluginsFeature bool) []commcreate.Section {
	conf, err := yamlx.FromFileV3[map[string]any](furyctlPath)
	if err != nil {
		logrus.Debugf("error while reading the plugins of the configuration file: %v", err)
	}

	return commcreate.OutOfScopeSections(conf, pluginsFeature, reasonEKSNotSupported)
}

// diffLines splits the changes of the configuration diff into lines.
func diffLines(text string) []string {
	lines := []string{}

	for line := range strings.SplitSeq(text, "\n") {
		if line != "" {
			lines = append(lines, line)
		}
	}

	return lines
}

func getPhasePath(
	furyctlPath string,
	workDir string,
	distroPath string,
	binPath string,
	phase string,
	minimalConf config.Furyctl,
	distroManifest config.KFD,
	upgradePathLocation string,
) (string, error) {
	absFuryctlPath, err := filepath.Abs(furyctlPath)
	if err != nil {
		return "", fmt.Errorf("error while getting absolute path of the configuration file: %w", err)
	}

	paths := cluster.CreatorPaths{
		ConfigPath: absFuryctlPath,
		WorkDir:    workDir,
		DistroPath: distroPath,
		BinPath:    binPath,
	}

	clusterCreator, err := cluster.NewCreator(
		minimalConf,
		distroManifest,
		paths,
		phase,
		true,
		false,
		false,
		true,
		[]string{"all"},
		true,
		upgradePathLocation,
		"",
		[]string{},
	)
	if err != nil {
		return "", fmt.Errorf("error while initializing cluster creator: %w", err)
	}

	phasePath, err := clusterCreator.GetPhasePath(phase)
	if err != nil {
		return "", fmt.Errorf("error while getting phase path: %w", err)
	}

	return phasePath, nil
}

func createDiffChecker(stateStore state.Storer, furyctlPath string) (diffs.Checker, error) {
	var diffChecker diffs.Checker

	storedCfg := map[string]any{}

	storedCfgStr, err := stateStore.GetConfig()
	if err != nil {
		return diffChecker, fmt.Errorf("error while getting current cluster config: %w", err)
	}

	if err := yamlx.UnmarshalV3(storedCfgStr, &storedCfg); err != nil {
		return diffChecker, fmt.Errorf("error while unmarshalling config file: %w", err)
	}

	newCfg, err := yamlx.FromFileV3[map[string]any](furyctlPath)
	if err != nil {
		return diffChecker, fmt.Errorf("error while reading config file: %w", err)
	}

	return diffs.NewBaseChecker(storedCfg, newCfg), nil
}

func getDiffs(diffChecker diffs.Checker, phasePath string) (diff.Changelog, error) {
	changeLog, err := diffChecker.GenerateDiff()
	if err != nil {
		return changeLog, fmt.Errorf("error while generating diff: %w", err)
	}

	return diffChecker.FilterDiffFromPhase(changeLog, phasePath), nil
}

func getDiffCommandFlags() (DiffCommandFlags, error) {
	var err error

	binPath := viper.GetString("bin-path")
	if binPath == "" {
		binPath = filepath.Join(viper.GetString("outdir"), ".furyctl", "bin")
	} else {
		binPath, err = filepath.Abs(binPath)
		if err != nil {
			return DiffCommandFlags{}, fmt.Errorf("error while getting absolute path for bin folder: %w", err)
		}
	}

	distroPatchesLocation := viper.GetString("distro-patches")
	if distroPatchesLocation != "" {
		distroPatchesLocation, err = filepath.Abs(distroPatchesLocation)
		if err != nil {
			return DiffCommandFlags{}, fmt.Errorf("error while getting absolute path of distro patches location: %w", err)
		}
	}

	phase := viper.GetString("phase")
	if err := cluster.CheckPhase(phase); err != nil {
		return DiffCommandFlags{}, fmt.Errorf("%w: %s: %s", ErrParsingFlag, "phase", err.Error())
	}

	gitProtocol := viper.GetString("git-protocol")

	typedGitProtocol, err := git.ParseProtocol(gitProtocol)
	if err != nil {
		return DiffCommandFlags{}, fmt.Errorf("%w: %w", ErrParsingFlag, err)
	}

	return DiffCommandFlags{
		ConfigOnly:            viper.GetBool("config-only"),
		SkipDepsDownload:      viper.GetBool("skip-deps-download"),
		SkipDepsValidation:    viper.GetBool("skip-deps-validation"),
		Debug:                 viper.GetBool("debug"),
		FuryctlPath:           viper.GetString("config"),
		DistroLocation:        viper.GetString("distro-location"),
		Phase:                 phase,
		NoTTY:                 viper.GetBool("no-tty"),
		GitProtocol:           typedGitProtocol,
		BinPath:               binPath,
		Outdir:                viper.GetString("outdir"),
		UpgradePathLocation:   viper.GetString("upgrade-path-location"),
		DistroPatchesLocation: distroPatchesLocation,
	}, nil
}
