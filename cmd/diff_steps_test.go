// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package cmd //nolint:testpackage // exercises the unexported steps of the command.

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/apis/config"
	commcreate "github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/common/create"
	"github.com/sighupio/furyctl/internal/cluster"
	dist "github.com/sighupio/furyctl/pkg/distribution"
)

var (
	errPreflightSSH  = errors.New("ssh: connection refused")
	errDownloadsFail = errors.New("cannot download")
)

// fakeCreator is the creator of a kind that cannot be compared, like EKSCluster.
type fakeCreator struct{}

func (fakeCreator) SetProperties([]cluster.CreatorProperty) {}
func (fakeCreator) SetProperty(string, any)                 {}
func (fakeCreator) Create(string, int, int) error           { return nil }
func (fakeCreator) GetPhasePath(string) (string, error)     { return "", nil }

// fakeDiffer is the creator of a kind that implements Diff().
type fakeDiffer struct {
	fakeCreator

	diff func(afterPreflight func()) ([]commcreate.Section, error)
}

func (f fakeDiffer) Diff(afterPreflight func()) ([]commcreate.Section, error) {
	return f.diff(afterPreflight)
}

type stepsRecorder struct {
	steps   []string
	creator cluster.Creator
	config  func() configurationResult
	out     bytes.Buffer
}

func (r *stepsRecorder) record(step string) { r.steps = append(r.steps, step) }

func (r *stepsRecorder) diffSteps() diffSteps {
	return diffSteps{
		downloadDistribution: func() (dist.DownloadResult, error) {
			r.record("distribution download")

			return dist.DownloadResult{}, nil
		},
		lock: func(dist.DownloadResult) (func(), error) {
			r.record("lock taken")

			return func() { r.record("lock removed") }, nil
		},
		downloadDependencies: func(dist.DownloadResult) error {
			r.record("dependency download")

			return nil
		},
		newCreator: func(dist.DownloadResult) (cluster.Creator, error) {
			r.record("creator built")

			return r.creator, nil
		},
		compareConfiguration: func(dist.DownloadResult) configurationResult {
			r.record("configuration comparison")

			return r.config()
		},
		outOfScope: func(dist.DownloadResult) []commcreate.Section {
			return commcreate.OutOfScopeSections(nil, false, reasonEKSNotSupported)
		},
		notCompared: func(_ dist.DownloadResult, reason string) []commcreate.Section {
			return commcreate.FailedSections("", nil, false, reason)
		},
		print: func(text string) {
			r.record("report printed")

			r.out.WriteString(text)
		},
	}
}

func noChanges() configurationResult { return configurationResult{} }

// The steps run in the order of apply, and the configuration is compared right after the preflight attempt.
func TestRunFullDiffRunsItsStepsInOrder(t *testing.T) {
	t.Parallel()

	r := &stepsRecorder{config: noChanges}
	r.creator = fakeDiffer{diff: func(afterPreflight func()) ([]commcreate.Section, error) {
		r.record("creator Diff")
		r.record("preflight attempt")
		afterPreflight()
		r.record("distribution Render")

		return []commcreate.Section{{Title: "Distribution", Status: commcreate.StatusCompared}}, nil
	}}

	require.NoError(t, runFullDiff(r.diffSteps()))

	require.Equal(t, []string{
		"distribution download", "lock taken", "dependency download", "creator built", "creator Diff",
		"preflight attempt", "configuration comparison", "distribution Render", "report printed", "lock removed",
	}, r.steps)
}

// A Diff error after the preflight attempt still prints the Configuration section, compared once, then the error
// as the reason the resources were not compared.
func TestRunFullDiffPrintsTheConfigurationWhenDiffFailsAfterThePreflight(t *testing.T) {
	t.Parallel()

	r := &stepsRecorder{config: func() configurationResult {
		return configurationResult{Lines: []string{".spec.kubernetes.podCidr: 10.0.0.0/16 -> 10.1.0.0/16"}}
	}}
	r.creator = fakeDiffer{diff: func(afterPreflight func()) ([]commcreate.Section, error) {
		afterPreflight()

		return nil, errDownloadsFail
	}}

	err := runFullDiff(r.diffSteps())

	require.ErrorIs(t, err, errDownloadsFail)
	require.ErrorIs(t, err, ErrDiffNotCompared)
	require.Equal(t, []string{
		"distribution download", "lock taken", "dependency download", "creator built", "configuration comparison",
		"report printed", "lock removed",
	}, r.steps)
	require.Contains(t, r.out.String(), "Configuration\n  .spec.kubernetes.podCidr: 10.0.0.0/16 -> 10.1.0.0/16\n\n"+
		"Distribution: not compared: error while comparing the resources: cannot download\n\n")
}

func TestRunFullDiffEKSClusterHasNoCreatorDiff(t *testing.T) {
	t.Parallel()

	r := &stepsRecorder{config: noChanges, creator: fakeCreator{}}

	require.NoError(t, runFullDiff(r.diffSteps()))

	require.Equal(t, []string{
		"distribution download", "lock taken", "dependency download", "creator built", "configuration comparison",
		"report printed", "lock removed",
	}, r.steps)
}

// The distribution download and the lock come before the run dir is touched: their error stops diff with no report.
func TestRunFullDiffStopsAtTheFirstStepThatReturnsAnError(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		fail      func(*diffSteps)
		wantSteps []string
	}{
		"distribution download": {
			fail: func(s *diffSteps) {
				s.downloadDistribution = func() (dist.DownloadResult, error) { return dist.DownloadResult{}, errDownloadsFail }
			},
			wantSteps: nil,
		},
		"lock": {
			fail: func(s *diffSteps) {
				s.lock = func(dist.DownloadResult) (func(), error) { return nil, errDownloadsFail }
			},
			wantSteps: []string{"distribution download"},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			r := &stepsRecorder{config: noChanges, creator: fakeCreator{}}
			steps := r.diffSteps()

			tc.fail(&steps)

			err := runFullDiff(steps)

			require.ErrorIs(t, err, errDownloadsFail)
			require.NotErrorIs(t, err, ErrDiffNotCompared)
			require.Empty(t, r.out.String())
			require.Equal(t, tc.wantSteps, r.steps)
		})
	}
}

// A step that fails after the lock still prints the report: the CONFIG table first, then the error as the reason
// the resources were not compared. diff exits with 1 without logging the error, which the report already holds.
func TestRunFullDiffPrintsTheConfigurationFirstWhenALaterStepFails(t *testing.T) {
	t.Parallel()

	errValidation := errors.New("error while validating configuration file: spec.kubernetes: missing property")
	errRender := errors.New("error while rendering config: template: bad")

	tests := map[string]struct {
		fail       func(*stepsRecorder, *diffSteps)
		cause      error
		wantReason string
		wantSteps  []string
	}{
		"dependency download": {
			fail: func(_ *stepsRecorder, s *diffSteps) {
				s.downloadDependencies = func(dist.DownloadResult) error { return errDownloadsFail }
			},
			cause:      errDownloadsFail,
			wantReason: "cannot download",
			wantSteps:  []string{"distribution download", "lock taken", "configuration comparison", "report printed", "lock removed"},
		},
		"configuration validation": {
			fail: func(_ *stepsRecorder, s *diffSteps) {
				s.downloadDependencies = func(dist.DownloadResult) error { return errValidation }
			},
			cause:      errValidation,
			wantReason: errValidation.Error(),
			wantSteps:  []string{"distribution download", "lock taken", "configuration comparison", "report printed", "lock removed"},
		},
		"creator setup": {
			fail: func(_ *stepsRecorder, s *diffSteps) {
				s.newCreator = func(dist.DownloadResult) (cluster.Creator, error) { return nil, errDownloadsFail }
			},
			cause:      errDownloadsFail,
			wantReason: "cannot download",
			wantSteps: []string{
				"distribution download", "lock taken", "dependency download", "configuration comparison", "report printed",
				"lock removed",
			},
		},
		"config render, before the preflight": {
			fail: func(r *stepsRecorder, _ *diffSteps) {
				r.creator = fakeDiffer{diff: func(func()) ([]commcreate.Section, error) { return nil, errRender }}
			},
			cause:      errRender,
			wantReason: "error while comparing the resources: " + errRender.Error(),
			wantSteps: []string{
				"distribution download", "lock taken", "dependency download", "creator built", "configuration comparison",
				"report printed", "lock removed",
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			r := &stepsRecorder{config: func() configurationResult {
				return configurationResult{Table: "CONFIG   1 change\n\nFIELD   CURRENT   NEW\n"}
			}, creator: fakeCreator{}}
			steps := r.diffSteps()

			tc.fail(r, &steps)

			err := runFullDiff(steps)

			require.ErrorIs(t, err, tc.cause)
			require.ErrorIs(t, err, ErrDiffNotCompared)
			require.Equal(t, tc.wantSteps, r.steps)
			require.Equal(t, reportHeader+"\n\n"+
				"CONFIG   1 change\nFIELD   CURRENT   NEW\n\n"+
				"Distribution: not compared: "+tc.wantReason+"\n\n"+
				"Plan: not available (some sections were not compared).\n", r.out.String())
		})
	}
}

// The preflight and the render fail inside Diff: the sections say so, after the CONFIG table.
func TestRunFullDiffPrintsTheConfigurationBeforeAPreflightOrRenderFailure(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"preflight": "preflight failed: ssh: connection refused",
		"render":    "render failed: error generating from template files",
	}

	for name, reason := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			r := &stepsRecorder{config: func() configurationResult {
				return configurationResult{Table: "CONFIG   no changes\n"}
			}}
			r.creator = fakeDiffer{diff: func(afterPreflight func()) ([]commcreate.Section, error) {
				afterPreflight()

				return []commcreate.Section{{Title: "Distribution", Status: commcreate.StatusFailed, Reason: reason}}, nil
			}}

			err := runFullDiff(r.diffSteps())

			require.ErrorIs(t, err, ErrDiffNotCompared)

			out := r.out.String()
			table := strings.Index(out, "CONFIG   no changes")

			require.Positive(t, table)
			require.Less(t, table, strings.Index(out, "Distribution: not compared: "+reason))
		})
	}
}

// The configuration is compared with the kubeconfig that the preflight left, whatever its outcome.
func TestRunFullDiffConfigurationSectionFollowsThePreflightAttempt(t *testing.T) {
	tests := []struct {
		name       string
		setsKube   string
		failure    string
		userKube   string
		wantKube   string
		wantReport string
	}{
		{
			"the preflight sets the kubeconfig and fails",
			"/work/.furyctl/lab/admin.conf",
			"changes to immutable fields detected",
			"/root/.kube/config",
			"/work/.furyctl/lab/admin.conf",
			"Configuration\n  .spec.kubernetes.podCidr: 10.0.0.0/16 -> 10.1.0.0/16\n\n" +
				"Distribution: not compared: preflight failed: changes to immutable fields detected\n\n",
		},
		{
			"the preflight fails before it sets the kubeconfig",
			"",
			errPreflightSSH.Error(),
			"/root/.kube/config",
			"/root/.kube/config",
			"Configuration\n  .spec.kubernetes.podCidr: 10.0.0.0/16 -> 10.1.0.0/16\n\n" +
				"Distribution: not compared: preflight failed: ssh: connection refused\n\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("KUBECONFIG", tc.userKube)

			usedKube := ""
			r := &stepsRecorder{}
			r.creator = fakeDiffer{diff: func(afterPreflight func()) ([]commcreate.Section, error) {
				if tc.setsKube != "" {
					t.Setenv("KUBECONFIG", tc.setsKube)
				}

				afterPreflight()

				return failedSections(tc.failure), nil
			}}
			r.config = func() configurationResult {
				usedKube = os.Getenv("KUBECONFIG")

				return configurationResult{Lines: []string{".spec.kubernetes.podCidr: 10.0.0.0/16 -> 10.1.0.0/16"}}
			}

			err := runFullDiff(r.diffSteps())

			require.ErrorIs(t, err, ErrDiffNotCompared)
			require.Equal(t, tc.wantKube, usedKube)
			require.Contains(t, r.out.String(), tc.wantReport)
			require.True(t, strings.HasSuffix(r.out.String(), "Plan: not available (some sections were not compared).\n"))
		})
	}
}

func failedSections(reason string) []commcreate.Section {
	return []commcreate.Section{{Title: "Distribution", Status: commcreate.StatusFailed, Reason: "preflight failed: " + reason}}
}

// An EKSCluster has its resource sections out of scope and exits with 0.
func TestRunFullDiffEKSClusterGivesOutOfScopeSectionsAndExitZero(t *testing.T) {
	t.Parallel()

	r := &stepsRecorder{creator: fakeCreator{}, config: func() configurationResult {
		return configurationResult{Lines: []string{".spec.distribution.modules.ingress.nginx.type: single -> dual"}}
	}}

	require.NoError(t, runFullDiff(r.diffSteps()))

	require.Contains(t, r.out.String(), "\n\nDistribution: not compared: EKSCluster is not supported by furyctl diff yet\n\n")
	require.True(t, strings.HasSuffix(r.out.String(), "\n\nPlan: nothing was compared.\n"))
}

// The logs of the run come before the report and none comes inside it.
func TestRunFullDiffLogsDoNotSeparateTheReport(t *testing.T) {
	combined := &bytes.Buffer{}

	logrus.SetOutput(combined)
	logrus.SetLevel(logrus.InfoLevel)
	logrus.SetFormatter(&logrus.TextFormatter{DisableTimestamp: true, DisableColors: true})

	t.Cleanup(func() { logrus.SetOutput(os.Stderr) })

	r := &stepsRecorder{config: noChanges}
	r.creator = fakeDiffer{diff: func(afterPreflight func()) ([]commcreate.Section, error) {
		logrus.Info("PLAY RECAP *** ansible output")
		afterPreflight()

		return []commcreate.Section{{Title: "Distribution", Status: commcreate.StatusCompared}}, nil
	}}

	steps := r.diffSteps()
	steps.print = func(text string) { combined.WriteString(text) }

	require.NoError(t, runFullDiff(steps))

	out := combined.String()
	header := strings.Index(out, reportHeader)

	require.Positive(t, header)
	require.Less(t, strings.Index(out, "PLAY RECAP"), header)
	require.NotContains(t, out[header:], "level=")
}

// The creator is built with the apply defaults.
func TestNewApplyDefaultsCreator(t *testing.T) {
	t.Parallel()

	var got struct {
		phase                  string
		skipVpn, vpnAutoConnct bool
		skipNodesUpgrade       bool
		dryRun                 bool
		force                  []string
		upgrade                bool
		upgradePath, node      string
		postApply              []string
		paths                  cluster.CreatorPaths
	}

	factory := func(
		_ config.Furyctl, _ config.KFD, paths cluster.CreatorPaths, phase string,
		skipVpn, vpnAutoConnect, skipNodesUpgrade, dryRun bool, force []string, upgrade bool,
		upgradePath, node string, postApply []string,
	) (cluster.Creator, error) {
		got.phase, got.skipVpn, got.vpnAutoConnct, got.skipNodesUpgrade = phase, skipVpn, vpnAutoConnect, skipNodesUpgrade
		got.dryRun, got.force, got.upgrade, got.upgradePath, got.node, got.postApply = dryRun, force, upgrade, upgradePath, node, postApply
		got.paths = paths

		return fakeCreator{}, nil
	}

	dir := t.TempDir()
	flags := DiffCommandFlags{
		FuryctlPath: filepath.Join(dir, "furyctl.yaml"), BinPath: "/bin-path", Outdir: dir,
		Phase: "distribution", UpgradePathLocation: "/upgrades",
	}

	_, err := newApplyDefaultsCreator(factory, flags, dist.DownloadResult{RepoPath: "/distro", MinimalConf: config.Furyctl{}})

	require.NoError(t, err)
	require.Equal(t, "distribution", got.phase)
	require.False(t, got.skipVpn || got.vpnAutoConnct || got.skipNodesUpgrade || got.dryRun || got.upgrade)
	require.Empty(t, got.force)
	require.Empty(t, got.node)
	require.Empty(t, got.postApply)
	require.Equal(t, "/upgrades", got.upgradePath)
	require.Equal(t, cluster.CreatorPaths{
		ConfigPath: flags.FuryctlPath,
		WorkDir:    filepath.Join(dir, ".furyctl"),
		DistroPath: "/distro",
		BinPath:    "/bin-path",
	}, got.paths)
}

func TestEKSOutOfScopeSectionsFollowThePluginsOfTheConfiguration(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "furyctl.yaml")

	require.NoError(t, os.WriteFile(path, []byte(
		"kind: EKSCluster\nspec:\n  plugins:\n    kustomize:\n      - name: alpha\n    helm:\n      releases:\n        - name: podinfo\n",
	), os.FileMode(0o600)))

	sections := eksOutOfScopeSections(path, true)

	titles := make([]string, 0, len(sections))
	for _, s := range sections {
		titles = append(titles, fmt.Sprintf("%s %s: %s", s.Title, s.Status, s.Reason))
	}

	reason := "EKSCluster is not supported by furyctl diff yet"

	require.Equal(t, []string{
		"Distribution OutOfScope: " + reason, "Plugins (kustomize) OutOfScope: " + reason, "Plugins (helm) OutOfScope: " + reason,
	}, titles)
}

// The sections a failed step reports follow the plugins of the configuration file and the phase.
func TestNotComparedSectionsFollowThePluginsOfTheConfiguration(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "furyctl.yaml")

	require.NoError(t, os.WriteFile(path, []byte(
		"kind: OnPremises\nspec:\n  plugins:\n    kustomize:\n      - name: alpha\n    helm:\n      releases:\n        - name: podinfo\n",
	), os.FileMode(0o600)))

	describe := func(sections []commcreate.Section) []string {
		out := make([]string, 0, len(sections))
		for _, s := range sections {
			out = append(out, fmt.Sprintf("%s %s: %s", s.Title, s.Status, s.Reason))
		}

		return out
	}

	require.Equal(t, []string{
		"Distribution Failed: boom", "Plugins (kustomize) Failed: boom", "Plugins (helm) Failed: boom",
	}, describe(notComparedSections(path, "", true, "boom")))
	require.Equal(t, []string{"Distribution Failed: boom"}, describe(notComparedSections(path, "distribution", true, "boom")))
	require.Equal(t, []string{"Distribution Failed: boom"}, describe(notComparedSections(path+".missing", "", true, "boom")))
}
