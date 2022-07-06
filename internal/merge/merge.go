// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package merge

import (
	"fmt"
)

type Merger struct {
	base   Mergeable
	custom Mergeable
}

func NewMerger(b, c Mergeable) *Merger {
	return &Merger{
		base:   b,
		custom: c,
	}
}

func (m *Merger) Merge() (map[string]any, error) {
	preparedBase, err := m.base.Get()
	if err != nil {
		return nil, fmt.Errorf("incorrect base file, %s", err.Error())
	}

	preparedCustom, err := m.custom.Get()
	if err != nil {
		return preparedBase, nil
	}

	mergedSection := deepCopy(preparedBase, preparedCustom)

	err = m.base.Walk(mergedSection)

	return m.base.Content(), err

}

func deepCopy(a, b map[string]any) map[string]any {
	out := make(map[string]any, len(a))
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		if v, ok := v.(map[string]any); ok {
			if bv, ok := out[k]; ok {
				if bv, ok := bv.(map[string]any); ok {
					out[k] = deepCopy(bv, v)
					continue
				}
			}
		}
		out[k] = v
	}
	return out
}
