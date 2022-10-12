// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package eks

import (
	"fmt"
	mapx "github.com/sighupio/furyctl/internal/x/map"
	"os"
	"path"
	"path/filepath"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/config"
	"github.com/sighupio/fury-distribution/pkg/schema"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/merge"
	"github.com/sighupio/furyctl/internal/template"
	yamlx "github.com/sighupio/furyctl/internal/x/yaml"
)

type Distribution struct {
	*cluster.CreationPhase
	furyctlConf schema.EksclusterKfdV1Alpha2
	kfdManifest config.KFD
	distroPath  string
}

func NewDistribution(
	furyctlConf schema.EksclusterKfdV1Alpha2,
	kfdManifest config.KFD,
	distroPath string,
) (*Distribution, error) {
	phase, err := cluster.NewCreationPhase(".distribution")
	if err != nil {
		return nil, err
	}

	return &Distribution{
		CreationPhase: phase,
		furyctlConf:   furyctlConf,
		kfdManifest:   kfdManifest,
		distroPath:    distroPath,
	}, nil
}

func (d *Distribution) Exec(dryRun bool) error {
	if err := d.CreateFolder(); err != nil {
		return err
	}

	if err := d.copyFromTemplate(dryRun); err != nil {
		return err
	}

	if err := d.createFolderStructure(); err != nil {
		return err
	}

	return nil
}

func (d *Distribution) createFolderStructure() error {
	manifestsPath := path.Join(d.Path, "manifests")
	//terraformPath := path.Join(d.Path, "terraform")

	if err := os.Mkdir(manifestsPath, 0o755); err != nil {
		return err
	}

	//if err := os.Mkdir(terraformPath, 0o755); err != nil {
	//	return err
	//}

	return d.CreateFolderStructure()
}

func (d *Distribution) copyFromTemplate(dryRun bool) error {
	var cfg template.Config

	defaultsFilePath := path.Join(d.distroPath, "furyctl-defaults.yaml")

	defaultsFile, err := yamlx.FromFileV2[map[any]any](defaultsFilePath)
	if err != nil {
		return fmt.Errorf("%s - %w", defaultsFilePath, err)
	}

	furyctlConfMergeModel := merge.NewDefaultModelFromStruct(d.furyctlConf, ".spec.distribution")

	merger := merge.NewMerger(
		merge.NewDefaultModel(defaultsFile, ".data"),
		furyctlConfMergeModel,
	)

	mergedDistribution, err := merger.Merge()
	if err != nil {
		return err
	}

	mergedTmpl, ok := mergedDistribution["templates"]
	if !ok {
		return fmt.Errorf("templates not found in merged distribution")
	}

	mergedData, ok := mergedDistribution["data"]
	if !ok {
		return fmt.Errorf("data not found in merged distribution")
	}

	mergedDataMap, ok := mergedData.(map[any]any)
	if !ok {
		return fmt.Errorf("data in merged distribution is not a map")
	}

	tmpl, err := template.NewTemplatesFromMap(mergedTmpl)
	if err != nil {
		return err
	}

	tmpl.Excludes = []string{"source/manifests", ".gitignore"}

	err = furyctlConfMergeModel.Walk(mergedDataMap)
	if err != nil {
		return err
	}

	cfg.Templates = *tmpl
	cfg.Data = mapx.ToMapStringAny(furyctlConfMergeModel.Content())
	cfg.Include = nil

	outYaml, err := yamlx.MarshalV2(cfg)
	if err != nil {
		return err
	}

	outDirPath, err := os.MkdirTemp("", "furyctl-dist-")
	if err != nil {
		return err
	}

	confPath := filepath.Join(outDirPath, "config.yaml")

	logrus.Debugf("config path = %s", confPath)

	if err = os.WriteFile(confPath, outYaml, os.ModePerm); err != nil {
		return err
	}

	templateModel, err := template.NewTemplateModel(
		path.Join(d.distroPath, "source"),
		path.Join(d.Path),
		confPath,
		outDirPath,
		".tpl",
		false,
		dryRun,
	)
	if err != nil {
		return err
	}

	return templateModel.Generate()
}
