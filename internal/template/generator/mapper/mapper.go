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
	context map[string]map[string]interface{}
}

func NewMapper(context map[string]map[string]interface{}) *Mapper {
	return &Mapper{context: context}
}

func (m *Mapper) MapDynamicValues() (map[string]map[string]interface{}, error) {
	mappedCtx := make(map[string]map[string]interface{}, len(m.context))

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
	m map[string]interface{},
	parent map[string]interface{},
	parentKey string,
) (map[string]interface{}, error) {
	for k, v := range m {
		vMap, checkMap := v.(map[string]interface{})
		if checkMap {
			if _, err := injectDynamicRes(vMap, m, k); err != nil {
				return nil, err
			}

			continue
		}

		vArr, checkArr := v.([]interface{})
		if checkArr {
			for _, j := range vArr {
				if j, ok := j.(map[string]interface{}); ok {
					if _, err := injectDynamicRes(j, m, k); err != nil {
						return nil, err
					}
				}
			}
			continue
		}

		spl := strings.Split(k, "://")

		if len(spl) > 1 {
			
			source := spl[0]
			sourceValue := spl[1]

			switch source {
			case Env:
				fmt.Printf("changing %+v to env var \n", k)
				parent[parentKey] = os.Getenv(sourceValue)
				break
			case File:
				content, err := readValueFromFile(sourceValue)
				if err != nil {
					return nil, err
				}
				parent[parentKey] = content
				break
			}
		}

	}

	return m, nil
}

func readValueFromFile(path string) (string, error) {
	val, err := os.ReadFile(path)

	return string(val), err
}
