// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package create

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

const kappDiffRun = `"$kappbin" deploy -a kfd -n kube-system -f out.yaml --allow-all-ns --diff-run --json > kapp-diff.json`

func TestCheckReadOnlyScript_DistributionScriptsPass(t *testing.T) {
	t.Parallel()

	for _, name := range []string{
		"onpremises-02-full-cilium-haproxy.diff.sh.txt",
		"kfddistribution-01-full-calico-nginx.diff.sh.txt",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			require.NoError(t, checkReadOnlyScript(filepath.Join("testdata", "diff-scripts", name)))
		})
	}
}

func TestCheckReadOnlyScript_RenderedPluginsScriptPasses(t *testing.T) {
	t.Parallel()

	s := newPluginsSetup(t, "v1.36.0", true, []string{"alpha", "beta"}, nil, false)

	_, _, err := s.plugins.render()
	require.NoError(t, err)

	require.NoError(t, checkReadOnlyScript(filepath.Join(s.plugins.Path, "scripts", "diff.sh")))
}

func TestCheckReadOnlyScript_MissingScriptIsRefused(t *testing.T) {
	t.Parallel()

	err := checkReadOnlyScript(filepath.Join(t.TempDir(), "diff.sh"))

	require.ErrorIs(t, err, ErrDiffScriptNotReadOnly)
}

func TestReadOnlyViolation_AllowedLines(t *testing.T) {
	t.Parallel()

	for _, line := range []string{
		kappDiffRun,
		`$kappbin deploy -a "kfd-plugin-a" -n kube-system -f out.yaml --diff-run=true --json > "kapp-diff-a.json"`,
		`if ! $kubectlbin get apiservice v1.monitoring.coreos.com > /dev/null 2>&1; then`,
		`kappbin="/opt/furyctl/bin/kapp/0.66.1/kapp"`,
		`kubectlbin="/opt/furyctl/bin/kubectl/1.33.4/kubectl"`,
		`additionalKappArgs="-f ../../vendor/modules/opa/katalog/tests/kapp/exists.yaml"`,
		`cat out.yaml | $yqbin 'select(.kind != "AlertmanagerConfig")' > out-filtered.yaml`,
		`# kubectl delete is never called, kapp deploy runs with --diff-run only`,
		`  # helm upgrade is out of scope`,
	} {
		require.Empty(t, readOnlyViolation("#!/usr/bin/env sh\nset -e\n"+line+"\n"), line)
	}
}

func TestReadOnlyViolation_WritesAreRefused(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		script string
		reason string
	}{
		{"kapp deploy without diff run", `"$kappbin" deploy -a kfd -f out.yaml --json`, "line 3: kapp deploy without --diff-run"},
		{"kapp deploy with -y", `$kappbin deploy -a kfd -f out.yaml --diff-run -y`, "line 3: -y confirms changes without a diff run"},
		{"kapp deploy with --yes", `$kappbin deploy -a kfd -f out.yaml --diff-run --yes`, "line 3: --yes confirms changes without a diff run"},
		{"kapp deploy with -y in a flag group", `$kappbin deploy -ay kfd -f out.yaml --diff-run`, "line 3: kapp with -ay confirms changes without a diff run"},
		{"diff run turned off", `$kappbin deploy -a kfd -f out.yaml --diff-run=false`, "line 3: --diff-run=false turns off the diff run"},
		{"yes through a variable", `additionalKappArgs="--yes"`, "line 3: --yes confirms changes without a diff run"},
		{"kapp delete", `$kappbin delete -a kfd`, "line 3: kapp delete writes to the cluster"},
		{"kapp deploy by name", `kapp deploy -a kfd -f out.yaml`, "line 3: kapp deploy without --diff-run"},
		{"kapp deploy by path", `/usr/local/bin/kapp deploy -a kfd -f out.yaml`, "line 3: kapp deploy without --diff-run"},
		{"kapp deploy through a braced variable", `"${kappbin}" deploy -a kfd -f out.yaml`, "line 3: kapp deploy without --diff-run"},
		{"kapp deploy split over lines", "$kappbin deploy -a kfd \\\n  -f out.yaml --json", "line 3: kapp deploy without --diff-run"},
		{"kubectl apply", `$kubectlbin apply -f out.yaml`, "line 3: kubectl apply writes to the cluster"},
		{"kubectl create", `kubectl create namespace x`, "line 3: kubectl create writes to the cluster"},
		{"kubectl delete through a braced variable", `${kubectlbin} delete ns x`, "line 3: kubectl delete writes to the cluster"},
		{"kubectl patch by path", `"$bindir/kubectl" -n x patch cm a -p '{}'`, "line 3: kubectl patch writes to the cluster"},
		{"kubectl replace", `$KUBECTL replace -f out.yaml`, "line 3: kubectl replace writes to the cluster"},
		{"kubectl edit", `$kubectl_bin edit cm a`, "line 3: kubectl edit writes to the cluster"},
		{"kubectl scale", `$kubectlbin scale deploy a --replicas 0`, "line 3: kubectl scale writes to the cluster"},
		{"kubectl annotate", `$kubectlbin annotate ns x a=b`, "line 3: kubectl annotate writes to the cluster"},
		{"kubectl label", `$kubectlbin label ns x a=b`, "line 3: kubectl label writes to the cluster"},
		{"kubectl after a read in a pipe", `$kubectlbin get cm -o yaml | $kubectlbin apply -f -`, "line 3: kubectl apply writes to the cluster"},
		{"kubectl in a command substitution", `x=$($kubectlbin delete cm a)`, "line 3: kubectl delete writes to the cluster"},
		{"kubectl inside a string run by a shell", `sh -c "kubectl delete cm a"`, "line 3: kubectl delete writes to the cluster"},
		{"kubectl after a comment that ends with a backslash", "# not continued \\\n$kubectlbin delete cm a", "line 4: kubectl delete writes to the cluster"},
		{"kubectl after a trailing comment", `true # ; kubectl delete cm a`, "line 3: kubectl delete writes to the cluster"},
		{"helm install", `$helmbin install a ./chart`, "line 3: helm install writes to the cluster"},
		{"helm upgrade", `helm upgrade --install a ./chart`, "line 3: helm upgrade writes to the cluster"},
		{"helm uninstall", `/usr/bin/helm uninstall a`, "line 3: helm uninstall writes to the cluster"},
		{"helmfile apply", `$helmfilebin apply`, "line 3: helmfile apply writes to the cluster"},
		{"helmfile sync", `helmfile sync`, "line 3: helmfile sync writes to the cluster"},
		{"helmfile destroy", `"${helmfilebin}" destroy`, "line 3: helmfile destroy writes to the cluster"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tt.reason, readOnlyViolation("#!/usr/bin/env sh\nset -e\n"+tt.script+"\n"+kappDiffRun+"\n"))
		})
	}
}

// The distribution is not compared, and its script is not run, when the script could write to the cluster.
func TestDiffDistribution_ScriptThatIsNotReadOnlyIsNotRun(t *testing.T) {
	t.Parallel()

	distro := filepath.Join(t.TempDir(), "distro")
	tpl := filepath.Join(distro, "templates", "distribution", "scripts")
	phase := t.TempDir()
	manifests := filepath.Join(phase, "manifests")

	require.NoError(t, os.MkdirAll(tpl, os.FileMode(0o755)))
	require.NoError(t, os.WriteFile(filepath.Join(tpl, "diff.sh.tpl"), []byte("#!/bin/sh\n"), os.FileMode(0o600)))
	require.NoError(t, os.MkdirAll(manifests, os.FileMode(0o755)))
	require.NoError(t, os.MkdirAll(filepath.Join(phase, "scripts"), os.FileMode(0o755)))
	require.NoError(t, os.WriteFile(filepath.Join(phase, "scripts", "diff.sh"),
		[]byte("#!/bin/sh\n\"$kappbin\" deploy -a kfd -n kube-system -f out.yaml --json > kapp-diff.json\n"), os.FileMode(0o600)))

	r := &fakeShell{dir: manifests}

	section := DiffDistribution(distro, phase, r)

	require.Equal(t, Section{
		Title:  "Distribution",
		Status: StatusFailed,
		Reason: "the diff script of the distribution is not read-only: line 2: kapp deploy without --diff-run",
	}, section)
	require.Empty(t, r.calls)
}

// Every kustomize plugin is not compared, and the script is not run, when the rendered plugins script could write
// to the cluster; the helm releases are still compared.
func TestPluginsDiff_ScriptThatIsNotReadOnlyIsNotRun(t *testing.T) {
	t.Parallel()

	s := newPluginsSetup(t, "v1.36.0", true, []string{"alpha", "beta"}, []string{"poc"}, false)
	s.helmf.stdout = "Comparing release=poc, chart=podinfo/podinfo\n" +
		"poc, poc, ConfigMap (v1) to be added.\nPlan: 1 to add, 0 to change, 0 to destroy.\n"

	tpl := filepath.Join(s.plugins.paths.DistroPath, "templates", "plugins", "scripts", "diff.sh.tpl")
	require.NoError(t, os.WriteFile(tpl, []byte("#!/bin/sh\n$kappbin deploy -a x -f out.yaml --diff-run --yes\n"), os.FileMode(0o600)))

	sections, err := s.plugins.Diff()

	require.NoError(t, err)

	reason := "the diff script of the distribution is not read-only: line 2: --yes confirms changes without a diff run"

	require.Equal(t,
		"Plugin kustomize/alpha Failed: "+reason+"; Plugin kustomize/beta Failed: "+reason+"; Plugin helm/poc Compared with 1 changes",
		describe(sections))
	require.Empty(t, s.shell.calls)
}
