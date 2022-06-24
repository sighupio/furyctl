package yaml

import (
	"gopkg.in/yaml.v3"
	"os"
)

func FromFile(path string) (map[string]interface{}, error) {
	var yamlRes map[string]interface{}

	res, err := os.ReadFile(path)
	if err != nil {
		return yamlRes, err
	}

	err = yaml.Unmarshal(res, &yamlRes)

	return yamlRes, err

}
