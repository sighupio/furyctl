// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mapx

import (
	"fmt"
	"reflect"
	"strings"
)

func FromStruct(s any, tagType string, skipEmpty bool) map[any]any {
	if s == nil {
		return nil
	}

	out := make(map[any]any)

	sType := reflect.TypeOf(s)

	if sType.Kind() != reflect.Struct {
		return nil
	}

	sVal := reflect.ValueOf(s)

	for i := 0; i < sVal.NumField(); i++ {
		if !sVal.Field(i).CanInterface() {
			continue
		}

		fieldName := sType.Field(i).Name

		if tagType != "" {
			tag, ok := sType.Field(i).Tag.Lookup(tagType)
			if ok {
				tag = strings.Split(tag, ",")[0]
				fieldName = tag
			}
		}

		val := sVal.Field(i)

		if val.Kind() == reflect.Ptr {
			val = reflect.Indirect(val)
		}

		if !val.IsValid() {
			if !skipEmpty {
				out[fieldName] = nil
			}

			continue
		}

		if skipEmpty && val.IsZero() {
			continue
		}

		if val.Kind() != reflect.Struct {
			out[fieldName] = val.Interface()

			continue
		}

		out[fieldName] = FromStruct(val.Interface(), tagType, skipEmpty)
	}

	return out
}

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
