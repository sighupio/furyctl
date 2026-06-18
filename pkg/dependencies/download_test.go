// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

//nolint:testpackage // miseToolsForKind is unexported.
package dependencies

import (
	"slices"
	"testing"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
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
	}

	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			managed, uts := miseToolsForKind(tC.kfd, tC.kind)

			if len(managed) != len(tC.wantManaged) {
				t.Errorf("managed = %v, want %v", managed, tC.wantManaged)
			}

			for k, v := range tC.wantManaged {
				if managed[k] != v {
					t.Errorf("managed[%s] = %q, want %q", k, managed[k], v)
				}
			}

			slices.Sort(uts)
			slices.Sort(tC.wantUts)

			if !slices.Equal(uts, tC.wantUts) {
				t.Errorf("uts = %v, want %v", uts, tC.wantUts)
			}
		})
	}
}
