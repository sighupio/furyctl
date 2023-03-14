// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package distribution

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/furyctl/internal/merge"
	"github.com/sighupio/furyctl/internal/template"
	yamlx "github.com/sighupio/furyctl/internal/x/yaml"
)

const (
	source           = "templates/distribution"
	defaultsFileName = "furyctl-defaults.yaml"
	suffix           = ".tpl"
)

var ErrSourceDirDoesNotExist = errors.New("source directory does not exist")

type IACBuilder struct {
	furyctlFile map[any]any
	distroPath  string
	outDir      string
	noOverwrite bool
	dryRun      bool
}

func NewIACBuilder(
	furyctlFile map[any]any,
	distroPath,
	outDir string,
	noOverwrite,
	dryRun bool,
) *IACBuilder {
	absOutDir, err := filepath.Abs(outDir)
	if err != nil {
		log.Fatalf("error getting absolute path for %s: %v", outDir, err)

		return nil
	}

	return &IACBuilder{
		furyctlFile: furyctlFile,
		distroPath:  distroPath,
		outDir:      absOutDir,
		noOverwrite: noOverwrite,
		dryRun:      dryRun,
	}
}

func (m *IACBuilder) Build() error {
	defaultsFile, err := m.defaultsFile()
	if err != nil {
		return fmt.Errorf("error getting defaults file: %w", err)
	}

	sourcePath, err := m.sourcePath()
	if err != nil {
		return fmt.Errorf("error getting source path: %w", err)
	}

	merger := merge.NewMerger(
		merge.NewDefaultModel(defaultsFile, ".data"),
		merge.NewDefaultModel(m.furyctlFile, ".spec.distribution"),
	)

	_, err = merger.Merge()
	if err != nil {
		return fmt.Errorf("error merging files: %w", err)
	}

	reverseMerger := merge.NewMerger(
		*merger.GetCustom(),
		*merger.GetBase(),
	)

	_, err = reverseMerger.Merge()
	if err != nil {
		return fmt.Errorf("error merging files: %w", err)
	}

	tmplCfg, err := template.NewConfig(reverseMerger, reverseMerger, []string{"source/terraform", ".gitignore"})
	if err != nil {
		return fmt.Errorf("error creating template config: %w", err)
	}

	outYaml, err := yamlx.MarshalV2(tmplCfg)
	if err != nil {
		return fmt.Errorf("error marshaling template config: %w", err)
	}

	outDirPath, err := os.MkdirTemp("", "furyctl-dist-")
	if err != nil {
		return fmt.Errorf("error creating temporary directory: %w", err)
	}

	confPath := filepath.Join(outDirPath, "config.yaml")

	logrus.Debugf("config path = %s", confPath)

	if err = os.WriteFile(confPath, outYaml, os.ModePerm); err != nil {
		return fmt.Errorf("error writing config file: %w", err)
	}

	if !m.noOverwrite {
		if err = os.RemoveAll(m.outDir); err != nil {
			return fmt.Errorf("error removing target directory: %w", err)
		}
	}

	logrus.Debugf("output directory = %s", m.outDir)

	templateModel, err := template.NewTemplateModel(
		sourcePath,
		m.outDir,
		confPath,
		outDirPath,
		suffix,
		m.noOverwrite,
		m.dryRun,
	)
	if err != nil {
		return fmt.Errorf("error creating template model: %w", err)
	}

	err = templateModel.Generate()
	if err != nil {
		return fmt.Errorf("error generating from template: %w", err)
	}

	return nil
}

func (m *IACBuilder) defaultsFile() (map[any]any, error) {
	defaultsFilePath := filepath.Join(m.distroPath, defaultsFileName)

	defaultsFile, err := yamlx.FromFileV2[map[any]any](defaultsFilePath)
	if err != nil {
		return nil, fmt.Errorf("%s - %w", defaultsFilePath, err)
	}

	return defaultsFile, nil
}

func (m *IACBuilder) sourcePath() (string, error) {
	sourcePath := filepath.Join(m.distroPath, source)

	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		return "", ErrSourceDirDoesNotExist
	}

	return sourcePath, nil
}
