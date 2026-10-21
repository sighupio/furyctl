// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package create_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/common/create"
	"github.com/sighupio/furyctl/internal/cluster"
)

const kappFixtures = "../../../../../tool/kapp/testdata/diff/"

// fakeRunner records its calls and writes the configured kapp files, as diff.sh would.
type fakeRunner struct {
	calls [][]string
	dir   string
	files map[string]string
	out   string
	code  int
	err   error
}

func (f *fakeRunner) RunCapture(args ...string) (string, int, error) {
	f.calls = append(f.calls, args)

	for name, content := range f.files {
		if err := os.WriteFile(filepath.Join(f.dir, name), []byte(content), os.FileMode(0o600)); err != nil {
			return "", 0, err
		}
	}

	return f.out, f.code, f.err
}

func readKappFixture(t *testing.T, name string) string {
	t.Helper()

	b, err := os.ReadFile(kappFixtures + name)
	require.NoError(t, err)

	return string(b)
}

func newDistroSource(t *testing.T, version string, withTemplate bool, phase string) string {
	t.Helper()

	dir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(dir, "kfd.yaml"), []byte("version: "+version+"\n"), os.FileMode(0o600)))

	if withTemplate {
		tplDir := filepath.Join(dir, "templates", phase, "scripts")
		require.NoError(t, os.MkdirAll(tplDir, os.FileMode(0o755)))
		require.NoError(t, os.WriteFile(filepath.Join(tplDir, "diff.sh.tpl"), []byte("#!/bin/sh\n"), os.FileMode(0o600)))
	}

	return dir
}

func newPhaseFolder(t *testing.T) (string, string) {
	t.Helper()

	phase := t.TempDir()
	manifests := filepath.Join(phase, "manifests")

	require.NoError(t, os.MkdirAll(manifests, os.FileMode(0o755)))
	require.NoError(t, os.MkdirAll(filepath.Join(phase, "scripts"), os.FileMode(0o755)))
	require.NoError(t, os.WriteFile(filepath.Join(phase, "scripts", "diff.sh"), []byte("#!/bin/sh\n"), os.FileMode(0o600)))

	return phase, manifests
}

func TestDiffDistribution_NotSupportedWithoutTemplate(t *testing.T) {
	t.Parallel()

	for _, version := range []string{"v1.35.1", "v1.34.2"} {
		t.Run(version, func(t *testing.T) {
			t.Parallel()

			phase, manifests := newPhaseFolder(t) // a rendered diff.sh is there and must not count
			distro := newDistroSource(t, version, false, "distribution")
			r := &fakeRunner{dir: manifests}

			section := create.DiffDistribution(distro, phase, r)

			require.Equal(t, create.StatusNotSupported, section.Status)
			require.Equal(t, "distribution "+version, section.Reason)
			require.Empty(t, r.calls)
		})
	}
}

func TestDiffDistribution_RunsTheScriptOnce(t *testing.T) {
	t.Parallel()

	phase, manifests := newPhaseFolder(t)
	distro := newDistroSource(t, "v1.36.0", true, "distribution")
	r := &fakeRunner{dir: manifests, files: map[string]string{"kapp-diff.json": readKappFixture(t, "kapp-0.66.1-a-nochanges.json")}}

	section := create.DiffDistribution(distro, phase, r)

	require.Equal(t, create.StatusCompared, section.Status)
	require.Equal(t, [][]string{{filepath.Join(phase, "scripts", "diff.sh")}}, r.calls)
}

func TestDiffDistribution_SectionFollowsExitCodeAndKappFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		file    string
		out     string
		code    int
		status  create.Status
		changes int
		reason  string
	}{
		{"changes", readKappFixture(t, "kapp-0.66.1-b-changes.json"), "Succeeded", 0, create.StatusCompared, 3, ""},
		{"no changes", readKappFixture(t, "kapp-0.66.1-a-nochanges.json"), "Succeeded", 0, create.StatusCompared, 0, ""},
		{"not ready is not a change", readKappFixture(t, "kapp-0.66.1-d-notready.json"), "Succeeded", 0, create.StatusCompared, 0, ""},
		{"no file", "", "Succeeded", 0, create.StatusFailed, 0, create.ReasonKappFormat},
		{"unknown op", `{"Tables":[{"Rows":[{"namespace":"demo","name":"cm","kind":"ConfigMap","op":"replace"}]}]}`, "Succeeded", 0, create.StatusFailed, 0, create.ReasonKappFormat},
		{
			"kapp error", readKappFixture(t, "kapp-0.66.1-e-unreachable.json"), "Target cluster 'https://192.168.200.250:6999'", 1, create.StatusFailed, 0,
			`scripts/diff.sh exited 1: kapp: Error: Getting app: Get "https://192.168.200.250:6999/api/v1/namespaces/kube-system/configmaps/kfd.apps.k14s.io": ` +
				`dial tcp 192.168.200.250:6999: connect: connection refused`,
		},
		{"valid file ignored on failure", readKappFixture(t, "kapp-0.66.1-b-changes.json"), "kapp failed late\n\nretry later\n", 1, create.StatusFailed, 0, "scripts/diff.sh exited 1: retry later"},
		{"failure before kapp", "", "building out.yaml\n\nError: accumulating resources\n", 2, create.StatusFailed, 0, "scripts/diff.sh exited 2: Error: accumulating resources"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			phase, manifests := newPhaseFolder(t)
			distro := newDistroSource(t, "v1.36.0", true, "distribution")
			r := &fakeRunner{dir: manifests, out: tc.out, code: tc.code}

			if tc.file != "" {
				r.files = map[string]string{"kapp-diff.json": tc.file}
			}

			section := create.DiffDistribution(distro, phase, r)

			require.Equal(t, tc.status, section.Status)
			require.Len(t, section.Rows, tc.changes)
			require.Equal(t, tc.reason, section.Reason)
		})
	}
}

func TestDiffDistribution_StaleKappOutputIsDeleted(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		stale  string
		out    string
		code   int
		reason string
	}{
		{"failure", "kapp-0.66.1-e-unreachable.json", "yq: invalid input\nsed failed here\n", 3, "scripts/diff.sh exited 3: sed failed here"},
		{"success without file", "kapp-0.66.1-a-nochanges.json", "Succeeded\n", 0, create.ReasonKappFormat},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			phase, manifests := newPhaseFolder(t)
			distro := newDistroSource(t, "v1.36.0", true, "distribution")

			require.NoError(t, os.WriteFile(filepath.Join(manifests, "kapp-diff.json"), []byte(readKappFixture(t, tc.stale)), os.FileMode(0o600)))
			require.NoError(t, os.WriteFile(filepath.Join(manifests, "kapp-diff-old.json"), []byte("{}"), os.FileMode(0o600)))

			section := create.DiffDistribution(distro, phase, &fakeRunner{dir: manifests, out: tc.out, code: tc.code})

			require.NoFileExists(t, filepath.Join(manifests, "kapp-diff.json"))
			require.NoFileExists(t, filepath.Join(manifests, "kapp-diff-old.json"))
			require.Equal(t, create.StatusFailed, section.Status)
			require.Equal(t, tc.reason, section.Reason)
		})
	}
}

func TestNewDistributionDiffer_RunsTheScriptInTheManifestsFolder(t *testing.T) {
	t.Parallel()

	workDir := t.TempDir()
	phase := filepath.Join(workDir, "distribution")
	manifests := filepath.Join(phase, "manifests")

	require.NoError(t, os.MkdirAll(filepath.Join(phase, "scripts"), os.FileMode(0o755)))
	require.NoError(t, os.MkdirAll(manifests, os.FileMode(0o755)))

	scriptPath := filepath.Join(phase, "scripts", "diff.sh")

	abs, err := filepath.Abs(kappFixtures)
	require.NoError(t, err)

	script := "pwd -P > ../cwd.txt\ncp " + abs + "/kapp-0.66.1-a-nochanges.json kapp-diff.json\n"

	require.NoError(t, os.WriteFile(scriptPath, []byte(script), os.FileMode(0o600)))

	distro := newDistroSource(t, "v1.36.0", true, "distribution")

	section := create.NewDistributionDiffer(cluster.CreatorPaths{WorkDir: workDir, DistroPath: distro})()

	require.Equal(t, create.StatusCompared, section.Status, section.Reason)

	got, err := os.ReadFile(filepath.Join(phase, "cwd.txt"))
	require.NoError(t, err)

	want, err := filepath.EvalSymlinks(manifests)
	require.NoError(t, err)
	require.Equal(t, want, strings.TrimSpace(string(got)))
}
