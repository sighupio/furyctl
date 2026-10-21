// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package cmd

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/diffview"
)

const (
	diffRulesFolder     = "../pkg/diffs/testdata/rules"
	diffSimulation      = "../internal/diffview/testdata/simulation"
	diffSimulationTable = "../internal/diffview/testdata/simulation.golden"
	diffNoRulesLine     = "RULE   not available: the distribution has no rules file for this kind"
)

var errDiffStore = errors.New("the cluster does not answer")

// diffStore is a state store that serves the stored configuration from memory.
type diffStore struct {
	config []byte
	err    error
}

func (*diffStore) StoreKFD() error                  { return nil }
func (*diffStore) StoreConfig(map[string]any) error { return nil }
func (s *diffStore) GetConfig() ([]byte, error)     { return s.config, s.err }

func (*diffStore) GetRenderedConfig() ([]byte, error) {
	return nil, errDiffStore
}

// diffCase is one run of the command after the download of the distribution.
type diffCase struct {
	stored string
	local  string
	// The rules field is the content of the rule file of the kind and ruleFile names
	// it. Without a name the distribution folder has no rule file.
	ruleFile  string
	rules     string
	storeErr  error
	phasePath string
}

// diffResult is what the command leaves: the two streams and its error.
type diffResult struct {
	stdout string
	stderr string
	err    error
}

func realRules(t *testing.T, kind string) (string, string) {
	t.Helper()

	name := strings.ToLower(kind) + "-kfd-v1alpha2.yaml"

	content, err := os.ReadFile(filepath.Join(diffRulesFolder, name))
	require.NoError(t, err)

	return name, string(content)
}

// runDiffCase runs the command with a fake store, a local file and a distribution
// folder built for the case. Standard output is not a terminal: no colour, no width.
// Standard error is what the logger writes.
func runDiffCase(t *testing.T, tC diffCase) diffResult {
	t.Helper()

	dir := t.TempDir()
	furyctlPath := filepath.Join(dir, "furyctl.yaml")
	distroPath := filepath.Join(dir, "distribution")

	require.NoError(t, os.WriteFile(furyctlPath, []byte(tC.local), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(distroPath, "rules"), 0o750))

	if tC.ruleFile != "" {
		require.NoError(t, os.WriteFile(filepath.Join(distroPath, "rules", tC.ruleFile), []byte(tC.rules), 0o600))
	}

	var stdout, stderr bytes.Buffer

	previous := logrus.StandardLogger().Out

	logrus.SetOutput(&stderr)
	t.Cleanup(func() { logrus.SetOutput(previous) })

	err := runDiff(diffRun{
		Out:         &stdout,
		Store:       &diffStore{config: []byte(tC.stored), err: tC.storeErr},
		FuryctlPath: furyctlPath,
		DistroPath:  distroPath,
		PhasePath:   tC.phasePath,
		Table:       diffview.Options{Colour: false, Width: 0},
	})

	return diffResult{stdout: stdout.String(), stderr: stderr.String(), err: err}
}

func immutableConfig(spec string) string {
	return "apiVersion: kfd.sighup.io/v1alpha2\nkind: Immutable\nmetadata:\n  name: example\nspec: " + spec + "\n"
}

// The tests of the command are not parallel: they capture the global logger.
//
//nolint:paralleltest // the logger is global.
func TestRunDiff(t *testing.T) {
	immutableRuleFile, immutableRules := realRules(t, "Immutable")

	t.Run("the command prints the table of the simulation", func(t *testing.T) {
		stored, err := os.ReadFile(filepath.Join(diffSimulation, "cur.yaml"))
		require.NoError(t, err)

		local, err := os.ReadFile(filepath.Join(diffSimulation, "new2.yaml"))
		require.NoError(t, err)

		golden, err := os.ReadFile(diffSimulationTable)
		require.NoError(t, err)

		require.Contains(t, string(stored), `passphrase: "old-example"`)
		require.Contains(t, string(local), `passphrase: "new-example"`)

		got := runDiffCase(t, diffCase{
			stored: string(stored), local: string(local), ruleFile: immutableRuleFile, rules: immutableRules,
		})

		require.NoError(t, got.err)
		assert.Equal(t, string(golden), got.stdout)
		assert.NotContains(t, got.stdout, "\x1b")

		for _, secret := range []string{"old-example", "new-example"} {
			assert.NotContains(t, got.stdout, secret)
			assert.NotContains(t, got.stderr, secret)
		}
	})

	t.Run("the phase flag limits the rows", func(t *testing.T) {
		tC := diffCase{
			stored: immutableConfig(`{distributionVersion: v1.35.1, kubernetes: {networking: {podCIDR: 172.16.128.0/17}},
			  distribution: {modules: {ingress: {baseDomain: one.example.test}}}}`),
			local: immutableConfig(`{distributionVersion: v1.36.0, kubernetes: {networking: {podCIDR: 10.42.0.0/16}},
			  distribution: {modules: {ingress: {baseDomain: two.example.test}}}}`),
			ruleFile: immutableRuleFile,
			rules:    immutableRules,
		}

		// The distribution phase also lists the version of the distribution, which an
		// upgrade changes with it.
		tC.phasePath = ".spec.distribution"
		got := runDiffCase(t, tC)

		require.NoError(t, got.err)
		assert.Equal(
			t,
			"CONFIG   2 changes\n"+
				"\n"+
				"   FIELD                       CURRENT           NEW               RULE\n"+
				"───────────────────────────────────────────────────────────────────────\n"+
				"General\n"+
				"~  spec.distributionVersion    v1.35.1           v1.36.0\n"+
				"Distribution\n"+
				"~  modules.ingress.baseDomain  one.example.test  two.example.test\n",
			got.stdout,
		)

		// The blocked change of the kubernetes phase is counted only when it is shown.
		tC.phasePath = ".spec.kubernetes"
		got = runDiffCase(t, tC)

		require.NoError(t, got.err)
		assert.True(t, strings.HasPrefix(got.stdout, "CONFIG   1 change · 1 blocked\n"), got.stdout)
		assert.Contains(t, got.stdout, "\nKubernetes\n~  networking.podCIDR  ")
		assert.NotContains(t, got.stdout, "Distribution")
		assert.NotContains(t, got.stdout, "General")

		tC.phasePath = ""
		got = runDiffCase(t, tC)

		require.NoError(t, got.err)
		assert.True(t, strings.HasPrefix(got.stdout, "CONFIG   3 changes · 1 blocked\n"), got.stdout)
		assert.Contains(t, got.stdout, "\nGeneral\n~  spec.distributionVersion  ")
	})

	t.Run("no changes", func(t *testing.T) {
		config := immutableConfig(`{distribution: {modules: {logging: {type: none}}}}`)

		got := runDiffCase(t, diffCase{stored: config, local: config, ruleFile: immutableRuleFile, rules: immutableRules})

		require.NoError(t, got.err)
		assert.Equal(t, "CONFIG   no changes\n", got.stdout)
	})

	t.Run("no changes under the phase", func(t *testing.T) {
		got := runDiffCase(t, diffCase{
			stored:    immutableConfig(`{distribution: {modules: {logging: {type: none}}}}`),
			local:     immutableConfig(`{distribution: {modules: {logging: {type: loki}}}}`),
			ruleFile:  immutableRuleFile,
			rules:     immutableRules,
			phasePath: ".spec.kubernetes",
		})

		require.NoError(t, got.err)
		assert.Equal(t, "CONFIG   no changes\n", got.stdout)
	})

	t.Run("the stored config cannot be read", func(t *testing.T) {
		got := runDiffCase(t, diffCase{
			local:    immutableConfig(`{distribution: {modules: {logging: {type: loki}}}}`),
			storeErr: errDiffStore,
			ruleFile: immutableRuleFile,
			rules:    immutableRules,
		})

		require.ErrorIs(t, got.err, errDiffStore)
		require.EqualError(
			t,
			got.err,
			"error while creating configuration diff checker: error while getting current cluster config: "+
				"the cluster does not answer",
		)
		assert.Empty(t, got.stdout)
	})

	t.Run("no rules file", func(t *testing.T) {
		onPremises := func(podCidr string) string {
			return "apiVersion: kfd.sighup.io/v1alpha2\nkind: OnPremises\nspec: {kubernetes: {podCidr: " + podCidr + "}}\n"
		}

		got := runDiffCase(t, diffCase{stored: onPremises("172.16.128.0/17"), local: onPremises("10.42.0.0/16")})

		require.NoError(t, got.err)
		assert.Equal(
			t,
			"CONFIG   1 change\n"+
				"\n"+
				"   FIELD    CURRENT          NEW           RULE\n"+
				"───────────────────────────────────────────────\n"+
				"Kubernetes\n"+
				"~  podCidr  172.16.128.0/17  10.42.0.0/16\n"+
				"\n"+
				diffNoRulesLine+"\n",
			got.stdout,
		)

		// With its rule file the same change is blocked.
		ruleFile, rules := realRules(t, "OnPremises")

		got = runDiffCase(t, diffCase{
			stored: onPremises("172.16.128.0/17"), local: onPremises("10.42.0.0/16"), ruleFile: ruleFile, rules: rules,
		})

		require.NoError(t, got.err)
		assert.True(t, strings.HasPrefix(got.stdout, "CONFIG   1 change · 1 blocked\n"), got.stdout)
		assert.NotContains(t, got.stdout, diffNoRulesLine)
	})

	t.Run("a rules file that cannot be read gives the table without verdicts, as apply skips its checks", func(t *testing.T) {
		tC := diffCase{
			stored:   immutableConfig(`{kubernetes: {networking: {podCIDR: 172.16.128.0/17}}}`),
			local:    immutableConfig(`{kubernetes: {networking: {podCIDR: 10.42.0.0/16}}}`),
			ruleFile: immutableRuleFile,
			rules:    "kubernetes: [not: closed\n",
		}

		got := runDiffCase(t, tC)

		require.NoError(t, got.err)
		assert.Equal(
			t,
			"CONFIG   1 change\n"+
				"\n"+
				"   FIELD               CURRENT          NEW           RULE\n"+
				"──────────────────────────────────────────────────────────\n"+
				"Kubernetes\n"+
				"~  networking.podCIDR  172.16.128.0/17  10.42.0.0/16\n"+
				"\n"+
				diffUnreadableRulesLine+"\n",
			got.stdout,
		)
		assert.NotContains(t, got.stdout, diffNoRulesLine)
		assert.Contains(t, got.stderr, "level=warning")
		assert.Contains(t, got.stderr, "cannot be read")
		assert.Contains(t, got.stderr, immutableRuleFile)

		// With no change there is no RULE column to explain.
		tC.local = tC.stored
		got = runDiffCase(t, tC)

		require.NoError(t, got.err)
		assert.Equal(t, "CONFIG   no changes\n", got.stdout)
	})

	t.Run("a field the conditions read and the stored config lacks is logged at debug level only", func(t *testing.T) {
		// The safe condition of the immutable rule of the tsdb start date reads the
		// logging type, which neither configuration sets.
		tC := diffCase{
			stored:   immutableConfig(`{distribution: {modules: {logging: {loki: {tsdbStartDate: "2024-01-01"}}}}}`),
			local:    immutableConfig(`{distribution: {modules: {logging: {loki: {tsdbStartDate: "2025-01-01"}}}}}`),
			ruleFile: immutableRuleFile,
			rules:    immutableRules,
		}

		got := runDiffCase(t, tC)

		require.NoError(t, got.err)
		assert.True(t, strings.HasPrefix(got.stdout, "CONFIG   1 change · 1 blocked\n"), got.stdout)
		assert.NotContains(t, got.stderr, "level=error")
		assert.NotContains(t, got.stderr, "error getting value for")

		previous := logrus.GetLevel()

		logrus.SetLevel(logrus.DebugLevel)
		t.Cleanup(func() { logrus.SetLevel(previous) })

		got = runDiffCase(t, tC)

		require.NoError(t, got.err)
		assert.NotContains(t, got.stderr, "level=error")
		assert.Contains(t, got.stderr, "level=debug")
		assert.Contains(t, got.stderr, "error getting value for .spec.distribution.modules.logging.type")
	})

	t.Run("the verdicts under the phase flag are decided on every change", func(t *testing.T) {
		// The kubernetes rule is safe when a distribution field changes in the same
		// apply: with the rows limited to kubernetes, the distribution change is not
		// shown but still makes the kubernetes change safe.
		rules := `---
kubernetes:
  - path: .spec.kubernetes.advanced.scheduler.profile
    immutable: true
    description: "the scheduler profile cannot be changed"
    safe:
      - fromNodes:
          - path: ".spec.distribution.modules.tracing.enabled"
            to: "on"
distribution: []
`
		tC := diffCase{
			stored: immutableConfig(`{kubernetes: {advanced: {scheduler: {profile: a}}},
			  distribution: {modules: {tracing: {enabled: "off"}}}}`),
			local: immutableConfig(`{kubernetes: {advanced: {scheduler: {profile: b}}},
			  distribution: {modules: {tracing: {enabled: "on"}}}}`),
			ruleFile: immutableRuleFile,
			rules:    rules,
		}

		got := runDiffCase(t, tC)

		require.NoError(t, got.err)
		require.True(t, strings.HasPrefix(got.stdout, "CONFIG   2 changes\n"), got.stdout)

		tC.phasePath = ".spec.kubernetes"
		got = runDiffCase(t, tC)

		require.NoError(t, got.err)
		assert.True(t, strings.HasPrefix(got.stdout, "CONFIG   1 change\n"), got.stdout)
		assert.Contains(t, got.stdout, "~  advanced.scheduler.profile  a        b\n")
		assert.NotContains(t, got.stdout, "blocked")

		// Without the distribution change the same row is blocked.
		tC.local = immutableConfig(`{kubernetes: {advanced: {scheduler: {profile: b}}},
		  distribution: {modules: {tracing: {enabled: "off"}}}}`)
		got = runDiffCase(t, tC)

		require.NoError(t, got.err)
		assert.True(t, strings.HasPrefix(got.stdout, "CONFIG   1 change · 1 blocked\n"), got.stdout)
	})

	t.Run("an explicit value absent from the stored config needs confirmation", func(t *testing.T) {
		got := runDiffCase(t, diffCase{
			stored:   immutableConfig(`{distribution: {modules: {logging: {minio: {storageSize: 20Gi}}}}}`),
			local:    immutableConfig(`{distribution: {modules: {logging: {minio: {storageSize: 20Gi}, type: loki}}}}`),
			ruleFile: immutableRuleFile,
			rules:    immutableRules,
		})

		require.NoError(t, got.err)
		assert.Equal(
			t,
			"CONFIG   1 change · 1 needs confirmation\n"+
				"\n"+
				"   FIELD                 CURRENT  NEW   RULE\n"+
				"────────────────────────────────────────────\n"+
				"Distribution\n"+
				"+  modules.logging.type  –        loki  ! needs confirmation: changes to the logging module type have "+
				"been detected. This will cause the reconfiguration or deletion of the current logging stack.\n",
			got.stdout,
		)

		// The same value in the stored config as the string none is the safe transition.
		got = runDiffCase(t, diffCase{
			stored:   immutableConfig(`{distribution: {modules: {logging: {type: none}}}}`),
			local:    immutableConfig(`{distribution: {modules: {logging: {type: loki}}}}`),
			ruleFile: immutableRuleFile,
			rules:    immutableRules,
		})

		require.NoError(t, got.err)
		assert.Contains(t, got.stdout, "~  modules.logging.type  none     loki  · migration: ")
	})

	t.Run("a kind without a rule profile is an error", func(t *testing.T) {
		got := runDiffCase(t, diffCase{stored: "kind: Unknown\nspec: {a: 1}\n", local: "kind: Unknown\nspec: {a: 2}\n"})

		require.ErrorIs(t, got.err, errDiffKind)
		assert.Empty(t, got.stdout)

		got = runDiffCase(t, diffCase{stored: "spec: {a: 1}\n", local: "spec: {a: 2}\n"})

		require.ErrorIs(t, got.err, errDiffKind)
		assert.Empty(t, got.stdout)
	})
}

func TestDiffTableOptions(t *testing.T) {
	t.Parallel()

	t.Run("an output that is not a terminal gets no colour and no width", func(t *testing.T) {
		t.Parallel()

		file, err := os.Create(filepath.Join(t.TempDir(), "stdout"))
		require.NoError(t, err)

		t.Cleanup(func() { _ = file.Close() })

		assert.Equal(t, diffview.Options{Colour: false, Width: 0}, diffTableOptions(file))
	})

	testCases := []struct {
		name   string
		output tableOutput
		want   diffview.Options
	}{
		{
			name:   "a terminal gets colour and its width",
			output: tableOutput{Terminal: true, Width: 80},
			want:   diffview.Options{Colour: true, Width: 80},
		},
		{
			name:   "a terminal of unknown width gets colour and nothing is cut",
			output: tableOutput{Terminal: true, Width: 0},
			want:   diffview.Options{Colour: true, Width: 0},
		},
		{
			name:   "no-tty on a terminal gets no colour and nothing is cut, as a pipe",
			output: tableOutput{Terminal: true, Width: 80, NoTTY: true, Debug: true},
			want:   diffview.Options{Colour: false, Width: 0},
		},
		{
			name:   "NO_COLOR on a terminal gets no colour and keeps the width",
			output: tableOutput{Terminal: true, Width: 100, NoColour: true},
			want:   diffview.Options{Colour: false, Width: 100},
		},
		{
			name:   "debug logs on a terminal get no colour and keep the width",
			output: tableOutput{Terminal: true, Width: 120, Debug: true},
			want:   diffview.Options{Colour: false, Width: 120},
		},
		{
			name:   "a pipe gets no colour and nothing is cut",
			output: tableOutput{Terminal: false, Width: 0},
			want:   diffview.Options{Colour: false, Width: 0},
		},
		{
			name:   "a pipe with NO_COLOR and no-tty gets no colour and nothing is cut",
			output: tableOutput{Terminal: false, NoTTY: true, Debug: true, NoColour: true},
			want:   diffview.Options{Colour: false, Width: 0},
		},
	}

	for _, tC := range testCases {
		t.Run(tC.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tC.want, tableOptionsOf(tC.output))
		})
	}
}

func TestDiffCmd_Help(t *testing.T) {
	t.Parallel()

	t.Run("the help of the command describes the table", func(t *testing.T) {
		t.Parallel()

		diffCmd := NewDiffCmd()

		assert.Contains(t, diffCmd.Short, "CONFIG table")

		for _, text := range []string{"CONFIG table", "FIELD", "CURRENT", "NEW", "RULE", "as written", "(hidden)"} {
			assert.Contains(t, diffCmd.Long, text)
		}
	})

	t.Run("the command accepts exactly the flags it accepted before", func(t *testing.T) {
		t.Parallel()

		want := []string{
			"config", "phase", "distro-location", "distro-patches", "bin-path", "upgrade-path-location",
			"airgap-bundle", "force-extract",
		}

		flagSet := NewDiffCmd().Flags()

		for _, name := range want {
			assert.NotNil(t, flagSet.Lookup(name), name)
		}

		// One usage line per flag: the command has no flag the list does not name.
		usages := strings.Count(flagSet.FlagUsagesWrapped(100000), "\n")
		assert.Equal(t, len(want), usages, flagSet.FlagUsages())
	})
}

func kindConfig(kind, spec string) string {
	return "apiVersion: kfd.sighup.io/v1alpha2\nkind: " + kind + "\nmetadata:\n  name: example\nspec: " + spec + "\n"
}

// The tests of the command are not parallel: they capture the global logger.
//
//nolint:paralleltest // the logger is global.
func TestRunDiff_EKSClusterAndKFDDistribution(t *testing.T) {
	const (
		stored = `{distributionVersion: v1.35.1, distribution: {modules: {policy: {type: kyverno}, logging: {type: none}}},
		  plugins: {helm: {releases: [{name: app, namespace: apps, chart: repo/app, version: 1.0.0}]}}}`
		local = `{distributionVersion: v1.36.0, distribution: {modules: {policy: {type: gatekeeper}, logging: {type: loki}}},
		  plugins: {helm: {releases: [{name: app, namespace: apps, chart: repo/app, version: 1.1.0}]}}}`

		policyReason = "currently, switching from kyverno to gatekeeper is not supported. " +
			"You need to first remove the current stack with type: none."
		loggingText = "changes to the logging module type have been detected. " +
			"This will cause the reconfiguration or deletion of the current logging stack."
	)

	for _, kind := range []string{"EKSCluster", "KFDDistribution"} {
		ruleFile, rules := realRules(t, kind)

		tC := diffCase{stored: kindConfig(kind, stored), local: kindConfig(kind, local), ruleFile: ruleFile, rules: rules}

		t.Run(kind+" with its rules file", func(t *testing.T) {
			got := runDiffCase(t, tC)

			require.NoError(t, got.err)
			assert.True(t, strings.HasPrefix(got.stdout, "CONFIG   4 changes · 1 blocked\n"), got.stdout)
			assert.Contains(t, got.stdout, "\nGeneral\n~  spec.distributionVersion  ")
			assert.Contains(t, got.stdout, "  ✗ blocked: "+policyReason+"\n")
			assert.Contains(t, got.stdout, "  · migration: "+loggingText+"\n")
			assert.Contains(t, got.stdout, "\nPlugins\n")
			assert.NotContains(t, got.stdout, diffNoRulesLine)
			assert.NotContains(t, got.stderr, "level=error")
		})

		t.Run(kind+" with the distribution phase", func(t *testing.T) {
			phased := tC
			phased.phasePath = ".spec.distribution"

			got := runDiffCase(t, phased)

			require.NoError(t, got.err)
			assert.True(t, strings.HasPrefix(got.stdout, "CONFIG   3 changes · 1 blocked\n"), got.stdout)
			assert.Contains(t, got.stdout, "\nGeneral\n~  spec.distributionVersion  ")
			assert.Contains(t, got.stdout, "\nDistribution\n")
			assert.NotContains(t, got.stdout, "Plugins")
		})

		t.Run(kind+" with the plugins phase", func(t *testing.T) {
			phased := tC
			phased.phasePath = ".spec.plugins"

			got := runDiffCase(t, phased)

			require.NoError(t, got.err)
			assert.True(t, strings.HasPrefix(got.stdout, "CONFIG   1 change\n"), got.stdout)
			assert.Contains(t, got.stdout, "\nPlugins\n")
			assert.Contains(t, got.stdout, "1.0.0")
			assert.Contains(t, got.stdout, "1.1.0")
			assert.NotContains(t, got.stdout, "General")
			assert.NotContains(t, got.stdout, "Distribution")
		})

		t.Run(kind+" without a rules file", func(t *testing.T) {
			bare := tC
			bare.ruleFile, bare.rules = "", ""

			got := runDiffCase(t, bare)

			require.NoError(t, got.err)
			assert.True(t, strings.HasPrefix(got.stdout, "CONFIG   4 changes\n"), got.stdout)
			assert.NotContains(t, got.stdout, "blocked")
			assert.NotContains(t, got.stdout, "migration")
			assert.True(t, strings.HasSuffix(got.stdout, "\n\n"+diffNoRulesLine+"\n"), got.stdout)
		})
	}
}
