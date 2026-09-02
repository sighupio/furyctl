// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package drydock

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//nolint:gochecknoglobals // Test flag, the usual way to refresh golden files.
var update = flag.Bool("update", false, "rewrite golden files")

const immutableSchema = "testdata/distro/schemas/public/immutable-kfd-v1alpha2.json"

func loadAnswers(t *testing.T) map[string]any {
	t.Helper()

	b, err := os.ReadFile("testdata/answers-immutable.json")
	require.NoError(t, err)

	var answers map[string]any
	require.NoError(t, json.Unmarshal(b, &answers))

	return answers
}

func renderImmutable(t *testing.T, answers map[string]any) string {
	t.Helper()

	reg, err := LoadEmbedded()
	require.NoError(t, err)

	_, tpl, err := reg.Find("Immutable", "v1.35.1")
	require.NoError(t, err)

	out, err := Render(tpl, answers)
	require.NoError(t, err)

	return out
}

func TestRenderImmutableGolden(t *testing.T) {
	t.Parallel()

	out := renderImmutable(t, loadAnswers(t))

	golden := filepath.Join("testdata", "golden", "immutable.yaml")
	if *update {
		require.NoError(t, os.WriteFile(golden, []byte(out), 0o600))
	}

	want, err := os.ReadFile(golden)
	require.NoError(t, err)
	assert.Equal(t, string(want), out)
}

func TestRenderedImmutableIsValid(t *testing.T) {
	t.Parallel()

	out := renderImmutable(t, loadAnswers(t))

	errs, err := Validate(immutableSchema, out)
	require.NoError(t, err)
	assert.Empty(t, errs, "schema errors:\n%s", out)
}

func TestRenderDerivesTopology(t *testing.T) {
	t.Parallel()

	out := renderImmutable(t, loadAnswers(t))

	for _, want := range []string{
		`address: "api.k8s.example.com:6443"`,
		`- hostname: "lb1.k8s.example.com"`,
		`ip: "192.168.1.201"`,
		`virtualRouterId: "1"`,
		`- hostname: "cp1.k8s.example.com"`,
		"- name: infra",
		"node-role.kubernetes.io/infra",
		`- name: "gpu"`,
		`"accelerator": "nvidia"`,
		`key: "nvidia.com/gpu"`,
		"dhcp4: true",
		"arch: arm64",
		`- "192.168.1.10/24"`,
		"wipe_table: true",
		`path: "/var/lib/longhorn"`,
		`name: "iscsid.service"`,
		"vm.max_map_count",
		`macAddress: "{env://CP1_MAC}"`,
		`configuration: "{file://./secrets/etcd-encryption-config.yaml}"`,
		"type: none", // kube-proxy
		`issuer_url: "https://login.example.com/realms/main"`,
		"backend: externalEndpoint",
		"validationFailureAction: Enforce",
		"type: on-premises",
		`password: "{env://AUTH_BASIC_PASSWORD}"`,
	} {
		assert.Contains(t, out, want)
	}

	assert.NotContains(t, out, "etcd:", "etcd members are only written for dedicated etcd")
	assert.NotContains(t, out, "\n    proxy:", "empty proxy group is omitted")
}

func TestRenderKeepalivedOnControlPlane(t *testing.T) {
	t.Parallel()

	answers := loadAnswers(t)

	topo, ok := answers["topology"].(map[string]any)
	require.True(t, ok)

	topo["lbMode"] = "keepalived-on-cp"
	topo["dedicatedEtcd"] = true

	nodes, ok := answers["nodes"].([]any)
	require.True(t, ok)

	kept := make([]any, 0, len(nodes))

	for _, n := range nodes {
		node, ok := n.(map[string]any)
		require.True(t, ok)

		if node["role"] != "lb" {
			kept = append(kept, n)
		}
	}

	first, ok := kept[0].(map[string]any)
	require.True(t, ok)

	etcd := map[string]any{}
	for k, v := range first {
		etcd[k] = v
	}

	etcd["hostname"], etcd["role"], etcd["macAddress"], etcd["ip"] = "etcd1.k8s.example.com", "etcd", "52:54:00:05:00:01", "192.168.1.20"
	answers["nodes"] = append(kept, etcd)

	out := renderImmutable(t, answers)

	assert.NotContains(t, out, "loadBalancers:")
	assert.Contains(t, out, "etcd:\n      members:\n        - hostname: \"etcd1.k8s.example.com\"")

	idx := strings.Index(out, "controlPlane:")
	require.Positive(t, idx)
	assert.Contains(t, out[idx:], "keepalived:\n        enabled: true")

	errs, err := Validate(immutableSchema, out)
	require.NoError(t, err)
	assert.Empty(t, errs)
}

func TestRenderHyperconverged(t *testing.T) {
	t.Parallel()

	answers := loadAnswers(t)

	topo, ok := answers["topology"].(map[string]any)
	require.True(t, ok)

	// Three machines, everything on them: no load balancer nodes, no workers, and the control
	// plane taints emptied so ordinary pods are scheduled there.
	topo["lbMode"] = "keepalived-on-cp"
	topo["schedulableControlPlane"] = true
	topo["infraCount"] = float64(0)
	topo["workersCount"] = float64(0)
	topo["extraGroups"] = []any{}

	nodes, ok := answers["nodes"].([]any)
	require.True(t, ok)

	cps := make([]any, 0, 3)

	for _, n := range nodes {
		node, ok := n.(map[string]any)
		require.True(t, ok)

		if node["role"] == "cp" {
			cps = append(cps, n)
		}
	}

	require.Len(t, cps, 3)
	answers["nodes"] = cps

	out := renderImmutable(t, answers)

	assert.Contains(t, out, "taints: []")
	assert.NotContains(t, out, "loadBalancers:")
	assert.Contains(t, out, "nodeGroups: []", "an explicit empty list, as the reference configuration writes it")
	assert.NotContains(t, out, "nodeSelector:", "with no infra nodes the distribution is not pinned anywhere")

	idx := strings.Index(out, "controlPlane:")
	require.Positive(t, idx)
	assert.Contains(t, out[idx:], "keepalived:\n        enabled: true")

	errs, err := Validate(immutableSchema, out)
	require.NoError(t, err)
	assert.Empty(t, errs)
}

func TestRenderTemplateErrorIsTyped(t *testing.T) {
	t.Parallel()

	_, err := Render(`{{ template "nope" }}`, map[string]any{})
	require.ErrorIs(t, err, ErrTemplate)
}

func TestValidateReportsPathsAndToleratesDynamicValues(t *testing.T) {
	t.Parallel()

	answers := loadAnswers(t)

	nodes, ok := answers["nodes"].([]any)
	require.True(t, ok)

	lb2, ok := nodes[1].(map[string]any)
	require.True(t, ok)

	lb2["macAddress"] = "not-a-mac"

	out := renderImmutable(t, answers)

	errs, err := Validate(immutableSchema, out)
	require.NoError(t, err)
	require.NotEmpty(t, errs)

	paths := make([]string, 0, len(errs))
	for _, e := range errs {
		paths = append(paths, e.Path)
	}

	assert.Contains(t, paths, "/spec/infrastructure/nodes/1/macAddress")
	assert.NotContains(t, paths, "/spec/infrastructure/nodes/2/macAddress", "{env://CP1_MAC} must not be reported")

	for _, e := range errs {
		if e.Path == "/spec/infrastructure/nodes/1/macAddress" {
			assert.False(t, e.Missing, "a wrong value is a mistake, not a blank")
		}
	}
}

func TestValidateMarksBlanksAsMissing(t *testing.T) {
	t.Parallel()

	answers := loadAnswers(t)

	cluster, ok := answers["cluster"].(map[string]any)
	require.True(t, ok)

	cluster["name"] = ""

	out := renderImmutable(t, answers)

	errs, err := Validate(immutableSchema, out)
	require.NoError(t, err)
	require.NotEmpty(t, errs)

	for _, e := range errs {
		if e.Path == "/metadata/name" {
			assert.True(t, e.Missing)

			return
		}
	}

	t.Fatal("no error reported for the empty name")
}

func TestValidateRejectsBrokenYAML(t *testing.T) {
	t.Parallel()

	_, err := Validate(immutableSchema, "a: [")
	require.Error(t, err)
}
