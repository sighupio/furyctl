package template

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"gopkg.in/yaml.v2"

	"github.com/sighupio/furyctl/internal/template/generator/mapper"
)

func toYAML(v interface{}) string {
	data, err := yaml.Marshal(v)
	if err != nil {
		// Swallow errors inside of a template.
		return ""
	}
	return strings.TrimSuffix(string(data), "\n")
}

func fromYAML(str string) map[string]interface{} {
	m := map[string]interface{}{}

	if err := yaml.Unmarshal([]byte(str), &m); err != nil {
		m["Error"] = err.Error()
	}
	return m
}

func funcMap() template.FuncMap {
	f := sprig.TxtFuncMap()
	f["toYaml"] = toYAML
	f["fromYaml"] = fromYAML
	return f
}

type genConfig struct {
	source  string
	target  string
	context map[string]map[string]interface{}
}

func (tm *TemplateModel) Generate() error {
	var excludeMode = true

	if len(tm.Config.Templates.Excludes) > 0 && len(tm.Config.Templates.Includes) > 0 {
		println("Both excludes and includes are defined in config file, so only includes will be used.")
		excludeMode = false
	}

	osErr := os.MkdirAll(tm.TargetPath, os.ModePerm)
	if osErr != nil {
		return osErr
	}

	return filepath.Walk(tm.SourcePath, func(relSource string, info os.FileInfo, err error) error {
		var skip = false
		if excludeMode {
			skip = tm.isExcluded(relSource)
		} else { //include
			skip = !tm.isIncluded(relSource)
		}

		if !skip {
			rel, err := filepath.Rel(tm.SourcePath, relSource)
			if err != nil {
				return err
			}
			currentTarget := filepath.Join(tm.TargetPath, rel)
			if !info.IsDir() {
				tmplSuffix := tm.Suffix

				context, cErr := tm.prepareContext()
				if cErr != nil {
					return cErr
				}

				realTarget, fErr := tm.prepareTargetFilename(context, currentTarget)
				if fErr != nil { //maybe we should fail back to real name instead?
					return fErr
				}

				ctxMapper := mapper.NewMapper(context)

				context, err = ctxMapper.MapDynamicValues()
				if err != nil {
					return err
				}

				currentTargetDir := filepath.Dir(realTarget)

				if _, err := os.Stat(currentTargetDir); os.IsNotExist(err) {
					if err := os.MkdirAll(currentTargetDir, os.ModePerm); err != nil {
						return err
					}
				}

				if strings.HasSuffix(info.Name(), tmplSuffix) {
					content, cErr := genConfig{
						source:  relSource,
						target:  realTarget,
						context: context,
					}.processTemplate()
					if cErr != nil {
						return cErr
					}

					return copyBufferToFile(content, relSource, realTarget)
				} else {
					if _, err := fsCopy(relSource, realTarget); err != nil {
						return err
					}
				}
			}
		}

		return err
	})
}

func (tm *TemplateModel) prepareTargetFilename(context map[string]map[string]interface{}, currentTarget string) (string, error) {
	var realTarget string
	if tm.Config.Templates.ProcessFilename { //try to process filename as template
		tpl := template.Must(
			template.New("currentTarget").Funcs(funcMap()).Parse(currentTarget))

		destination := bytes.NewBufferString("")

		if err := tpl.Execute(destination, context); err != nil {
			return "", err
		}
		realTarget = destination.String()
	} else {
		realTarget = currentTarget
	}
	suf := tm.Suffix
	if strings.HasSuffix(realTarget, suf) {
		realTarget = realTarget[:len(realTarget)-len(tm.Suffix)] //cut off extension (.tmpl) from the end
	}
	return realTarget, nil

}

func (tm *TemplateModel) prepareContext() (map[string]map[string]interface{}, error) {
	context := make(map[string]map[string]interface{})
	envMap := mapEnvironmentVars()
	context["Env"] = envMap
	for k, v := range tm.Config.Data {
		context[k] = v
	}

	for k, v := range tm.Config.Include {
		var cPath string
		if filepath.IsAbs(v) {
			cPath = v
		} else {
			cPath = filepath.Join(filepath.Dir(tm.ConfigPath), v) //if relative, it is relative to master config
		}

		if yamlConfig, err := readYamlConfig(cPath); err != nil {
			return nil, err
		} else {
			context[k] = yamlConfig
		}
	}

	return context, nil
}

func readYamlConfig(yamlFilePath string) (map[string]interface{}, error) {
	var body map[string]interface{}

	yamlFile, err := ioutil.ReadFile(yamlFilePath)
	if err != nil {
		return nil, err
	}
	err = yaml.Unmarshal(yamlFile, &body)
	if err != nil {
		return nil, err
	}

	return body, nil
}

func (c genConfig) processTemplate() (bytes.Buffer, error) {
	var generatedContent bytes.Buffer

	tpl := template.Must(
		template.New(filepath.Base(c.source)).Funcs(funcMap()).ParseFiles(c.source))

	if err := tpl.Execute(&generatedContent, c.context); err != nil {
		return generatedContent, err
	}

	return generatedContent, nil
}

func copyBufferToFile(b bytes.Buffer, source, target string) error {
	if b.String() != "" {
		fmt.Printf("%s --> %s\n", source, target)
		destination, err := os.Create(target)
		if err != nil {
			return err
		}

		_, err = b.WriteTo(destination)
		if err != nil {
			return err
		}

		defer destination.Close()
	} else {
		fmt.Printf("%s --> resulted in an empty file (%d bytes). Skipping.\n", source, b.Len())
	}

	return nil
}

func fsCopy(src, dst string) (int64, error) {
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return 0, err
	}

	if !sourceFileStat.Mode().IsRegular() {
		return 0, fmt.Errorf("%s is not a regular file", src)
	}

	source, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return 0, err
	}
	defer destination.Close()
	nBytes, err := io.Copy(destination, source)
	return nBytes, err
}

func mapEnvironmentVars() map[string]interface{} {
	envMap := make(map[string]interface{})

	for _, v := range os.Environ() {
		part := strings.Split(v, "=")
		envMap[part[0]] = part[1]
	}

	return envMap
}
