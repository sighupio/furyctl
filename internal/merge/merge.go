package merge

import (
	"os"

	"gopkg.in/yaml.v3"
)

func Merge(base, custom map[string]interface{}) (map[string]interface{}, error) {
	if parent, ok := custom["spec"]; ok {
		if d, ok := parent.(map[string]interface{})["distribution"]; ok {
			base["data"] = DeepCopy(base["data"].(map[string]interface{}), d.(map[string]interface{}))
		}
	}

	return base, nil
}

func ReadYAMLfromFile(path string) (map[string]interface{}, error) {
	var yamlRes map[string]interface{}

	res, err := os.ReadFile(path)
	if err != nil {
		return yamlRes, err
	}

	err = yaml.Unmarshal(res, &yamlRes)

	return yamlRes, err

}

func DeepCopy(a, b map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(a))
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		if v, ok := v.(map[string]interface{}); ok {
			if bv, ok := out[k]; ok {
				if bv, ok := bv.(map[string]interface{}); ok {
					out[k] = DeepCopy(bv, v)
					continue
				}
			}
		}
		out[k] = v
	}
	return out
}
