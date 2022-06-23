package merge

import (
	"os"

	"gopkg.in/yaml.v3"
)

func Merge(base, custom map[string]interface{}) map[string]interface{} {
	if parent, ok := custom["spec"]; ok {
		if d, ok := parent.(map[string]interface{})["distribution"]; ok {
			base["data"] = DeepCopy(base, d.(map[string]interface{}))
		}
	}

	return base
}

func readYAMLfromFile(path string) (map[string]interface{}, error) {
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
		if vMap, checkMap := v.(map[string]interface{}); checkMap {
			if bv, ok := out[k].(map[string]interface{}); ok {
				out[k] = DeepCopy(bv, vMap)
				continue
			} else {
				out[k] = DeepCopy(map[string]interface{}{}, vMap)
			}
		}

		vArr, checkArr := v.([]interface{})
		if checkArr {
			for _, j := range vArr {
				if j, ok := j.(map[string]interface{}); ok {
					if bv, ok := out[k].(map[string]interface{}); ok {
						out[k] = DeepCopy(bv, j)
					}
				}
			}
			continue
		}

		out[k] = v
	}
	return out
}
