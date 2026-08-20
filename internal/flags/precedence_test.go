// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package flags_test

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/flags"
)

// The documented priority is: configuration file < environment variable < command line flag.
// The merger keeps this priority with the `!viper.IsSet(key)` guard. Viper must thus know the
// flags before the merge runs. A command that merges first gives the configuration file the
// higher priority.
//
// The case "flags do not override existing viper values" of TestMerger_MergeIntoViper covers the
// explicit values of viper.Set. This test covers a flag of the command line, which is the source
// that the guard must protect.
func TestMergeIntoViper_CommandLineFlagWins(t *testing.T) {
	cmd := &cobra.Command{Use: "apply"}
	cmd.Flags().String("distro-location", "", "Distribution location")
	require.NoError(t, cmd.ParseFlags([]string{"--distro-location=/from-cli"}))

	viper.Reset()
	t.Cleanup(viper.Reset)

	require.NoError(t, viper.BindPFlags(cmd.Flags()))

	cfg := &flags.FlagsConfig{Apply: map[string]any{"distroLocation": "/from-config"}}
	require.NoError(t, flags.NewMerger().MergeIntoViper(cfg, flags.CommandApply))

	assert.Equal(t, "/from-cli", viper.GetString("distro-location"))
}
