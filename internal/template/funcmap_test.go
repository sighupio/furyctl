// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package template_test

import (
	"github.com/sighupio/furyctl/internal/template"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNewFuncMap(t *testing.T) {
	f := template.NewFuncMap()

	assert.True(t, len(f.FuncMap) > 0)
}

func TestFuncMap_Add(t *testing.T) {
	f := template.NewFuncMap()

	f.Add("test", func() string {
		return "test"
	})

	assert.NotNil(t, f.FuncMap["test"])
}

func TestFuncMap_Delete(t *testing.T) {
	f := template.NewFuncMap()

	f.Add("test", func() string {
		return "test"
	})

	f.Delete("test")

	assert.Nil(t, f.FuncMap["test"])
}
