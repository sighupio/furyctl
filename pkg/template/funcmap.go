// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package template

import (
	"errors"
	"fmt"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"gopkg.in/yaml.v2"
)

var (
	ErrDigAnyInsufficientArgs = errors.New("digAny: not enough arguments, needs at least 3 arguments")
	ErrDigAnyInvalidKeyType   = errors.New("digAny: argument is not a string")
	ErrDigAnyInvalidDictType  = errors.New("digAny: last argument must be a map[any]any")
)

type FuncMap struct {
	FuncMap template.FuncMap
}

func NewFuncMap() FuncMap {
	return FuncMap{FuncMap: sprig.TxtFuncMap()}
}

func (f *FuncMap) Add(name string, fn any) {
	f.FuncMap[name] = fn
}

func (f *FuncMap) Delete(name string) {
	delete(f.FuncMap, name)
}

func ToYAML(v any) string {
	//nolint:errcheck // we don't care about the error here because we recover from it
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

func digFromDictAny(dict map[any]any, d any, ks []string) (any, error) {
	k, ns := ks[0], ks[1:]
	step, has := dict[k]

	if !has {
		return d, nil
	}

	if len(ns) == 0 {
		return step, nil
	}

	// Ensure the next step is a map before recursing. If it's not a map,
	// return the default value.
	next, ok := step.(map[any]any)
	if !ok {
		return d, nil
	}

	return digFromDictAny(next, d, ns)
}

// This is a copy of the Sprig dig function that recurse a dict with any keys
// instead of map keys.
// `digAny` will recurse all the specified keys of the given dict and return the
// last key value if found.
// If any of the keys does not exist, digAny will return the default value
// passed as the last argument.
// Usage in templates:
//
//	{{ digAny "key1" "keyN" <default value> <dict> }}
func DigAny(ps ...any) (any, error) {
	const minArgs = 3
	if len(ps) < minArgs {
		return nil, ErrDigAnyInsufficientArgs
	}

	// Build keys slice, validating each key is a string.
	const knownArgs = 2
	count := len(ps) - knownArgs

	ks := make([]string, count)

	for i := range count {
		s, ok := ps[i].(string)
		if !ok {
			return nil, fmt.Errorf("%w (position %d)", ErrDigAnyInvalidKeyType, i+1)
		}

		ks[i] = s
	}

	// Default value is the penultimate argument.
	def := ps[len(ps)-2]

	// Last argument should be a dict (map[any]any).
	last := ps[len(ps)-1]
	dict, ok := last.(map[any]any)

	if !ok {
		return nil, ErrDigAnyInvalidDictType
	}

	return digFromDictAny(dict, def, ks)
}

func HasKeyAny(m map[any]any, key any) bool {
	v, ok := m[key]
	if !ok {
		return false
	}

	if v == nil {
		return false
	}

	val, ok := v.(map[any]any)
	if ok {
		return len(val) > 0
	}

	return true
}
