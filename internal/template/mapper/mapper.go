// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mapper

import (
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strings"
)

const (
	Env  = "env"
	File = "file"
)

type Mapper struct {
	context map[string]map[any]any
}

func NewMapper(context map[string]map[any]any) *Mapper {
	return &Mapper{context: context}
}

func (m *Mapper) MapDynamicValues() (map[string]map[any]any, error) {
	mappedCtx := make(map[string]map[any]any, len(m.context))

	for k, c := range m.context {
		res, err := injectDynamicRes(c)
		mappedCtx[k] = res

		if err != nil {
			return nil, err
		}
	}

	return mappedCtx, nil
}

func (*Mapper) MapEnvironmentVars() map[any]any {
	envMap := make(map[any]any)

	for _, v := range os.Environ() {
		part := strings.Split(v, "=")
		envMap[part[0]] = part[1]
	}

	return envMap
}

func injectDynamicRes(
	m map[any]any,
) (map[any]any, error) {
	for k, v := range m {
		if v == nil {
			continue
		}

		t := reflect.TypeOf(v)
		switch t.Kind() {
		case reflect.Map:
			if mapVal, ok := v.(map[any]any); ok {
				if _, err := injectDynamicRes(mapVal); err != nil {
					return nil, err
				}
			}

		case reflect.String:
			if stringVal, ok := v.(string); ok {
				injectedStringVal, err := injectDynamicResString(stringVal)
				if err != nil {
					return nil, err
				}

				m[k] = injectedStringVal
			}

		case reflect.Slice:
			switch t.Elem().Kind() {
			case reflect.Map:
				if arrVal, ok := v.([]map[any]any); ok {
					for _, arrChildVal := range arrVal {
						if _, err := injectDynamicRes(arrChildVal); err != nil {
							return nil, err
						}
					}
				}

			case reflect.Interface:
				if arrVal, ok := v.([]any); ok {
					for arrChildK, arrChildVal := range arrVal {
						switch reflect.TypeOf(arrChildVal).Kind() {
						case reflect.String:
							injectedStringVal, err := injectDynamicResString(arrChildVal.(string))
							if err != nil {
								return nil, err
							}

							arrVal[arrChildK] = injectedStringVal

						default:
						}
					}
				}

			default:
			}

		default:
		}
	}

	return m, nil
}

func injectDynamicResString(val string) (string, error) {
	dynamicValues := regexp.MustCompile(`{(.*?)}`).FindAllString(val, -1)
	for _, dynamicValue := range dynamicValues {
		parsedDynamicValue, err := ParseDynamicValue(dynamicValue)
		if err != nil {
			return "", err
		}

		val = strings.Replace(val, dynamicValue, parsedDynamicValue, 1)
	}

	return val, nil
}

func readValueFromFile(path string) (string, error) {
	val, err := os.ReadFile(path)

	return string(val), err
}

func ParseDynamicValue(val any) (string, error) {
	strVal := fmt.Sprintf("%v", val)

	spl := strings.Split(strVal, "://")

	if len(spl) > 1 {
		source := strings.TrimPrefix(spl[0], "{")
		sourceValue := strings.TrimSuffix(spl[1], "}")

		switch source {
		case Env:
			envVar := os.Getenv(sourceValue)

			envVar = strings.TrimRight(envVar, "\n")

			return envVar, nil

		case File:
			content, err := readValueFromFile(sourceValue)
			if err != nil {
				return "", fmt.Errorf("%w", err)
			}

			content = strings.TrimRight(content, "\n")

			return content, nil

		default:
			return strVal, nil
		}
	}

	return strVal, nil
}
