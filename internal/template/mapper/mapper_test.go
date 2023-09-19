// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package mapper_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

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

	m := mapper.NewMapper(
		dummyContext,
		"dummy/furyctlconf/path/furyctl.yaml",
	)

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

	m := mapper.NewMapper(
		dummyContext,
		"dummy/furyctlconf/path/furyctl.yaml",
	)

	err := os.Setenv("TEST_MAPPER_ENV", "test")

	assert.NoError(t, err)

	defer os.Setenv("TEST_MAPPER_ENV", "")

	envMap := m.MapEnvironmentVars()

	assert.Equal(t, expectedEnvMap["TEST_MAPPER_ENV"], envMap["TEST_MAPPER_ENV"])
}

func TestMapper_MapDynamicValuesAndPaths(t *testing.T) {
	path, err := os.MkdirTemp("", "test")

	assert.NoError(t, err)

	exampleStr := "test!"

	err = os.WriteFile(path+"/test_file.txt", []byte(exampleStr), os.ModePerm)

	defer os.RemoveAll(path)

	assert.NoError(t, err)

	dummyContext := map[string]map[any]any{
		"data": {
			"meta": map[any]any{
				"name":  "{env://TEST_MAPPER_DYNAMIC_VALUE}",
				"value": fmt.Sprintf("{file://%s/test_file.txt}", path),
			},
		},
	}

	m := mapper.NewMapper(
		dummyContext,
		"dummy/furyctlconf/path/furyctl.yaml",
	)

	err = os.Setenv("TEST_MAPPER_DYNAMIC_VALUE", "test")

	assert.NoError(t, err)

	defer os.Setenv("TEST_MAPPER_DYNAMIC_VALUE", "")

	filledContext, err := m.MapDynamicValuesAndPaths()

	assert.NoError(t, err)

	meta := filledContext["data"]["meta"]

	mapMeta, ok := meta.(map[any]any)

	assert.True(t, ok)

	assert.Equal(t, "test", mapMeta["name"])

	assert.Equal(t, exampleStr, mapMeta["value"])
}

func TestMapper_MapDynamicValuesAndPaths_RelativePath(t *testing.T) {
	path := "../.."

	timestamp := time.Now().Unix()

	fileName := fmt.Sprintf("test_file-%v.txt", timestamp)

	filePath := filepath.Join(path, fileName)

	exampleStr := "test!"

	err := os.WriteFile(filePath, []byte(exampleStr), os.ModePerm)

	defer os.RemoveAll(filePath)

	assert.NoError(t, err)

	dummyContext := map[string]map[any]any{
		"data": {
			"meta": map[any]any{
				"name":  "{env://TEST_MAPPER_DYNAMIC_VALUE}",
				"value": fmt.Sprintf("{file://%s}", filePath),
			},
		},
	}

	m := mapper.NewMapper(
		dummyContext,
		"dummy/furyctlconf/path/furyctl.yaml",
	)

	err = os.Setenv("TEST_MAPPER_DYNAMIC_VALUE", "test")

	assert.NoError(t, err)

	defer os.Setenv("TEST_MAPPER_DYNAMIC_VALUE", "")

	filledContext, err := m.MapDynamicValuesAndPaths()

	assert.NoError(t, err)

	meta := filledContext["data"]["meta"]

	mapMeta, ok := meta.(map[any]any)

	assert.True(t, ok)

	assert.Equal(t, "test", mapMeta["name"])

	assert.Equal(t, exampleStr, mapMeta["value"])
}

func TestMapper_MapDynamicValuesAndPaths_Combined(t *testing.T) {
	path, err := os.MkdirTemp("", "test")

	assert.NoError(t, err)

	exampleStr := "test!"

	err = os.WriteFile(path+"/test_file.txt", []byte(exampleStr), os.ModePerm)

	defer os.RemoveAll(path)

	assert.NoError(t, err)

	dummyContext := map[string]map[any]any{
		"data": {
			"meta": map[any]any{
				"name":   fmt.Sprintf("{env://TEST_MAPPER_DYNAMIC_VALUE}/plaintext/{file://%s/test_file.txt}", path),
				"value":  fmt.Sprintf("{file://%s/test_file.txt}/plaintext/{env://TEST_MAPPER_DYNAMIC_VALUE}", path),
				"double": "{env://TEST_MAPPER_DYNAMIC_VALUE}/{env://TEST_MAPPER_DYNAMIC_VALUE}",
			},
		},
	}

	m := mapper.NewMapper(
		dummyContext,
		"dummy/furyctlconf/path/furyctl.yaml",
	)

	err = os.Setenv("TEST_MAPPER_DYNAMIC_VALUE", "test")

	assert.NoError(t, err)

	defer os.Setenv("TEST_MAPPER_DYNAMIC_VALUE", "")

	filledContext, err := m.MapDynamicValuesAndPaths()

	assert.NoError(t, err)

	meta := filledContext["data"]["meta"]

	mapMeta, ok := meta.(map[any]any)

	assert.True(t, ok)

	assert.Equal(t, "test/plaintext/test!", mapMeta["name"])

	assert.Equal(t, "test!/plaintext/test", mapMeta["value"])

	assert.Equal(t, "test/test", mapMeta["double"])
}

func TestMapper_MapDynamicValuesAndPaths_SliceString(t *testing.T) {
	dummyContext := map[string]map[any]any{
		"data": {
			"meta": []any{
				"{env://TEST_MAPPER_DYNAMIC_VALUE}",
			},
		},
	}

	m := mapper.NewMapper(
		dummyContext,
		"dummy/furyctlconf/path/furyctl.yaml",
	)

	err := os.Setenv("TEST_MAPPER_DYNAMIC_VALUE", "test")

	assert.NoError(t, err)

	defer os.Setenv("TEST_MAPPER_DYNAMIC_VALUE", "")

	filledContext, err := m.MapDynamicValuesAndPaths()

	assert.NoError(t, err)

	meta := filledContext["data"]["meta"]

	sliceMeta, ok := meta.([]any)

	assert.True(t, ok)

	assert.Equal(t, "test", sliceMeta[0])
}

func TestMapper_MapDynamicValuesAndPaths_SliceMap(t *testing.T) {
	dummyContext := map[string]map[any]any{
		"data": {
			"meta": []any{
				map[any]any{
					"value": "{env://TEST_MAPPER_DYNAMIC_VALUE}",
				},
			},
		},
	}

	m := mapper.NewMapper(
		dummyContext,
		"dummy/furyctlconf/path/furyctl.yaml",
	)

	err := os.Setenv("TEST_MAPPER_DYNAMIC_VALUE", "test")

	assert.NoError(t, err)

	defer os.Setenv("TEST_MAPPER_DYNAMIC_VALUE", "")

	filledContext, err := m.MapDynamicValuesAndPaths()

	assert.NoError(t, err)

	meta := filledContext["data"]["meta"]

	sliceMeta, ok := meta.([]any)
	sliceMeta0, ok := sliceMeta[0].(map[any]any)

	assert.True(t, ok)

	assert.Equal(t, "test", sliceMeta0["value"])
}

func TestMapper_MapDynamicValuesAndPaths_RelativePathToFuryctlConf(t *testing.T) {
	dummyContext := map[string]map[any]any{
		"data": {
			"meta": map[any]any{
				"singleDot":     "./test_file.txt",
				"doubleDots":    "../test_file.txt",
				"twoDoubleDots": "../../test_file.txt",
			},
		},
	}

	m := mapper.NewMapper(
		dummyContext,
		"dummy/furyctlconf/path/furyctl.yaml",
	)

	filledContext, err := m.MapDynamicValuesAndPaths()

	assert.NoError(t, err)

	meta := filledContext["data"]["meta"]

	mapMeta, ok := meta.(map[any]any)

	assert.True(t, ok)

	assert.Equal(t, "dummy/furyctlconf/path/test_file.txt", mapMeta["singleDot"])
	assert.Equal(t, "dummy/furyctlconf/test_file.txt", mapMeta["doubleDots"])
	assert.Equal(t, "dummy/test_file.txt", mapMeta["twoDoubleDots"])
}
