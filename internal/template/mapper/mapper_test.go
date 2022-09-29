// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package mapper_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sighupio/furyctl/internal/template/mapper"
)

func TestNewMapper(t *testing.T) {
	dummyContext := map[string]map[any]any{
		"data": {
			"meta": map[any]any{
				"name": "test",
			},
		},
	}

	m := mapper.NewMapper(dummyContext)

	assert.NotNil(t, m)
}

func TestMapper_MapEnvironmentVars(t *testing.T) {
	dummyContext := map[string]map[any]any{
		"data": {
			"meta": map[any]any{
				"name": "test",
			},
		},
	}

	expectedEnvMap := map[string]string{
		"TEST_MAPPER_ENV": "test",
	}

	m := mapper.NewMapper(dummyContext)

	err := os.Setenv("TEST_MAPPER_ENV", "test")

	assert.NoError(t, err)

	defer os.Setenv("TEST_MAPPER_ENV", "")

	envMap := m.MapEnvironmentVars()

	assert.Equal(t, expectedEnvMap["TEST_MAPPER_ENV"], envMap["TEST_MAPPER_ENV"])
}

func TestMapper_MapDynamicValues(t *testing.T) {
	path, err := os.MkdirTemp("", "test")

	assert.NoError(t, err)

	exampleStr := "test!"

	err = os.WriteFile(path+"/test_file.txt", []byte(exampleStr), os.ModePerm)

	defer os.RemoveAll(path)

	assert.NoError(t, err)

	dummyContext := map[string]map[any]any{
		"data": {
			"meta": map[any]any{
				"name":  map[any]any{"env://TEST_MAPPER_DYNAMIC_VALUE": ""},
				"value": map[any]any{fmt.Sprintf("file://%s/test_file.txt", path): ""},
			},
		},
	}

	m := mapper.NewMapper(dummyContext)

	err = os.Setenv("TEST_MAPPER_DYNAMIC_VALUE", "test")

	assert.NoError(t, err)

	defer os.Setenv("TEST_MAPPER_DYNAMIC_VALUE", "")

	filledContext, err := m.MapDynamicValues()

	assert.NoError(t, err)

	meta := filledContext["data"]["meta"]

	mapMeta, ok := meta.(map[any]any)

	assert.True(t, ok)

	assert.Equal(t, "test", mapMeta["name"])

	assert.Equal(t, exampleStr, mapMeta["value"])
}
