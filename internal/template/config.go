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

	mergedTmpl, ok := (*tplSource.GetCustom()).Content()["templates"]
	if !ok {
		return cfg, ErrTemplatesNotFound
	}

	tmpl, err := newTemplatesFromMap(mergedTmpl)
	if err != nil {
		return cfg, err
	}

	tmpl.Excludes = append(tmpl.Excludes, excluded...)

	cfg.Templates = *tmpl
	cfg.Data = mapx.ToMapStringAny((*data.GetBase()).Content())
	cfg.Include = nil

	return cfg, nil
}

func newTemplatesFromMap(t any) (*Templates, error) {
	m, ok := t.(map[any]any)
	if !ok {
		return nil, fmt.Errorf("cannot convert %v to map", t)
	}

	inc, err := toTypeSlice[string](m["includes"])
	if err != nil {
		return nil, err
	}

	exc, err := toTypeSlice[string](m["excludes"])
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

func toTypeSlice[T any](t any) ([]T, error) {
	if t == nil {
		return nil, nil
	}

	s, ok := t.([]any)
	if !ok {
		return nil, fmt.Errorf("error while converting to slice")
	}

	ret := make([]T, len(s))

	for i, v := range s {
		ret[i], ok = v.(T)
		if !ok {
			return nil, fmt.Errorf("error while converting to %s slice", reflect.TypeOf(ret[i]))
		}
	}

	return ret, nil
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
