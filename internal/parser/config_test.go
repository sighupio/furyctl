// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package parserx_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	parserx "github.com/sighupio/furyctl/internal/parser"
)

func TestNewConfigParser(t *testing.T) {
	t.Parallel()

	cfgParser := parserx.NewConfigParser("dummy/base/dir")

	assert.NotNil(t, cfgParser)
}

func TestConfigParser_ParseDynamicValue(t *testing.T) {
	t.Parallel()

	pathTmpDir, err := os.MkdirTemp("", "test")
	require.NoError(t, err)

	testCases := []struct {
		name     string
		setup    func() (baseDir, value string, teardown func())
		expected string
	}{
		{
			name: "no parsing",
			setup: func() (string, string, func()) {
				return "", "hello test", func() {}
			},
			expected: "hello test",
		},
		{
			name: "unknown token parsing",
			setup: func() (string, string, func()) {
				return "", "{unknown://hello test}", func() {}
			},
			expected: "{unknown://hello test}",
		},
		{
			name: "parsing env",
			setup: func() (string, string, func()) {
				require.NoError(t, os.Setenv("TEST_ENV_VAR", "hello test"))

				return "", "{env://TEST_ENV_VAR}", func() {
					require.NoError(t, os.Unsetenv("TEST_ENV_VAR"))
				}
			},
			expected: "hello test",
		},
		{
			name: "parsing file - relative path",
			setup: func() (string, string, func()) {
				tmpDir, err := os.MkdirTemp("", "test")
				require.NoError(t, err)

				require.NoError(t, os.WriteFile(tmpDir+"/test_file.txt", []byte("hello test"), os.ModePerm))

				return tmpDir, fmt.Sprintf("{file://./test_file.txt}"), func() {
					os.RemoveAll(tmpDir)
				}
			},
			expected: "hello test",
		},
		{
			name: "parsing file - absolute path",
			setup: func() (string, string, func()) {
				tmpDir, err := os.MkdirTemp("", "test")
				require.NoError(t, err)

				require.NoError(t, os.WriteFile(tmpDir+"/test_file.txt", []byte("hello test"), os.ModePerm))

				return tmpDir, fmt.Sprintf("{file://%s}", path.Join(tmpDir, "test_file.txt")), func() {
					os.RemoveAll(tmpDir)
				}
			},
			expected: "hello test",
		},
		{
			name: "parsing http",
			setup: func() (string, string, func()) {
				// httptest.NewServer binds its port synchronously and is ready to
				// serve as soon as it returns, so there is no listener-readiness
				// race and no random-port collision.
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					fmt.Fprint(w, "hello test")
				}))

				return "", fmt.Sprintf("{%s}", server.URL), server.Close
			},
			expected: "hello test",
		},
		{
			name: "parsing path relative - one level",
			setup: func() (string, string, func()) {
				require.NoError(t, os.WriteFile(pathTmpDir+"/test_file.txt", []byte("hello test"), os.ModePerm))

				return pathTmpDir, fmt.Sprintf("{path://./test_file.txt}"), func() {}
			},
			expected: path.Join(pathTmpDir, "/test_file.txt"),
		},
		{
			name: "parsing path relative - two levels",
			setup: func() (string, string, func()) {
				err := os.Mkdir(pathTmpDir+"/test_dir", os.ModePerm)
				require.NoError(t, err)

				require.NoError(t, os.WriteFile(pathTmpDir+"/test_file_2.txt", []byte("hello test"), os.ModePerm))

				return path.Join(pathTmpDir, "/test_dir"), fmt.Sprintf("{path://../test_file_2.txt}"), func() {}
			},
			expected: path.Join(pathTmpDir, "/test_file_2.txt"),
		},
	}

	t.Run("group", func(t *testing.T) {
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				baseDir, value, teardownFn := tc.setup()

				defer teardownFn()

				cfgParser := parserx.NewConfigParser(baseDir)

				res, err := cfgParser.ParseDynamicValue(value)

				assert.NoError(t, err)
				assert.Equal(t, tc.expected, res)
			})
		}
	})

	os.RemoveAll(pathTmpDir)
}
