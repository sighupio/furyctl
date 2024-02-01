// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package parser_test

import (
	"fmt"
	"os"
	"path"
	"testing"

	"github.com/sighupio/furyctl/internal/parser"
	"github.com/stretchr/testify/assert"
)

func TestNewConfigParser(t *testing.T) {
	t.Parallel()

	cfgParser := parser.NewConfigParser("dummy/base/dir")

	assert.NotNil(t, cfgParser)
}

func TestConfigParser_ParseDynamicValue(t *testing.T) {
	t.Parallel()

	tmpDir, err := os.MkdirTemp("", "test")
	if err != nil {
		t.Fatal(err)
	}

	exampleStr := "test"

	err = os.WriteFile(tmpDir+"/test_file.txt", []byte(exampleStr), os.ModePerm)

	defer os.RemoveAll(tmpDir)

	assert.NoError(t, err)

	testCases := []struct {
		name     string
		baseDir  string
		envName  string
		envValue string
		value    any
		expected string
	}{
		{
			name:     "parsing env",
			baseDir:  "dummy/base/dir",
			envName:  "TEST_ENV_VAR",
			envValue: "test",
			value:    "{env://TEST_ENV_VAR}",
			expected: "test",
		},
		{
			name:     "parsing file - relative path",
			baseDir:  tmpDir,
			value:    fmt.Sprintf("{file://./test_file.txt}"),
			expected: "test",
		},
		{
			name:     "parsing file - absolute path",
			baseDir:  tmpDir,
			value:    fmt.Sprintf("{file://%s}", path.Join(tmpDir, "test_file.txt")),
			expected: "test",
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			cfgParser := parser.NewConfigParser(tc.baseDir)

			if tc.envName != "" {
				err := os.Setenv(tc.envName, tc.envValue)

				assert.NoError(t, err)
			}

			res, err := cfgParser.ParseDynamicValue(tc.value)

			assert.NoError(t, err)
			assert.Equal(t, tc.expected, res)
		})
	}
}
