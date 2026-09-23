// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

//nolint:testpackage // miseToolsForKind and materializeTool are unexported.
package dependencies

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/apis/config"
)

func tool(v string) config.KFDTool { return config.KFDTool{Version: v} }

func Test_miseToolsForKind(t *testing.T) {
	t.Parallel()

	// New layout: opentofu/furyagent under eks. Old layout: under common.
	newLayout := config.KFD{
		Tools: config.KFDTools{
			Common: config.KFDToolsCommon{Kubectl: tool("1.34.4"), Kustomize: tool("5.6.0")},
			Eks:    config.KFDToolsEks{Awscli: tool("2.8.12"), OpenTofu: tool("1.10.0"), Furyagent: tool("0.4.0")},
		},
	}
	oldLayout := config.KFD{
		Tools: config.KFDTools{
			Common: config.KFDToolsCommon{
				Kubectl: tool("1.34.4"), Kustomize: tool("5.6.0"),
				OpenTofu: tool("1.10.0"), Furyagent: tool("0.4.0"),
			},
			Eks: config.KFDToolsEks{Awscli: tool("2.8.12")},
		},
	}
	// Distribution that pins ansible per provider — only OnPremises/Immutable should pick it up.
	ansibleLayout := config.KFD{
		Tools: config.KFDTools{
			Common:     config.KFDToolsCommon{Kubectl: tool("1.34.4"), Kustomize: tool("5.6.0")},
			OnPremises: config.KFDToolsOnPremises{Ansible: config.KFDToolAnsible{Version: "2.21.0"}},
			Immutable:  config.KFDToolsImmutable{Ansible: config.KFDToolAnsible{Version: "2.21.0"}},
		},
	}

	testCases := []struct {
		desc        string
		kfd         config.KFD
		kind        string
		wantManaged map[string]string
		wantUts     []string
	}{
		{
			desc:        "EKS new layout: eks tools managed, awscli is host",
			kfd:         newLayout,
			kind:        "EKSCluster",
			wantManaged: map[string]string{"kubectl": "1.34.4", "kustomize": "5.6.0", "opentofu": "1.10.0", "furyagent": "0.4.0"},
			wantUts:     []string{"awscli"},
		},
		{
			desc:        "OnPremises new layout: only common tools, no eks tools",
			kfd:         newLayout,
			kind:        "OnPremises",
			wantManaged: map[string]string{"kubectl": "1.34.4", "kustomize": "5.6.0"},
			wantUts:     []string{},
		},
		{
			desc:        "OnPremises old layout: opentofu/furyagent resolved from common (backward compat)",
			kfd:         oldLayout,
			kind:        "OnPremises",
			wantManaged: map[string]string{"kubectl": "1.34.4", "kustomize": "5.6.0", "opentofu": "1.10.0", "furyagent": "0.4.0"},
			wantUts:     []string{},
		},
		{
			desc:        "OnPremises with ansible pinned: ansible is mise-managed",
			kfd:         ansibleLayout,
			kind:        "OnPremises",
			wantManaged: map[string]string{"kubectl": "1.34.4", "kustomize": "5.6.0", "ansible": "2.21.0"},
			wantUts:     []string{},
		},
		{
			desc:        "Immutable with ansible pinned: ansible is mise-managed",
			kfd:         ansibleLayout,
			kind:        "Immutable",
			wantManaged: map[string]string{"kubectl": "1.34.4", "kustomize": "5.6.0", "ansible": "2.21.0"},
			wantUts:     []string{},
		},
		{
			desc:        "EKSCluster with ansible pinned: ansible NOT managed (not used by this kind)",
			kfd:         ansibleLayout,
			kind:        "EKSCluster",
			wantManaged: map[string]string{"kubectl": "1.34.4", "kustomize": "5.6.0"},
			wantUts:     []string{},
		},
		{
			desc:        "OnPremises without ansible pinned: stays host (backward compat)",
			kfd:         newLayout,
			kind:        "OnPremises",
			wantManaged: map[string]string{"kubectl": "1.34.4", "kustomize": "5.6.0"},
			wantUts:     []string{},
		},
	}

	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			managed, uts := miseToolsForKind(tC.kfd, tC.kind)

			assert.Equal(t, tC.wantManaged, managed, "managed")

			for k, v := range tC.wantManaged {
				assert.Equal(t, v, managed[k], "managed[%s]", k)
			}

			slices.Sort(uts)
			slices.Sort(tC.wantUts)

			assert.ElementsMatch(t, tC.wantUts, uts, "uts")
		})
	}
}

// Test_materializeToolThroughSymlinkedBinPath reproduces /tmp -> /private/tmp on macOS: binPath is a
// symlink and the target is a physical path, as filepath.EvalSymlinks returns it.
func Test_materializeToolThroughSymlinkedBinPath(t *testing.T) {
	t.Parallel()

	physical := t.TempDir()
	logical := filepath.Join(t.TempDir(), "bin")
	require.NoError(t, os.Symlink(physical, logical))

	venv := filepath.Join(physical, "mise", "installs", "ansible-core")
	require.NoError(t, os.MkdirAll(venv, 0o755))

	require.NoError(t, materializeTool(logical, "ansible", "2.20.0", "venv", venv))

	link := filepath.Join(logical, "ansible", "2.20.0", "venv")

	got, err := filepath.EvalSymlinks(link)
	require.NoError(t, err)
	assert.Equal(t, physicalPath(venv), got)

	target, err := os.Readlink(link)
	require.NoError(t, err)
	assert.False(t, filepath.IsAbs(target), "expected a relative link, got %q", target)
}
