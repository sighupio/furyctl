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
	"github.com/sighupio/furyctl/internal/upgrade"
	yamlx "github.com/sighupio/furyctl/internal/x/yaml"
)

type PreUpgrade struct {
	*cluster.OperationPhase
	distroPath      string
	furyctlConfPath string
	dryRun          bool
	kubeconfig      string
	kind            string
	upgrade         *upgrade.Upgrade
}

func NewPreUpgrade(
	paths cluster.CreatorPaths,
	kfdManifest config.KFD,
	kind string,
	dryRun bool,
	kubeconfig string,
	upgr *upgrade.Upgrade,
) (*PreUpgrade, error) {
	phaseOp, err := cluster.NewOperationPhase(path.Join(paths.WorkDir, "upgrades"), kfdManifest.Tools, paths.BinPath)
	if err != nil {
		return nil, fmt.Errorf("error creating preupgrade phase: %w", err)
	}

	return &PreUpgrade{
		OperationPhase:  phaseOp,
		distroPath:      paths.DistroPath,
		furyctlConfPath: paths.ConfigPath,
		dryRun:          dryRun,
		kubeconfig:      kubeconfig,
		kind:            kind,
		upgrade:         upgr,
	}, nil
}

func (p *PreUpgrade) Exec() error {
	logrus.Info("Running preupgrade phase...")

	if err := p.CreateFolder(); err != nil {
		return fmt.Errorf("error creating preupgrade phase folder: %w", err)
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

	outDirPath1, err := os.MkdirTemp("", "furyctl-preupgrade-")
	if err != nil {
		return fmt.Errorf("error creating temp dir: %w", err)
	}

	confPath := filepath.Join(outDirPath1, "config.yaml")

	logrus.Debugf("config path = %s", confPath)

	if err = os.WriteFile(confPath, outYaml, os.ModePerm); err != nil {
		return fmt.Errorf("error writing config file: %w", err)
	}

	templateModel, err := template.NewTemplateModel(
		path.Join(p.distroPath, "templates", "upgrades"),
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

	logrus.Info("Preupgrade phase completed successfully")

	return nil
}

func (p *PreUpgrade) createFuryctlMerger() (*merge.Merger, error) {
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
