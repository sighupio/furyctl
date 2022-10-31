// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package merge

import (
	"errors"
	"fmt"
	"strings"

	mapx "github.com/sighupio/furyctl/internal/x/map"
)

var (
	errCannotAccessKey = errors.New("cannot access key")
	errInvalidData     = errors.New("data structure is invalid on key")
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

func NewDefaultModelFromStruct(content any, path string, skipEmpty bool) *DefaultModel {
	builder := mapx.NewBuilder(skipEmpty)

	c := builder.FromStruct(content, "json")

	return NewDefaultModel(c, path)
}

func (b *DefaultModel) Content() map[any]any {
	return b.content
}

func (b *DefaultModel) Path() string {
	return b.path
}

func (b *DefaultModel) Get() (map[any]any, error) {
	ret := b.content

	fields := strings.Split(b.path[1:], ".")

	for _, f := range fields {
		mapAtKey, ok := ret[f]
		if !ok {
			return nil, fmt.Errorf("%w %s on map", errCannotAccessKey, f)
		}

		ret, ok = mapAtKey.(map[any]any)
		if !ok {
			return nil, fmt.Errorf("%w %s", errInvalidData, f)
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
			return fmt.Errorf("%w %s on map", errCannotAccessKey, f)
		}

		ret, ok = ret[f].(map[any]any)
		if !ok {
			return fmt.Errorf("%w %s", errInvalidData, f)
		}
	}

	ret[fields[len(fields)-1]] = mergedSection

	return nil
}
