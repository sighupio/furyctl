// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package template

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/sighupio/furyctl/internal/merge"
	mapx "github.com/sighupio/furyctl/internal/x/map"
)

var (
	ErrTemplatesNotFound         = errors.New("templates key not found in template source custom")
	ErrTemplateSourceCustomIsNil = errors.New("template source custom is nil")
	ErrDataSourceBaseIsNil       = errors.New("data source base is nil")
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

func NewConfig(tplSource, data *merge.Merger, excluded []string) (Config, error) {
	var cfg Config

	if *tplSource.GetCustom() == nil {
		return cfg, ErrTemplateSourceCustomIsNil
	}

	if *data.GetBase() == nil {
		return cfg, ErrDataSourceBaseIsNil
	}

	tmpl := Templates{}

	mergedTmpl, ok := (*tplSource.GetCustom()).Content()["templates"]
	if ok {
		tmplMap, err := newTemplatesFromMap(mergedTmpl)
		if err != nil {
			return cfg, err
		}

		tmpl = *tmplMap
	}

	tmpl.Excludes = append(tmpl.Excludes, excluded...)

	cfg.Templates = tmpl
	cfg.Data = mapx.ToMapStringAny((*data.GetBase()).Content())
	cfg.Include = nil

	return cfg, nil
}

func newTemplatesFromMap(t any) (*Templates, error) {
	var exc []string
	var inc []string
	var err error

	m, ok := t.(map[any]any)
	if !ok {
		return nil, fmt.Errorf("cannot convert %v to map", t)
	}

	incS, ok := m["includes"].([]any)
	if !ok {
		incS = nil
	}

	inc, err = toTypeSlice[string](incS)
	if err != nil {
		return nil, err
	}

	excS, ok := m["excludes"].([]any)
	if !ok {
		excS = nil
	}

	exc, err = toTypeSlice[string](excS)
	if err != nil {
		return nil, err
	}

	suf, err := toType[string](m["suffix"])
	if err != nil {
		return nil, err
	}

	pro, err := toType[bool](m["processFilename"])
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

func toTypeSlice[T any](t []any) ([]T, error) {
	s := make([]T, len(t))

	if t == nil {
		return s, nil
	}

	for i, v := range t {
		sV, err := toType[T](v)
		if err != nil {
			return s, err
		}

		s[i] = sV
	}

	return s, nil
}

func toType[T any](t any) (T, error) {
	var s T

	if t == nil {
		return s, nil
	}

	s, ok := t.(T)
	if !ok {
		return s, fmt.Errorf("error while converting to %s", reflect.TypeOf(s))
	}

	return s, nil
}
