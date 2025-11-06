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

// HasField checks if a nested field exists in a data structure using dot-notation path.
// Returns true if the field exists (even if nil/empty), false if any intermediate key is missing.
//
// Example:
//
//	HasField(data, "spec.kubernetes.apiServer.privateAccess")
//	HasField(data, "metadata.name")
//
// This function never panics and always returns false for malformed paths or type mismatches.
func HasField(data any, path string) bool {
	if data == nil || path == "" {
		return false
	}

	keys := strings.Split(path, ".")
	current := data

	for _, key := range keys {
		if key == "" {
			return false
		}

		// Try map[any]any first (most common in templates).
		if m, ok := current.(map[any]any); ok {
			val, exists := m[key]
			if !exists {
				return false
			}

			current = val

			continue
		}

		// Try map[string]any as fallback.
		if m, ok := current.(map[string]any); ok {
			val, exists := m[key]
			if !exists {
				return false
			}

			current = val

			continue
		}

		// Current value is not a map, can't traverse further.
		return false
	}

	return true
}

// GetFieldOrDefault retrieves a nested field value using dot-notation path.
// Returns the field value if found, otherwise returns the provided default value.
//
// Example:
//
//	GetFieldOrDefault(data, "spec.kubernetes.version", "1.28")
//	GetFieldOrDefault(data, "metadata.labels.env", "production")
//
// This function never panics. It returns the default value for:
//   - Missing keys at any level
//   - Nil values at any level
//   - Type mismatches during traversal
//   - Empty or malformed paths
func GetFieldOrDefault(data any, path string, defaultValue any) any {
	if data == nil || path == "" {
		return defaultValue
	}

	keys := strings.Split(path, ".")
	current := data

	for i, key := range keys {
		if key == "" {
			return defaultValue
		}

		// Try map[any]any first (most common in templates).
		if m, ok := current.(map[any]any); ok {
			val, exists := m[key]
			if !exists {
				return defaultValue
			}

			// If this is the last key, return the value (even if nil).
			if i == len(keys)-1 {
				if val == nil {
					return defaultValue
				}

				return val
			}

			current = val

			continue
		}

		// Try map[string]any as fallback.
		if m, ok := current.(map[string]any); ok {
			val, exists := m[key]
			if !exists {
				return defaultValue
			}

			// If this is the last key, return the value (even if nil).
			if i == len(keys)-1 {
				if val == nil {
					return defaultValue
				}

				return val
			}

			current = val

			continue
		}

		// Current value is not a map, can't traverse further.
		return defaultValue
	}

	// Should not reach here, but return default as safety.
	return defaultValue
}
