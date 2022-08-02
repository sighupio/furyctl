// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package merge

import (
	"fmt"
	"strings"
)

type Mergeable interface {
	Get() (map[any]any, error)
	Walk(map[any]any) error
	Content() map[any]any
	Path() string
}

type DefaultModel struct {
	content map[any]any
	path    string
}

func NewDefaultModel(content map[any]any, path string) *DefaultModel {
	return &DefaultModel{
		content: content,
		path:    path,
	}
}
func (b *DefaultModel) Content() map[any]any {
	return b.content
}

func (b *DefaultModel) Path() string {
	return b.path
}

func (b *DefaultModel) Get() (map[any]any, error) {
	ret := b.content

	fields := strings.Split((*b).path[1:], ".")

	for _, f := range fields {
		mapAtKey, ok := ret[f]
		if !ok {
			return nil, fmt.Errorf("cannot access key %s on map", f)
		}

		ret, ok = mapAtKey.(map[any]any)
		if !ok {
			return nil, fmt.Errorf("data structure is invalid on key %s", f)
		}
	}

	return ret, nil
}

func (b *DefaultModel) Walk(mergedSection map[any]any) error {
	ret := b.content

	fields := strings.Split(b.Path()[1:], ".")

	for _, f := range fields[:len(fields)-1] {
		_, ok := ret[f]
		if !ok {
			return fmt.Errorf("cannot access key %s on map", f)
		}

		ret, ok = ret[f].(map[any]any)
		if !ok {
			return fmt.Errorf("data structure is invalid on key %s", f)
		}
	}

	ret[fields[len(fields)-1]] = mergedSection

	return nil
}
