// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package del

import (
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/sighupio/furyctl/internal/apis/config"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/tool/helmfile"
	"github.com/sighupio/furyctl/internal/tool/kapp"
	"github.com/sighupio/furyctl/internal/tool/kubectl"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	iox "github.com/sighupio/furyctl/internal/x/io"
	osx "github.com/sighupio/furyctl/internal/x/os"
	templatex "github.com/sighupio/furyctl/pkg/template"
	yamlx "github.com/sighupio/furyctl/pkg/x/yaml"
)

// NewPluginsPhase builds the plugins delete with the real tools, in the plugins folder that apply uses.
func NewPluginsPhase(paths cluster.DeleterPaths, kfdManifest config.KFD, dryRun bool) *Plugins {
	phase := cluster.NewOperationPhase(path.Join(paths.WorkDir, cluster.OperationPhasePlugins), kfdManifest.Tools,
		paths.BinPath)
	std := execx.NewStdExecutor()

	runners := PluginsRunners{
		Kapp:    kapp.NewRunner(std, kapp.Paths{Kapp: phase.KappPath, WorkDir: phase.Path}, false),
		Kubectl: kubectl.NewRunner(std, kubectl.Paths{Kubectl: phase.KubectlPath, WorkDir: phase.Path}, false, true, false),
		Helm:    helmCLI{executor: std, helm: phase.HelmPath, workDir: phase.Path},
		Helmfile: helmfile.NewRunner(std, helmfile.Paths{
			Helmfile:   phase.HelmfilePath,
			WorkDir:    phase.Path,
			PluginsDir: path.Join(paths.BinPath, "helm", "plugins"),
		}),
	}

	return NewPlugins(runners, func() (map[any]any, error) { return render(phase, paths, kfdManifest, dryRun) },
		kfdManifest, dryRun)
}

// render renders templates/plugins as common/create/plugins.go does, which four kinds share and stays unchanged.
func render(
	phase *cluster.OperationPhase, paths cluster.DeleterPaths, kfdManifest config.KFD, dryRun bool,
) (map[any]any, error) {
	if err := phase.CreateRootFolder(); err != nil {
		return nil, fmt.Errorf("error creating plugins phase folder: %w", err)
	}

	out, err := templateConfig(phase, paths, kfdManifest)
	if err != nil {
		return nil, err
	}

	tmpDir, err := os.MkdirTemp("", "furyctl-plugins-")
	if err != nil {
		return nil, fmt.Errorf("error creating temp dir: %w", err)
	}

	// The config file holds the secrets of the configuration; a signal skips the defer.
	osx.RegisterTempDir(tmpDir)
	defer os.RemoveAll(tmpDir)

	confPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(confPath, out, iox.FullRWPermAccess); err != nil {
		return nil, fmt.Errorf("error writing config file: %w", err)
	}

	model, err := templatex.NewTemplateModel(
		path.Join(paths.DistroPath, "templates", cluster.OperationPhasePlugins),
		phase.Path, confPath, tmpDir, paths.ConfigPath, ".tpl", false, dryRun,
	)
	if err != nil {
		return nil, fmt.Errorf("error creating template model: %w", err)
	}

	if err := model.Generate(); err != nil {
		return nil, fmt.Errorf("error generating from template files: %w", err)
	}

	if spec, ok := model.Config.Data["spec"]["plugins"].(map[any]any); ok {
		return spec, nil
	}

	return map[any]any{}, nil
}

// templateConfig returns the merged furyctl config with the phase paths, as the plugins templates read it.
func templateConfig(phase *cluster.OperationPhase, paths cluster.DeleterPaths, kfdManifest config.KFD) ([]byte, error) {
	cfg, err := mergedConfig(phase, paths)
	if err != nil {
		return nil, err
	}

	phase.CopyPathsToConfig(&cfg)

	if !distribution.HasFeature(kfdManifest, distribution.FeatureKubeconfigInSchema) {
		cfg.Data["paths"]["kubeconfig"] = os.Getenv("KUBECONFIG")
	}

	out, err := yamlx.MarshalV2(cfg)
	if err != nil {
		return nil, fmt.Errorf("error marshaling template config: %w", err)
	}

	return out, nil
}
