// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package reducers_test

import (
	"testing"

	r3diff "github.com/r3labs/diff/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/pkg/diffs"
	"github.com/sighupio/furyctl/pkg/reducers"
	rules "github.com/sighupio/furyctl/pkg/rulesextractor"
)

// kubeProxyExtractor returns a BaseExtractor with a single kubernetes-phase rule
// carrying a reducer on .spec.kubernetes.advanced.kubeProxy.type (mirrors the
// onpremises rules for kube-proxy migration).
func kubeProxyExtractor() rules.Extractor {
	return rules.NewBaseExtractor(rules.Spec{
		Kubernetes: &[]rules.Rule{
			{
				Path: ".spec.kubernetes.advanced.kubeProxy.type",
				Reducers: &[]rules.Reducer{
					{Key: "kubeProxyType", Lifecycle: "post-kubernetes"},
				},
			},
		},
	})
}

// TestBuild_KubeProxyType_NilToNftables guards the nil->value case: when the
// whole kubeProxy object is added (the field is new, e.g. on a pre-1.35 cluster),
// r3diff emits the change on the parent path. Without expanding it to per-leaf
// changes the reducer would not match, so the migration would silently not run.
func TestBuild_KubeProxyType_NilToNftables(t *testing.T) {
	t.Parallel()

	parentCreate := r3diff.Changelog{
		{Type: "create", Path: []string{"spec", "kubernetes", "advanced", "kubeProxy"}, From: nil, To: map[string]any{"type": "nftables"}},
	}

	// Without expansion the parent-level change does not match the leaf rule.
	rawRdcs := reducers.Build(parentCreate, kubeProxyExtractor(), "kubernetes")
	assert.Empty(t, filterNonNil(rawRdcs), "raw parent-level change should not match the leaf reducer")

	// With expansion the leaf change matches and carries from=nil, to=nftables.
	rdcs := filterNonNil(reducers.Build(diffs.ExpandMapChanges(parentCreate), kubeProxyExtractor(), "kubernetes"))
	require.Len(t, rdcs, 1)
	assert.Equal(t, "kubeProxyType", rdcs[0].GetKey())
	assert.Equal(t, "post-kubernetes", rdcs[0].GetLifecycle())
	assert.Nil(t, rdcs[0].GetFrom())
	assert.Equal(t, "nftables", rdcs[0].GetTo())
}

// TestBuild_KubeProxyType_IpvsToNftables covers the plain leaf transition.
func TestBuild_KubeProxyType_IpvsToNftables(t *testing.T) {
	t.Parallel()

	cl := r3diff.Changelog{
		{Type: "update", Path: []string{"spec", "kubernetes", "advanced", "kubeProxy", "type"}, From: "ipvs", To: "nftables"},
	}

	rdcs := filterNonNil(reducers.Build(diffs.ExpandMapChanges(cl), kubeProxyExtractor(), "kubernetes"))
	require.Len(t, rdcs, 1)
	assert.Equal(t, "kubeProxyType", rdcs[0].GetKey())
	assert.Equal(t, "ipvs", rdcs[0].GetFrom())
	assert.Equal(t, "nftables", rdcs[0].GetTo())
}

func filterNonNil(rs reducers.Reducers) reducers.Reducers {
	out := reducers.Reducers{}

	for _, r := range rs {
		if r != nil {
			out = append(out, r)
		}
	}

	return out
}
