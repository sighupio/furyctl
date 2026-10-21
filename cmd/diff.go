// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"bytes"
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
	"golang.org/x/term"

	"github.com/sighupio/furyctl/internal/airgap"
	"github.com/sighupio/furyctl/internal/apis/config"
	commcreate "github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/common/create"
	ekssupported "github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/ekscluster/supported"
	immutablesupported "github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/immutable/supported"
	distrosupported "github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/kfddistribution/supported"
	onpremsupported "github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/onpremises/supported"
	"github.com/sighupio/furyctl/internal/cluster"
	icfg "github.com/sighupio/furyctl/internal/config"
	"github.com/sighupio/furyctl/internal/diffview"
	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/flags"
	"github.com/sighupio/furyctl/internal/git"
	"github.com/sighupio/furyctl/internal/lockfile"
	"github.com/sighupio/furyctl/internal/state"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	"github.com/sighupio/furyctl/pkg/dependencies"
	"github.com/sighupio/furyctl/pkg/diffs"
	dist "github.com/sighupio/furyctl/pkg/distribution"
	rules "github.com/sighupio/furyctl/pkg/rulesextractor"
	netx "github.com/sighupio/furyctl/pkg/x/net"
	yamlx "github.com/sighupio/furyctl/pkg/x/yaml"
)

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
	diffCmd := &cobra.Command{
		Args: cobra.NoArgs,
		Use:  "diff",
		Short: "Show the CONFIG table of the changes between the cluster configuration and the local one, " +
			"and the Kubernetes resources an apply would change",
		Long: `Compare the configuration stored in the cluster with the local configuration file and print
the CONFIG table: one row for each changed field, grouped by phase, with the columns FIELD,
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

Without --config-only, diff also shows the Kubernetes resources that apply would create, update
or delete. It runs the preflight, which can connect to the nodes and write admin.conf, and
renders into the same run dir as apply, so it takes the lock file of apply and stops when apply
holds it. It writes nothing to the cluster. It exits with 1 when a section could not be compared.
When a step fails before the resources are compared, diff still prints the CONFIG table first,
then why the resources were not compared.

With --config-only, diff prints only the CONFIG table: it runs no preflight and renders nothing.`,
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

			if flags.ConfigOnly {
				return runConfigOnlyDiff(flags)
			}

			return runFullDiff(newDiffSteps(flags))
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
		"Only print the CONFIG table, without running the preflight or rendering anything. Without it, diff "+
			"runs the preflight and renders into the same run dir as apply, with the lock file of apply",
	)

	airgap.RegisterFlags(diffCmd)

	return diffCmd
}

// runConfigOnlyDiff prints the CONFIG table and nothing else: no dependency, no preflight, no render.
func runConfigOnlyDiff(cmdFlags DiffCommandFlags) error {
	res, err := downloadDistribution(cmdFlags)
	if err != nil {
		return err
	}

	run, err := newDiffRun(cmdFlags, res, os.Stdout, diffTableOptions(os.Stdout))
	if err != nil {
		return err
	}

	return runDiff(run)
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

// newDiffRun gives what the CONFIG table of the downloaded distribution needs: the stored configuration of the
// cluster and the path of the phase, written to out.
func newDiffRun(
	cmdFlags DiffCommandFlags,
	res dist.DownloadResult,
	out io.Writer,
	table diffview.Options,
) (diffRun, error) {
	basePath := filepath.Join(cmdFlags.Outdir, ".furyctl", res.MinimalConf.Metadata.Name)

	stateStore := state.NewStore(
		res.RepoPath,
		cmdFlags.FuryctlPath,
		basePath,
		res.DistroManifest.Tools.Common.Kubectl.Version,
		cmdFlags.BinPath,
	)

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
		return diffRun{}, fmt.Errorf("error while getting phase path: %w", err)
	}

	return diffRun{
		Out:         out,
		Store:       stateStore,
		FuryctlPath: cmdFlags.FuryctlPath,
		DistroPath:  res.RepoPath,
		PhasePath:   phasePath,
		Table:       table,
	}, nil
}

// compareConfiguration builds the CONFIG table of the configuration file against the one stored in the cluster,
// for the phase, so that the report prints it in place of its Configuration section.
func compareConfiguration(cmdFlags DiffCommandFlags, res dist.DownloadResult) configurationResult {
	out := &bytes.Buffer{}

	run, err := newDiffRun(cmdFlags, res, out, diffTableOptions(os.Stdout))
	if err != nil {
		return configurationResult{Err: err}
	}

	if err := runDiff(run); err != nil {
		return configurationResult{Err: err}
	}

	return configurationResult{Table: out.String()}
}

// diffSteps are the steps of the full comparison, so that the tests can replace them.
type diffSteps struct {
	downloadDistribution func() (dist.DownloadResult, error)
	// Lock takes the lock file of apply, before anything is written in the run dir. It returns the function
	// that removes it.
	lock                 func(res dist.DownloadResult) (func(), error)
	downloadDependencies func(res dist.DownloadResult) error
	newCreator           func(res dist.DownloadResult) (cluster.Creator, error)
	compareConfiguration func(res dist.DownloadResult) configurationResult
	outOfScope           func(res dist.DownloadResult) []commcreate.Section
	// NotCompared gives the resource sections of the configuration, all failed with the reason, for a step
	// that failed before the kind could compare them.
	notCompared func(res dist.DownloadResult, reason string) []commcreate.Section
	print       func(report string)
}

func newDiffSteps(cmdFlags DiffCommandFlags) diffSteps {
	return diffSteps{
		downloadDistribution: func() (dist.DownloadResult, error) { return downloadDistribution(cmdFlags) },
		lock: func(res dist.DownloadResult) (func(), error) {
			return lockRunDir(res.MinimalConf.Metadata.Name)
		},
		downloadDependencies: func(res dist.DownloadResult) error { return downloadDiffDependencies(cmdFlags, res) },
		newCreator: func(res dist.DownloadResult) (cluster.Creator, error) {
			return newApplyDefaultsCreator(cluster.NewCreator, cmdFlags, res)
		},
		compareConfiguration: func(res dist.DownloadResult) configurationResult {
			return compareConfiguration(cmdFlags, res)
		},
		outOfScope: func(res dist.DownloadResult) []commcreate.Section {
			pluginsFeature := distribution.HasFeature(res.DistroManifest, distribution.FeaturePlugins)

			return eksOutOfScopeSections(cmdFlags.FuryctlPath, pluginsFeature)
		},
		notCompared: func(res dist.DownloadResult, reason string) []commcreate.Section {
			pluginsFeature := distribution.HasFeature(res.DistroManifest, distribution.FeaturePlugins)

			return notComparedSections(cmdFlags.FuryctlPath, cmdFlags.Phase, pluginsFeature, reason)
		},
		print: func(report string) { fmt.Printf("%s", report) },
	}
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
	// Table has the colour and the width of the terminal.
	Table diffview.Options
}

var (
	// ErrDiffNotCompared is returned, after the report was printed, when a section could not be compared. The
	// program exits with 1 without logging it, so that the final line of the report stays the last one.
	ErrDiffNotCompared = errors.New("some sections were not compared")

	errDiffKind  = errors.New("no rule profile for the kind of the configuration")
	errDiffRules = errors.New("error while reading the rules of the distribution")
)

const (
	// The reason why the resource sections of an EKSCluster are not compared.
	reasonEKSNotSupported = "EKSCluster is not supported by furyctl diff yet"
	// The path of the distribution phase of every kind.
	diffDistributionPhasePath = ".spec.distribution"
	// The version of the distribution. It is not below
	// the path of the distribution phase, but an upgrade changes it together with the
	// distribution, so the distribution phase lists it, in the group General.
	diffDistributionVersionPath = ".spec.distributionVersion"
	// The last line of a table whose kind has a rules file that cannot be read.
	diffUnreadableRulesLine = "RULE   not available: the rules file of the distribution cannot be read"
)

// diffTable is the CONFIG table and the line, if any, printed after its rows.
type diffTable struct {
	model diffview.Model
	note  string
}

// diffTableOptions gives the colour and the width the table gets on that output.
func diffTableOptions(out *os.File) diffview.Options {
	output := tableOutput{
		Terminal: false,
		Width:    0,
		NoTTY:    execx.NoTTY,
		Debug:    execx.AnimationDisabled(),
		NoColour: os.Getenv("NO_COLOR") != "",
	}

	if fd := int(out.Fd()); term.IsTerminal(fd) {
		output.Terminal = true

		if width, _, err := term.GetSize(fd); err == nil {
			output.Width = width
		}
	}

	return tableOptionsOf(output)
}

// tableOutput is what decides how the table is painted on an output.
type tableOutput struct {
	// Terminal is true when the output is a terminal, and Width is its width when known.
	Terminal bool
	Width    int
	// NoTTY is the --no-tty flag.
	NoTTY bool
	// Debug is true when live terminal output is turned off, by --no-tty or by debug logs.
	Debug bool
	// NoColour is true when the NO_COLOR environment variable is set and not empty.
	NoColour bool
}

// tableOptionsOf gives the options of the table. A file, a pipe and --no-tty get
// the table without colour and with every value in full. On a terminal, the table
// is cut to its width, and coloured unless NO_COLOR is set or debug logs are on.
func tableOptionsOf(output tableOutput) diffview.Options {
	if !output.Terminal || output.NoTTY {
		return diffview.Options{Colour: false, Width: 0}
	}

	return diffview.Options{Colour: !output.Debug && !output.NoColour, Width: output.Width}
}

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

	table, err := diffModel(diffChecker, changelog, run.DistroPath, run.PhasePath)
	if err != nil {
		return err
	}

	if err := diffview.Render(run.Out, table.model, run.Table); err != nil {
		return fmt.Errorf("error while printing the differences: %w", err)
	}

	// A table with no changes has no RULE column to explain.
	if table.note != "" && table.model.Changes > 0 {
		if _, err := fmt.Fprintf(run.Out, "\n%s\n", table.note); err != nil {
			return fmt.Errorf("error while printing the differences: %w", err)
		}
	}

	return nil
}

// diffModel builds the table of the changes under the phase path. The verdicts are
// computed on the whole changelog and the rows are filtered afterwards, so that a rule
// whose conditions look at another field sees every change.
func diffModel(
	diffChecker diffs.Checker,
	changelog diff.Changelog,
	distroPath string,
	phasePath string,
) (diffTable, error) {
	current, updated := diffChecker.GetCurrentConfig(), diffChecker.GetNewConfig()

	kind, isString := updated["kind"].(string)
	if !isString {
		return diffTable{}, fmt.Errorf("%w: %v", errDiffKind, updated["kind"])
	}

	profile, ok := diffs.ProfileForKind(kind)
	if !ok {
		return diffTable{}, fmt.Errorf("%w: %q", errDiffKind, kind)
	}

	// The rules get the stored configuration as the current one, as the preflight of
	// an apply does.
	extractor, rulesPath, err := diffRulesExtractor(kind, distroPath, current)
	if err != nil {
		if !errors.Is(err, rules.ErrReadingRulesFile) {
			return diffTable{}, fmt.Errorf("%w: %w", errDiffRules, err)
		}

		// Without rules the table has no verdict, as apply skips its rule checks when
		// the distribution has no rules file for the kind or the file cannot be read.
		leaves := lo.Filter(diffs.ExpandMapChanges(changelog), func(leaf diff.Change, _ int) bool {
			return isUnderPhase(leaf.Path, phasePath)
		})

		table := diffTable{model: diffview.NewModelWithoutRules(leaves, current, updated), note: ""}

		if _, statErr := os.Stat(rulesPath); errors.Is(statErr, os.ErrNotExist) {
			return table, nil
		}

		logrus.Warnf("The rules file %s cannot be read, the table has no verdict: %v", rulesPath, err)

		// The last line the table prints by itself says that the file is missing.
		table.model.RulesAvailable = true
		table.note = diffUnreadableRulesLine

		return table, nil
	}

	verdicts := lo.Filter(
		verdictsWithQuietLookups(changelog, extractor, profile),
		func(verdict diffs.ChangeVerdict, _ int) bool {
			return isUnderPhase(verdict.Change.Path, phasePath)
		},
	)

	return diffTable{model: diffview.NewModel(verdicts, current, updated), note: ""}, nil
}

// verdictsWithQuietLookups computes the verdicts with the error lines of the rules
// logged at debug level. The rules get the stored configuration as written, which can
// lack a field that a condition of a rule reads: the extractor logs an error line for
// each such lookup, while the condition is only not met. The other lines keep their
// level, and every line keeps its order.
func verdictsWithQuietLookups(
	changelog diff.Changelog,
	extractor rules.Extractor,
	profile diffs.KindProfile,
) []diffs.ChangeVerdict {
	logger := logrus.StandardLogger()
	captured := &diffLogCapture{entries: nil}

	output := logger.Out
	hooks := logger.ReplaceHooks(logrus.LevelHooks{})

	logger.AddHook(captured)
	logger.SetOutput(io.Discard)

	verdicts := diffs.Verdicts(changelog, extractor, profile)

	logger.SetOutput(output)
	logger.ReplaceHooks(hooks)

	for _, entry := range captured.entries {
		level := entry.Level
		if level == logrus.ErrorLevel {
			level = logrus.DebugLevel
		}

		logger.WithFields(entry.Data).Log(level, entry.Message)
	}

	return verdicts
}

// diffLogCapture keeps the log entries of every level.
type diffLogCapture struct {
	entries []logrus.Entry
}

func (*diffLogCapture) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (c *diffLogCapture) Fire(entry *logrus.Entry) error {
	c.entries = append(c.entries, *entry)

	return nil
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

// diffRulesExtractor builds the rules extractor of the kind as its preflight does, and
// gives the path of the rule file it reads.
func diffRulesExtractor(kind, distroPath string, current map[string]any) (rules.Extractor, string, error) {
	rulesPath := filepath.Join(distroPath, "rules", strings.ToLower(kind)+"-kfd-v1alpha2.yaml")

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
		return nil, rulesPath, fmt.Errorf("%w: %q", errDiffKind, kind)
	}

	if err != nil {
		return nil, rulesPath, fmt.Errorf("error while creating rules builder: %w", err)
	}

	return extractor, rulesPath, nil
}

// runFullDiff compares the configuration and the resources of the cluster and prints the report. It returns
// ErrDiffNotCompared, after the report, when a section failed. A step that fails after the lock is taken still
// gets the report: the configuration, then the resource sections with the error as the reason they were not
// compared.
func runFullDiff(steps diffSteps) error {
	res, err := steps.downloadDistribution()
	if err != nil {
		return err
	}

	unlock, err := steps.lock(res)
	if err != nil {
		return err
	}

	defer unlock()

	var (
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

	sections, err := diffResources(steps, res, compare)
	if err != nil {
		compare()

		steps.print(buildDiffReport(configuration, steps.notCompared(res, err.Error())).Text)

		return fmt.Errorf("%w: %w", ErrDiffNotCompared, err)
	}

	compare()

	report := buildDiffReport(configuration, sections)

	steps.print(report.Text)

	if report.Failed {
		return ErrDiffNotCompared
	}

	return nil
}

// diffResources runs the steps that lead to the resource sections: the dependencies, the creator and its Diff.
// It calls compare right after the preflight attempt, when Diff gets that far.
func diffResources(steps diffSteps, res dist.DownloadResult, compare func()) ([]commcreate.Section, error) {
	if err := steps.downloadDependencies(res); err != nil {
		return nil, err
	}

	creator, err := steps.newCreator(res)
	if err != nil {
		return nil, err
	}

	differ, ok := creator.(Differ)
	if !ok {
		return steps.outOfScope(res), nil
	}

	sections, err := differ.Diff(compare)
	if err != nil {
		return nil, fmt.Errorf("error while comparing the resources: %w", err)
	}

	return sections, nil
}

// lockRunDir takes the lock file that apply takes for the cluster, so that diff and apply never write in the run
// dir at the same time. It fails when the file exists. The function it returns removes the file; a signal also
// removes it.
func lockRunDir(clusterName string) (func(), error) {
	lockFileHandler := lockfile.NewLockFile(clusterName)

	if err := lockFileHandler.Verify(); err != nil {
		return nil, fmt.Errorf("error while verifying lock file %s: %w", lockFileHandler.Path, err)
	}

	if err := lockFileHandler.Create(); err != nil {
		return nil, fmt.Errorf("error while creating lock file %s: %w", lockFileHandler.Path, err)
	}

	remove := func() {
		logrus.Debugf("Removing lock file %s", lockFileHandler.Path)

		if err := lockFileHandler.Remove(); err != nil {
			logrus.Errorf("error while removing lock file %s: %v", lockFileHandler.Path, err)
		}
	}

	execx.OnStop(remove)

	return func() {
		execx.OnStop(nil)
		remove()
	}, nil
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

// notComparedSections are the resource sections that the phase compares, all failed with the reason.
func notComparedSections(furyctlPath, phase string, pluginsFeature bool, reason string) []commcreate.Section {
	conf, err := yamlx.FromFileV3[map[string]any](furyctlPath)
	if err != nil {
		logrus.Debugf("error while reading the plugins of the configuration file: %v", err)
	}

	return commcreate.FailedSections(phase, conf, pluginsFeature, reason)
}

// diffLines splits a text into its lines that are not empty.
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
