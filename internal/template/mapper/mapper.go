// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mapper

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
)

const (
	Env  = "env"
	File = "file"
)

type Mapper struct {
	context            map[string]map[any]any
	envRegexp          *regexp.Regexp
	relativePathRegexp *regexp.Regexp
	furyctlConfDir     string
}

func NewMapper(
	context map[string]map[any]any,
	furyctlConfPath string,
) *Mapper {
	furyctlConfDir := filepath.Dir(furyctlConfPath)

	return &Mapper{
		context:            context,
		envRegexp:          regexp.MustCompile(`{(.*?)}`),
		relativePathRegexp: regexp.MustCompile(`^\.{1,}\/`),
		furyctlConfDir:     furyctlConfDir,
	}
}

func (m *Mapper) MapDynamicValuesAndPaths() (map[string]map[any]any, error) {
	mappedCtx := make(map[string]map[any]any, len(m.context))

	for k, c := range m.context {
		res, err := m.mapDynamicValuesAndPaths(c)
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

func (m *Mapper) mapDynamicValuesAndPaths(
	context map[any]any,
) (map[any]any, error) {
	for k, v := range context {
		if v == nil {
			continue
		}

		switch reflect.TypeOf(v).Kind() {
		case reflect.Map:
			if mapVal, ok := v.(map[any]any); ok {
				if _, err := m.mapDynamicValuesAndPaths(mapVal); err != nil {
					return nil, err
				}
			}

		case reflect.String:
			if stringVal, ok := v.(string); ok {
				// If the key is relativeVendorPath, we ignore it.
				if k == "relativeVendorPath" {
					break
				}

				injectedStringVal, err := m.mapDynamicValuesAndPathsString(stringVal)
				if err != nil {
					return nil, err
				}

				context[k] = injectedStringVal
			}

		case reflect.Slice:
			if arrVal, ok := v.([]any); ok {
				for arrChildK, arrChildVal := range arrVal {
					switch reflect.TypeOf(arrChildVal).Kind() {
					case reflect.Map:
						if mapVal, ok := arrChildVal.(map[any]any); ok {
							if _, err := m.mapDynamicValuesAndPaths(mapVal); err != nil {
								return nil, err
							}
						}

					case reflect.String:
						injectedStringVal, err := m.mapDynamicValuesAndPathsString(arrChildVal.(string))
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
	}

	return context, nil
}

func (m *Mapper) mapDynamicValuesAndPathsString(value string) (string, error) {
	// If the value contains dynamic values, we need to parse them.
	dynamicValues := m.envRegexp.FindAllString(value, -1)
	for _, dynamicValue := range dynamicValues {
		parsedDynamicValue, err := ParseDynamicValue(dynamicValue)
		if err != nil {
			return "", err
		}

		value = strings.Replace(value, dynamicValue, parsedDynamicValue, 1)
	}

	// If the value is a relative path, we need to convert it to an absolute path.
	isRelativePath := m.relativePathRegexp.MatchString(value)
	if isRelativePath {
		value = filepath.Clean(value)
		value = filepath.Join(m.furyctlConfDir, value)
	}

	return value, nil
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
