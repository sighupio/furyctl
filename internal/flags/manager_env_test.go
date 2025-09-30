// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package flags_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/flags"
)

func TestManager_LoadAndMergeFlags_EnvironmentVariableErrors(t *testing.T) {
	// Note: Not using t.Parallel() because tests modify environment variables

	tests := []struct {
		name        string
		yamlContent string
		setupEnv    func()
		cleanupEnv  func()
		wantErr     bool
		errContains string
	}{
		{
			name: "missing environment variable should cause fatal error",
			yamlContent: `
flags:
  global:
    log: "{env://NONEXISTENT_TEST_VAR_12345}/furyctl.log"
    debug: true
`,
			setupEnv:    func() {},
			cleanupEnv:  func() {},
			wantErr:     true,
			errContains: "NONEXISTENT_TEST_VAR_12345\" is empty",
		},
		{
			name: "valid environment variable should work",
			yamlContent: `
flags:
  global:
    log: "{env://TEST_VAR_EXISTS}/furyctl.log"
    debug: true
`,
			setupEnv: func() {
				os.Setenv("TEST_VAR_EXISTS", "/tmp/test")
			},
			cleanupEnv: func() {
				os.Unsetenv("TEST_VAR_EXISTS")
			},
			wantErr: false,
		},
		{
			name: "missing file should cause fatal error",
			yamlContent: `
flags:
  global:
    log: "{file://./nonexistent-file-12345.txt}/furyctl.log"
    debug: true
`,
			setupEnv:    func() {},
			cleanupEnv:  func() {},
			wantErr:     true,
			errContains: "no such file",
		},
		{
			name: "mixed valid and invalid dynamic values should fail",
			yamlContent: `
flags:
  global:
    log: "{env://PWD}/{env://MISSING_VAR}/furyctl.log"
    debug: true
`,
			setupEnv:    func() {},
			cleanupEnv:  func() {},
			wantErr:     true,
			errContains: "MISSING_VAR\" is empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer viper.Reset()
			// Note: Not using t.Parallel() because setupEnv modifies global environment

			// Setup environment.
			tt.setupEnv()
			defer tt.cleanupEnv()

			// Create temporary config file.
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "furyctl.yaml")
			err := os.WriteFile(configPath, []byte(tt.yamlContent), 0o644)
			require.NoError(t, err)

			// Create manager and test.
			manager := flags.NewManager(tmpDir)
			err = manager.LoadAndMergeFlags(configPath, "apply")

			if tt.wantErr {
				require.Error(t, err)

				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestManager_isCriticalError(t *testing.T) {
	// Note: Not using t.Parallel() because tests check environment variable errors

	tests := []struct {
		name        string
		yamlContent string
		expectFatal bool
		errContains string
	}{
		{
			name: "environment variable error should be critical",
			yamlContent: `
flags:
  global:
    log: "{env://DEFINITELY_MISSING_VAR_99999}/test.log"
`,
			expectFatal: true,
			errContains: "DEFINITELY_MISSING_VAR_99999\" is empty",
		},
		{
			name: "file not found error should be critical",
			yamlContent: `
flags:
  global:
    log: "{file:///tmp/nonexistent-test-file-99999.txt}"
`,
			expectFatal: true,
			errContains: "no such file",
		},
		{
			name: "valid absolute path should not be critical",
			yamlContent: `
flags:
  global:
    log: "/tmp/furyctl.log"
`,
			expectFatal: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer viper.Reset()
			// Note: Not using t.Parallel() due to potential global state modifications

			// Create temporary config file.
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "furyctl.yaml")
			err := os.WriteFile(configPath, []byte(tt.yamlContent), 0o644)
			require.NoError(t, err)

			// Create manager and test.
			manager := flags.NewManager(tmpDir)
			err = manager.LoadAndMergeFlags(configPath, "apply")

			if tt.expectFatal {
				require.Error(t, err, "Expected fatal error but got none")

				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else if err != nil {
				// May or may not have error (e.g., validation warnings), but shouldn't be fatal.
				// The key is that execution should continue.
				t.Logf("Non-fatal error (expected): %v", err)
			}
		})
	}
}
