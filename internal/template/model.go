package template

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/sighupio/furyctl/internal/io"
	"github.com/sighupio/furyctl/internal/template/mapper"
	yaml2 "github.com/sighupio/furyctl/internal/yaml"
	fTemplate "github.com/sighupio/furyctl/pkg/template"

	"gopkg.in/yaml.v2"
)

type Model struct {
	SourcePath           string
	TargetPath           string
	ConfigPath           string
	Config               Config
	Suffix               string
	Context              map[string]map[any]any
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
			return nil, err
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

	context["Env"] = ctxMapper.MapEnvironmentVars()

	funcMap := NewFuncMap()
	funcMap.Add("toYaml", fTemplate.ToYAML)
	funcMap.Add("fromYaml", fTemplate.FromYAML)

	return filepath.Walk(tm.SourcePath, func(
		relSource string,
		info os.FileInfo,
		err error,
	) error {
		return tm.applyTemplates(
			relSource,
			info,
			context,
			funcMap,
			err,
		)
	})
}

func (tm *Model) applyTemplates(
	relSource string,
	info os.FileInfo,
	context map[string]map[any]any,
	funcMap FuncMap,
	err error,
) error {
	if tm.isExcluded(relSource) {
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
		context,
		funcMap,
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

	if strings.HasSuffix(info.Name(), tm.Suffix) {
		content, cErr := gen.processFile()
		if cErr != nil {
			return cErr
		}

		return io.CopyBufferToFile(content, relSource, realTarget)
	}

	_, err = io.CopyFromSourceToTarget(relSource, realTarget)

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

		yamlConfig, err := yaml2.FromFile[map[any]any](cPath)
		if err != nil {
			return nil, err
		}

		context[k] = yamlConfig
	}

	return context, nil
}
