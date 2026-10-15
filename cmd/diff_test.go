// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package cmd_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/cmd"
)

func readGolden(t *testing.T, name string) string {
	t.Helper()

	b, err := os.ReadFile(filepath.Join("testdata", "diff", name))
	require.NoError(t, err)

	return string(b)
}

func exitCode(run diffRun) int {
	if run.Err != nil {
		return 1
	}

	return 0
}

// diff-command-01: --config-only is the diff command of the base commit c9527433, byte for byte.
func TestDiffConfigOnly(t *testing.T) {
	tests := []struct {
		name        string
		extraConfig string
		args        []string
	}{
		{"the --config-only flag", "", []string{"--config-only"}},
		{"configOnly: true under flags.diff", "flags:\n  diff:\n    configOnly: true\n", nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			run := runDiff(t, tc.extraConfig, tc.args...)

			require.Equal(t, readGolden(t, "config-only.golden"), run.Stdout+"--- exit code: "+strconv.Itoa(exitCode(run))+"\n")
			require.Equal(t, readGolden(t, "config-only.log.golden"), run.Logs)
		})
	}
}

// diff-command-05.
func TestDiffHelpText(t *testing.T) {
	for _, text := range []string{
		"--config-only",
		"runs the preflight",
		"renders into the same run dir as apply",
		"must not run at the same time as apply",
	} {
		t.Run(text, func(t *testing.T) {
			viper.Reset()

			c := cmd.NewDiffCmd()
			out := &bytes.Buffer{}

			c.SetOut(out)
			c.SetArgs([]string{"--help"})

			require.NoError(t, c.Execute())
			require.Contains(t, out.String(), text)
		})
	}
}
