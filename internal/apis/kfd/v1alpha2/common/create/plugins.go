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

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/merge"
	"github.com/sighupio/furyctl/internal/template"
	"github.com/sighupio/furyctl/internal/tool/helmfile"
	"github.com/sighupio/furyctl/internal/tool/shell"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	yamlx "github.com/sighupio/furyctl/internal/x/yaml"
)

type Plugins struct {
	*cluster.OperationPhase
	helmfileRunner  *helmfile.Runner
	shellRunner     *shell.Runner
	distroPath      string
	furyctlConfPath string
	dryRun          bool
	kubeconfig      string
	kind            string
}

func NewPlugins(
	paths cluster.CreatorPaths,
	kfdManifest config.KFD,
	kind string,
	dryRun bool,
	kubeconfig string,
) (*Plugins, error) {
	distroDir := path.Join(paths.WorkDir, cluster.OperationPhasePlugins)

	phaseOp, err := cluster.NewOperationPhase(distroDir, kfdManifest.Tools, paths.BinPath)
	if err != nil {
		return nil, fmt.Errorf("error creating plugins phase: %w", err)
	}

	return &Plugins{
		OperationPhase:  phaseOp,
		distroPath:      paths.DistroPath,
		furyctlConfPath: paths.ConfigPath,
		dryRun:          dryRun,
		kubeconfig:      kubeconfig,
		kind:            kind,
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
	}, nil
}

func (p *Plugins) Exec() error {
	logrus.Info("Applying plugins...")

	if err := p.CreateFolder(); err != nil {
		return fmt.Errorf("error creating plugins phase folder: %w", err)
	}

	furyctlMerger, err := p.createFuryctlMerger()
	if err != nil {
		return err
	}

	mCfg, err := template.NewConfigWithoutData(furyctlMerger, []string{})
	if err != nil {
		return fmt.Errorf("error creating template config: %w", err)
	}

	mCfg.Data["paths"] = map[any]any{
		"helm":       p.HelmPath,
		"kustomize":  p.KustomizePath,
		"kubeconfig": p.kubeconfig,
		"kubectl":    p.KubectlPath,
		"yq":         p.YqPath,
	}

	outYaml, err := yamlx.MarshalV2(mCfg)
	if err != nil {
		return fmt.Errorf("error marshaling template config: %w", err)
	}

	outDirPath1, err := os.MkdirTemp("", "furyctl-plugins-")
	if err != nil {
		return fmt.Errorf("error creating temp dir: %w", err)
	}

	confPath := filepath.Join(outDirPath1, "config.yaml")

	logrus.Debugf("config path = %s", confPath)

	if err = os.WriteFile(confPath, outYaml, os.ModePerm); err != nil {
		return fmt.Errorf("error writing config file: %w", err)
	}

	templateModel, err := template.NewTemplateModel(
		path.Join(p.distroPath, "templates", cluster.OperationPhasePlugins),
		path.Join(p.Path),
		confPath,
		outDirPath1,
		p.furyctlConfPath,
		".tpl",
		false,
		p.dryRun,
	)
	if err != nil {
		return fmt.Errorf("error creating template model: %w", err)
	}

	err = templateModel.Generate()
	if err != nil {
		return fmt.Errorf("error generating from template files: %w", err)
	}

	specPlugins, hasPlugins := templateModel.Config.Data["spec"]["plugins"].(map[any]any)
	if !hasPlugins {
		logrus.Info("Skipping plugins phase as spec.plugins is not defined")

		return nil
	}

	specPluginsHelmReleases := []any{}

	specPluginsHelm, hasSpecPluginsHelm := specPlugins["helm"].(map[any]any)
	if hasSpecPluginsHelm {
		//nolint:errcheck // We don't care about the error here.
		specPluginsHelmReleases, _ = specPluginsHelm["releases"].([]any)
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
		if _, err := p.shellRunner.Run(path.Join(p.Path, "scripts", "apply.sh"), "false", p.kubeconfig); err != nil {
			return fmt.Errorf("error applying plugins with kustomize: %w", err)
		}
	}

	logrus.Info("Plugins installed successfully")

	return nil
}

func (p *Plugins) createFuryctlMerger() (*merge.Merger, error) {
	defaultsFilePath := path.Join(p.distroPath, "defaults", fmt.Sprintf("%s-kfd-v1alpha2.yaml", strings.ToLower(p.kind)))

	defaultsFile, err := yamlx.FromFileV2[map[any]any](defaultsFilePath)
	if err != nil {
		return &merge.Merger{}, fmt.Errorf("%s - %w", defaultsFilePath, err)
	}

	furyctlConf, err := yamlx.FromFileV2[map[any]any](p.furyctlConfPath)
	if err != nil {
		return &merge.Merger{}, fmt.Errorf("%s - %w", p.furyctlConfPath, err)
	}

	merger := merge.NewMerger(
		merge.NewDefaultModel(defaultsFile, ".data"),
		merge.NewDefaultModel(furyctlConf, ".spec.distribution"),
	)

	_, err = merger.Merge()
	if err != nil {
		return nil, fmt.Errorf("error merging furyctl config: %w", err)
	}

	reverseMerger := merge.NewMerger(
		*merger.GetCustom(),
		*merger.GetBase(),
	)

	_, err = reverseMerger.Merge()
	if err != nil {
		return nil, fmt.Errorf("error merging furyctl config: %w", err)
	}

	return reverseMerger, nil
}
