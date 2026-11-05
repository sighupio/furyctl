// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/r3labs/diff/v3"
	"github.com/samber/lo"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/sighupio/furyctl/internal/airgap"
	"github.com/sighupio/furyctl/internal/apis/config"
	ekssupported "github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/ekscluster/supported"
	immutablesupported "github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/immutable/supported"
	distrosupported "github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/kfddistribution/supported"
	onpremsupported "github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/onpremises/supported"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/flags"
	"github.com/sighupio/furyctl/internal/git"
	"github.com/sighupio/furyctl/internal/state"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	"github.com/sighupio/furyctl/pkg/diffs"
	dist "github.com/sighupio/furyctl/pkg/distribution"
	rules "github.com/sighupio/furyctl/pkg/rulesextractor"
	netx "github.com/sighupio/furyctl/pkg/x/net"
	yamlx "github.com/sighupio/furyctl/pkg/x/yaml"
)

type DiffCommandFlags struct {
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
	ConfigOnly            bool
	SkipDepsDownload      bool
	SkipDepsValidation    bool
}

func NewDiffCmd() *cobra.Command {
	diffCmd := &cobra.Command{
		Args:  cobra.NoArgs,
		Use:   "diff",
		Short: "Show the changes an apply would make: the configuration and the resources of the cluster",
		Long: `Compare the configuration stored in the cluster with the local configuration file and print
the CONFIG table: one row for each changed field, sorted by field, with the columns FIELD,
CURRENT, NEW and RULE.

RULE tells what an apply does with the change, from the rules of the distribution: blocked
(the apply stops), needs confirmation (the apply asks before it continues) or migration (the
apply runs a migration). An empty RULE means that no rule matches the change, not that the
change has no effect.

The two configurations are compared as written, without the defaults of the distribution.
An apply compares the configurations with their defaults when the cluster stores one, so it
can decide differently for a field that only one of the two files sets.

A secret is printed as (hidden): the value of a field whose name contains pass, secret,
token, key, credential or authorization, of a field known to hold credentials (helm set
values, webhook URLs, inline file contents, the encryption configuration and other free
text), and the user and password of a URL.

Then, unless --config-only is set, the diff renders the distribution and plugins phases as
"furyctl apply --dry-run" does, with the same --phase, and compares them with the cluster: the
RESOURCES section lists what kapp and helm would create (+), update (~) or delete (-), without
the values of the helm releases. The dry run writes nothing to the cluster and takes the lock of
apply. A change that the apply would refuse is reported as not compared, with the reason, and the
exit code is 1. An upgrade (a change of distributionVersion) is not compared.
The kind EKSCluster, the phases infrastructure and kubernetes, and a distribution without the diff
scripts print the CONFIG table only, with exit code 0.`,
		PreRun: func(cmd *cobra.Command, _ []string) {
			// Bind the flags first: a flag on the command line has precedence over the configuration file.
			if err := viper.BindPFlags(cmd.Flags()); err != nil {
				logrus.Fatalf("error while binding flags: %v", err)
			}

			if err := flags.LoadAndMergeCommandFlags("diff"); err != nil {
				logrus.Fatalf("failed to load flags from configuration: %v", err)
			}
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			// Air-gapped: extract --airgap-bundle (if set) and rewire to run offline before reading flags.
			if err := airgap.MaybePrepare(); err != nil {
				return fmt.Errorf("error preparing air-gapped bundle: %w", err)
			}

			flags, err := getDiffCommandFlags()
			if err != nil {
				return err
			}

			execx.Debug = flags.Debug

			client := netx.NewGoGetterClient()

			distrodl := dist.NewDownloader(client, flags.GitProtocol, flags.DistroPatchesLocation)

			if flags.DistroLocation == "" {
				distrodl = dist.NewCachingDownloader(client, flags.Outdir, flags.GitProtocol, flags.DistroPatchesLocation)
			}

			logrus.Info("Downloading distribution...")
			res, err := distrodl.Download(flags.DistroLocation, flags.FuryctlPath)
			if err != nil {
				return fmt.Errorf("error while downloading distribution: %w", err)
			}

			basePath := filepath.Join(flags.Outdir, ".furyctl", res.MinimalConf.Metadata.Name)

			stateStore := state.NewStore(
				res.RepoPath,
				flags.FuryctlPath,
				basePath,
				res.DistroManifest.Tools.Common.Kubectl.Version,
				flags.BinPath,
			)

			phasePath, err := getPhasePath(
				flags.FuryctlPath,
				basePath,
				res.RepoPath,
				flags.BinPath,
				flags.Phase,
				res.MinimalConf,
				res.DistroManifest,
				flags.UpgradePathLocation,
			)
			if err != nil {
				return fmt.Errorf("error while getting phase path: %w", err)
			}

			if err := runDiff(diffRun{
				Out:         os.Stdout,
				Store:       stateStore,
				FuryctlPath: flags.FuryctlPath,
				DistroPath:  res.RepoPath,
				PhasePath:   phasePath,
			}); err != nil {
				return err
			}

			if flags.ConfigOnly {
				return nil
			}

			return diffResources(os.Stdout, flags, res, basePath)
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
		"Print only the CONFIG table, without comparing the resources of the cluster",
	)

	diffCmd.Flags().Bool(
		"skip-deps-download",
		false,
		"Skip downloading the distribution modules, installers and binaries",
	)

	diffCmd.Flags().Bool(
		"skip-deps-validation",
		false,
		"Skip validating dependencies",
	)

	airgap.RegisterFlags(diffCmd)

	return diffCmd
}

// diffRun holds what the comparison needs once the distribution is on disk.
type diffRun struct {
	// Out is where the CONFIG table is written.
	Out io.Writer
	// Store gives the configuration stored in the cluster.
	Store state.Storer
	// FuryctlPath is the path of the local configuration file.
	FuryctlPath string
	// DistroPath is the folder of the distribution, which holds the rules of the kind.
	DistroPath string
	// PhasePath limits the table to the changes under it. Empty means every change.
	PhasePath string
}

var (
	errDiffKind  = errors.New("no rule profile for the kind of the configuration")
	errDiffRules = errors.New("error while reading the rules of the distribution")
)

const (
	// The path of the distribution phase of every kind.
	diffDistributionPhasePath = ".spec.distribution"
	// The version of the distribution. It is not below
	// the path of the distribution phase, but an upgrade changes it together with the
	// distribution, so the distribution phase lists it.
	diffDistributionVersionPath = ".spec.distributionVersion"
)

// runDiff compares the stored configuration with the local file, both as written,
// and writes the CONFIG table.
func runDiff(run diffRun) error {
	diffChecker, err := createDiffChecker(run.Store, run.FuryctlPath)
	if err != nil {
		return fmt.Errorf("error while creating configuration diff checker: %w", err)
	}

	changelog, err := diffChecker.GenerateDiff()
	if err != nil {
		return fmt.Errorf("error while getting diffs: error while generating diff: %w", err)
	}

	verdicts, err := diffVerdicts(diffChecker, changelog, run.DistroPath, run.PhasePath)
	if err != nil {
		return err
	}

	if err := diffs.WriteTable(run.Out, verdicts); err != nil {
		return fmt.Errorf("error while printing the differences: %w", err)
	}

	return nil
}

// diffVerdicts gives the verdicts of the changes under the phase path. The verdicts are
// computed on the whole changelog and the rows are filtered afterwards, so that a rule
// whose conditions look at another field sees every change.
func diffVerdicts(
	diffChecker diffs.Checker,
	changelog diff.Changelog,
	distroPath string,
	phasePath string,
) ([]diffs.ChangeVerdict, error) {
	current, updated := diffChecker.GetCurrentConfig(), diffChecker.GetNewConfig()

	kind, isString := updated["kind"].(string)
	if !isString {
		return nil, fmt.Errorf("%w: %v", errDiffKind, updated["kind"])
	}

	profile, ok := diffs.ProfileForKind(kind)
	if !ok {
		return nil, fmt.Errorf("%w: %q", errDiffKind, kind)
	}

	// The rules get the stored configuration as the current one, as the preflight of
	// an apply does.
	extractor, err := diffRulesExtractor(kind, distroPath, current)
	if err != nil {
		if !errors.Is(err, rules.ErrReadingRulesFile) {
			return nil, fmt.Errorf("%w: %w", errDiffRules, err)
		}

		// Without rules the table has no verdict, as apply skips its rule checks when
		// the distribution has no rules file for the kind or the file cannot be read.
		leaves := lo.Filter(diffs.ExpandMapChanges(changelog), func(leaf diff.Change, _ int) bool {
			return isUnderPhase(leaf.Path, phasePath)
		})

		logrus.Warnf("The RULE column is empty, the rules of the distribution are not available: %v", err)

		return diffs.Undecided(leaves), nil
	}

	verdicts := lo.Filter(
		diffs.Verdicts(changelog, extractor, profile),
		func(verdict diffs.ChangeVerdict, _ int) bool {
			return isUnderPhase(verdict.Change.Path, phasePath)
		},
	)

	return verdicts, nil
}

// isUnderPhase reports whether a path is the phase path or below it. An empty phase
// path holds every path. The distribution phase also holds the version of the
// distribution.
func isUnderPhase(path []string, phasePath string) bool {
	joined := "." + strings.Join(path, ".")

	if phasePath == diffDistributionPhasePath && joined == diffDistributionVersionPath {
		return true
	}

	return phasePath == "" || joined == phasePath || strings.HasPrefix(joined, phasePath+".")
}

// diffRulesExtractor builds the rules extractor of the kind as its preflight does.
func diffRulesExtractor(kind, distroPath string, current map[string]any) (rules.Extractor, error) {
	var (
		extractor rules.Extractor
		err       error
	)

	switch kind {
	case distribution.OnPremisesKind:
		extractor, err = rules.NewOnPremClusterRulesExtractor(distroPath, current, onpremsupported.Phases())

	case distribution.ImmutableKind:
		extractor, err = rules.NewImmutableClusterRulesExtractor(distroPath, current, immutablesupported.Phases())

	case distribution.EKSClusterKind:
		extractor, err = rules.NewEKSClusterRulesExtractor(distroPath, current, ekssupported.Phases())

	case distribution.KFDDistributionKind:
		extractor, err = rules.NewDistroClusterRulesExtractor(distroPath, current, distrosupported.Phases())

	default:
		return nil, fmt.Errorf("%w: %q", errDiffKind, kind)
	}

	if err != nil {
		return nil, fmt.Errorf("error while creating rules builder: %w", err)
	}

	return extractor, nil
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
		ConfigOnly:            viper.GetBool("config-only"),
		SkipDepsDownload:      viper.GetBool("skip-deps-download"),
		SkipDepsValidation:    viper.GetBool("skip-deps-validation"),
	}, nil
}
