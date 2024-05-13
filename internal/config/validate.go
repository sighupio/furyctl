// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package config

import (
	"fmt"
	"html/template"
	"os"
	"path/filepath"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/furyctl/internal/analytics"
	"github.com/sighupio/furyctl/internal/apis"
	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/schema/santhosh"
	iox "github.com/sighupio/furyctl/internal/x/io"
	yamlx "github.com/sighupio/furyctl/internal/x/yaml"
	dist "github.com/sighupio/furyctl/pkg/distribution"
)

func Create(
	res dist.DownloadResult,
	furyctlPath string,
	cmdEvent analytics.Event,
	tracker *analytics.Tracker,
	data map[string]string,
) (*os.File, error) {
	tplPath, err := distribution.GetConfigTemplatePath(res.RepoPath, res.MinimalConf)
	if err != nil {
		return nil, fmt.Errorf("error getting schema path: %w", err)
	}

	tplContent, err := os.ReadFile(tplPath)
	if err != nil {
		cmdEvent.AddErrorMessage(err)
		tracker.Track(cmdEvent)

		return nil, fmt.Errorf("error reading furyctl yaml template: %w", err)
	}

	tmpl, err := template.New("furyctl.yaml").Parse(string(tplContent))
	if err != nil {
		cmdEvent.AddErrorMessage(err)
		tracker.Track(cmdEvent)

		return nil, fmt.Errorf("error parsing furyctl yaml template: %w", err)
	}

	out, err := createNewEmptyConfigFile(furyctlPath)
	if err != nil {
		cmdEvent.AddErrorMessage(err)
		tracker.Track(cmdEvent)

		return nil, err
	}

	if err := tmpl.Execute(out, data); err != nil {
		cmdEvent.AddErrorMessage(err)
		tracker.Track(cmdEvent)

		return nil, fmt.Errorf("error executing furyctl yaml template: %w", err)
	}

	return out, nil
}

func createNewEmptyConfigFile(path string) (*os.File, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("error getting absolute path: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(absPath), iox.FullPermAccess); err != nil {
		return nil, fmt.Errorf("error creating directory: %w", err)
	}

	out, err := os.Create(absPath)
	if err != nil {
		return nil, fmt.Errorf("error creating file: %w", err)
	}

	return out, nil
}

// Validate the furyctl.yaml file.
func Validate(path, repoPath string) error {
	miniConf, err := loadFromFile(path)
	if err != nil {
		return err
	}

	schemaPath, err := distribution.GetPublicSchemaPath(repoPath, miniConf)
	if err != nil {
		return fmt.Errorf("error getting schema path: %w", err)
	}

	schema, err := santhosh.LoadSchema(schemaPath)
	if err != nil {
		return fmt.Errorf("error loading schema: %w", err)
	}

	conf, err := yamlx.FromFileV3[any](path)
	if err != nil {
		return err
	}

	if err = schema.Validate(conf); err != nil {
		return fmt.Errorf("error while validating against schema: %w", err)
	}

	esv := apis.NewExtraSchemaValidatorFactory(miniConf.APIVersion, miniConf.Kind)
	if err = esv.Validate(path); err != nil {
		return fmt.Errorf("error while validating against extra schema rules: %w", err)
	}

	return nil
}

func loadFromFile(path string) (config.Furyctl, error) {
	conf, err := yamlx.FromFileV3[config.Furyctl](path)
	if err != nil {
		return config.Furyctl{}, err
	}

	if err := config.NewValidator().Struct(conf); err != nil {
		return config.Furyctl{}, fmt.Errorf("%w: %v", dist.ErrValidateConfig, err)
	}

	return conf, err
}
