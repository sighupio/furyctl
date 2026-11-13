// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package del

import (
	"fmt"
	"path"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/furyctl/internal/apis/config"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/tool/shell"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	templatex "github.com/sighupio/furyctl/pkg/template"
)

// ShellRunner is shell.Runner.
type ShellRunner interface {
	Run(args ...string) (string, error)
}

// Script renders templates/<phase> of the distribution in the work dir and runs its scripts/delete.sh, the way
// onpremises/delete/distribution.go does: the delete logic of each distribution version lives in its templates.
type Script struct {
	*cluster.OperationPhase

	name   string
	paths  cluster.DeleterPaths
	dryRun bool
	shell  ShellRunner
}

// NewDistribution deletes with templates/distribution/scripts/delete.sh. A nil runner is the real shell.
func NewDistribution(kfdManifest config.KFD, paths cluster.DeleterPaths, dryRun bool, runner ShellRunner) *Script {
	return newScript(cluster.OperationPhaseDistribution, kfdManifest, paths, dryRun, runner)
}

// NewPlugins deletes the plugins with templates/plugins/scripts/delete.sh. A nil runner is the real shell.
func NewPlugins(kfdManifest config.KFD, paths cluster.DeleterPaths, dryRun bool, runner ShellRunner) *Script {
	return newScript(cluster.OperationPhasePlugins, kfdManifest, paths, dryRun, runner)
}

func newScript(
	name string, kfdManifest config.KFD, paths cluster.DeleterPaths, dryRun bool, runner ShellRunner,
) *Script {
	phase := cluster.NewOperationPhase(path.Join(paths.WorkDir, name), kfdManifest.Tools, paths.BinPath)

	if runner == nil {
		runner = shell.NewRunner(execx.NewStdExecutor(), shell.Paths{Shell: "sh", WorkDir: phase.Path})
	}

	return &Script{OperationPhase: phase, name: name, paths: paths, dryRun: dryRun, shell: runner}
}

// Exec renders the templates and, outside a dry run, runs scripts/delete.sh with args.
func (s *Script) Exec(args ...string) error {
	if err := s.render(); err != nil {
		return err
	}

	script := path.Join(s.Path, "scripts", "delete.sh")

	if s.dryRun {
		logrus.Infof("Rendered %s, not run (dry-run mode)", script)

		return nil
	}

	out, err := s.shell.Run(append([]string{script}, args...)...)
	if err != nil {
		return fmt.Errorf("error deleting the %s: %w", s.name, err)
	}

	warnStuck(out)

	return nil
}

func (s *Script) render() error {
	if err := s.CreateRootFolder(); err != nil {
		return fmt.Errorf("error creating %s phase folder: %w", s.name, err)
	}

	merger, err := s.CreateFuryctlMerger(s.paths.DistroPath, s.paths.ConfigPath, "kfd-v1alpha2", "immutable")
	if err != nil {
		return fmt.Errorf("error creating furyctl merger: %w", err)
	}

	cfg, err := templatex.NewConfigWithoutData(merger, []string{"terraform", ".gitignore", "manifests/aws"})
	if err != nil {
		return fmt.Errorf("error creating template config: %w", err)
	}

	s.CopyPathsToConfig(&cfg)

	// The manifests are rendered for the script, which does not read them, so no storage class check is needed.
	cfg.Data["checks"] = map[any]any{"storageClassAvailable": true}

	if err := s.CopyKFDToConfig(&cfg, s.paths.DistroPath); err != nil {
		return fmt.Errorf("error copying kfd to config: %w", err)
	}

	if err := s.CopyFromTemplate(
		cfg, s.name, path.Join(s.paths.DistroPath, "templates", s.name), s.Path, s.paths.ConfigPath,
	); err != nil {
		return fmt.Errorf("error rendering the %s templates: %w", s.name, err)
	}

	return nil
}

// warnStuck keeps the objects the script left stuck on a finalizer: the output region of the script is cleared when
// it ends.
func warnStuck(out string) {
	for line := range strings.SplitSeq(out, "\n") {
		if strings.HasPrefix(line, "stuck: ") {
			logrus.Warn(line)
		}
	}
}
