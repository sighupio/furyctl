package template

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"

	"gopkg.in/yaml.v2"
)

type TemplateModel struct {
	SourcePath           string
	TargetPath           string
	ConfigPath           string
	Config               Config
	Suffix               string
	StopIfTargetNotEmpty bool
}

func (tm *TemplateModel) isExcluded(source string) bool {
	for _, exc := range tm.Config.Templates.Excludes {
		regex := regexp.MustCompile(exc)
		if regex.MatchString(source) {
			return true
		}
	}
	return false
}

func (tm *TemplateModel) isIncluded(source string) bool {
	for _, incl := range tm.Config.Templates.Includes {
		regex := regexp.MustCompile(incl)
		if regex.MatchString(source) {
			return true
		}
	}
	return false
}

func NewTemplateModel(source, target, configPath, suffix string, stopIfNotEmpty bool) (*TemplateModel, error) {
	if len(source) < 1 {
		return nil, fmt.Errorf("source must be set")
	}
	if len(target) < 1 {
		return nil, fmt.Errorf("target must be set")
	}

	if stopIfNotEmpty {
		if _, err := os.Stat(target); os.IsExist(err) {
			err := filepath.Walk(target, func(path string, info os.FileInfo, err error) error {
				return fmt.Errorf("the target directory is not empty: %s", path)
			})
			if err != nil {
				return nil, err
			}
		}
	}

	var model Config
	if len(configPath) > 0 {
		readFile, err := ioutil.ReadFile(configPath)
		if err != nil {
			panic(err)
		}

		if err = yaml.Unmarshal(readFile, &model); err != nil {
			return nil, err
		}
	}

	return &TemplateModel{
		SourcePath:           source,
		TargetPath:           target,
		ConfigPath:           configPath,
		Config:               model,
		Suffix:               suffix,
		StopIfTargetNotEmpty: stopIfNotEmpty,
	}, nil
}
