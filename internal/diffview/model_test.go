// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package diffview_test

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	r3diff "github.com/r3labs/diff/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/immutable/supported"
	"github.com/sighupio/furyctl/internal/diffview"
	"github.com/sighupio/furyctl/pkg/diffs"
	rules "github.com/sighupio/furyctl/pkg/rulesextractor"
	yamlx "github.com/sighupio/furyctl/pkg/x/yaml"
)

const (
	// The folder that holds the real rule files of the distribution
	// (pkg/diffs/testdata/rules, see its PROVENANCE).
	realRules = "../../pkg/diffs/testdata"

	titleGeneral        = "General"
	titleInfrastructure = "Infrastructure"
	titleKubernetes     = "Kubernetes"
	titleDistribution   = "Distribution"
	titlePlugins        = "Plugins"

	// A node group named worker with three nodes, as YAML under spec.kubernetes.
	workers = `{nodeGroups: [{name: worker, nodes: [
		{hostname: worker1.example.test}, {hostname: worker2.example.test}, {hostname: worker3.example.test}%s]}]}`
)

func config(t *testing.T, content string) map[string]any {
	t.Helper()

	parsed := map[string]any{}

	require.NoError(t, yamlx.UnmarshalV3([]byte(content), &parsed))

	return parsed
}

func simulation(t *testing.T) (map[string]any, map[string]any) {
	t.Helper()

	current, err := yamlx.FromFileV3[map[string]any](filepath.Join("testdata", "simulation", "cur.yaml"))
	require.NoError(t, err)

	updated, err := yamlx.FromFileV3[map[string]any](filepath.Join("testdata", "simulation", "new2.yaml"))
	require.NoError(t, err)

	return current, updated
}

func changelogOf(t *testing.T, current, updated map[string]any) r3diff.Changelog {
	t.Helper()

	changelog, err := diffs.NewBaseChecker(current, updated).GenerateDiff()
	require.NoError(t, err)

	return changelog
}

// modelOf builds the table as furyctl diff does for an Immutable cluster: the verdicts
// of the changelog with the real rule file, then the model.
func modelOf(t *testing.T, current, updated map[string]any) diffview.Model {
	t.Helper()

	extractor, err := rules.NewImmutableClusterRulesExtractor(realRules, current, supported.Phases())
	require.NoError(t, err)

	profile, ok := diffs.ProfileForKind("Immutable")
	require.True(t, ok)

	model := diffview.NewModel(diffs.Verdicts(changelogOf(t, current, updated), extractor, profile), current, updated)

	require.NoError(t, model.Validate())

	return model
}

// simulationModel is the table of the simulation configurations.
func simulationModel(t *testing.T) diffview.Model {
	t.Helper()

	current, updated := simulation(t)

	return modelOf(t, current, updated)
}

// modelOfYAML builds the table from two configurations given as the YAML value of
// their spec key.
func modelOfYAML(t *testing.T, currentSpec, updatedSpec string) diffview.Model {
	t.Helper()

	return modelOf(t, config(t, "spec: "+currentSpec), config(t, "spec: "+updatedSpec))
}

func titles(model diffview.Model) []string {
	out := make([]string, 0, len(model.Groups))

	for _, group := range model.Groups {
		out = append(out, group.Title)
	}

	return out
}

func fields(model diffview.Model, title string) []string {
	out := []string{}

	for _, group := range model.Groups {
		if title != "" && group.Title != title {
			continue
		}

		for _, row := range group.Rows {
			out = append(out, row.Field)
		}
	}

	return out
}

// rowOf gives the single row with that FIELD under that group title.
func rowOf(t *testing.T, model diffview.Model, title, field string) diffview.Row {
	t.Helper()

	found := []diffview.Row{}

	for _, group := range model.Groups {
		for _, row := range group.Rows {
			if group.Title == title && row.Field == field {
				found = append(found, row)
			}
		}
	}

	require.Len(t, found, 1, "rows %s under %s; the table has %v", field, title, fields(model, ""))

	return found[0]
}

// cells are the marker and the two value cells of a row.
type cells struct {
	marker  diffview.Marker
	current string
	updated string
}

func cellsOf(row diffview.Row) cells {
	return cells{row.Marker, row.Current, row.New}
}

func TestNewModel_Layout(t *testing.T) {
	t.Parallel()

	t.Run("changes are grouped by phase in a fixed order", func(t *testing.T) {
		t.Parallel()

		model := modelOfYAML(
			t,
			`{distribution: {modules: {logging: {type: none}}}, kubernetes: {networking: {podCIDR: 172.16.128.0/17}},
			  infrastructure: {loadBalancers: {keepalived: {passphrase: one}}}}`,
			`{distribution: {modules: {logging: {type: loki}}}, kubernetes: {networking: {podCIDR: 10.42.0.0/16}},
			  infrastructure: {loadBalancers: {keepalived: {passphrase: two}}}}`,
		)

		assert.Equal(t, []string{titleInfrastructure, titleKubernetes, titleDistribution}, titles(model))
		assert.Equal(
			t,
			[]string{"loadBalancers.keepalived.passphrase", "networking.podCIDR", "modules.logging.type"},
			fields(model, ""),
		)
	})

	t.Run("a change under plugins is the last group", func(t *testing.T) {
		t.Parallel()

		model := modelOfYAML(
			t,
			`{plugins: {kustomize: {folder: a}}, distribution: {modules: {dr: {type: none}}}}`,
			`{plugins: {kustomize: {folder: b}}, distribution: {modules: {dr: {type: eks}}}}`,
		)

		assert.Equal(t, []string{titleDistribution, "Plugins"}, titles(model))
		assert.Equal(t, []string{"kustomize.folder"}, fields(model, "Plugins"))
	})

	t.Run("the phase prefix is removed from FIELD", func(t *testing.T) {
		t.Parallel()

		model := modelOfYAML(
			t,
			`{distribution: {modules: {logging: {type: none}}}}`,
			`{distribution: {modules: {logging: {type: loki}}}}`,
		)

		row := rowOf(t, model, titleDistribution, "modules.logging.type")

		assert.Equal(t, cells{diffview.MarkerChanged, "none", "loki"}, cellsOf(row))
		assert.Equal(t, []string{"spec", "distribution", "modules", "logging", "type"}, row.Path)
		assert.False(t, row.Sensitive)
	})

	t.Run("markers and absent sides", func(t *testing.T) {
		t.Parallel()

		model := modelOfYAML(
			t,
			`{distribution: {modules: {monitoring: {prometheus: {retentionSize: 10GB}}, tracing: {type: none, tempo: {}}}}}`,
			`{distribution: {modules: {monitoring: {prometheus: {retentionSize: 10GB, retentionTime: 15d}}, tracing: {tempo: {}}}}}`,
		)

		assert.Equal(
			t,
			cells{diffview.MarkerAdded, "–", "15d"},
			cellsOf(rowOf(t, model, titleDistribution, "modules.monitoring.prometheus.retentionTime")),
		)
		assert.Equal(
			t,
			cells{diffview.MarkerRemoved, "none", "–"},
			cellsOf(rowOf(t, model, titleDistribution, "modules.tracing.type")),
		)
	})

	t.Run("changes outside the phases go to General", func(t *testing.T) {
		t.Parallel()

		model := modelOf(
			t,
			config(t, `{kind: Immutable, spec: {distributionVersion: v1.35.1, distribution: {modules: {logging: {type: none}}}}}`),
			config(t, `{kind: Immutable, spec: {distributionVersion: v1.36.0, distribution: {modules: {logging: {type: loki}}}}}`),
		)

		assert.Equal(t, []string{titleGeneral, titleDistribution}, titles(model))
		assert.Equal(
			t,
			cells{diffview.MarkerChanged, "v1.35.1", "v1.36.0"},
			cellsOf(rowOf(t, model, titleGeneral, "spec.distributionVersion")),
		)
	})

	t.Run("a change outside spec keeps its full path", func(t *testing.T) {
		t.Parallel()

		model := modelOf(t, config(t, `{metadata: {name: one}}`), config(t, `{metadata: {name: two}}`))

		assert.Equal(t, []string{titleGeneral}, titles(model))
		assert.Equal(t, []string{"metadata.name"}, fields(model, titleGeneral))
	})
}

func TestNewModel_Summary(t *testing.T) {
	t.Parallel()

	verdict := func(i int, v diffs.Verdict) diffs.ChangeVerdict {
		change := r3diff.Change{
			Type: r3diff.UPDATE,
			Path: []string{"spec", "distribution", "modules", fmt.Sprintf("module%02d", i), "type"},
			From: "a",
			To:   "b",
		}

		return diffs.ChangeVerdict{Change: change, Reported: change, Verdict: v}
	}

	t.Run("summary line counts changes and verdicts", func(t *testing.T) {
		t.Parallel()

		verdicts := make([]diffs.ChangeVerdict, 0, 10)

		for i := range 10 {
			v := diffs.VerdictNone

			switch i {
			case 1, 4:
				v = diffs.VerdictBlocked
			case 7:
				v = diffs.VerdictNeedsConfirmation
			case 8:
				v = diffs.VerdictMigration
			}

			verdicts = append(verdicts, verdict(i, v))
		}

		model := diffview.NewModel(verdicts, nil, nil)

		require.NoError(t, model.Validate())
		assert.Equal(t, 10, model.Changes)
		assert.Equal(t, 2, model.Blocked)
		assert.Equal(t, 1, model.NeedsConfirmation)
		assert.True(t, model.RulesAvailable)
		assert.Len(t, fields(model, titleDistribution), 10)
	})

	t.Run("the simulation has 10 changes, 2 blocked and 1 that needs confirmation", func(t *testing.T) {
		t.Parallel()

		model := simulationModel(t)

		assert.Equal(t, 10, model.Changes)
		assert.Equal(t, 2, model.Blocked)
		assert.Equal(t, 1, model.NeedsConfirmation)
	})

	t.Run("summary omits zero counts and uses the singular", func(t *testing.T) {
		t.Parallel()

		model := diffview.NewModel([]diffs.ChangeVerdict{verdict(0, diffs.VerdictNone)}, nil, nil)

		require.NoError(t, model.Validate())
		assert.Equal(t, 1, model.Changes)
		assert.Zero(t, model.Blocked)
		assert.Zero(t, model.NeedsConfirmation)
	})

	t.Run("a model whose counts are not the ones of its rows is not valid", func(t *testing.T) {
		t.Parallel()

		for name, tamper := range map[string]func(*diffview.Model){
			"changes":            func(m *diffview.Model) { m.Changes++ },
			"blocked":            func(m *diffview.Model) { m.Blocked-- },
			"needs confirmation": func(m *diffview.Model) { m.NeedsConfirmation++ },
		} {
			model := simulationModel(t)

			tamper(&model)

			require.ErrorIs(t, model.Validate(), diffview.ErrInvalidModel, name)
		}
	})
}

func TestNewModel_Order(t *testing.T) {
	t.Parallel()

	t.Run("repeated runs give identical output", func(t *testing.T) {
		t.Parallel()

		current, updated := simulation(t)
		first := modelOf(t, current, updated)

		for range 50 {
			require.Equal(t, first, modelOf(t, current, updated))
		}
	})

	t.Run("rows are sorted inside a group", func(t *testing.T) {
		t.Parallel()

		sans := func(format string) string {
			items := make([]string, 0, 11)

			for i := range 11 {
				items = append(items, fmt.Sprintf(format, i))
			}

			return "[" + strings.Join(items, ", ") + "]"
		}

		model := modelOfYAML(
			t,
			`{distribution: {modules: {monitoring: {type: none}, ingress: {nginx: {type: none}}}, common: {networkPoliciesEnabled: false}},
			  kubernetes: {advanced: {apiServer: {certSANs: `+sans("api%d.example.test")+`}}}}`,
			`{distribution: {modules: {monitoring: {type: prometheus}, ingress: {nginx: {type: single}}}, common: {networkPoliciesEnabled: true}},
			  kubernetes: {advanced: {apiServer: {certSANs: `+strings.NewReplacer(
				"api2.", "new2.", "api9.", "new9.", "api10.", "new10.",
			).Replace(sans("api%d.example.test"))+`}}}}`,
		)

		assert.Equal(
			t,
			[]string{"common.networkPoliciesEnabled", "modules.ingress.nginx.type", "modules.monitoring.type"},
			fields(model, titleDistribution),
		)
		assert.Equal(
			t,
			[]string{"advanced.apiServer.certSANs[2]", "advanced.apiServer.certSANs[9]", "advanced.apiServer.certSANs[10]"},
			fields(model, titleKubernetes),
		)
	})

	t.Run("a path that is a prefix of another comes first", func(t *testing.T) {
		t.Parallel()

		model := diffview.NewModelWithoutRules(r3diff.Changelog{
			{Type: r3diff.UPDATE, Path: []string{"spec", "distribution", "common", "registry", "mirror"}, From: "a", To: "b"},
			{Type: r3diff.UPDATE, Path: []string{"spec", "distribution", "common", "registry"}, From: "a", To: "b"},
			{Type: r3diff.UPDATE, Path: []string{"spec", "distribution", "common", "Registry"}, From: "a", To: "b"},
		}, nil, nil)

		require.NoError(t, model.Validate())
		assert.Equal(
			t,
			[]string{"common.Registry", "common.registry", "common.registry.mirror"},
			fields(model, titleDistribution),
		)
	})

	t.Run("a model whose rows are not sorted is not valid", func(t *testing.T) {
		t.Parallel()

		model := simulationModel(t)

		require.NotEmpty(t, model.Groups)

		rows := model.Groups[len(model.Groups)-1].Rows

		rows[0], rows[1] = rows[1], rows[0]

		require.ErrorIs(t, model.Validate(), diffview.ErrInvalidModel)
	})
}

func TestNewModel_LeafRows(t *testing.T) {
	t.Parallel()

	t.Run("an added object becomes one row per leaf", func(t *testing.T) {
		t.Parallel()

		model := modelOfYAML(
			t,
			`{distribution: {modules: {monitoring: {type: prometheus}}}}`,
			`{distribution: {modules: {monitoring: {type: prometheus, prometheus: {retentionTime: 15d, storageSize: 50Gi}}}}}`,
		)

		assert.Equal(
			t,
			[]string{"modules.monitoring.prometheus.retentionTime", "modules.monitoring.prometheus.storageSize"},
			fields(model, ""),
		)
		assert.Equal(
			t,
			cells{diffview.MarkerAdded, "–", "15d"},
			cellsOf(rowOf(t, model, titleDistribution, "modules.monitoring.prometheus.retentionTime")),
		)
		assert.Equal(
			t,
			cells{diffview.MarkerAdded, "–", "50Gi"},
			cellsOf(rowOf(t, model, titleDistribution, "modules.monitoring.prometheus.storageSize")),
		)
	})

	t.Run("a removed object becomes one row per leaf", func(t *testing.T) {
		t.Parallel()

		model := modelOfYAML(
			t,
			`{distribution: {modules: {tracing: {type: tempo, tempo: {backend: minio, retentionTime: 720h}}}}}`,
			`{distribution: {modules: {tracing: {type: tempo}}}}`,
		)

		assert.Equal(t, []string{"modules.tracing.tempo.backend", "modules.tracing.tempo.retentionTime"}, fields(model, ""))
		assert.Equal(
			t,
			cells{diffview.MarkerRemoved, "minio", "–"},
			cellsOf(rowOf(t, model, titleDistribution, "modules.tracing.tempo.backend")),
		)
	})

	t.Run("a value that is not a scalar is compact JSON", func(t *testing.T) {
		t.Parallel()

		model := modelOfYAML(
			t,
			`{distribution: {modules: {ingress: {nginx: {type: single}}}}}`,
			`{distribution: {customPatches: {}, modules: {ingress: {nginx: {type: single,
			  tolerations: [{operator: Exists, effect: NoSchedule}]}}}}}`,
		)

		assert.Equal(
			t,
			cells{diffview.MarkerAdded, "–", `[{"effect":"NoSchedule","operator":"Exists"}]`},
			cellsOf(rowOf(t, model, titleDistribution, "modules.ingress.nginx.tolerations")),
		)
		assert.Equal(
			t,
			cells{diffview.MarkerAdded, "–", "{}"},
			cellsOf(rowOf(t, model, titleDistribution, "customPatches")),
		)
	})

	t.Run("scalars are printed as furyctl.yaml writes them", func(t *testing.T) {
		t.Parallel()

		model := modelOfYAML(
			t,
			`{distribution: {common: {networkPoliciesEnabled: false, replicas: 1, ratio: 0.5, html: "<a>"}}}`,
			`{distribution: {common: {networkPoliciesEnabled: true, replicas: 3, ratio: 1.5, html: "<a&b>", tags: ["<a&b>"]}}}`,
		)

		assert.Equal(
			t,
			cells{diffview.MarkerChanged, "false", "true"},
			cellsOf(rowOf(t, model, titleDistribution, "common.networkPoliciesEnabled")),
		)
		assert.Equal(t, cells{diffview.MarkerChanged, "1", "3"}, cellsOf(rowOf(t, model, titleDistribution, "common.replicas")))
		assert.Equal(t, cells{diffview.MarkerChanged, "0.5", "1.5"}, cellsOf(rowOf(t, model, titleDistribution, "common.ratio")))
		// JSON is for reading, not for a browser: the characters stay as they are.
		assert.Equal(t, cells{diffview.MarkerChanged, "<a>", "<a&b>"}, cellsOf(rowOf(t, model, titleDistribution, "common.html")))
		assert.Equal(t, cells{diffview.MarkerAdded, "–", `["<a&b>"]`}, cellsOf(rowOf(t, model, titleDistribution, "common.tags")))
	})
}

func TestNewModel_ListLabels(t *testing.T) {
	t.Parallel()

	t.Run("a node added to a named group", func(t *testing.T) {
		t.Parallel()

		model := modelOfYAML(
			t,
			"{kubernetes: "+fmt.Sprintf(workers, "")+"}",
			"{kubernetes: "+fmt.Sprintf(workers, ", {hostname: worker4.example.test}")+"}",
		)

		row := rowOf(t, model, titleKubernetes, "nodeGroups[worker].nodes[worker4.example.test].hostname")

		assert.Equal(t, cells{diffview.MarkerAdded, "–", "worker4.example.test"}, cellsOf(row))
		assert.Equal(t, []string{"spec", "kubernetes", "nodeGroups", "0", "nodes", "3", "hostname"}, row.Path)
	})

	t.Run("the name of an item wins over its hostname", func(t *testing.T) {
		t.Parallel()

		model := modelOfYAML(
			t,
			`{infrastructure: {nodes: [{name: first, hostname: node1.example.test, arch: x86-64}]}}`,
			`{infrastructure: {nodes: [{name: first, hostname: node1.example.test, arch: arm64}]}}`,
		)

		assert.Equal(t, []string{"nodes[first].arch"}, fields(model, titleInfrastructure))
	})

	t.Run("items without name or hostname keep the position", func(t *testing.T) {
		t.Parallel()

		model := modelOfYAML(
			t,
			`{kubernetes: {advanced: {apiServer: {certSANs: [api0.example.test, api.example.test]}}}}`,
			`{kubernetes: {advanced: {apiServer: {certSANs: [api0.example.test, api2.example.test]}}}}`,
		)

		assert.Equal(
			t,
			cells{diffview.MarkerChanged, "api.example.test", "api2.example.test"},
			cellsOf(rowOf(t, model, titleKubernetes, "advanced.apiServer.certSANs[1]")),
		)
	})

	t.Run("an object without name or hostname, or with one that is not a string, keeps the position",
		func(t *testing.T) {
			t.Parallel()

			model := modelOfYAML(
				t,
				`{distribution: {modules: {monitoring: {prometheus: {remoteWrite: [{url: a}, {name: 7, url: c}]}}}}}`,
				`{distribution: {modules: {monitoring: {prometheus: {remoteWrite: [{url: b}, {name: 7, url: d}]}}}}}`,
			)

			assert.Equal(
				t,
				[]string{"modules.monitoring.prometheus.remoteWrite[0].url", "modules.monitoring.prometheus.remoteWrite[1].url"},
				fields(model, titleDistribution),
			)
		})

	t.Run("a removed item is labelled from the current configuration", func(t *testing.T) {
		t.Parallel()

		model := modelOfYAML(
			t,
			"{kubernetes: "+fmt.Sprintf(workers, "")+"}",
			"{kubernetes: "+strings.Replace(fmt.Sprintf(workers, ""), ", {hostname: worker3.example.test}", "", 1)+"}",
		)

		assert.Equal(
			t,
			cells{diffview.MarkerRemoved, "worker3.example.test", "–"},
			cellsOf(rowOf(t, model, titleKubernetes, "nodeGroups[worker].nodes[worker3.example.test].hostname")),
		)
	})
}

func TestNewModel_ListLabelsOfRemovedItems(t *testing.T) {
	t.Parallel()

	nodes := func(hostnames ...string) string {
		items := make([]string, 0, len(hostnames))

		for _, hostname := range hostnames {
			items = append(items, "{hostname: "+hostname+"}")
		}

		return "{kubernetes: {nodeGroups: [{name: worker, nodes: [" + strings.Join(items, ", ") + "]}]}}"
	}

	t.Run("an item removed from the middle keeps its own label", func(t *testing.T) {
		t.Parallel()

		model := modelOfYAML(
			t,
			nodes("a.example.test", "b.example.test", "c.example.test"),
			nodes("a.example.test", "c.example.test"),
		)

		assert.Equal(t, []string{"nodeGroups[worker].nodes[b.example.test].hostname"}, fields(model, ""))
		assert.Equal(
			t,
			cells{diffview.MarkerRemoved, "b.example.test", "–"},
			cellsOf(rowOf(t, model, titleKubernetes, "nodeGroups[worker].nodes[b.example.test].hostname")),
		)
		assert.NotContains(t, fmt.Sprintf("%+v", model), "[c.example.test]")
	})

	t.Run("an item added in the middle is labelled from the new configuration", func(t *testing.T) {
		t.Parallel()

		model := modelOfYAML(
			t,
			nodes("a.example.test", "c.example.test"),
			nodes("a.example.test", "b.example.test", "c.example.test"),
		)

		assert.Equal(t, []string{"nodeGroups[worker].nodes[b.example.test].hostname"}, fields(model, ""))
		assert.Equal(
			t,
			cells{diffview.MarkerAdded, "–", "b.example.test"},
			cellsOf(rowOf(t, model, titleKubernetes, "nodeGroups[worker].nodes[b.example.test].hostname")),
		)
	})

	t.Run("a removed group is labelled with its name in the current configuration", func(t *testing.T) {
		t.Parallel()

		model := modelOfYAML(
			t,
			`{kubernetes: {nodeGroups: [{name: infra, size: 1}, {name: worker, size: 2}]}}`,
			`{kubernetes: {nodeGroups: [{name: worker, size: 2}]}}`,
		)

		assert.Equal(t, []string{"nodeGroups[infra].name", "nodeGroups[infra].size"}, fields(model, ""))
	})
}

func TestNewModel_SecretValues(t *testing.T) {
	t.Parallel()

	t.Run("a changed secret shows no value", func(t *testing.T) {
		t.Parallel()

		model := modelOfYAML(
			t,
			`{infrastructure: {loadBalancers: {keepalived: {passphrase: sentinel-old-4f1c}}}}`,
			`{infrastructure: {loadBalancers: {keepalived: {passphrase: sentinel-new-9a7e}}}}`,
		)

		row := rowOf(t, model, titleInfrastructure, "loadBalancers.keepalived.passphrase")

		assert.Equal(t, cells{diffview.MarkerChanged, "(hidden)", "(hidden)"}, cellsOf(row))
		assert.True(t, row.Sensitive)
		assert.NotContains(t, fmt.Sprintf("%+v", model), "sentinel-")
	})

	t.Run("a secret added or removed is hidden only where it exists", func(t *testing.T) {
		t.Parallel()

		model := modelOfYAML(
			t,
			`{distribution: {modules: {auth: {oldToken: sentinel-removed-60ff, provider: {type: none}}}}}`,
			`{distribution: {modules: {auth: {newKey: sentinel-added-50ee, provider: {type: none}}}}}`,
		)

		assert.Equal(
			t,
			cells{diffview.MarkerAdded, "–", "(hidden)"},
			cellsOf(rowOf(t, model, titleDistribution, "modules.auth.newKey")),
		)
		assert.Equal(
			t,
			cells{diffview.MarkerRemoved, "(hidden)", "–"},
			cellsOf(rowOf(t, model, titleDistribution, "modules.auth.oldToken")),
		)
		assert.NotContains(t, fmt.Sprintf("%+v", model), "sentinel-")
	})

	t.Run("everything below a sensitive key is hidden", func(t *testing.T) {
		t.Parallel()

		model := modelOfYAML(
			t,
			`{distribution: {modules: {auth: {dex: {issuer: a}}}}}`,
			`{distribution: {modules: {auth: {dex: {issuer: a,
			  clientSecret: {value: sentinel-nested-22b0, id: sentinel-nested-33c1, refs: [sentinel-nested-44d2]}}}}}}`,
		)

		for _, field := range []string{
			"modules.auth.dex.clientSecret.id",
			"modules.auth.dex.clientSecret.refs",
			"modules.auth.dex.clientSecret.value",
		} {
			row := rowOf(t, model, titleDistribution, field)

			assert.Equal(t, cells{diffview.MarkerAdded, "–", "(hidden)"}, cellsOf(row), field)
			assert.True(t, row.Sensitive, field)
		}

		assert.NotContains(t, fmt.Sprintf("%+v", model), "sentinel-")
	})

	t.Run("a secret inside a value printed as JSON is hidden", func(t *testing.T) {
		t.Parallel()

		model := modelOfYAML(
			t,
			`{distribution: {modules: {auth: {dex: {issuer: a}}}}}`,
			`{distribution: {modules: {auth: {dex: {issuer: a, connectors: [{id: ldap, bindPassword: sentinel-json-71d3}]}}}}}`,
		)

		row := rowOf(t, model, titleDistribution, "modules.auth.dex.connectors")

		assert.Equal(t, cells{diffview.MarkerAdded, "–", `[{"bindPassword":"(hidden)","id":"ldap"}]`}, cellsOf(row))
		assert.False(t, row.Sensitive)
		assert.NotContains(t, fmt.Sprintf("%+v", model), "sentinel-")
	})

	t.Run("a changed helm set value is hidden", func(t *testing.T) {
		t.Parallel()

		model := modelOfYAML(
			t,
			`{plugins: {helm: {releases: [{name: app, set: [{name: auth.adminPassword, value: sentinel-old-5b2e}]}]}}}`,
			`{plugins: {helm: {releases: [{name: app, set: [{name: auth.adminPassword, value: sentinel-new-6c3f}]}]}}}`,
		)

		row := rowOf(t, model, titlePlugins, "helm.releases[app].set[auth.adminPassword].value")

		assert.Equal(t, cells{diffview.MarkerChanged, "(hidden)", "(hidden)"}, cellsOf(row))
		assert.True(t, row.Sensitive)
		assert.NotContains(t, fmt.Sprintf("%+v", model), "sentinel-")
	})

	t.Run("the helm set values of a release added or removed are hidden", func(t *testing.T) {
		t.Parallel()

		withRelease := `{plugins: {helm: {releases: [{name: app, chart: example/app,
		  set: [{name: auth.adminPassword, value: sentinel-set-7d40}]}]}}}`

		added := modelOfYAML(t, `{plugins: {helm: {releases: []}}}`, withRelease)
		removed := modelOfYAML(t, withRelease, `{plugins: {helm: {releases: []}}}`)

		assert.Equal(
			t,
			cells{diffview.MarkerAdded, "–", `[{"name":"auth.adminPassword","value":"(hidden)"}]`},
			cellsOf(rowOf(t, added, titlePlugins, "helm.releases[app].set")),
		)
		assert.Equal(
			t,
			cells{diffview.MarkerRemoved, `[{"name":"auth.adminPassword","value":"(hidden)"}]`, "–"},
			cellsOf(rowOf(t, removed, titlePlugins, "helm.releases[app].set")),
		)
		assert.NotContains(t, fmt.Sprintf("%+v", added), "sentinel-")
		assert.NotContains(t, fmt.Sprintf("%+v", removed), "sentinel-")
	})

	t.Run("a helm set item added is hidden", func(t *testing.T) {
		t.Parallel()

		model := modelOfYAML(
			t,
			`{plugins: {helm: {releases: [{name: app, set: []}]}}}`,
			`{plugins: {helm: {releases: [{name: app, set: [{name: auth.token, value: sentinel-item-8e51}]}]}}}`,
		)

		assert.NotContains(t, fmt.Sprintf("%+v", model), "sentinel-")
	})

	t.Run("an item of a secret list is labelled by its position", func(t *testing.T) {
		t.Parallel()

		model := modelOfYAML(
			t,
			`{distribution: {customPatches: {secretGenerator: []}}}`,
			`{distribution: {customPatches: {secretGenerator: [{name: sentinel-label-9f62, literals: [a=b]}]}}}`,
		)

		assert.Equal(
			t,
			[]string{"customPatches.secretGenerator[0].literals", "customPatches.secretGenerator[0].name"},
			fields(model, titleDistribution),
		)
		assert.NotContains(t, fmt.Sprintf("%+v", model), "sentinel-")
	})

	t.Run("the user and password of a URL are hidden", func(t *testing.T) {
		t.Parallel()

		model := modelOfYAML(
			t,
			`{infrastructure: {proxy: {https: "http://proxy.example.test:3128"}}}`,
			`{infrastructure: {proxy: {https: "http://user:sentinel-url-a073@proxy.example.test:3128"}}}`,
		)

		assert.Equal(
			t,
			cells{diffview.MarkerChanged, "http://proxy.example.test:3128", "http://(hidden)@proxy.example.test:3128"},
			cellsOf(rowOf(t, model, titleInfrastructure, "proxy.https")),
		)
		assert.NotContains(t, fmt.Sprintf("%+v", model), "sentinel-")
	})

	t.Run("the key of a toleration is shown", func(t *testing.T) {
		t.Parallel()

		model := modelOfYAML(
			t,
			`{distribution: {common: {tolerations: [{key: node.example.test/infra, operator: Exists}]}}}`,
			`{distribution: {common: {tolerations: [{key: node.example.test/ingress, operator: Exists}]}}}`,
		)

		row := rowOf(t, model, titleDistribution, "common.tolerations[0].key")

		assert.Equal(t, cells{diffview.MarkerChanged, "node.example.test/infra", "node.example.test/ingress"}, cellsOf(row))
		assert.False(t, row.Sensitive)
	})
}

func TestNewModel_Rules(t *testing.T) {
	t.Parallel()

	t.Run("the simulation inputs give the expected table", func(t *testing.T) {
		t.Parallel()

		model := simulationModel(t)

		assert.Equal(t, []string{titleInfrastructure, titleKubernetes, titleDistribution}, titles(model))
		assert.Len(t, fields(model, titleInfrastructure), 1)
		assert.Len(t, fields(model, titleKubernetes), 2)
		assert.Len(t, fields(model, titleDistribution), 7)

		byVerdict := map[diffs.Verdict][]string{}

		for _, group := range model.Groups {
			for _, row := range group.Rows {
				byVerdict[row.Verdict] = append(byVerdict[row.Verdict], row.Field)
			}
		}

		assert.Equal(t, []string{"networking.podCIDR", "modules.ingress.nginx.type"}, byVerdict[diffs.VerdictBlocked])
		assert.Equal(t, []string{"common.networkPoliciesEnabled"}, byVerdict[diffs.VerdictNeedsConfirmation])
		assert.Equal(t, []string{"modules.logging.type", "modules.monitoring.type"}, byVerdict[diffs.VerdictMigration])
		assert.Equal(
			t,
			[]string{
				"loadBalancers.keepalived.passphrase",
				"nodeGroups[worker].nodes[worker4.example.test].hostname",
				"modules.monitoring.prometheus.retentionTime",
				"modules.monitoring.prometheus.storageSize",
				"modules.tracing.type",
			},
			byVerdict[diffs.VerdictNone],
		)

		podCIDR := rowOf(t, model, titleKubernetes, "networking.podCIDR")
		assert.Equal(t, "Pod CIDR cannot be changed after cluster creation", podCIDR.Text)

		tracing := rowOf(t, model, titleDistribution, "modules.tracing.type")
		assert.Equal(t, cells{diffview.MarkerRemoved, "none", "–"}, cellsOf(tracing))
		assert.Empty(t, tracing.Text)
	})

	t.Run("rules not available", func(t *testing.T) {
		t.Parallel()

		current := config(t, `{spec: {distribution: {modules: {logging: {type: none}, monitoring: {type: none}}}}}`)
		updated := config(t, `{spec: {distribution: {modules: {logging: {type: loki},
		  monitoring: {type: none, prometheus: {retentionTime: 15d, storageSize: 50Gi}}}}}}`)

		model := diffview.NewModelWithoutRules(changelogOf(t, current, updated), current, updated)

		require.NoError(t, model.Validate())
		assert.False(t, model.RulesAvailable)
		assert.Equal(t, 3, model.Changes)
		assert.Zero(t, model.Blocked)
		assert.Zero(t, model.NeedsConfirmation)
		assert.Equal(
			t,
			[]string{
				"modules.logging.type",
				"modules.monitoring.prometheus.retentionTime",
				"modules.monitoring.prometheus.storageSize",
			},
			fields(model, titleDistribution),
		)

		require.Len(t, model.Groups, 1)

		for _, row := range model.Groups[0].Rows {
			assert.Equal(t, diffs.VerdictNone, row.Verdict, row.Field)
			assert.Empty(t, row.Text, row.Field)
		}
	})

	t.Run("a table without rows is one line", func(t *testing.T) {
		t.Parallel()

		current, _ := simulation(t)
		model := modelOf(t, current, current)

		assert.Empty(t, model.Groups)
		assert.Zero(t, model.Changes)
		assert.True(t, model.RulesAvailable)
	})
}

// These tests give the model changes that no pair of configurations produces through
// the diff library, to cover the edges of its lookups: a position missing from the
// new configuration, a position past the end of its list and a change of a phase key.
func TestNewModel_EdgesOfTheLookups(t *testing.T) {
	t.Parallel()

	t.Run("a removed position is labelled from a list the new configuration lacks", func(t *testing.T) {
		t.Parallel()

		current := config(t, "spec: {kubernetes: {nodeGroups: [{name: worker, size: 3}]}}")
		updated := config(t, "spec: {kubernetes: {dnsZone: example.test}}")

		removed := r3diff.Change{
			Type: r3diff.DELETE,
			Path: []string{"spec", "kubernetes", "nodeGroups", "0", "size"},
			From: 3,
			To:   nil,
		}

		model := diffview.NewModelWithoutRules(r3diff.Changelog{removed}, current, updated)

		require.NoError(t, model.Validate())
		assert.Equal(t, []string{"nodeGroups[worker].size"}, fields(model, titleKubernetes))
	})

	t.Run("a position after the last item of its list keeps the position", func(t *testing.T) {
		t.Parallel()

		both := config(t, "spec: {kubernetes: {nodeGroups: [{name: worker, size: 3}]}}")

		model := diffview.NewModelWithoutRules(
			r3diff.Changelog{changeOf(".spec.kubernetes.nodeGroups.1.size", 3, 4)},
			both,
			both,
		)

		require.NoError(t, model.Validate())
		assert.Equal(t, []string{"nodeGroups[1].size"}, fields(model, titleKubernetes))
	})

	t.Run("a change of the phase key itself goes to General with its whole path", func(t *testing.T) {
		t.Parallel()

		model := diffview.NewModelWithoutRules(
			r3diff.Changelog{changeOf(".spec.kubernetes", "a", "b")},
			nil,
			nil,
		)

		require.NoError(t, model.Validate())
		assert.Equal(t, []string{titleGeneral}, titles(model))
		assert.Equal(t, []string{"spec.kubernetes"}, fields(model, titleGeneral))
	})
}
