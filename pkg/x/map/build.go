// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mapx

import (
	"fmt"
	"reflect"
	"strings"
)

type Builder struct {
	skipEmpty bool
}

func NewBuilder(skipEmpty bool) *Builder {
	return &Builder{
		skipEmpty: skipEmpty,
	}
}

func (b *Builder) FromStruct(s any, tagType string) map[any]any {
	if s == nil {
		return nil
	}

	out := make(map[any]any)

	sType := reflect.TypeOf(s)

	if sType.Kind() != reflect.Struct {
		return nil
	}

	sVal := reflect.ValueOf(s)

	for field, value := range sVal.Fields() {
		if !value.CanInterface() {
			continue
		}

		fieldName := field.Name

		if tagType != "" {
			if tag, ok := field.Tag.Lookup(tagType); ok {
				fieldName = strings.Split(tag, ",")[0]
			}
		}

		if value.Kind() == reflect.Pointer {
			value = reflect.Indirect(value)
		}

		if !value.IsValid() {
			if !b.skipEmpty {
				out[fieldName] = nil
			}

			continue
		}

		if b.skipEmpty && value.IsZero() {
			continue
		}

		if value.Kind() != reflect.Struct {
			out[fieldName] = value.Interface()

			continue
		}

		out[fieldName] = b.FromStruct(value.Interface(), tagType)
	}

	return out
}

func (*Builder) ToMapStringAny(t map[any]any) map[string]map[any]any {
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
