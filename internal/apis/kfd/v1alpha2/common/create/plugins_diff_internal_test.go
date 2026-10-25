// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package create

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	logrustest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/apis/config"
	"github.com/sighupio/furyctl/internal/cluster"
)

const (
	kappFixtureDir   = "../../../../../tool/kapp/testdata/diff/"
	helmFixtureDir   = "../../../../../tool/helmfile/testdata/diff/"
	pluginsTemplates = "testdata/plugins-templates"
)

type fakeShell struct {
	// staleSeen records the kapp files that existed in dir when the script was run.
	staleSeen []string
	dir       string
	files     map[string]string
	out       string
	code      int
	calls     [][]string
}

func (f *fakeShell) Run(...string) (string, error) { return "", nil }

func (f *fakeShell) RunCapture(args ...string) (string, int, error) {
	f.calls = append(f.calls, args)

	f.staleSeen, _ = filepath.Glob(filepath.Join(f.dir, "kapp-diff*.json"))

	for name, content := range f.files {
		if err := os.WriteFile(filepath.Join(f.dir, name), []byte(content), os.FileMode(0o600)); err != nil {
			return "", 0, err
		}
	}

	return f.out, f.code, nil
}

type fakeHelmfile struct {
	calls  []string
	stdout string
	stderr string
	code   int
}

func (f *fakeHelmfile) Init(helm string) error {
	f.calls = append(f.calls, "init --force --helm-binary "+helm)

	return nil
}

func (f *fakeHelmfile) Apply() error {
	f.calls = append(f.calls, "apply")

	return nil
}

func (f *fakeHelmfile) Diff() (string, string, int, error) {
	f.calls = append(f.calls, "diff")

	return f.stdout, f.stderr, f.code, nil
}

type pluginsSetup struct {
	plugins *Plugins
	shell   *fakeShell
	helmf   *fakeHelmfile
}

// newPluginsSetup builds a plugins phase over a temporary distribution source and configuration.
func newPluginsSetup(t *testing.T, version string, withDiffTpl bool, kustomize, releases []string, noSpecPlugins bool) pluginsSetup {
	t.Helper()

	root := t.TempDir()
	distro := filepath.Join(root, "distro")

	require.NoError(t, os.MkdirAll(filepath.Join(distro, "defaults"), os.FileMode(0o755)))
	require.NoError(t, os.WriteFile(filepath.Join(distro, "defaults", "kfddistribution-kfd-v1alpha2.yaml"), []byte("data: {}\n"), os.FileMode(0o600)))
	require.NoError(t, os.WriteFile(filepath.Join(distro, "kfd.yaml"), []byte("version: "+version+"\n"), os.FileMode(0o600)))
	require.NoError(t, os.CopyFS(filepath.Join(distro, "templates", "plugins"), os.DirFS(pluginsTemplates)))

	if !withDiffTpl {
		require.NoError(t, os.Remove(filepath.Join(distro, "templates", "plugins", "scripts", "diff.sh.tpl")))
	}

	var cfg strings.Builder

	cfg.WriteString("apiVersion: kfd.sighup.io/v1alpha2\nkind: KFDDistribution\nspec:\n  distribution: {}\n")

	if !noSpecPlugins {
		cfg.WriteString("  plugins:\n")

		if len(kustomize) > 0 {
			cfg.WriteString("    kustomize:\n")

			for _, n := range kustomize {
				fmt.Fprintf(&cfg, "      - name: %s\n        folder: ./%s\n", n, n)
			}
		}

		if len(releases) > 0 {
			cfg.WriteString("    helm:\n      releases:\n")

			for _, r := range releases {
				fmt.Fprintf(&cfg, "        - name: %s\n          chart: podinfo/podinfo\n", r)
			}
		}
	}

	configPath := filepath.Join(root, "furyctl.yaml")

	require.NoError(t, os.WriteFile(configPath, []byte(cfg.String()), os.FileMode(0o600)))

	p := NewPlugins(
		cluster.CreatorPaths{ConfigPath: configPath, WorkDir: filepath.Join(root, "work"), DistroPath: distro, BinPath: "/opt/furyctl/bin"},
		config.KFD{Version: version},
		"KFDDistribution",
		false,
	)

	sh := &fakeShell{dir: p.Path}
	hf := &fakeHelmfile{}

	p.shellRunner = sh
	p.helmfileRunner = hf
	p.HelmPath = "/opt/furyctl/bin/helm"

	return pluginsSetup{plugins: p, shell: sh, helmf: hf}
}

func fixtureText(t *testing.T, dir, name string) string {
	t.Helper()

	b, err := os.ReadFile(dir + name)
	require.NoError(t, err)

	return string(b)
}

func describe(sections []Section) string {
	parts := make([]string, 0, len(sections))

	for _, s := range sections {
		switch s.Status {
		case StatusCompared:
			parts = append(parts, fmt.Sprintf("%s Compared with %d changes", s.Title, len(s.Rows)))
		case StatusFailed:
			parts = append(parts, fmt.Sprintf("%s Failed: %s", s.Title, s.Reason))
		case StatusNotSupported:
			parts = append(parts, fmt.Sprintf("%s NotSupported for %s", s.Title, s.Reason))
		case StatusOutOfScope:
			parts = append(parts, fmt.Sprintf("%s OutOfScope: %s", s.Title, s.Reason))
		default:
			parts = append(parts, fmt.Sprintf("%q with status %q", s.Title, s.Status))
		}
	}

	return strings.Join(parts, "; ")
}

func TestPluginsDiff_NoPluginsFeature(t *testing.T) {
	t.Parallel()

	s := newPluginsSetup(t, "v1.25.7", true, []string{"alpha"}, []string{"poc-podinfo"}, false)

	sections, err := s.plugins.Diff()

	require.NoError(t, err)
	require.Empty(t, sections)
	require.Empty(t, s.shell.calls)
	require.Empty(t, s.helmf.calls)
}

func TestPluginsDiff_KustomizeSections(t *testing.T) {
	t.Parallel()

	b := fixtureText(t, kappFixtureDir, "kapp-0.66.1-b-changes.json")
	a := fixtureText(t, kappFixtureDir, "kapp-0.66.1-a-nochanges.json")
	n := fixtureText(t, kappFixtureDir, "kapp-0.66.1-plugin-new-app.json")

	tests := []struct {
		name     string
		version  string
		withTpl  bool
		plugins  []string
		files    map[string]string
		calls    int
		sections string
	}{
		{
			"both compared", "v1.36.0", true,
			[]string{"alpha", "beta"},
			map[string]string{"kapp-diff-alpha.json": b, "kapp-diff-beta.json": a},
			1,
			"Plugin kustomize/alpha Compared with 3 changes; Plugin kustomize/beta Compared with 0 changes",
		},
		{
			"one file missing", "v1.36.0", true,
			[]string{"alpha", "beta"},
			map[string]string{"kapp-diff-alpha.json": n},
			1,
			"Plugin kustomize/alpha Compared with 1 changes; Plugin kustomize/beta Failed: kapp output did not match the expected format",
		},
		{
			"no template", "v1.35.1", false,
			[]string{"alpha", "beta"},
			nil, 0,
			"Plugin kustomize/alpha NotSupported for distribution v1.35.1; Plugin kustomize/beta NotSupported for distribution v1.35.1",
		},
		{"no kustomize plugin", "v1.36.0", true, nil, nil, 0, ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s := newPluginsSetup(t, tc.version, tc.withTpl, tc.plugins, nil, false)
			s.shell.files = tc.files
			s.shell.out = "Succeeded"

			sections, err := s.plugins.Diff()

			require.NoError(t, err)
			require.Equal(t, tc.sections, describe(sections))
			require.Len(t, s.shell.calls, tc.calls)
		})
	}
}

func TestPluginsDiff_ScriptRunsFromPluginsFolderOnFreshFiles(t *testing.T) {
	t.Parallel()

	s := newPluginsSetup(t, "v1.36.0", true, []string{"alpha"}, nil, false)
	s.shell.out = "Error: accumulating resources"
	s.shell.code = 1

	require.NoError(t, os.MkdirAll(filepath.Join(s.plugins.Path), os.FileMode(0o755)))
	require.NoError(t, os.WriteFile(
		filepath.Join(s.plugins.Path, "kapp-diff-alpha.json"),
		[]byte(fixtureText(t, kappFixtureDir, "kapp-0.66.1-e-unreachable.json")), os.FileMode(0o600),
	))

	sections, err := s.plugins.Diff()

	require.NoError(t, err)
	require.Empty(t, s.shell.staleSeen, "the earlier kapp output must be deleted before the script runs")
	require.Equal(t, [][]string{{filepath.Join(s.plugins.Path, "scripts", "diff.sh")}}, s.shell.calls)
	require.Equal(t, "Plugin kustomize/alpha Failed: scripts/diff.sh exited 1: Error: accumulating resources", describe(sections))
	require.NoFileExists(t, filepath.Join(s.plugins.Path, "kapp-diff-alpha.json"))
}

func TestPluginsDiff_FailingScriptFailsOnlyThePluginsWithoutACleanResult(t *testing.T) {
	t.Parallel()

	a := fixtureText(t, kappFixtureDir, "kapp-0.66.1-a-nochanges.json")
	b := fixtureText(t, kappFixtureDir, "kapp-0.66.1-b-changes.json")
	e := fixtureText(t, kappFixtureDir, "kapp-0.66.1-e-unreachable.json")
	n := fixtureText(t, kappFixtureDir, "kapp-0.66.1-plugin-new-app.json")
	kappErr := `kapp: Error: Getting app: Get "https://192.168.200.250:6999/api/v1/namespaces/kube-system/configmaps/kfd.apps.k14s.io": ` +
		`dial tcp 192.168.200.250:6999: connect: connection refused`
	failed := func(name, hint string) string {
		return fmt.Sprintf("Plugin kustomize/%s Failed: scripts/diff.sh exited 1: %s", name, hint)
	}

	tests := []struct {
		name  string
		files map[string]string
		out   string
		want  []string
	}{
		{
			"clean plugin kept, last output line for the rest",
			map[string]string{"kapp-diff-alpha.json": b},
			"building beta\n\nError: accumulating resources\n",
			[]string{
				"Plugin kustomize/alpha Compared with 3 changes",
				failed("beta", "Error: accumulating resources"), failed("gamma", "Error: accumulating resources"),
			},
		},
		{
			"kapp error of a later file",
			map[string]string{"kapp-diff-alpha.json": a, "kapp-diff-beta.json": e},
			"Target cluster 'https://192.168.200.250:6999'\n",
			[]string{"Plugin kustomize/alpha Compared with 0 changes", failed("beta", kappErr), failed("gamma", kappErr)},
		},
		{
			"invalid file of the failing plugin",
			map[string]string{"kapp-diff-alpha.json": n, "kapp-diff-beta.json": ""},
			"kapp failed late\n",
			[]string{
				"Plugin kustomize/alpha Compared with 1 changes",
				failed("beta", "kapp failed late"), failed("gamma", "kapp failed late"),
			},
		},
		{
			"no file at all",
			nil,
			"boom\n",
			[]string{failed("alpha", "boom"), failed("beta", "boom"), failed("gamma", "boom")},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s := newPluginsSetup(t, "v1.36.0", true, []string{"alpha", "beta", "gamma"}, nil, false)
			s.shell.files = tc.files
			s.shell.out = tc.out
			s.shell.code = 1

			sections, err := s.plugins.Diff()

			require.NoError(t, err)
			require.Equal(t, strings.Join(tc.want, "; "), describe(sections))
		})
	}
}

func TestPluginsDiff_HelmReleasesNextToKustomize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		version  string
		withTpl  bool
		sections string
	}{
		{
			"with diff.sh.tpl", "v1.36.0", true,
			"Plugin kustomize/alpha Compared with 0 changes; Plugin helm/poc-podinfo Compared with 2 changes; Plugin helm/poc-metrics Compared with 9 changes",
		},
		{
			"without diff.sh.tpl", "v1.35.1", false,
			"Plugin kustomize/alpha NotSupported for distribution v1.35.1; Plugin helm/poc-podinfo Compared with 2 changes; Plugin helm/poc-metrics Compared with 9 changes",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s := newPluginsSetup(t, tc.version, tc.withTpl, []string{"alpha"}, []string{"poc-podinfo", "poc-metrics"}, false)
			s.shell.files = map[string]string{"kapp-diff-alpha.json": fixtureText(t, kappFixtureDir, "kapp-0.66.1-a-nochanges.json")}
			s.shell.out = "Succeeded"
			s.helmf.stdout = fixtureText(t, helmFixtureDir, "helmfile-diff-simple-new-releases.stdout")

			sections, err := s.plugins.Diff()

			require.NoError(t, err)
			require.Equal(t, tc.sections, describe(sections))
			require.Equal(t, []string{"init --force --helm-binary /opt/furyctl/bin/helm", "diff"}, s.helmf.calls)
		})
	}
}

func TestPluginsDiff_HelmFailureIsOneSection(t *testing.T) {
	t.Parallel()

	s := newPluginsSetup(t, "v1.36.0", true, nil, []string{"poc-podinfo", "poc-metrics"}, false)
	s.helmf.code = 1
	s.helmf.stderr = "Error: Failed to get release\n"

	sections, err := s.plugins.Diff()

	require.NoError(t, err)
	require.Equal(t, "Plugins (helm) Failed: helmfile diff exited 1: Error: Failed to get release", describe(sections))
}

func TestPluginsDiff_HelmReleasesAreComparedWhenKustomizeScriptFailsAfterACleanPlugin(t *testing.T) {
	t.Parallel()

	s := newPluginsSetup(t, "v1.36.0", true, []string{"alpha"}, []string{"poc-podinfo", "poc-metrics"}, false)
	s.shell.files = map[string]string{"kapp-diff-alpha.json": fixtureText(t, kappFixtureDir, "kapp-0.66.1-b-changes.json")}
	s.shell.out = "Error: accumulating resources"
	s.shell.code = 1
	s.helmf.stdout = fixtureText(t, helmFixtureDir, "helmfile-diff-simple-new-releases.stdout")

	sections, err := s.plugins.Diff()

	require.NoError(t, err)
	require.Equal(t,
		"Plugin kustomize/alpha Compared with 3 changes; "+
			"Plugin helm/poc-podinfo Compared with 2 changes; Plugin helm/poc-metrics Compared with 9 changes",
		describe(sections))
}

func TestPluginsExec_KeepsItsBehaviour(t *testing.T) {
	tests := []struct {
		name      string
		noPlugins bool
		dryRun    bool
		applied   bool
		calls     []string
		log       string
	}{
		{"apply", false, false, true, []string{"init --force --helm-binary /opt/furyctl/bin/helm", "apply"}, "Plugins installed successfully"},
		{"dry-run", false, true, true, nil, "Plugins installed successfully (dry-run mode)"},
		{"no spec.plugins", true, false, false, nil, "Skipping plugins phase as spec.plugins is not defined"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			hook := logrustest.NewGlobal()
			defer hook.Reset()

			logrus.SetLevel(logrus.InfoLevel)

			s := newPluginsSetup(t, "v1.36.0", true, nil, []string{"poc-podinfo"}, tc.noPlugins)
			s.plugins.dryRun = tc.dryRun

			require.NoError(t, s.plugins.Exec())

			require.Equal(t, tc.applied, s.plugins.Applied())
			require.Equal(t, tc.calls, s.helmf.calls)

			var logged []string
			for _, e := range hook.AllEntries() {
				logged = append(logged, e.Message)
			}

			require.Contains(t, logged, tc.log)
		})
	}
}

func TestPluginsRender_RealDiffTemplateIsSafe(t *testing.T) {
	t.Parallel()

	s := newPluginsSetup(t, "v1.36.0", true, []string{"alpha", "beta"}, []string{"poc-podinfo"}, false)

	_, _, err := s.plugins.render()
	require.NoError(t, err)

	script, err := os.ReadFile(filepath.Join(s.plugins.Path, "scripts", "diff.sh"))
	require.NoError(t, err)

	text := string(script)

	require.Contains(t, text, "kapp-diff-alpha.json")
	require.Contains(t, text, "kapp-diff-beta.json")

	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "$kappbin") {
			require.Contains(t, line, "--diff-run")
			require.NotRegexp(t, `(^| )(-y|--yes)( |$)`, line)
		}

		if strings.HasPrefix(line, "$kubectlbin") {
			require.True(t, strings.HasPrefix(line, "$kubectlbin get "), line)
		}
	}

	require.FileExists(t, filepath.Join(s.plugins.Path, "helmfile.yaml"))
}

// When the script fails, a kapp file that has a table but also a kapp error is not a clean result: the plugin
// section fails with that kapp error instead of showing the table as compared.
func TestPluginsDiff_FailingScriptFileWithTableAndKappErrorIsNotClean(t *testing.T) {
	t.Parallel()

	withError := `{"Tables":[{"Rows":[{"namespace":"demo","name":"cm","kind":"ConfigMap","op":"create"}]}],` +
		`"Lines":["Target cluster 'https://cp1:6443'","kapp: Error: Applying create configmap/cm: forbidden"]}`

	s := newPluginsSetup(t, "v1.36.0", true, []string{"alpha"}, nil, false)
	s.shell.files = map[string]string{"kapp-diff-alpha.json": withError}
	s.shell.out = "last line\n"
	s.shell.code = 1

	sections, err := s.plugins.Diff()

	require.NoError(t, err)
	require.Equal(t,
		"Plugin kustomize/alpha Failed: scripts/diff.sh exited 1: kapp: Error: Applying create configmap/cm: forbidden",
		describe(sections))
}

// A release that is not compared does not stop the comparison of the releases after it.
func TestPluginsDiff_ReleaseAfterANotComparedOneIsStillCompared(t *testing.T) {
	t.Parallel()

	s := newPluginsSetup(t, "v1.36.0", true, nil, []string{"poc-a", "poc-b"}, false)
	s.helmf.stdout = "Comparing release=poc-a, chart=podinfo/podinfo\nPlan: 1 to add, 0 to change, 0 to destroy.\n" +
		"Comparing release=poc-b, chart=podinfo/podinfo\n" +
		"poc, poc-b, ConfigMap (v1) to be added.\nPlan: 1 to add, 0 to change, 0 to destroy.\n"

	sections, err := s.plugins.Diff()

	require.NoError(t, err)
	require.Equal(t,
		"Plugin helm/poc-a Failed: helmfile diff output did not match the expected format; "+
			"Plugin helm/poc-b Compared with 1 changes",
		describe(sections))
}

// An invalid kapp file without a kapp error does not hide the kapp error of a later file from the hint.
func TestPluginsDiff_InvalidFileBeforeAKappErrorKeepsTheKappErrorHint(t *testing.T) {
	t.Parallel()

	e := fixtureText(t, kappFixtureDir, "kapp-0.66.1-e-unreachable.json")
	kappErr := `kapp: Error: Getting app: Get "https://192.168.200.250:6999/api/v1/namespaces/kube-system/configmaps/kfd.apps.k14s.io": ` +
		`dial tcp 192.168.200.250:6999: connect: connection refused`

	s := newPluginsSetup(t, "v1.36.0", true, []string{"alpha", "beta"}, nil, false)
	s.shell.files = map[string]string{"kapp-diff-alpha.json": "", "kapp-diff-beta.json": e}
	s.shell.out = "last line\n"
	s.shell.code = 1

	sections, err := s.plugins.Diff()

	require.NoError(t, err)
	require.Equal(t,
		"Plugin kustomize/alpha Failed: scripts/diff.sh exited 1: "+kappErr+"; "+
			"Plugin kustomize/beta Failed: scripts/diff.sh exited 1: "+kappErr,
		describe(sections))
}
