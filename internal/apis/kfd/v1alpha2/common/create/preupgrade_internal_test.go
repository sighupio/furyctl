// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package create

import (
	"os"
	"path"
	"strings"
	"testing"

	"github.com/sighupio/furyctl/internal/apis/config"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/distribution"
)

// TestUpgradePathIncludesWorkerNodes runs against templates that hold the two shapes of the
// call. OnPremises gives the playbook a numeric prefix and Immutable does not, and the
// detection must find the worker upgrade in both.
func TestUpgradePathIncludesWorkerNodes(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		template string
		expected bool
	}{
		{
			name:     "onpremises calls the playbook with a numeric prefix",
			template: `{{ $.paths.ansiblePlaybook }} 56.upgrade-worker-nodes.yml --limit "{{ $h.name }}"`,
			expected: true,
		},
		{
			name:     "immutable calls the playbook without a prefix",
			template: "{{ .paths.ansiblePlaybook }} upgrade-worker-nodes.yml --become",
			expected: true,
		},
		{
			name:     "a path that upgrades no worker",
			template: "{{ .paths.ansiblePlaybook }} upgrade-control-plane.yml --become",
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			upgradesPath := t.TempDir()
			transition := "1.0.0-1.1.0"

			if err := os.Mkdir(path.Join(upgradesPath, transition), 0o755); err != nil {
				t.Fatalf("error creating the transition folder: %v", err)
			}

			templatePath := path.Join(upgradesPath, transition, "pre-kubernetes.sh.tpl")
			if err := os.WriteFile(templatePath, []byte(tc.template), 0o600); err != nil {
				t.Fatalf("error writing the template: %v", err)
			}

			got, err := upgradePathIncludesWorkerNodes(upgradesPath, transition)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, got)
			}
		})
	}
}

// TestUpgradePathIncludesWorkerNodesWithoutTemplate covers an upgrade path that gives no
// pre-kubernetes template. Many paths give only a pre-distribution one.
func TestUpgradePathIncludesWorkerNodesWithoutTemplate(t *testing.T) {
	t.Parallel()

	got, err := upgradePathIncludesWorkerNodes(t.TempDir(), "1.0.0-1.1.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got {
		t.Error("expected false when the template is absent")
	}
}

// TestNewPreUpgradeUsesTheAnsibleOfTheKind checks that the upgrade scripts get the ansible that
// furyctl installs for the kind, and not the ansible of the other kind.
func TestNewPreUpgradeUsesTheAnsibleOfTheKind(t *testing.T) {
	t.Parallel()

	tools := config.KFDTools{
		OnPremises: config.KFDToolsOnPremises{Ansible: config.KFDToolAnsible{Version: "2.20.0"}},
		Immutable:  config.KFDToolsImmutable{Ansible: config.KFDToolAnsible{Version: "2.21.0"}},
	}

	testCases := []struct {
		kind    string
		version string
	}{
		{kind: distribution.OnPremisesKind, version: "2.20.0"},
		{kind: distribution.ImmutableKind, version: "2.21.0"},
	}

	for _, tc := range testCases {
		t.Run(tc.kind, func(t *testing.T) {
			t.Parallel()

			p := NewPreUpgrade(
				cluster.CreatorPaths{BinPath: "/bin"},
				config.KFD{Tools: tools},
				tc.kind,
				false, false, nil, nil, nil, nil, "", false,
			)

			want := path.Join("/bin", "ansible", tc.version, "venv", "bin", "python")
			if got := p.AnsiblePlaybookCmd(); !strings.Contains(got, want) {
				t.Errorf("AnsiblePlaybookCmd() = %q, want it to contain %q", got, want)
			}
		})
	}
}
