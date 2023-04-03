// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mapper

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

const (
	Env  = "env"
	File = "file"
)

var errUnknownKey = errors.New("unknown key")

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
		vMap, checkMap := v.(map[any]any)
		if checkMap {
			if _, err := injectDynamicRes(vMap); err != nil {
				return nil, err
			}

			continue
		}

		vArr, checkArr := v.([]any)
		if checkArr {
			for _, j := range vArr {
				if j, ok := j.(map[any]any); ok {
					if _, err := injectDynamicRes(j); err != nil {
						return nil, err
					}
				}
			}

			continue
		}

		val, ok := v.(string)
		if !ok {
			continue
		}

		spl := strings.Split(val, "://")

		if len(spl) > 1 {
			val, err := ParseDynamicValue(val)
			if err != nil {
				return nil, err
			}

			m[k] = val

			continue
		}
	}

	return m, nil
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
			return "", fmt.Errorf("%w %s", errUnknownKey, source)
		}
	}

	return strVal, nil
}
