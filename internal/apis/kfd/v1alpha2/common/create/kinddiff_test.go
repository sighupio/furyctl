// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package create_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/common/create"
)

var errBlocked = errors.New("changes to immutable fields detected:\n    .spec.kubernetes.podCidr")

func renderedWithPlugins() map[string]any {
	return map[string]any{
		"spec": map[string]any{
			"plugins": map[string]any{
				"kustomize": []any{map[string]any{"name": "alpha"}},
				"helm":      map[string]any{"releases": []any{map[string]any{"name": "poc-podinfo"}}},
			},
		},
	}
}

type recorder struct {
	calls     []string
	exists    bool
	preErr    error
	renderErr error
}

func (r *recorder) input(phase string) create.KindDiff {
	return create.KindDiff{
		Phase:          phase,
		RenderedConfig: renderedWithPlugins(),
		PluginsFeature: true,
		Preflight: func() (bool, error) {
			r.calls = append(r.calls, "preflight")

			return r.exists, r.preErr
		},
		RenderDistribution: func() error {
			r.calls = append(r.calls, "distribution Render")

			return r.renderErr
		},
		DiffDistribution: func() create.Section {
			r.calls = append(r.calls, "distribution diff")

			return create.Section{Title: "Distribution", Status: create.StatusCompared}
		},
		DiffPlugins: func() ([]create.Section, error) {
			r.calls = append(r.calls, "plugins Diff")

			return []create.Section{{Title: "Plugin helm/poc-podinfo", Status: create.StatusCompared}}, nil
		},
	}
}

func describeSections(sections []create.Section) string {
	parts := make([]string, 0, len(sections))

	for _, s := range sections {
		parts = append(parts, s.Title+" "+string(s.Status)+": "+s.Reason)
	}

	return strings.Join(parts, "; ")
}

func TestRunDiff_OnlyPreflightRenderAndDiffsRun(t *testing.T) {
	t.Parallel()

	r := &recorder{exists: true}

	sections := create.RunDiff(r.input(""))

	require.Equal(t, []string{"preflight", "distribution Render", "distribution diff", "plugins Diff"}, r.calls)
	require.Equal(t, "Distribution Compared: ; Plugin helm/poc-podinfo Compared: ", describeSections(sections))
}

func TestRunDiff_PreflightErrorFailsEveryResourceSection(t *testing.T) {
	t.Parallel()

	r := &recorder{preErr: errBlocked}

	sections := create.RunDiff(r.input(""))

	reason := "preflight failed: changes to immutable fields detected: .spec.kubernetes.podCidr"

	require.Equal(t,
		"Distribution Failed: "+reason+"; Plugins (kustomize) Failed: "+reason+"; Plugins (helm) Failed: "+reason,
		describeSections(sections))
	require.Equal(t, []string{"preflight"}, r.calls)
}

func TestRunDiff_NoClusterRendersNothing(t *testing.T) {
	t.Parallel()

	r := &recorder{exists: false}

	sections := create.RunDiff(r.input(""))

	require.Equal(t,
		"Distribution Failed: no cluster found; Plugins (kustomize) Failed: no cluster found; Plugins (helm) Failed: no cluster found",
		describeSections(sections))
	require.Equal(t, []string{"preflight"}, r.calls)
}

func TestRunDiff_PhasesWithoutPlan(t *testing.T) {
	t.Parallel()

	for _, phase := range []string{"infrastructure", "kubernetes"} {
		t.Run(phase, func(t *testing.T) {
			t.Parallel()

			r := &recorder{exists: true}

			sections := create.RunDiff(r.input(phase))

			reason := "no plan available for this phase"

			require.Equal(t,
				"Distribution OutOfScope: "+reason+"; Plugins (kustomize) OutOfScope: "+reason+"; Plugins (helm) OutOfScope: "+reason,
				describeSections(sections))
			require.Empty(t, r.calls)
		})
	}
}

func TestRunDiff_PhaseFlagLimitsTheComparedSections(t *testing.T) {
	t.Parallel()

	tests := map[string][]string{
		"distribution": {"preflight", "distribution Render", "distribution diff"},
		"plugins":      {"preflight", "plugins Diff"},
		"":             {"preflight", "distribution Render", "distribution diff", "plugins Diff"},
	}

	for phase, calls := range tests {
		t.Run("phase "+phase, func(t *testing.T) {
			t.Parallel()

			r := &recorder{exists: true}

			create.RunDiff(r.input(phase))

			require.Equal(t, calls, r.calls)
		})
	}
}

func TestRunDiff_NoPluginsFeatureNoPluginsSection(t *testing.T) {
	t.Parallel()

	r := &recorder{exists: true}
	in := r.input("")
	in.PluginsFeature = false

	sections := create.RunDiff(in)

	require.Equal(t, "Distribution Compared: ", describeSections(sections))
	require.Equal(t, []string{"preflight", "distribution Render", "distribution diff"}, r.calls)
}

func TestRunDiff_RenderErrorFailsTheDistributionOnly(t *testing.T) {
	t.Parallel()

	r := &recorder{exists: true, renderErr: errors.New("bad\n  template")}

	sections := create.RunDiff(r.input(""))

	require.Equal(t, "Distribution Failed: render failed: bad template; Plugin helm/poc-podinfo Compared: ", describeSections(sections))
	require.NotContains(t, r.calls, "distribution diff")
}

func TestRunDiff_NoPluginsSectionWithoutThePluginsFeature(t *testing.T) {
	t.Parallel()

	for name, setup := range map[string]func(*recorder){
		"cluster does not exist": func(r *recorder) { r.exists = false },
		"preflight fails":        func(r *recorder) { r.preErr = errors.New("cannot reach the cluster") },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			r := &recorder{}
			setup(r)

			in := r.input("")
			in.PluginsFeature = false

			sections := create.RunDiff(in)

			require.Len(t, sections, 1)
			require.Equal(t, "Distribution", sections[0].Title)
			require.Equal(t, create.StatusFailed, sections[0].Status)
			require.Equal(t, []string{"preflight"}, r.calls)
		})
	}
}

func TestOutOfScopeSections(t *testing.T) {
	t.Parallel()

	reason := "EKSCluster is not supported by furyctl diff yet"

	tests := []struct {
		name     string
		rendered map[string]any
		plugins  bool
		want     string
	}{
		{
			"kustomize plugins and helm releases", renderedWithPlugins(), true,
			"Distribution OutOfScope: " + reason + "; Plugins (kustomize) OutOfScope: " + reason + "; Plugins (helm) OutOfScope: " + reason,
		},
		{"no plugins in the configuration", map[string]any{"spec": map[string]any{}}, true, "Distribution OutOfScope: " + reason},
		{"no plugins feature", renderedWithPlugins(), false, "Distribution OutOfScope: " + reason},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tc.want, describeSections(create.OutOfScopeSections(tc.rendered, tc.plugins, reason)))
		})
	}
}

// A step that fails before the kind compares anything fails the sections of the phase, or every section for a phase
// with no plan to show.
func TestFailedSections(t *testing.T) {
	t.Parallel()

	reason := "error while validating configuration file"

	tests := []struct {
		name    string
		phase   string
		plugins bool
		want    string
	}{
		{
			"every phase", "", true,
			"Distribution Failed: " + reason + "; Plugins (kustomize) Failed: " + reason + "; Plugins (helm) Failed: " + reason,
		},
		{"distribution phase", "distribution", true, "Distribution Failed: " + reason},
		{"plugins phase", "plugins", true, "Plugins (kustomize) Failed: " + reason + "; Plugins (helm) Failed: " + reason},
		{"plugins phase without the plugins feature", "plugins", false, "Distribution Failed: " + reason},
		{
			"phase with no plan", "kubernetes", true,
			"Distribution Failed: " + reason + "; Plugins (kustomize) Failed: " + reason + "; Plugins (helm) Failed: " + reason,
		},
		{"no plugins feature", "", false, "Distribution Failed: " + reason},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tc.want, describeSections(create.FailedSections(tc.phase, renderedWithPlugins(), tc.plugins, reason)))
		})
	}
}

// afterPreflight runs once, right after the preflight attempt, whatever its outcome.
func TestRunDiff_AfterPreflightRunsOnceRightAfterThePreflightAttempt(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		phase string
		r     *recorder
		calls []string
	}{
		"preflight succeeds": {
			"", &recorder{exists: true},
			[]string{"preflight", "afterPreflight", "distribution Render", "distribution diff", "plugins Diff"},
		},
		"preflight fails":            {"", &recorder{preErr: errBlocked}, []string{"preflight", "afterPreflight"}},
		"preflight finds no cluster": {"", &recorder{exists: false}, []string{"preflight", "afterPreflight"}},
		"phase without a plan":       {"kubernetes", &recorder{exists: true}, []string{"afterPreflight"}},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			in := tc.r.input(tc.phase)
			in.AfterPreflight = func() { tc.r.calls = append(tc.r.calls, "afterPreflight") }

			create.RunDiff(in)

			require.Equal(t, tc.calls, tc.r.calls)
		})
	}
}
