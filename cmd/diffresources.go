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
	"regexp"
	"slices"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/lockfile"
	"github.com/sighupio/furyctl/internal/tool/helmfile"
	"github.com/sighupio/furyctl/internal/tool/kapp"
	"github.com/sighupio/furyctl/internal/tool/shell"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	"github.com/sighupio/furyctl/pkg/dependencies"
	dist "github.com/sighupio/furyctl/pkg/distribution"
	netx "github.com/sighupio/furyctl/pkg/x/net"
	yamlx "github.com/sighupio/furyctl/pkg/x/yaml"
)

const (
	// The exit code of `helmfile diff --detailed-exitcode` when a release changes.
	helmfileDiffChanges = 2
	// The timeouts apply uses by default.
	diffProcessTimeout         = 3600
	diffPodRunningCheckTimeout = 300
)

var (
	// The error of a diff that could not compare a resource section.
	errDiffNotCompared = errors.New("the resources could not be compared")
	// The error kapp reported in its diff output.
	errKappDiff = errors.New("kapp diff failed")
	// The header helm-diff prints before the changes of an object: "<namespace>, <name>, <kind> (<group>) has
	// changed:", or "has been added:" or "has been removed:".
	helmDiffHeader = regexp.MustCompile(
		`^(?P<namespace>\S*), (?P<name>\S+), (?P<kind>\S+) \([^)]*\) has (?P<op>changed|been added|been removed):$`,
	)
	ansiEscape = regexp.MustCompile(`\x1b\[[0-9;]*m`)
)

// diffResources compares the resources of the distribution and plugins phases with the cluster. The phases are
// rendered by `furyctl apply --dry-run [--phase <phase>]`, which writes nothing to the cluster, and then compared by
// the scripts/diff.sh the distribution renders for each of them.
func diffResources(out io.Writer, flags DiffCommandFlags, res dist.DownloadResult, basePath string) error {
	if res.MinimalConf.Kind == distribution.EKSClusterKind {
		return writeOut(out, "\nRESOURCES   not compared for kind %s\n", res.MinimalConf.Kind)
	}

	phases := diffPhases(flags.Phase)
	if len(phases) == 0 {
		return writeOut(out, "\nRESOURCES   not compared for the %s phase\n", flags.Phase)
	}

	for _, phase := range phases {
		if _, err := os.Stat(filepath.Join(res.RepoPath, "templates", phase, "scripts", "diff.sh.tpl")); err != nil {
			return writeOut(out, "\nRESOURCES   not supported by distribution %s\n", res.DistroManifest.Version)
		}
	}

	lock := lockfile.NewLockFile(res.MinimalConf.Metadata.Name)
	if err := lock.Verify(); err != nil {
		return fmt.Errorf("error while verifying lock file %s: %w", lock.Path, err)
	}

	if err := lock.Create(); err != nil {
		return fmt.Errorf("error while creating lock file %s: %w", lock.Path, err)
	}
	defer func() {
		if err := lock.Remove(); err != nil {
			logrus.Errorf("error while removing lock file %s: %v", lock.Path, err)
		}
	}()

	// A signal exits without running the deferred calls: the lock must not stay for the next apply.
	execx.OnStop(func() {
		if err := lock.Remove(); err != nil {
			logrus.Errorf("error while removing lock file %s: %v", lock.Path, err)
		}
	})
	defer execx.OnStop(nil)

	if err := diffDependencies(flags, res, basePath); err != nil {
		return err
	}

	configPath, err := filepath.Abs(flags.FuryctlPath)
	if err != nil {
		return fmt.Errorf("error while getting absolute path of the configuration file: %w", err)
	}

	paths := cluster.CreatorPaths{
		ConfigPath: configPath,
		WorkDir:    basePath,
		DistroPath: res.RepoPath,
		BinPath:    flags.BinPath,
	}

	// A file left by an earlier diff must not be read as the result of this one.
	for _, phase := range phases {
		stale, err := filepath.Glob(filepath.Join(diffWorkDir(paths.WorkDir, phase), "kapp-diff*.json"))
		if err != nil {
			return fmt.Errorf("error while listing the kapp diff files: %w", err)
		}

		for _, file := range stale {
			if err := os.Remove(file); err != nil {
				return fmt.Errorf("error while removing %s: %w", file, err)
			}
		}
	}

	if err := dryRunApply(paths, flags, res); err != nil {
		if werr := writeOut(out, "\nRESOURCES   not compared: %v\n", err); werr != nil {
			return werr
		}

		return errDiffNotCompared
	}

	compared := true

	for _, phase := range phases {
		rows, err := diffPhase(phase, paths.WorkDir)
		if err != nil {
			compared = false

			if werr := writeOut(out, "\nRESOURCES   %s: not compared: %v\n", phase, err); werr != nil {
				return werr
			}

			continue
		}

		if err := writeResources(out, phase, rows); err != nil {
			return err
		}

		if phase != cluster.OperationPhasePlugins || !hasHelmReleases(configPath) {
			continue
		}

		helmCompared, err := writeHelmReleases(out, paths, res)
		if err != nil {
			return err
		}

		compared = compared && helmCompared
	}

	if !compared {
		return errDiffNotCompared
	}

	return nil
}

// diffPhases gives the phases whose resources the diff compares for the --phase flag.
func diffPhases(phase string) []string {
	switch phase {
	case "":
		return []string{cluster.OperationPhaseDistribution, cluster.OperationPhasePlugins}

	case cluster.OperationPhaseDistribution, cluster.OperationPhasePlugins:
		return []string{phase}

	default:
		return nil
	}
}

// diffDependencies downloads and validates the tools the phases run, as apply does.
func diffDependencies(flags DiffCommandFlags, res dist.DownloadResult, basePath string) error {
	if !flags.SkipDepsDownload {
		depsdl := dependencies.NewCachingDownloader(
			netx.NewGoGetterClient(), flags.Outdir, basePath, flags.BinPath, flags.GitProtocol,
		)

		if errs, _ := depsdl.DownloadAll(res.DistroManifest, res.MinimalConf.Kind); len(errs) > 0 {
			return fmt.Errorf("%w: %v", ErrDownloadDependenciesFailed, errs)
		}
	}

	if !flags.SkipDepsValidation {
		depsvl := dependencies.NewValidator(execx.NewStdExecutor(), flags.BinPath, flags.FuryctlPath)

		if err := depsvl.Validate(res); err != nil {
			return fmt.Errorf("error while validating dependencies: %w", err)
		}
	}

	return nil
}

// dryRunApply renders the phases as `furyctl apply --dry-run` does, with the same --phase.
func dryRunApply(paths cluster.CreatorPaths, flags DiffCommandFlags, res dist.DownloadResult) error {
	creator, err := cluster.NewCreator(
		res.MinimalConf,
		res.DistroManifest,
		paths,
		flags.Phase,
		true,  // Skip the VPN.
		false, // No VPN auto connect.
		false, // Do not skip the nodes upgrade.
		true,  // Dry run.
		// A dry run applies nothing: the confirmations of unsafe changes would only stop the diff, whose CONFIG
		// table already shows them as "needs confirmation". The immutable and unsupported checks still run.
		[]string{cluster.ForceFeatureMigrations},
		false, // No upgrade.
		flags.UpgradePathLocation,
		"",
		[]string{},
	)
	if err != nil {
		return fmt.Errorf("error while initializing the dry-run apply: %w", err)
	}

	if err := creator.Create("", diffProcessTimeout, diffPodRunningCheckTimeout); err != nil {
		return fmt.Errorf("the dry-run apply failed: %w", err)
	}

	return nil
}

// diffWorkDir is the folder the diff script of the phase runs in and writes its kapp files to.
func diffWorkDir(workDir, phase string) string {
	if phase == cluster.OperationPhaseDistribution {
		return filepath.Join(workDir, phase, "manifests")
	}

	return filepath.Join(workDir, phase)
}

// diffPhase runs the diff script of a rendered phase. It gives the changed resources.
func diffPhase(phase, baseDir string) ([]kapp.Row, error) {
	workDir := diffWorkDir(baseDir, phase)

	runner := shell.NewRunner(execx.NewStdExecutor(), shell.Paths{Shell: "sh", WorkDir: workDir})
	if _, err := runner.Run(filepath.Join(baseDir, phase, "scripts", "diff.sh")); err != nil {
		return nil, fmt.Errorf("scripts/diff.sh failed: %w", err)
	}

	files, err := filepath.Glob(filepath.Join(workDir, "kapp-diff*.json"))
	if err != nil {
		return nil, fmt.Errorf("error while listing the kapp diff files: %w", err)
	}

	slices.Sort(files)

	var rows []kapp.Row

	for _, file := range files {
		result, err := kapp.ParseDiff(file)
		if err != nil {
			if result.KappError != "" {
				return nil, fmt.Errorf("%w: %s", errKappDiff, result.KappError)
			}

			return nil, fmt.Errorf("error while reading %s: %w", filepath.Base(file), err)
		}

		rows = append(rows, result.Changes()...)
	}

	return rows, nil
}

// writeResources writes the changed resources of a phase, one per line.
func writeResources(out io.Writer, phase string, rows []kapp.Row) error {
	if len(rows) == 0 {
		return writeOut(out, "\nRESOURCES   %s: no changes\n", phase)
	}

	noun := "changes"
	if len(rows) == 1 {
		noun = "change"
	}

	if err := writeOut(out, "\nRESOURCES   %s: %d %s\n", phase, len(rows), noun); err != nil {
		return err
	}

	markers := map[string]string{"create": "+", "update": "~", "delete": "-"}

	for _, row := range rows {
		name := row.Name
		if row.Namespace != "" && row.Namespace != "(cluster)" {
			name = row.Namespace + "/" + row.Name
		}

		if err := writeOut(out, "  %s %s %s\n", markers[row.Op], row.Kind, name); err != nil {
			return err
		}
	}

	return nil
}

// hasHelmReleases reports whether the configuration file lists helm releases in spec.plugins.helm.releases.
func hasHelmReleases(configPath string) bool {
	conf, err := yamlx.FromFileV3[map[string]any](configPath)
	if err != nil {
		return false
	}

	var node any = conf

	for _, key := range []string{"spec", "plugins", "helm", "releases"} {
		parent, ok := node.(map[string]any)
		if !ok {
			return false
		}

		node = parent[key]
	}

	releases, ok := node.([]any)

	return ok && len(releases) > 0
}

// writeHelmReleases writes the objects that the helm releases change. It reports whether they were compared.
func writeHelmReleases(out io.Writer, paths cluster.CreatorPaths, res dist.DownloadResult) (bool, error) {
	rows, changed, err := diffHelmReleases(paths, res)
	if err != nil {
		return false, writeOut(out, "\nRESOURCES   helm releases: not compared: %v\n", err)
	}

	if changed && len(rows) == 0 {
		return true, writeOut(out, "\nRESOURCES   helm releases: changes, not listed\n")
	}

	return true, writeResources(out, "helm releases", rows)
}

// diffHelmReleases runs `helmfile diff` in the rendered plugins phase. It gives the objects that change and whether
// a release changes. The output of helmfile is never printed: --suppress-secrets hides only the Secret objects, and a
// value of a release can be in any other object.
func diffHelmReleases(paths cluster.CreatorPaths, res dist.DownloadResult) ([]kapp.Row, bool, error) {
	phase := cluster.NewOperationPhase(filepath.Join(paths.WorkDir, cluster.OperationPhasePlugins),
		res.DistroManifest.Tools, paths.BinPath)

	// The helm-diff plugin is installed by helmfile init, which a dry-run apply does not run.
	if err := helmfile.NewRunner(execx.NewStdExecutor(), helmfile.Paths{
		Helmfile:   phase.HelmfilePath,
		WorkDir:    phase.Path,
		PluginsDir: filepath.Join(paths.BinPath, "helm", "plugins"),
	}).Init(phase.HelmPath); err != nil {
		return nil, false, fmt.Errorf("helmfile init failed: %w", err)
	}

	// Sensitive: the output, which can hold values of the releases, is not written to the terminal, to the log
	// file or to a file of failed commands (exit code 2 means changes, not a failure).
	cmd := execx.NewCmd(phase.HelmfilePath, execx.CmdOptions{
		Args:      []string{"diff", "--detailed-exitcode", "--suppress-secrets"},
		Executor:  execx.NewStdExecutor(),
		WorkDir:   phase.Path,
		Env:       []string{"HELMFILE_UPGRADE_NOTICE_DISABLED=1"},
		Sensitive: true,
	})

	out, err := execx.CombinedOutput(cmd)
	if err == nil {
		return nil, false, nil
	}

	if cmd.ProcessState != nil && cmd.ProcessState.ExitCode() == helmfileDiffChanges {
		return helmDiffRows(out), true, nil
	}

	return nil, false, fmt.Errorf("helmfile diff failed: %w", err)
}

// helmDiffRows gives the objects of the headers of a helm-diff output, and nothing of the changes themselves.
func helmDiffRows(out string) []kapp.Row {
	ops := map[string]string{"been added": "create", "changed": "update", "been removed": "delete"}

	var rows []kapp.Row

	for line := range strings.SplitSeq(ansiEscape.ReplaceAllString(out, ""), "\n") {
		m := helmDiffHeader.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}

		rows = append(rows, kapp.Row{
			Namespace: m[helmDiffHeader.SubexpIndex("namespace")],
			Name:      m[helmDiffHeader.SubexpIndex("name")],
			Kind:      m[helmDiffHeader.SubexpIndex("kind")],
			Op:        ops[m[helmDiffHeader.SubexpIndex("op")]],
		})
	}

	return rows
}

// writeOut writes one line of the RESOURCES section.
func writeOut(out io.Writer, format string, a ...any) error {
	if _, err := fmt.Fprintf(out, format, a...); err != nil {
		return fmt.Errorf("error while writing the diff: %w", err)
	}

	return nil
}
