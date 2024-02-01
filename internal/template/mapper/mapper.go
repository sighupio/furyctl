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

	"github.com/sighupio/furyctl/internal/parser"
)

var (
	EnvRegexp          = regexp.MustCompile(`{(.*?)}`)
	RelativePathRegexp = regexp.MustCompile(`^\.{1,}\/`)
)

type Mapper struct {
	context        map[string]map[any]any
	furyctlConfDir string
}

func NewMapper(
	context map[string]map[any]any,
	furyctlConfPath string,
) *Mapper {
	return &Mapper{
		context:        context,
		furyctlConfDir: filepath.Dir(furyctlConfPath),
	}
}

func (m *Mapper) MapDynamicValuesAndPaths() (map[string]map[any]any, error) {
	mappedCtx := make(map[string]map[any]any, len(m.context))

	for k, c := range m.context {
		res, err := m.injectDynamicValuesAndPaths(c)
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

func (m *Mapper) injectDynamicValuesAndPaths(
	context map[any]any,
) (map[any]any, error) {
	for k, v := range context {
		if v == nil {
			continue
		}

		switch reflect.TypeOf(v).Kind() {
		case reflect.Map:
			if mapVal, ok := v.(map[any]any); ok {
				if _, err := m.injectDynamicValuesAndPaths(mapVal); err != nil {
					return nil, err
				}
			}

		case reflect.String:
			// If the key is relativeVendorPath, we ignore it.
			if k == "relativeVendorPath" {
				break
			}

			if stringVal, ok := v.(string); ok {
				injectedStringVal, err := m.injectDynamicValuesAndPathsString(stringVal)
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
							if _, err := m.injectDynamicValuesAndPaths(mapVal); err != nil {
								return nil, err
							}
						}

					case reflect.String:
						injectedStringVal, err := m.injectDynamicValuesAndPathsString(arrChildVal.(string))
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

func (m *Mapper) injectDynamicValuesAndPathsString(value string) (string, error) {
	cfgParser := parser.NewConfigParser(m.furyctlConfDir)

	// If the value contains dynamic values, we need to parse them.
	dynamicValues := EnvRegexp.FindAllString(value, -1)
	for _, dynamicValue := range dynamicValues {
		parsedDynamicValue, err := cfgParser.ParseDynamicValue(dynamicValue)
		if err != nil {
			return "", fmt.Errorf("error parsing dynamic value: %w", err)
		}

		value = strings.Replace(value, dynamicValue, parsedDynamicValue, 1)
	}

	// If the value is a relative path, we need to convert it to an absolute path.
	isRelativePath := RelativePathRegexp.MatchString(value)
	if isRelativePath {
		value = filepath.Clean(value)
		value = filepath.Join(m.furyctlConfDir, value)
	}

	return value, nil
}
