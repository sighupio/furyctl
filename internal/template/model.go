package template

import (
	"fmt"
	"github.com/sighupio/furyctl/internal/io"
	"github.com/sighupio/furyctl/internal/template/mapper"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v2"
)

type Model struct {
	SourcePath           string
	TargetPath           string
	ConfigPath           string
	Config               Config
	Suffix               string
	StopIfTargetNotEmpty bool
}

func NewTemplateModel(source, target, configPath, suffix string, stopIfNotEmpty bool) (*Model, error) {
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
			panic(err)
		}

		if err = yaml.Unmarshal(readFile, &model); err != nil {
			return nil, err
		}
	}

	if stopIfNotEmpty {
		err := io.CheckDirIsEmpty(target)
		if err != nil {
			return nil, err
		}
	}

	return &Model{
		SourcePath:           source,
		TargetPath:           target,
		ConfigPath:           configPath,
		Config:               model,
		Suffix:               suffix,
		StopIfTargetNotEmpty: stopIfNotEmpty,
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

func (tm *Model) isIncluded(source string) bool {
	for _, incl := range tm.Config.Templates.Includes {
		regex := regexp.MustCompile(incl)
		if regex.MatchString(source) {
			return true
		}
	}
	return false
}

func (tm *Model) Generate() error {
	if len(tm.Config.Templates.Excludes) > 0 && len(tm.Config.Templates.Includes) > 0 {
		println("Both excludes and includes are defined in config file, so only includes will be used.")
	}

	osErr := os.MkdirAll(tm.TargetPath, os.ModePerm)
	if osErr != nil {
		return osErr
	}

	context, cErr := CreateContextFromModel(tm)
	if cErr != nil {
		return cErr
	}

	ctxMapper := mapper.NewMapper(context)

	context, err := ctxMapper.MapDynamicValues()
	if err != nil {
		return err
	}

	return filepath.Walk(tm.SourcePath, func(
		relSource string,
		info os.FileInfo,
		err error,
	) error {
		return applyTemplates(
			tm,
			relSource,
			info,
			context,
			err,
		)
	})
}

func applyTemplates(
	tm *Model,
	relSource string,
	info os.FileInfo,
	context map[string]map[interface{}]interface{},
	err error,
) error {
	if !tm.isExcluded(relSource) && tm.isIncluded(relSource) {
		rel, err := filepath.Rel(tm.SourcePath, relSource)
		if err != nil {
			return err
		}
		currentTarget := filepath.Join(tm.TargetPath, rel)
		if !info.IsDir() {
			tmplSuffix := tm.Suffix

			gen := NewGenerator(
				relSource,
				currentTarget,
				context,
			)

			realTarget, fErr := gen.processFilename(tm)
			if fErr != nil { //maybe we should fail back to real name instead?
				return fErr
			}

			gen.updateTarget(realTarget)

			currentTargetDir := filepath.Dir(realTarget)

			if _, err := os.Stat(currentTargetDir); os.IsNotExist(err) {
				if err := os.MkdirAll(currentTargetDir, os.ModePerm); err != nil {
					return err
				}
			}

			if strings.HasSuffix(info.Name(), tmplSuffix) {
				content, cErr := gen.processFile()
				if cErr != nil {
					return cErr
				}

				return io.CopyBufferToFile(content, relSource, realTarget)
			} else {
				if _, err := io.CopyFromSourceToTarget(relSource, realTarget); err != nil {
					return err
				}
			}
		}
	}

	return err
}
