package mapper

import (
	"fmt"
	"os"
	"strings"
)

const (
	Env  = "env"
	File = "file"
)

type Mapper struct {
	context map[string]map[interface{}]interface{}
}

func NewMapper(context map[string]map[interface{}]interface{}) *Mapper {
	return &Mapper{context: context}
}

func (m *Mapper) MapDynamicValues() (map[string]map[interface{}]interface{}, error) {
	mappedCtx := make(map[string]map[interface{}]interface{}, len(m.context))

	for k, c := range m.context {
		res, err := injectDynamicRes(c, c, k)
		mappedCtx[k] = res

		if err != nil {
			return nil, err
		}
	}

	return mappedCtx, nil
}

func injectDynamicRes(
	m map[interface{}]interface{},
	parent map[interface{}]interface{},
	parentKey string,
) (map[interface{}]interface{}, error) {
	for k, v := range m {
		spl := strings.Split(k.(string), "://")

		if len(spl) > 1 {

			source := spl[0]
			sourceValue := spl[1]

			switch source {
			case Env:
				envVar := os.Getenv(sourceValue)
				fmt.Printf("changing %+v to env var value %+v \n", k, envVar)
				parent[parentKey] = envVar
			case File:
				content, err := readValueFromFile(sourceValue)
				if err != nil {
					return nil, err
				}
				parent[parentKey] = content
			}

			continue
		}

		vMap, checkMap := v.(map[interface{}]interface{})
		if checkMap {
			if _, err := injectDynamicRes(vMap, m, k.(string)); err != nil {
				return nil, err
			}

			continue
		}

		vArr, checkArr := v.([]interface{})
		if checkArr {
			for _, j := range vArr {
				if j, ok := j.(map[interface{}]interface{}); ok {
					if _, err := injectDynamicRes(j, m, k.(string)); err != nil {
						return nil, err
					}
				}
			}
			continue
		}
	}

	return m, nil
}

func readValueFromFile(path string) (string, error) {
	val, err := os.ReadFile(path)

	return string(val), err
}
