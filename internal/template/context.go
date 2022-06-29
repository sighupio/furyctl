package template

import (
	yaml2 "github.com/sighupio/furyctl/internal/yaml"
	"os"
	"path/filepath"
	"strings"
)

func NewContext(tm *Model) (map[string]map[any]any, error) {
	context := make(map[string]map[any]any)

	context["Env"] = mapEnvironmentVars()

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

func mapEnvironmentVars() map[any]any {
	envMap := make(map[any]any)

	for _, v := range os.Environ() {
		part := strings.Split(v, "=")
		envMap[part[0]] = part[1]
	}

	return envMap
}
