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

var errKeyIsNotAString = errors.New("key is not a string")

type Mapper struct {
	context map[string]map[any]any
}

func NewMapper(context map[string]map[any]any) *Mapper {
	return &Mapper{context: context}
}

func (m *Mapper) MapDynamicValues() (map[string]map[any]any, error) {
	mappedCtx := make(map[string]map[any]any, len(m.context))

	for k, c := range m.context {
		res, err := injectDynamicRes(c, c, k)
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
	parent map[any]any,
	parentKey string,
) (map[any]any, error) {
	for k, v := range m {
		key, ok := k.(string)
		if !ok {
			return nil, fmt.Errorf("%v %w", k, errKeyIsNotAString)
		}

		spl := strings.Split(key, "://")

		if len(spl) > 1 {
			source := spl[0]
			sourceValue := spl[1]

			switch source {
			case Env:
				envVar := os.Getenv(sourceValue)
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

		vMap, checkMap := v.(map[any]any)
		if checkMap {
			if _, err := injectDynamicRes(vMap, m, k.(string)); err != nil {
				return nil, err
			}

			continue
		}

		vArr, checkArr := v.([]any)
		if checkArr {
			for _, j := range vArr {
				if j, ok := j.(map[any]any); ok {
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
