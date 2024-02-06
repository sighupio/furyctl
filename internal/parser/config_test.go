// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package parser_test

import (
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"path"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sighupio/furyctl/internal/parser"
)

func TestNewConfigParser(t *testing.T) {
	t.Parallel()

	cfgParser := parser.NewConfigParser("dummy/base/dir")

	assert.NotNil(t, cfgParser)
}

func TestConfigParser_ParseDynamicValue(t *testing.T) {
	t.Parallel()

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
				if err := os.Setenv("TEST_ENV_VAR", "hello test"); err != nil {
					t.Fatal(err)
				}

				return "", "{env://TEST_ENV_VAR}", func() {
					if err := os.Unsetenv("TEST_ENV_VAR"); err != nil {
						t.Fatal(err)
					}
				}
			},
			expected: "hello test",
		},
		{
			name: "parsing file - relative path",
			setup: func() (string, string, func()) {
				tmpDir, err := os.MkdirTemp("", "test")
				if err != nil {
					t.Fatal(err)
				}

				if err := os.WriteFile(tmpDir+"/test_file.txt", []byte("hello test"), os.ModePerm); err != nil {
					t.Fatal(err)
				}

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
				if err != nil {
					t.Fatal(err)
				}

				if err := os.WriteFile(tmpDir+"/test_file.txt", []byte("hello test"), os.ModePerm); err != nil {
					t.Fatal(err)
				}

				return tmpDir, fmt.Sprintf("{file://%s}", path.Join(tmpDir, "test_file.txt")), func() {
					os.RemoveAll(tmpDir)
				}
			},
			expected: "hello test",
		},
		{
			name: "parsing http",
			setup: func() (string, string, func()) {
				// Get a random port between 1024 and 65535
				port := rand.Intn(65535-1024) + 1024
				server := &http.Server{Addr: fmt.Sprintf(":%d", port)}

				wg := &sync.WaitGroup{}

				wg.Add(1)

				go func(t *testing.T) {
					wg.Done()

					if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
						t.Fatal(err)
					}
				}(t)

				wg.Wait()

				http.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
					fmt.Fprint(w, "hello test")
				})

				return "", fmt.Sprintf("{http://localhost:%d}", port), func() {
					if err := server.Close(); err != nil {
						t.Fatal(err)
					}
				}
			},
			expected: "hello test",
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			baseDir, value, teardownFn := tc.setup()

			defer teardownFn()

			cfgParser := parser.NewConfigParser(baseDir)

			res, err := cfgParser.ParseDynamicValue(value)

			assert.NoError(t, err)
			assert.Equal(t, tc.expected, res)
		})
	}
}
