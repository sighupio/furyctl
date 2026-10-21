// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package kapp_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/tool/kapp"
)

const target = "Target cluster 'https://cp1.homelab.poc:6443' (nodes: cp1.homelab.poc)"

func parseContent(t *testing.T, content string) (kapp.DiffResult, error) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "kapp-diff.json")

	require.NoError(t, os.WriteFile(path, []byte(content), os.FileMode(0o600)))

	return kapp.ParseDiff(path)
}

func parseFixture(t *testing.T, name string) (kapp.DiffResult, error) {
	t.Helper()

	return kapp.ParseDiff(filepath.Join("testdata", "diff", name))
}

func TestParseDiff_Fixtures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		fixture string
		rows    int
		changes int
	}{
		{"kapp-0.66.1-a-nochanges.json", 0, 0},
		{"kapp-0.66.1-b-changes.json", 3, 3},
		{"kapp-0.66.1-d-notready.json", 1, 0},
		{"kapp-0.66.1-plugin-new-app.json", 1, 1},
	}

	for _, tc := range tests {
		t.Run(tc.fixture, func(t *testing.T) {
			t.Parallel()

			res, err := parseFixture(t, tc.fixture)

			require.NoError(t, err)
			require.Len(t, res.Rows, tc.rows)
			require.Len(t, res.Changes(), tc.changes)
			require.Equal(t, target, res.Target)
		})
	}
}

func TestParseDiff_GoldenRows(t *testing.T) {
	t.Parallel()

	tests := []struct {
		fixture string
		row     kapp.Row
	}{
		{"kapp-0.66.1-b-changes.json", kapp.Row{Namespace: "default", Kind: "ConfigMap", Name: "furyctl-diff-probe-dd8m84k84c", Op: "create"}},
		{"kapp-0.66.1-b-changes.json", kapp.Row{Namespace: "kube-system", Kind: "Deployment", Name: "hubble-relay", Op: "update"}},
		{"kapp-0.66.1-b-changes.json", kapp.Row{Namespace: "kube-system", Kind: "ConfigMap", Name: "hubble-ui-nginx", Op: "delete"}},
		{"kapp-0.66.1-d-notready.json", kapp.Row{Namespace: "poc-probe", Kind: "Deployment", Name: "poc-probe-broken", Op: ""}},
		{"kapp-0.66.1-plugin-new-app.json", kapp.Row{Namespace: "default", Kind: "ConfigMap", Name: "furyctl-diff-plugin-probe-h45m7c46kt", Op: "create"}},
	}

	for _, tc := range tests {
		t.Run(tc.fixture+"/"+tc.row.Name, func(t *testing.T) {
			t.Parallel()

			res, err := parseFixture(t, tc.fixture)

			require.NoError(t, err)
			require.Contains(t, res.Rows, tc.row)
		})
	}
}

func TestParseDiff_InvalidFiles(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"empty":               "",
		"truncated":           `{"Tables":[{"Rows":[`,
		"no tables key":       `{"Lines":["Succeeded"]}`,
		"null tables":         `{"Tables":null,"Lines":["Succeeded"]}`,
		"no table":            `{"Tables":[],"Lines":["Succeeded"]}`,
		"row without op":      `{"Tables":[{"Rows":[{"namespace":"demo","name":"cm","kind":"ConfigMap"}]}]}`,
		"unknown op":          `{"Tables":[{"Rows":[{"namespace":"demo","name":"cm","kind":"ConfigMap","op":"replace"}]}]}`,
		"empty kind":          `{"Tables":[{"Rows":[{"namespace":"demo","name":"cm","kind":"","op":"create"}]}]}`,
		"empty name":          `{"Tables":[{"Rows":[{"namespace":"demo","name":"","kind":"ConfigMap","op":"create"}]}]}`,
		"op not a string":     `{"Tables":[{"Rows":[{"namespace":"demo","name":"cm","kind":"ConfigMap","op":1}]}]}`,
		"row without fields":  `{"Tables":[{"Rows":[{"namespace":"demo","name":"cm","kind":"ConfigMap","op":"create"}]},{"Rows":[{}]}]}`,
		"namespace missing":   `{"Tables":[{"Rows":[{"name":"cm","kind":"ConfigMap","op":"create"}]}]}`,
		"namespace not text":  `{"Tables":[{"Rows":[{"namespace":null,"name":"cm","kind":"ConfigMap","op":"create"}]}]}`,
		"not a json document": `kapp: Error: nope`,
	}

	for name, content := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := parseContent(t, content)

			require.ErrorIs(t, err, kapp.ErrInvalidDiff)
		})
	}
}

func TestParseDiff_MissingFile(t *testing.T) {
	t.Parallel()

	_, err := kapp.ParseDiff(filepath.Join(t.TempDir(), "kapp-diff.json"))

	require.ErrorIs(t, err, kapp.ErrInvalidDiff)
}

func TestParseDiff_RowsOfEveryTableAreCounted(t *testing.T) {
	t.Parallel()

	res, err := parseContent(t, `{"Tables":[{"Rows":[{"namespace":"demo","name":"a","kind":"ConfigMap","op":"create"}]},`+
		`{"Rows":[{"namespace":"(cluster)","name":"demo","kind":"Namespace","op":"delete"},`+
		`{"namespace":"demo","name":"b","kind":"Secret","op":"exists"}]}]}`)

	require.NoError(t, err)
	require.Len(t, res.Rows, 3)
	require.Len(t, res.Changes(), 2)
}

func TestParseDiff_NoopRowsAreNotChanges(t *testing.T) {
	t.Parallel()

	res, err := parseContent(t, `{"Tables":[{"Rows":[{"namespace":"demo","name":"a","kind":"ConfigMap","op":"exists"},`+
		`{"namespace":"demo","name":"b","kind":"Deployment","op":""}]}]}`)

	require.NoError(t, err)
	require.Len(t, res.Rows, 2)
	require.Empty(t, res.Changes())
}

func TestParseDiff_KappError(t *testing.T) {
	t.Parallel()

	unreachable, err := parseFixture(t, "kapp-0.66.1-e-unreachable.json")

	require.ErrorIs(t, err, kapp.ErrInvalidDiff)
	require.Equal(t, `kapp: Error: Getting app: Get "https://192.168.200.250:6999/api/v1/namespaces/kube-system/configmaps/kfd.apps.k14s.io": `+
		`dial tcp 192.168.200.250:6999: connect: connection refused`, unreachable.KappError)

	last, err := parseContent(t, `{"Tables":null,"Lines":["kapp: Error: first","Target cluster 'x'","kapp: Error: second:\n    detail"]}`)

	require.ErrorIs(t, err, kapp.ErrInvalidDiff)
	require.Equal(t, "kapp: Error: second: detail", last.KappError)

	valid, err := parseContent(t, `{"Tables":[{"Rows":[]}],"Lines":["Succeeded"]}`)

	require.NoError(t, err)
	require.Empty(t, valid.KappError)
	require.Empty(t, valid.Target)
}
