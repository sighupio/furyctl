// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package flags_test

import (
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/cmd"
	"github.com/sighupio/furyctl/internal/flags"
)

// commandsPerSection maps a section of the `flags` field to the commands that read it. A command
// calls LoadAndMergeCommandFlags with the name of its section.
//
// `create config` is absent on purpose: it stops when the configuration file is already present,
// thus it can never read a flag from that file. Only `create pki` reads one.
var commandsPerSection = map[string][]string{
	flags.CommandGlobal:   {"furyctl"},
	flags.CommandApply:    {"furyctl apply"},
	flags.CommandDelete:   {"furyctl delete cluster"},
	flags.CommandCreate:   {"furyctl create pki"},
	flags.CommandGet:      {"furyctl get kubeconfig", "furyctl get cluster-info", "furyctl get upgrade-paths", "furyctl get supported-versions"},
	flags.CommandDiff:     {"furyctl diff"},
	flags.CommandValidate: {"furyctl validate config", "furyctl validate dependencies"},
	flags.CommandDownload: {"furyctl download dependencies", "furyctl download air-gapped-bundle"},
	flags.CommandConnect:  {"furyctl connect openvpn"},
	flags.CommandRenew:    {"furyctl renew certificates", "furyctl renew kubeconfigs"},
	flags.CommandDump:     {"furyctl dump template", "furyctl dump cli-reference"},
}

// flagsNotInTheTable names the flags that the table does not list on purpose.
var flagsNotInTheTable = []string{
	// furyctl reads `config` to find the configuration file. A value in that file would load
	// another file.
	"config",
	// cobra adds `help` to each command.
	"help",
	// `https` is deprecated in favor of `git-protocol`.
	"https",
}

var flagName = regexp.MustCompile(`--([a-z][a-z0-9-]*)`)

// TestTableMatchesTheCommands keeps the supported flags and the commands together. It fails when a
// command gains a flag that the table does not list, and when the table lists a flag that no
// command has.
func TestTableMatchesTheCommands(t *testing.T) {
	t.Parallel()

	tree := map[string]*cobra.Command{}
	root := cmd.NewRootCmd()
	collect(root.Command, tree)

	supported := flags.GetSupportedFlags()

	require.NotEmpty(t, supported, "furyctl supports no section at all")

	// A command can declare a flag that the global section already holds, for example `workdir`.
	// The global section makes it settable, thus the section of that command needs no entry.
	globalFlags := map[string]bool{}

	for name := range supported[flags.CommandGlobal] {
		globalFlags[flags.CamelToKebab(name)] = true
	}

	for section, sectionFlags := range supported {
		commands, mapped := commandsPerSection[section]
		require.True(t, mapped, "the %q section has no command in commandsPerSection", section)

		real := map[string]bool{}

		for _, name := range commands {
			command, found := tree[name]
			require.True(t, found, "the command %q of the %q section is not in the tree", name, section)

			usage := command.Flags().FlagUsages()
			if section == flags.CommandGlobal {
				usage = root.PersistentFlags().FlagUsages()
			}

			for _, m := range flagName.FindAllStringSubmatch(usage, -1) {
				real[m[1]] = true
			}
		}

		declared := map[string]bool{}

		for name := range sectionFlags {
			kebab := flags.CamelToKebab(name)
			declared[kebab] = true

			assert.True(t, real[kebab],
				"the %q section lists %q, but no command of that section has the flag --%s",
				section, name, kebab)
		}

		for name := range real {
			if slices.Contains(flagsNotInTheTable, name) {
				continue
			}

			if section != flags.CommandGlobal && globalFlags[name] {
				continue
			}

			assert.True(t, declared[name],
				"a command of the %q section has the flag --%s, but the section does not list it",
				section, name)
		}
	}
}

func collect(command *cobra.Command, out map[string]*cobra.Command) {
	out[fullName(command)] = command

	for _, sub := range command.Commands() {
		collect(sub, out)
	}
}

func fullName(command *cobra.Command) string {
	var parts []string

	for c := command; c != nil; c = c.Parent() {
		parts = append([]string{strings.Fields(c.Use)[0]}, parts...)
	}

	return strings.Join(parts, " ")
}
