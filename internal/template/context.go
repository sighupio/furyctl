package template

import (
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

func CreateContextFromModel(
	tm *Model,
) (map[string]map[interface{}]interface{}, error) {
	context := make(map[string]map[interface{}]interface{})
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

func readYamlConfig(yamlFilePath string) (map[interface{}]interface{}, error) {
	var body map[interface{}]interface{}

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

func mapEnvironmentVars() map[interface{}]interface{} {
	envMap := make(map[interface{}]interface{})

	for _, v := range os.Environ() {
		part := strings.Split(v, "=")
		envMap[part[0]] = part[1]
	}

	return envMap
}
