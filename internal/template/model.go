// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package template

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v2"

	"github.com/sighupio/furyctl/internal/template/mapper"
	iox "github.com/sighupio/furyctl/internal/x/io"
	yamlx "github.com/sighupio/furyctl/internal/x/yaml"
)

type Model struct {
	SourcePath           string
	TargetPath           string
	ConfigPath           string
	OutputPath           string
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
	suffix string,
	stopIfNotEmpty,
	dryRun bool,
) (*Model, error) {
	var model Config

	if len(source) < 1 {
		return nil, fmt.Errorf("source must be set")
	}

	if len(target) < 1 {
		return nil, fmt.Errorf("target must be set")
	}

	if len(configPath) > 0 {
		readFile, err := ioutil.ReadFile(configPath)
		if err != nil {
			return nil, err
		}

		if err = yaml.Unmarshal(readFile, &model); err != nil {
			return nil, err
		}
	}

	if stopIfNotEmpty {
		err := iox.CheckDirIsEmpty(target)
		if err != nil {
			return nil, err
		}
	}

	funcMap := NewFuncMap()
	funcMap.Add("toYaml", toYAML)
	funcMap.Add("fromYaml", fromYAML)

	return &Model{
		SourcePath:           source,
		TargetPath:           target,
		ConfigPath:           configPath,
		OutputPath:           outPath,
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
	osErr := os.MkdirAll(tm.TargetPath, os.ModePerm)
	if osErr != nil {
		return osErr
	}

	context, cErr := tm.generateContext()
	if cErr != nil {
		return cErr
	}

	ctxMapper := mapper.NewMapper(context)

	context, err := ctxMapper.MapDynamicValues()
	if err != nil {
		return err
	}

	tm.Context = context

	return filepath.Walk(tm.SourcePath, tm.applyTemplates)
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
		return err
	}

	currentTarget := filepath.Join(tm.TargetPath, rel)

	gen := NewGenerator(
		relSource,
		currentTarget,
		tm.Context,
		tm.FuncMap,
		tm.DryRun,
	)

	realTarget, fErr := gen.ProcessFilename(tm)
	if fErr != nil { // maybe we should fail back to real name instead?
		return fErr
	}

	gen.UpdateTarget(realTarget)

	currentTargetDir := filepath.Dir(realTarget)

	if _, err := os.Stat(currentTargetDir); os.IsNotExist(err) {
		if err := os.MkdirAll(currentTargetDir, os.ModePerm); err != nil {
			return err
		}
	}

	if strings.HasSuffix(info.Name(), tm.Suffix) {
		tmpl, err := gen.ProcessTemplate()
		if err != nil {
			return err
		}

		if tmpl == nil {
			return fmt.Errorf("no template found for %s", relSource)
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
			return fmt.Errorf("%+v filePath: %s", cErr, relSource)
		}

		return iox.CopyBufferToFile(content, relSource, realTarget)
	}

	_, err = iox.CopyFromSourceToTarget(relSource, realTarget)

	return err
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
