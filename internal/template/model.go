// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package template

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v2"

	"github.com/sighupio/furyctl/internal/template/mapper"
	iox "github.com/sighupio/furyctl/internal/x/io"
	yamlx "github.com/sighupio/furyctl/internal/x/yaml"
)

var (
	errSourceMustbeSet  = errors.New("source must be set")
	errTargetMustbeSet  = errors.New("target must be set")
	errTemplateNotFound = errors.New("no template found")
)

type Model struct {
	SourcePath           string
	TargetPath           string
	ConfigPath           string
	OutputPath           string
	FuryctlConfPath      string
	Config               Config
	Suffix               string
	Context              map[string]map[any]any
	FuncMap              FuncMap
	StopIfTargetNotEmpty bool
	DryRun               bool
}

func NewTemplateModel(
	source,
	target,
	configPath,
	outPath,
	furyctlConfPath,
	suffix string,
	stopIfNotEmpty,
	dryRun bool,
) (*Model, error) {
	var model Config

	if len(source) < 1 {
		return nil, errSourceMustbeSet
	}

	if len(target) < 1 {
		return nil, errTargetMustbeSet
	}

	if len(configPath) > 0 {
		readFile, err := os.ReadFile(configPath)
		if err != nil {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}

		if err = yaml.Unmarshal(readFile, &model); err != nil {
			return nil, fmt.Errorf("error parsing config file: %w", err)
		}
	}

	funcMap := NewFuncMap()
	funcMap.Add("toYaml", ToYAML)
	funcMap.Add("fromYaml", FromYAML)
	funcMap.Add("hasKeyAny", HasKeyAny)

	return &Model{
		SourcePath:           source,
		TargetPath:           target,
		ConfigPath:           configPath,
		OutputPath:           outPath,
		FuryctlConfPath:      furyctlConfPath,
		Config:               model,
		Suffix:               suffix,
		FuncMap:              funcMap,
		StopIfTargetNotEmpty: stopIfNotEmpty,
		DryRun:               dryRun,
	}, nil
}

func (tm *Model) isExcluded(source string) bool {
	for _, exc := range tm.Config.Templates.Excludes {
		regex := regexp.MustCompile(exc)
		if regex.MatchString(source) {
			return true
		}
	}

	return false
}

func (tm *Model) Generate() error {
	if tm.StopIfTargetNotEmpty {
		err := iox.CheckDirIsEmpty(tm.TargetPath)
		if err != nil {
			return fmt.Errorf("target directory is not empty: %w", err)
		}
	}

	if err := os.MkdirAll(tm.TargetPath, os.ModePerm); err != nil {
		return fmt.Errorf("error creating target directory: %w", err)
	}

	context, cErr := tm.generateContext()
	if cErr != nil {
		return cErr
	}

	ctxMapper := mapper.NewMapper(
		context,
		tm.FuryctlConfPath,
	)

	context, err := ctxMapper.MapDynamicValuesAndPaths()
	if err != nil {
		return fmt.Errorf("error mapping dynamic values: %w", err)
	}

	tm.Context = context

	if err := filepath.Walk(tm.SourcePath, tm.applyTemplates); err != nil {
		return fmt.Errorf("error applying templates: %w", err)
	}

	return nil
}

func (tm *Model) applyTemplates(
	relSource string,
	info os.FileInfo,
	err error,
) error {
	if tm.isExcluded(relSource) {
		return err
	}

	if info == nil {
		return err
	}

	if info.IsDir() {
		return err
	}

	rel, err := filepath.Rel(tm.SourcePath, relSource)
	if err != nil {
		return fmt.Errorf("error getting relative path: %w", err)
	}

	currentTarget := filepath.Join(tm.TargetPath, rel)

	gen := NewGenerator(
		tm.SourcePath,
		relSource,
		currentTarget,
		tm.Context,
		tm.FuncMap,
		tm.DryRun,
	)

	realTarget, fErr := gen.ProcessFilename(tm)
	if fErr != nil { // Maybe we should fail back to real name instead?
		return fErr
	}

	gen.UpdateTarget(realTarget)

	currentTargetDir := filepath.Dir(realTarget)

	if _, err := os.Stat(currentTargetDir); os.IsNotExist(err) {
		if err := os.MkdirAll(currentTargetDir, os.ModePerm); err != nil {
			return fmt.Errorf("error creating target directory: %w", err)
		}
	}

	if strings.HasSuffix(info.Name(), tm.Suffix) {
		tmpl, err := gen.ProcessTemplate()
		if err != nil {
			return err
		}

		if tmpl == nil {
			return fmt.Errorf("%w for %s", errTemplateNotFound, relSource)
		}

		if tm.DryRun {
			missingKeys := gen.GetMissingKeys(tmpl)

			err := gen.WriteMissingKeysToFile(missingKeys, relSource, tm.OutputPath)
			if err != nil {
				return err
			}
		}

		content, cErr := gen.ProcessFile(tmpl)
		if cErr != nil {
			return fmt.Errorf("%w filePath: %s", cErr, relSource)
		}

		err = iox.CopyBufferToFile(content, realTarget)
		if err != nil {
			return fmt.Errorf("error writing file: %w", err)
		}

		return nil
	}

	err = iox.CopyFile(relSource, realTarget)
	if err != nil {
		return fmt.Errorf("error copying file: %w", err)
	}

	return nil
}

func (tm *Model) generateContext() (map[string]map[any]any, error) {
	context := make(map[string]map[any]any)

	for k, v := range tm.Config.Data {
		context[k] = v
	}

	for k, v := range tm.Config.Include {
		cPath := filepath.Join(filepath.Dir(tm.ConfigPath), v)

		if filepath.IsAbs(v) {
			cPath = v
		}

		yamlConfig, err := yamlx.FromFileV2[map[any]any](cPath)
		if err != nil {
			return nil, err
		}

		context[k] = yamlConfig
	}

	return context, nil
}
