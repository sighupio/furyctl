// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package del

import (
	"encoding/json"
	"fmt"
	"path"
	"strings"
)

// selector is a kapp resource filter: an object matches when it matches every list that is set.
type selector struct {
	Kinds      []string `json:"kinds,omitempty"`
	Namespaces []string `json:"namespaces,omitempty"`
	Names      []string `json:"names,omitempty"`
}

// exclusion lists the objects that a kapp delete pass leaves alone.
type exclusion []selector

// withExclusion returns params plus the kapp --filter of the exclusion, if any.
func withExclusion(params []string, skip exclusion) ([]string, error) {
	if len(skip) == 0 {
		return params, nil
	}

	resources := make([]map[string]selector, len(skip))
	for i, s := range skip {
		resources[i] = map[string]selector{"resource": s}
	}

	filter, err := json.Marshal(map[string]any{"not": map[string]any{"or": resources}})
	if err != nil {
		return nil, fmt.Errorf("error building the kapp filter: %w", err)
	}

	return append(append([]string{}, params...), "--filter", string(filter)), nil
}

func (e exclusion) excludes(o object) bool {
	for _, s := range e {
		if s.matches(o) {
			return true
		}
	}

	return false
}

func (s selector) matches(o object) bool {
	return matchesAny(s.Kinds, o.Kind) && matchesAny(s.Namespaces, o.Metadata.Namespace) &&
		matchesAny(s.Names, o.Metadata.Name)
}

// matchesAny reports whether value matches one of the kapp patterns, where % matches any text; no pattern matches all.
func matchesAny(patterns []string, value string) bool {
	if len(patterns) == 0 {
		return true
	}

	for _, p := range patterns {
		if ok, err := path.Match(strings.ReplaceAll(p, "%", "*"), value); err == nil && ok {
			return true
		}
	}

	return false
}
