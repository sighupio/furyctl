// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package flags_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/flags"
)

// The merger must know each section of the `flags` field. The section must also have flags that
// furyctl supports. furyctl parses a section with no map, or with an empty map, and then drops it.
// The `tools` section had this fault.
func TestEverySectionHasSupportedFlags(t *testing.T) {
	t.Parallel()

	merger := flags.NewMerger()
	cfg := reflect.TypeOf(flags.FlagsConfig{})

	for i := range cfg.NumField() {
		section := strings.Split(cfg.Field(i).Tag.Get("yaml"), ",")[0]

		t.Run(section, func(t *testing.T) {
			t.Parallel()

			supported := merger.GetSupportedFlagsForCommand(section)

			require.NotNil(t, supported, "the merger does not know the %q section", section)
			assert.NotEmpty(t, supported, "the %q section has no supported flags", section)
		})
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

			cfg := sectionConfig(tc.section, map[string]any{tc.flag: "/from-config"})
			require.NoError(t, flags.NewMerger().MergeIntoViper(cfg, tc.section))

			assert.Equal(t, "/from-config", viper.GetString(tc.key))
		})
	}
}

// sectionConfig puts the given flags in the named section of a FlagsConfig.
func sectionConfig(section string, values map[string]any) *flags.FlagsConfig {
	cfg := &flags.FlagsConfig{}

	switch section {
	case flags.CommandValidate:
		cfg.Validate = values

	case flags.CommandDownload:
		cfg.Download = values

	case flags.CommandConnect:
		cfg.Connect = values

	case flags.CommandRenew:
		cfg.Renew = values

	case flags.CommandDump:
		cfg.Dump = values
	}

	return cfg
}
