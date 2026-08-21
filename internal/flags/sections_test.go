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

// Each section that furyctl supports must list its flags. A section with an empty map reaches the
// user as a section that furyctl parses and then drops. The `tools` section had this fault.
func TestEverySectionHasSupportedFlags(t *testing.T) {
	t.Parallel()

	supported := flags.GetSupportedFlags()

	require.NotEmpty(t, supported, "furyctl supports no section at all")

	for section, sectionFlags := range supported {
		assert.NotEmpty(t, sectionFlags, "the %q section lists no flag", section)
	}
}

// A flag of a command section reaches viper. Before this change the merger dropped the sections
// validate, download, connect, renew and dump. The validation of the configuration file also
// stopped with "unsupported flags command".
func TestMergeIntoViper_CommandSections(t *testing.T) {
	tests := []struct {
		section string
		flag    string
		key     string
	}{
		{flags.CommandValidate, "binPath", "bin-path"},
		{flags.CommandDownload, "bundleOutput", "bundle-output"},
		{flags.CommandConnect, "profile", "profile"},
		{flags.CommandRenew, "distroLocation", "distro-location"},
		{flags.CommandDump, "distroPatches", "distro-patches"},
	}

	for _, tc := range tests {
		t.Run(tc.section, func(t *testing.T) {
			cmd := &cobra.Command{Use: tc.section}
			cmd.Flags().String(tc.key, "", "")
			require.NoError(t, cmd.ParseFlags([]string{}))

			viper.Reset()
			t.Cleanup(viper.Reset)

			require.NoError(t, viper.BindPFlags(cmd.Flags()))

			cfg := flags.FlagsConfig{tc.section: {tc.flag: "/from-config"}}
			require.NoError(t, flags.NewMerger().MergeIntoViper(cfg, tc.section))

			assert.Equal(t, "/from-config", viper.GetString(tc.key))
		})
	}
}

// furyctl merges the global flags for a command that it does not support, and nothing else. The
// merge relies on the absent entry in the supported flags, thus this test keeps that behavior.
func TestMergeIntoViper_UnsupportedCommandGetsGlobalOnly(t *testing.T) {
	cmd := &cobra.Command{Use: "serve"}
	cmd.Flags().String("outdir", "", "")
	cmd.Flags().String("address", "", "")
	require.NoError(t, cmd.ParseFlags([]string{}))

	viper.Reset()
	t.Cleanup(viper.Reset)

	require.NoError(t, viper.BindPFlags(cmd.Flags()))

	cfg := flags.FlagsConfig{
		flags.CommandGlobal: {"outdir": "/from-global"},
		"serve":             {"address": "1.2.3.4"},
	}
	require.NoError(t, flags.NewMerger().MergeIntoViper(cfg, "serve"))

	assert.Equal(t, "/from-global", viper.GetString("outdir"))
	assert.Empty(t, viper.GetString("address"), "furyctl must not merge the flags of a command that it does not support")
}

// A configuration without a `flags` field gives no error and no validation message.
func TestNilConfigIsHarmless(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	require.NoError(t, flags.NewMerger().MergeIntoViper(nil, flags.CommandApply))
	assert.Empty(t, flags.NewValidator().Validate(nil))
}
