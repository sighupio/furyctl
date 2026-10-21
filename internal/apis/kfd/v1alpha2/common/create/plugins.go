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

	helmfileRunner helmfileRunner
	shellRunner    shellRunner
	dryRun         bool
	kfd            config.KFD
	kind           string
	paths          cluster.CreatorPaths
}

type helmfileRunner interface {
	Init(helmBinary string) error
	Apply() error
	Diff() (string, string, int, error)
}

type shellRunner interface {
	Run(args ...string) (string, error)
	RunCapture(args ...string) (string, int, error)
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

func (p *Plugins) Exec() error {
	logrus.Info("Applying plugins...")

	p.applied = false

	specPlugins, hasPlugins, err := p.render()
	if err != nil {
		return err
	}

	if !hasPlugins {
		logrus.Info("Skipping plugins phase as spec.plugins is not defined")

		return nil
	}

	p.applied = pluginsApplied(specPlugins)

	if p.dryRun {
		logrus.Info("Plugins installed successfully (dry-run mode)")

		return nil
	}

	if hasHelmReleases(specPlugins) {
		if err := p.helmfileRunner.Init(p.HelmPath); err != nil {
			return fmt.Errorf("error applying plugins with helmfile: %w", err)
		}

		if err := p.helmfileRunner.Apply(); err != nil {
			return fmt.Errorf("error applying plugins with helmfile: %w", err)
		}
	}

	if hasKustomizeEntries(specPlugins) {
		if _, err := p.shellRunner.Run(path.Join(p.Path, "scripts", "apply.sh"), "false"); err != nil {
			return fmt.Errorf("error applying plugins with kustomize: %w", err)
		}
	}

	logrus.Info("Plugins installed successfully")

	return nil
}

// render generates the plugins phase folder from the distribution templates. It returns the spec.plugins of the
// configuration, and whether it is defined.
func (p *Plugins) render() (map[any]any, bool, error) {
	if err := p.CreateRootFolder(); err != nil {
		return nil, false, fmt.Errorf("error creating plugins phase folder: %w", err)
	}

	furyctlMerger, err := p.CreateFuryctlMerger(
		p.paths.DistroPath,
		p.paths.ConfigPath,
		"kfd-v1alpha2",
		strings.ToLower(p.kind),
	)
	if err != nil {
		return nil, false, fmt.Errorf("error creating furyctl merger: %w", err)
	}

	mCfg, err := templatex.NewConfigWithoutData(furyctlMerger, []string{})
	if err != nil {
		return nil, false, fmt.Errorf("error creating template config: %w", err)
	}

	p.CopyPathsToConfig(&mCfg)

	if !distribution.HasFeature(p.kfd, distribution.FeatureKubeconfigInSchema) {
		mCfg.Data["paths"]["kubeconfig"] = os.Getenv("KUBECONFIG")
	}

	outDirPath1, confPath, err := p.writeTemplateConfig(mCfg)
	if err != nil {
		return nil, false, err
	}

	templateModel, err := templatex.NewTemplateModel(
		path.Join(p.paths.DistroPath, "templates", cluster.OperationPhasePlugins),
		p.Path,
		confPath,
		outDirPath1,
		p.paths.ConfigPath,
		".tpl",
		false,
		p.dryRun,
	)
	if err != nil {
		return nil, false, fmt.Errorf("error creating template model: %w", err)
	}

	err = templateModel.Generate()
	if err != nil {
		return nil, false, fmt.Errorf("error generating from template files: %w", err)
	}

	specPlugins, hasPlugins := templateModel.Config.Data["spec"]["plugins"].(map[any]any)

	return specPlugins, hasPlugins, nil
}

// writeTemplateConfig writes the template configuration in a new temp dir. It returns the dir and the file.
func (p *Plugins) writeTemplateConfig(mCfg templatex.Config) (string, string, error) {
	outYaml, err := yamlx.MarshalV2(mCfg)
	if err != nil {
		return "", "", fmt.Errorf("error marshaling template config: %w", err)
	}

	outDirPath1, err := os.MkdirTemp("", "furyctl-plugins-")
	if err != nil {
		return "", "", fmt.Errorf("error creating temp dir: %w", err)
	}

	// The commands that come next run for minutes. A signal skips a defer. On a dry run, the template
	// model writes tmpl-debug.log in this folder and tells the user its path.
	if !p.dryRun {
		osx.RegisterTempDir(outDirPath1)
	}

	confPath := filepath.Join(outDirPath1, "config.yaml")

	logrus.Debugf("Plugins configuration file path %s", confPath)

	if err = os.WriteFile(confPath, outYaml, iox.FullRWPermAccess); err != nil {
		return "", "", fmt.Errorf("error writing config file: %w", err)
	}

	return outDirPath1, confPath, nil
}

// pluginsApplied reports whether spec.plugins holds something to apply: a Helm release or a
// kustomize entry. Helm repositories alone apply nothing.
func pluginsApplied(specPlugins map[any]any) bool {
	return hasHelmReleases(specPlugins) || hasKustomizeEntries(specPlugins)
}
