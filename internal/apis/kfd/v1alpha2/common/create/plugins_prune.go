// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package create

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/furyctl/internal/apis/config"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/state"
	"github.com/sighupio/furyctl/internal/tool/shell"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	iox "github.com/sighupio/furyctl/internal/x/io"
)

// OperationPhasePluginsPrune is the work folder of the prune, next to the one of the plugins phase.
const OperationPhasePluginsPrune = "plugins-prune"

var (
	ErrPruneScriptMissing = errors.New(
		"the distribution in use has no templates/plugins/scripts/prune.sh.tpl, so --prune-plugins cannot " +
			"delete the removed plugins: use a distribution version that provides it, or keep the resources " +
			"outside of furyctl with --force plugins-unmanage",
	)
	ErrPruneCanceled = errors.New("prune of the removed plugins canceled by the user: nothing was deleted")
)

type scriptRunner interface {
	Run(args ...string) (string, error)
}

// PluginsPrune deletes the resources of the plugins removed from spec.plugins with the distribution's
// prune.sh, rendered from the configuration stored in the cluster: only that configuration declares them.
type PluginsPrune struct {
	*cluster.OperationPhase

	stateStore  state.Storer
	shellRunner scriptRunner
	confirm     func(force bool, msg string) (bool, error)
	removed     []RemovedPlugin
	paths       cluster.CreatorPaths
	kfd         config.KFD
	kind        string
	dryRun      bool
	force       []string
}

func NewPluginsPrune(
	paths cluster.CreatorPaths,
	kfdManifest config.KFD,
	kind string,
	dryRun bool,
	force []string,
	stateStore state.Storer,
	removed []RemovedPlugin,
) *PluginsPrune {
	phaseOp := cluster.NewOperationPhase(
		path.Join(paths.WorkDir, OperationPhasePluginsPrune),
		kfdManifest.Tools,
		paths.BinPath,
	)

	return &PluginsPrune{
		OperationPhase: phaseOp,
		stateStore:     stateStore,
		shellRunner: shell.NewRunner(
			execx.NewStdExecutor(),
			shell.Paths{
				Shell:   "sh",
				WorkDir: phaseOp.Path,
			},
		),
		confirm: cluster.AskConfirmationWithMessage,
		removed: removed,
		paths:   paths,
		kfd:     kfdManifest,
		kind:    kind,
		dryRun:  dryRun,
		force:   force,
	}
}

func (p *PluginsPrune) Exec() error {
	if len(p.removed) == 0 {
		logrus.Info("No plugin was removed from spec.plugins: nothing to prune")

		return nil
	}

	tpl := path.Join(p.paths.DistroPath, "templates", cluster.OperationPhasePlugins, "scripts", "prune.sh.tpl")
	if _, err := os.Stat(tpl); err != nil {
		return fmt.Errorf("%w: %w", ErrPruneScriptMissing, err)
	}

	if err := p.render(); err != nil {
		return err
	}

	targets := make([]string, len(p.removed))
	for i, r := range p.removed {
		targets[i] = r.Target()
	}

	script := path.Join(p.Path, "scripts", "prune.sh")

	plan, err := p.shellRunner.Run(append([]string{script, "plan"}, targets...)...)
	if err != nil {
		return fmt.Errorf("error while listing the resources of the removed plugins: %w", err)
	}

	logrus.Infof("Plugins removed from spec.plugins to prune:\n%s", plan)

	if p.dryRun {
		logrus.Info("Dry run: the removed plugins are not deleted")

		return nil
	}

	// Only --force all skips the question: --force migrations is for configuration changes, not deletions.
	ok, err := p.confirm(
		slices.Contains(p.force, cluster.ForceFeatureAll),
		"\nWARNING: You are about to delete the resources of the plugins listed above.",
	)
	if err != nil {
		return fmt.Errorf("error while asking for confirmation: %w", err)
	}

	if !ok {
		return ErrPruneCanceled
	}

	if _, err := p.shellRunner.Run(append([]string{script, "delete"}, targets...)...); err != nil {
		return fmt.Errorf("error while deleting the removed plugins: %w", err)
	}

	logrus.Info("Removed plugins pruned")

	return nil
}

// render renders templates/plugins from the stored configuration into the prune folder, never into the folder
// of the plugins phase.
func (p *PluginsPrune) render() error {
	stored, err := p.stateStore.GetConfig()
	if err != nil {
		return fmt.Errorf("error while reading the configuration stored in the cluster: %w", err)
	}

	dir, err := os.MkdirTemp("", "furyctl-plugins-prune-")
	if err != nil {
		return fmt.Errorf("error creating temp dir: %w", err)
	}

	defer os.RemoveAll(dir)

	// The stored configuration holds secrets: only its owner reads it, and only until the render ends.
	cfgPath := filepath.Join(dir, "furyctl.yaml")
	if err := os.WriteFile(cfgPath, stored, iox.FullRWPermAccess); err != nil {
		return fmt.Errorf("error writing the stored configuration: %w", err)
	}

	if _, err := renderPlugins(p.OperationPhase, p.paths, p.kfd, p.kind, cfgPath, p.dryRun); err != nil {
		return fmt.Errorf("error rendering the plugins of the stored configuration: %w", err)
	}

	return nil
}
