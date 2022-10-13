// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package template

import (
	"fmt"

	mapx "github.com/sighupio/furyctl/internal/x/map"
)

type Templates struct {
	Includes        []string `yaml:"includes,omitempty"`
	Excludes        []string `yaml:"excludes,omitempty"`
	Suffix          string   `yaml:"suffix,omitempty"`
	ProcessFilename bool     `yaml:"processFilename,omitempty"`
}

type Config struct {
	Data      map[string]map[any]any `yaml:"data,omitempty"`
	Include   map[string]string      `yaml:"include,omitempty"`
	Templates Templates              `yaml:"templates,omitempty"`
}

func NewTemplatesFromMap(t any) (*Templates, error) {
	m, ok := t.(map[any]any)
	if !ok {
		return nil, fmt.Errorf("cannot convert %v to map", t)
	}

	inc, err := mapx.ToTypeSlice[string](m["includes"])
	if err != nil {
		return nil, err
	}

	exc, err := mapx.ToTypeSlice[string](m["excludes"])
	if err != nil {
		return nil, err
	}

	suf, err := mapx.ToType[string](m["suffix"])
	if err != nil {
		return nil, err
	}

	pro, err := mapx.ToType[bool](m["processFilename"])
	if err != nil {
		return nil, err
	}

	return &Templates{
		Includes:        inc,
		Excludes:        exc,
		Suffix:          suf,
		ProcessFilename: pro,
	}, nil
}
