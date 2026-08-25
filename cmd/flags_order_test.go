// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package cmd_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Each command must bind its flags to viper before it merges the flags of the configuration file.
// The merger writes a value only when viper does not have the key yet (`!viper.IsSet`). A merge
// that runs first thus gives the configuration file precedence over the command line. The
// documented priority is the opposite: configuration file < environment variable < command line.
func TestCommandsBindFlagsBeforeTheConfigMerge(t *testing.T) {
	t.Parallel()

	const (
		bindCall  = "viper.BindPFlags(cmd.Flags())"
		mergeCall = "flags.LoadAndMergeCommandFlags("
	)

	checked := 0

	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}

		b, readErr := os.ReadFile(path)
		require.NoError(t, readErr)

		src := string(b)
		bind := strings.Index(src, bindCall)
		merge := strings.Index(src, mergeCall)

		if bind < 0 || merge < 0 {
			return nil
		}

		checked++

		assert.Less(t, bind, merge,
			"%s merges the flags of the configuration file before it binds the flags of the command line",
			path)

		return nil
	})
	require.NoError(t, err)

	assert.Positive(t, checked, "no command calls both BindPFlags and LoadAndMergeCommandFlags")
}
