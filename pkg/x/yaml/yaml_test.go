// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package yamlx_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	yamlx "github.com/sighupio/furyctl/pkg/x/yaml"
)

type TestYaml struct {
	Test string `yaml:"test"`
}

func TestFromFileV2(t *testing.T) {
	test := TestYaml{
		"test",
	}

	path, err := os.MkdirTemp("", "test")

	assert.NoError(t, err)

	file, err := os.Create(path + "/test.yaml")

	assert.NoError(t, err)

	testBytes, err := yamlx.MarshalV2(test)

	assert.NoError(t, err)

	_, err = file.Write(testBytes)

	assert.NoError(t, err)

	defer os.RemoveAll(path)

	testRes, err := yamlx.FromFileV2[TestYaml](path + "/test.yaml")

	assert.NoError(t, err)

	assert.Equal(t, test, testRes)
}

func TestFromFileV3(t *testing.T) {
	test := TestYaml{
		"test",
	}

	path, err := os.MkdirTemp("", "test")

	assert.NoError(t, err)

	file, err := os.Create(path + "/test.yaml")

	assert.NoError(t, err)

	testBytes, err := yamlx.MarshalV3(test)

	assert.NoError(t, err)

	_, err = file.Write(testBytes)

	assert.NoError(t, err)

	defer os.RemoveAll(path)

	testRes, err := yamlx.FromFileV3[TestYaml](path + "/test.yaml")

	assert.NoError(t, err)

	assert.Equal(t, test, testRes)
}
