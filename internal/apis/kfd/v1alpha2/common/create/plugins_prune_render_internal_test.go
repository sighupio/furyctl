// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package create

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	templatex "github.com/sighupio/furyctl/pkg/template"
	yamlx "github.com/sighupio/furyctl/pkg/x/yaml"
)

// pruneStub stands for kapp, kubectl, helm and helmfile: it records its argv and answers from the files of
// a state folder, so a test describes the cluster with files.
const pruneStub = `#!/bin/sh
tool="$(basename "$0")"
printf '%s\n' "$tool $*" >> "$PRUNE_LOG"
case "$tool $1" in
"kapp inspect")
  if [ -f "$PRUNE_STATE/kapp-$3.json" ]; then cat "$PRUNE_STATE/kapp-$3.json"; exit 0; fi
  echo "App '$3' (namespace: kube-system) does not exist" >&2
  exit 1 ;;
"helm status")
  if [ -f "$PRUNE_STATE/helm-$2.yaml" ]; then exit 0; fi
  echo "Error: release: not found" >&2
  exit 1 ;;
"helm get") cat "$PRUNE_STATE/helm-$3.yaml" ;;
"helmfile -f")
  if [ "$5" = template ]; then cat "$PRUNE_STATE/template.yaml" 2>/dev/null; fi ;;
"kubectl get")
  case "$2" in
  crd) [ -f "$PRUNE_STATE/crd-$3" ]; exit $? ;;
  pvc)
    if [ "$3" = -A ]; then cat "$PRUNE_STATE/pvcs-all" 2>/dev/null; else cat "$PRUNE_STATE/pvcs-$4" 2>/dev/null; fi ;;
  esac ;;
esac
exit 0
`

const (
	kappAppA = `{"Tables":[{"Rows":[` +
		`{"kind":"StorageClass","name":"fast","namespace":"(cluster)"},` +
		`{"kind":"PersistentVolumeClaim","name":"data-a-0","namespace":"a-ns"},` +
		`{"kind":"Deployment","name":"a","namespace":"a-ns"}]}]}`
	kappAppB     = `{"Tables":[{"Rows":[{"kind":"Deployment","name":"b","namespace":"b-ns"}]}]}`
	releaseX     = "---\napiVersion: apps/v1\nkind: StatefulSet\nmetadata:\n  name: x\nspec:\n  volumeClaimTemplates:\n    - metadata:\n        name: data\n"
	releaseY     = "---\napiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: y\n"
	chartWithCRD = "---\napiVersion: apiextensions.k8s.io/v1\nkind: CustomResourceDefinition\nmetadata:\n  name: gadgets.example.com\n" +
		"---\napiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: x\n"
)

type pruneScript struct {
	dir    string
	script string
	state  string
	log    string
}

// requireTool skips the test when shellcheck, the only optional tool of these tests, does not run on this host.
func requireTool(t *testing.T, name string) string {
	t.Helper()

	p, err := exec.LookPath(name)
	if err == nil {
		err = exec.Command(p, "--version").Run()
	}

	if err != nil {
		t.Skipf("%s is not available on this host (%v): skipping the prune.sh test", name, err)
	}

	return p
}

// renderPruneScript renders testdata/plugins, the copy of the distribution's templates/plugins, with
// .paths pointing at the recording stub, and Helm releases given as namespace/name.
func renderPruneScript(t *testing.T, spec map[string]any) *pruneScript {
	t.Helper()

	// Only a missing shellcheck may skip (task 4.2): without yq the behaviour tests must fail, not pass silently.
	yq, err := exec.LookPath("yq")
	require.NoError(t, err, "yq runs the rendered prune.sh")

	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	p := &pruneScript{
		dir:    dir,
		script: filepath.Join(dir, "plugins", "scripts", "prune.sh"),
		state:  filepath.Join(dir, "state"),
		log:    filepath.Join(dir, "calls.log"),
	}

	require.NoError(t, os.MkdirAll(bin, 0o755))
	require.NoError(t, os.MkdirAll(p.state, 0o755))

	paths := map[string]any{"yq": yq, "kustomize": filepath.Join(bin, "kustomize")}

	for _, tool := range []string{"kapp", "kubectl", "helm", "helmfile"} {
		paths[tool] = filepath.Join(bin, tool)
		require.NoError(t, os.WriteFile(filepath.Join(bin, tool), []byte(pruneStub), 0o755)) //nolint:gosec // an executable stub.
	}

	cfg, err := yamlx.MarshalV2(map[string]any{"data": map[string]any{"spec": spec, "paths": paths}})
	require.NoError(t, err)

	cfgPath := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(cfgPath, cfg, 0o600))

	tm, err := templatex.NewTemplateModel(
		filepath.Join("testdata", "plugins"),
		filepath.Join(dir, "plugins"),
		cfgPath,
		dir,
		cfgPath,
		".tpl",
		false,
		false,
	)
	require.NoError(t, err)
	require.NoError(t, tm.Generate())

	return p
}

func specWithPlugins(kustomize []string, releases ...string) map[string]any {
	ks := []any{}
	for _, n := range kustomize {
		ks = append(ks, map[string]any{"name": n, "folder": "./plugins/" + n})
	}

	rs := []any{}
	for _, r := range releases {
		ns, n, _ := strings.Cut(r, "/")
		rs = append(rs, map[string]any{"name": n, "namespace": ns, "chart": "repo/" + n})
	}

	return map[string]any{"plugins": map[string]any{"kustomize": ks, "helm": map[string]any{"releases": rs}}}
}

func (p *pruneScript) given(t *testing.T, file, content string) {
	t.Helper()

	require.NoError(t, os.WriteFile(filepath.Join(p.state, file), []byte(content), 0o600))
}

func (p *pruneScript) run(t *testing.T, args ...string) (string, int) {
	t.Helper()

	cmd := exec.Command("sh", append([]string{p.script}, args...)...)
	cmd.Dir = filepath.Join(p.dir, "plugins")
	cmd.Env = append(os.Environ(), "PRUNE_STATE="+p.state, "PRUNE_LOG="+p.log)

	out, err := cmd.CombinedOutput()

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return string(out), exitErr.ExitCode()
	}

	require.NoError(t, err)

	return string(out), 0
}

func (p *pruneScript) calls(t *testing.T) []string {
	t.Helper()

	b, err := os.ReadFile(p.log)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}

	require.NoError(t, err)

	return strings.Split(strings.TrimSpace(string(b)), "\n")
}

// writes are the recorded calls that change the cluster, without the binary folder of the helmfile path.
func (p *pruneScript) writes(t *testing.T) []string {
	t.Helper()

	w := []string{}

	for _, c := range p.calls(t) {
		if strings.Contains(c, " delete") || strings.Contains(c, " destroy") {
			w = append(w, strings.ReplaceAll(c, filepath.Dir(p.script)+"/../helmfile.yaml", "helmfile.yaml"))
		}
	}

	return w
}

func TestPruneScriptRenders(t *testing.T) {
	t.Parallel()

	shellcheck := requireTool(t, "shellcheck")

	for desc, spec := range map[string]map[string]any{
		"both plugin types": specWithPlugins([]string{"a"}, "n/x"),
		"no spec.plugins":   {},
	} {
		t.Run(desc, func(t *testing.T) {
			t.Parallel()

			p := renderPruneScript(t, spec)

			out, err := exec.Command("sh", "-n", p.script).CombinedOutput()
			require.NoError(t, err, string(out))

			out, err = exec.Command(shellcheck, p.script).CombinedOutput()
			require.NoError(t, err, string(out))
		})
	}
}

func TestPruneScriptRefersToEveryPlugin(t *testing.T) {
	t.Parallel()

	p := renderPruneScript(t, specWithPlugins([]string{"a"}, "n/x"))
	p.given(t, "kapp-kfd-plugin-a.json", kappAppB)
	p.given(t, "helm-x.yaml", releaseY)

	out, code := p.run(t, "plan", "kustomize:a", "helm:n/x")

	require.Equal(t, 0, code, out)
	assert.Contains(t, p.calls(t), "kapp inspect -a kfd-plugin-a -n kube-system --json")
	assert.Contains(t, p.calls(t), "helm status x -n n")
}

func TestPruneScriptUnknownTarget(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc   string
		spec   map[string]any
		args   []string
		target string
	}{
		{desc: "no spec.plugins, kustomize", spec: map[string]any{}, args: []string{"delete", "kustomize:a"}, target: "kustomize:a"},
		{desc: "no spec.plugins, helm", spec: map[string]any{}, args: []string{"delete", "helm:n/x"}, target: "helm:n/x"},
		{
			desc:   "a name not in the configuration",
			spec:   specWithPlugins([]string{"a"}),
			args:   []string{"delete", "kustomize:not-in-config"},
			target: "not-in-config",
		},
		{desc: "a release in another namespace", spec: specWithPlugins(nil, "n/x"), args: []string{"delete", "helm:m/x"}, target: "helm:m/x"},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()

			p := renderPruneScript(t, tc.spec)

			out, code := p.run(t, tc.args...)

			assert.NotEqual(t, 0, code)
			assert.Contains(t, out, tc.target)
			assert.Empty(t, p.writes(t))
		})
	}
}

func TestPruneScriptDeletesOnlyTheNamedPlugins(t *testing.T) {
	t.Parallel()

	p := renderPruneScript(t, specWithPlugins([]string{"a", "b"}, "n/x", "n/y"))
	p.given(t, "kapp-kfd-plugin-a.json", kappAppB)
	p.given(t, "kapp-kfd-plugin-b.json", kappAppB)
	p.given(t, "helm-x.yaml", releaseY)
	p.given(t, "helm-y.yaml", releaseY)

	out, code := p.run(t, "delete", "kustomize:a", "helm:n/x")

	require.Equal(t, 0, code, out)
	assert.Equal(t, []string{
		"kapp delete -a kfd-plugin-a -n kube-system --apply-ignored -y",
		"helmfile -f helmfile.yaml -l name=x,namespace=n destroy",
	}, p.writes(t))
}

func TestPruneScriptKustomize(t *testing.T) {
	t.Parallel()

	t.Run("kapp delete command", func(t *testing.T) {
		t.Parallel()

		p := renderPruneScript(t, specWithPlugins([]string{"a"}))
		p.given(t, "kapp-kfd-plugin-a.json", kappAppB)

		out, code := p.run(t, "delete", "kustomize:a")

		require.Equal(t, 0, code, out)
		assert.Equal(t, []string{"kapp delete -a kfd-plugin-a -n kube-system --apply-ignored -y"}, p.writes(t))

		// The guard pass and the delete pass share one description of the target.
		inspects := 0

		for _, c := range p.calls(t) {
			if strings.HasPrefix(c, "kapp inspect") {
				inspects++
			}
		}

		assert.Equal(t, 1, inspects)
	})

	t.Run("already deleted by hand", func(t *testing.T) {
		t.Parallel()

		p := renderPruneScript(t, specWithPlugins([]string{"a"}))

		out, code := p.run(t, "delete", "kustomize:a")

		require.Equal(t, 0, code, out)
		assert.Contains(t, out, "Nothing to delete for a")
		assert.Empty(t, p.writes(t))
	})
}

func TestPruneScriptHelm(t *testing.T) {
	t.Parallel()

	t.Run("delete order", func(t *testing.T) {
		t.Parallel()

		p := renderPruneScript(t, specWithPlugins(nil, "n/x"))
		p.given(t, "template.yaml", chartWithCRD)
		p.given(t, "crd-gadgets.example.com", "")
		p.given(t, "helm-x.yaml", releaseX)
		p.given(t, "pvcs-n", "data-x-0\ndata-x-1\ndata-other-0\n")

		out, code := p.run(t, "delete", "helm:n/x")

		require.Equal(t, 0, code, out)
		assert.Equal(t, []string{
			"kubectl delete gadgets.example.com --all -A --ignore-not-found",
			"helmfile -f helmfile.yaml -l name=x,namespace=n destroy",
			"kubectl delete pvc -n n data-x-0 --ignore-not-found",
			"kubectl delete pvc -n n data-x-1 --ignore-not-found",
			"kubectl delete crd gadgets.example.com --ignore-not-found",
		}, p.writes(t))
	})

	t.Run("release already uninstalled", func(t *testing.T) {
		t.Parallel()

		p := renderPruneScript(t, specWithPlugins(nil, "n/x"))
		p.given(t, "template.yaml", chartWithCRD)

		out, code := p.run(t, "delete", "helm:n/x")

		require.Equal(t, 0, code, out)
		assert.Contains(t, out, "Nothing to delete for x")
		assert.Empty(t, p.writes(t))
	})

	t.Run("release uninstalled with its CRDs left", func(t *testing.T) {
		t.Parallel()

		p := renderPruneScript(t, specWithPlugins(nil, "n/x"))
		p.given(t, "template.yaml", chartWithCRD)
		p.given(t, "crd-gadgets.example.com", "")

		out, code := p.run(t, "delete", "helm:n/x")

		require.Equal(t, 0, code, out)
		assert.Equal(t, []string{
			"kubectl delete gadgets.example.com --all -A --ignore-not-found",
			"kubectl delete crd gadgets.example.com --ignore-not-found",
		}, p.writes(t))
	})
}

func TestPruneScriptPlanIsReadOnly(t *testing.T) {
	t.Parallel()

	p := renderPruneScript(t, specWithPlugins([]string{"a"}, "n/x"))
	p.given(t, "kapp-kfd-plugin-a.json", kappAppA)
	p.given(t, "helm-x.yaml", releaseX)
	p.given(t, "pvcs-n", "data-x-0\n")
	p.given(t, "pvcs-all", "a-ns/data-a-0 fast\nn/data-x-0 standard\n")

	out, code := p.run(t, "plan", "kustomize:a", "helm:n/x")

	require.Equal(t, 0, code, out)
	assert.Empty(t, p.writes(t))

	for _, c := range p.calls(t) {
		assert.Regexp(t, `^(kapp inspect|kubectl get|helm (status|get manifest)|helmfile .* template) `, c)
	}

	for _, s := range []string{
		"Plugin a (kustomize): installed",
		"Plugin x (helm release, namespace n): installed",
		"StorageClasses: fast",
		"a-ns/data-a-0",
		"n/data-x-0",
	} {
		assert.Contains(t, out, s)
	}
}

func TestPruneScriptStorageClassGuard(t *testing.T) {
	t.Parallel()

	for _, mode := range []string{"delete", "plan"} {
		t.Run("a foreign PVC blocks "+mode, func(t *testing.T) {
			t.Parallel()

			p := renderPruneScript(t, specWithPlugins([]string{"a"}))
			p.given(t, "kapp-kfd-plugin-a.json", kappAppA)
			p.given(t, "pvcs-all", "a-ns/data-a-0 fast\napps/data-0 fast\napps/logs-0 standard\nb/no-class \n")

			out, code := p.run(t, mode, "kustomize:a")

			assert.NotEqual(t, 0, code)
			assert.Contains(t, out, "apps/data-0 (StorageClass fast)")
			assert.NotContains(t, out, "apps/logs-0")
			assert.NotContains(t, out, "b/no-class")
			assert.Empty(t, p.writes(t))
		})
	}

	t.Run("the plugin's own PVCs pass", func(t *testing.T) {
		t.Parallel()

		p := renderPruneScript(t, specWithPlugins([]string{"a"}))
		p.given(t, "kapp-kfd-plugin-a.json", kappAppA)
		p.given(t, "pvcs-all", "a-ns/data-a-0 fast\napps/logs-0 standard\n")

		out, code := p.run(t, "delete", "kustomize:a")

		require.Equal(t, 0, code, out)
		assert.Equal(t, []string{"kapp delete -a kfd-plugin-a -n kube-system --apply-ignored -y"}, p.writes(t))
	})
}
