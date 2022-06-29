package yaml

import (
	"gopkg.in/yaml.v3"
	"os"
)

func FromFile[T any](path string) (T, error) {
	var yamlRes T

	res, err := os.ReadFile(path)
	if err != nil {
		return yamlRes, err
	}

	err = yaml.Unmarshal(res, &yamlRes)

	return yamlRes, err

}
