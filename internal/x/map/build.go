// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mapx

import (
	"fmt"
	"reflect"
)

func ToMapStringAny(t map[any]any) map[string]map[any]any {
	out := make(map[string]map[any]any)

	for k, v := range t {
		val, ok := v.(map[any]any)
		if !ok {
			continue
		}

		out[fmt.Sprintf("%v", k)] = val
	}

	return out
}

func ToTypeSlice[T any](t any) ([]T, error) {
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

func ToType[T any](t any) (T, error) {
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
