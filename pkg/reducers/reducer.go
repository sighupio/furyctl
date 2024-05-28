// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package v1alpha2

import "fmt"

type Reducers []Reducer

type Reducer interface {
	Prepare() map[any]any
	GetLifecycle() string
	GetKey() string
	GetPath() string
	GetFrom() any
	GetTo() any
}

type BaseReducer struct {
	Path      string
	Key       string
	From      any
	To        any
	Lifecycle string
}

func NewBaseReducer(key string, from, to any, lifecycle, path string) *BaseReducer {
	return &BaseReducer{
		Key:       key,
		From:      from,
		To:        to,
		Lifecycle: lifecycle,
		Path:      path,
	}
}

func (r *BaseReducer) Prepare() map[any]any {
	return map[any]any{
		r.Key: map[string]any{
			"from": r.From,
			"to":   r.To,
		},
	}
}

func (r *BaseReducer) GetLifecycle() string {
	return r.Lifecycle
}

func (r *BaseReducer) GetKey() string {
	return r.Key
}

func (r *BaseReducer) GetPath() string {
	return r.Path
}

func (r *BaseReducer) GetFrom() any {
	return r.From
}

func (r *BaseReducer) GetTo() any {
	return r.To
}

func (rs Reducers) ByLifecycle(lifecycle string) Reducers {
	var filtered Reducers

	if len(rs) == 0 {
		return filtered
	}

	for _, r := range rs {
		if r == nil {
			continue
		}

		if r.GetLifecycle() == lifecycle {
			filtered = append(filtered, r)
		}
	}

	return filtered
}

func (rs Reducers) Combine(origin map[string]map[any]any, key string) map[string]map[any]any {
	for _, r := range rs {
		if origin[key] == nil {
			origin[key] = make(map[any]any)
		}

		origin[key][r.GetKey()] = r.Prepare()[r.GetKey()]
	}

	return origin
}

func (rs Reducers) ToString() string {
	res := ""

	for _, r := range rs {
		if r == nil {
			continue
		}

		res += fmt.Sprintf("%s: %v -> %v\n", r.GetPath(), r.GetFrom(), r.GetTo())
	}

	return res
}
