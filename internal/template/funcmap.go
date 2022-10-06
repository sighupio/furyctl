// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package template

import (
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"gopkg.in/yaml.v2"
)

type FuncMap struct {
	FuncMap template.FuncMap
}

func NewFuncMap() FuncMap {
	return FuncMap{FuncMap: sprig.TxtFuncMap()}
}

func (f *FuncMap) Add(name string, fn interface{}) {
	f.FuncMap[name] = fn
}

func (f *FuncMap) Delete(name string) {
	delete(f.FuncMap, name)
}

func ToYAML(v any) string {
	defer func() {
		_ = recover()
	}()

	data, err := yaml.Marshal(v)
	if err != nil {
		// Swallow errors inside of a template.
		return ""
	}
	return strings.TrimSuffix(string(data), "\n")
}

func FromYAML(str string) map[string]any {
	m := map[string]any{}

	if err := yaml.Unmarshal([]byte(str), &m); err != nil {
		m["Error"] = err.Error()
	}
	return m
}
