// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package create

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/furyctl/internal/apis/config"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/tool/helmfile"
	"github.com/sighupio/furyctl/internal/tool/shell"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	iox "github.com/sighupio/furyctl/internal/x/io"
	osx "github.com/sighupio/furyctl/internal/x/os"
	templatex "github.com/sighupio/furyctl/pkg/template"
	yamlx "github.com/sighupio/furyctl/pkg/x/yaml"
)

type Plugins struct {
	*cluster.OperationPhase

	// True when the last Exec applied plugins.
	applied bool

	helmfileRunner *helmfile.Runner
	shellRunner    *shell.Runner
	dryRun         bool
	kfd            config.KFD
	kind           string
	paths          cluster.CreatorPaths

	// When set, prune runs once before the next Exec applies the plugins.
	prune *PluginsPrune
}

func NewPlugins(
	paths cluster.CreatorPaths,
	kfdManifest config.KFD,
	kind string,
	dryRun bool,
) *Plugins {
	phaseOp := cluster.NewOperationPhase(
		path.Join(paths.WorkDir, cluster.OperationPhasePlugins),
		kfdManifest.Tools,
		paths.BinPath,
	)

	return &Plugins{
		OperationPhase: phaseOp,
		dryRun:         dryRun,
		kind:           kind,
		helmfileRunner: helmfile.NewRunner(
			execx.NewStdExecutor(),
			helmfile.Paths{
				Helmfile:   phaseOp.HelmfilePath,
				WorkDir:    phaseOp.Path,
				PluginsDir: path.Join(paths.BinPath, "helm", "plugins"),
			},
		),
		shellRunner: shell.NewRunner(
			execx.NewStdExecutor(),
			shell.Paths{
				Shell:   "sh",
				WorkDir: phaseOp.Path,
			},
		),
		kfd:   kfdManifest,
		paths: paths,
	}
}

// Applied reports whether the last Exec applied plugins.
func (p *Plugins) Applied() bool {
	return p.applied
}

// PruneFirst makes the next Exec delete the removed plugins before it applies the plugins: the prune runs
// right before the plugins phase, so a failed earlier phase deletes nothing.
func (p *Plugins) PruneFirst(prune *PluginsPrune) {
	p.prune = prune
}

func (p *Plugins) Exec() error {
	if p.prune != nil {
		if err := p.prune.Exec(); err != nil {
			return err
		}

		p.prune = nil
	}

	logrus.Info("Applying plugins...")

	p.applied = false

	templateModel, err := renderPlugins(p.OperationPhase, p.paths, p.kfd, p.kind, p.paths.ConfigPath, p.dryRun)
	if err != nil {
		return err
	}

	specPlugins, hasPlugins := templateModel.Config.Data["spec"]["plugins"].(map[any]any)
	if !hasPlugins {
		logrus.Info("Skipping plugins phase as spec.plugins is not defined")

		return nil
	}

	p.applied = pluginsApplied(specPlugins)

	specPluginsHelmReleases := []any{}

	specPluginsHelm, hasSpecPluginsHelm := specPlugins["helm"].(map[any]any)
	if hasSpecPluginsHelm {
		releases, hasSpecPluginHelmReleases := specPluginsHelm["releases"].([]any)
		if hasSpecPluginHelmReleases {
			specPluginsHelmReleases = releases
		}
	}

	specPluginsKustomize, hasSpecPluginsKustomize := specPlugins["kustomize"].([]any)

	if p.dryRun {
		logrus.Info("Plugins installed successfully (dry-run mode)")

		return nil
	}

	if hasSpecPluginsHelm && len(specPluginsHelmReleases) > 0 {
		if err := p.helmfileRunner.Init(p.HelmPath); err != nil {
			return fmt.Errorf("error applying plugins with helmfile: %w", err)
		}

		if err := p.helmfileRunner.Apply(); err != nil {
			return fmt.Errorf("error applying plugins with helmfile: %w", err)
		}
	}

	if hasSpecPluginsKustomize && len(specPluginsKustomize) > 0 {
		if _, err := p.shellRunner.Run(path.Join(p.Path, "scripts", "apply.sh"), "false"); err != nil {
			return fmt.Errorf("error applying plugins with kustomize: %w", err)
		}
	}

	logrus.Info("Plugins installed successfully")

	return nil
}

// pluginsApplied reports whether spec.plugins holds something to apply: a Helm release or a
// kustomize entry. Helm repositories alone apply nothing.
func pluginsApplied(specPlugins map[any]any) bool {
	if helm, ok := specPlugins["helm"].(map[any]any); ok {
		if releases, ok := helm["releases"].([]any); ok && len(releases) > 0 {
			return true
		}
	}

	kustomize, ok := specPlugins["kustomize"].([]any)

	return ok && len(kustomize) > 0
}

// renderPlugins renders the distribution's templates/plugins into the folder of op, from the furyctl
// configuration in cfgPath merged with the distribution defaults. Relative paths of the configuration resolve
// from the folder of paths.ConfigPath.
func renderPlugins( //nolint:revive // A dry run keeps the debug folder of the template model.
	op *cluster.OperationPhase,
	paths cluster.CreatorPaths,
	kfd config.KFD,
	kind string,
	cfgPath string,
	dryRun bool,
) (*templatex.Model, error) {
	if err := op.CreateRootFolder(); err != nil {
		return nil, fmt.Errorf("error creating plugins phase folder: %w", err)
	}

	furyctlMerger, err := op.CreateFuryctlMerger(
		paths.DistroPath,
		cfgPath,
		"kfd-v1alpha2",
		strings.ToLower(kind),
	)
	if err != nil {
		return nil, fmt.Errorf("error creating furyctl merger: %w", err)
	}

	mCfg, err := templatex.NewConfigWithoutData(furyctlMerger, []string{})
	if err != nil {
		return nil, fmt.Errorf("error creating template config: %w", err)
	}

	op.CopyPathsToConfig(&mCfg)

	if !distribution.HasFeature(kfd, distribution.FeatureKubeconfigInSchema) {
		mCfg.Data["paths"]["kubeconfig"] = os.Getenv("KUBECONFIG")
	}

	outYaml, err := yamlx.MarshalV2(mCfg)
	if err != nil {
		return nil, fmt.Errorf("error marshaling template config: %w", err)
	}

	outDirPath1, err := os.MkdirTemp("", "furyctl-plugins-")
	if err != nil {
		return nil, fmt.Errorf("error creating temp dir: %w", err)
	}

	// The commands that come next run for minutes. A signal skips a defer. On a dry run, the template
	// model writes tmpl-debug.log in this folder and tells the user its path.
	if !dryRun {
		osx.RegisterTempDir(outDirPath1)
	}

	confPath := filepath.Join(outDirPath1, "config.yaml")

	// The dry run keeps the folder for tmpl-debug.log, not the configuration file: it holds the secrets of the
	// configuration. The folder goes too when the template model wrote nothing in it.
	if dryRun {
		defer func() {
			_ = os.Remove(confPath)
			_ = os.Remove(outDirPath1)
		}()
	}

	logrus.Debugf("Plugins configuration file path %s", confPath)

	if err = os.WriteFile(confPath, outYaml, iox.FullRWPermAccess); err != nil {
		return nil, fmt.Errorf("error writing config file: %w", err)
	}

	templateModel, err := templatex.NewTemplateModel(
		path.Join(paths.DistroPath, "templates", cluster.OperationPhasePlugins),
		op.Path,
		confPath,
		outDirPath1,
		paths.ConfigPath,
		".tpl",
		false,
		dryRun,
	)
	if err != nil {
		return nil, fmt.Errorf("error creating template model: %w", err)
	}

	if err := templateModel.Generate(); err != nil {
		return nil, fmt.Errorf("error generating from template files: %w", err)
	}

	return templateModel, nil
}
