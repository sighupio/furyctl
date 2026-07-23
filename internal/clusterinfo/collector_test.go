// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

//nolint:testpackage // white-box tests for internal helpers
package clusterinfo

import (
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrimaryRole(t *testing.T) {
	t.Parallel()

	mk := func(labels ...string) map[string]string {
		m := map[string]string{}
		for _, r := range labels {
			m["node-role.kubernetes.io/"+r] = ""
		}

		return m
	}

	tests := []struct {
		name   string
		labels map[string]string
		want   string
	}{
		{"control-plane only", mk("control-plane"), "control-plane"},
		{"master only", mk("master"), "master"},
		{"both control-plane and master", mk("control-plane", "master"), "control-plane"},
		{"multiple roles", mk("infra", "worker", "db"), "db"},
		{"no role labels", map[string]string{"foo": "bar"}, "<none>"},
	}

	for _, tt := range tests {
		got := primaryRole(tt.labels)
		require.Equal(t, tt.want, got, "primaryRole()")
	}
}

func TestRoleSort(t *testing.T) {
	t.Parallel()

	input := []string{"<none>", "worker", "master", "infra", "control-plane", "b", "a"}
	want := []string{"control-plane", "master", "a", "b", "infra", "worker", "<none>"}

	slices.SortFunc(input, roleSort)

	require.Equal(t, want, input, "roleSort order")
}

func TestParseCPU(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   string
		want int64
	}{
		{"4", 4},
		{"500m", 0},
		{"1000m", 1},
		{"1500m", 1},
		{"0", 0},
		{"", 0},
		{"abc", 0},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()

			got := parseCPU(tt.in)
			assert.Equal(t, tt.want, got, "parseCPU(%q)", tt.in)
		})
	}
}

func almostEqual(a, b, tol float64) bool {
	if a > b {
		return a-b <= tol
	}

	return b-a <= tol
}

func TestParseMemoryGb(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   string
		want float64
	}{
		{"1Gi", 1.0},
		{"1024Mi", 1.0},
		{"1048576Ki", 1.0},
		{"2Gi", 2.0},
		{"1Ti", 1024.0},
		{"1T", 1000.0},
		{"1G", 1.0},
		{"1M", 0.001},
		{"", 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()

			got := parseMemoryGb(tt.in)
			if !almostEqual(got, tt.want, 1e-6) {
				t.Errorf("parseMemoryGb(%q) = %f, want %f", tt.in, got, tt.want)
			}
		})
	}
}

func TestHasCustomPatches(t *testing.T) {
	t.Parallel()

	// Absent.
	require.False(t, hasCustomPatches(map[string]any{}), "expected false when customPatches absent")

	// Present but empty arrays.
	m1 := map[string]any{
		"spec": map[string]any{
			"distribution": map[string]any{
				"customPatches": map[string]any{
					"networking": []any{},
				},
			},
		},
	}

	require.False(t, hasCustomPatches(m1), "expected false when all arrays are empty")

	// One non-empty.
	m2 := map[string]any{
		"spec": map[string]any{
			"distribution": map[string]any{
				"customPatches": map[string]any{
					"networking": []any{"x"},
					"logging":    []any{},
				},
			},
		},
	}

	require.True(t, hasCustomPatches(m2), "expected true when one array has elements")

	// Not a map.
	m3 := map[string]any{
		"spec": map[string]any{
			"distribution": map[string]any{
				"customPatches": []any{"oops"},
			},
		},
	}

	require.False(t, hasCustomPatches(m3), "expected false when customPatches is not a map")
}

func TestEtcdTopology(t *testing.T) {
	t.Parallel()

	// Non OnPremises.
	got := etcdTopology("EKSCluster", nil)
	require.Empty(t, got, "expected empty for EKSCluster")

	got = etcdTopology("KFDDistribution", nil)
	require.Empty(t, got, "expected empty for KFDDistribution")

	// OnPremises cases.
	got = etcdTopology("OnPremises", map[string]any{})
	require.Equal(t, "Stacked", got, "expected Stacked when etcd missing")

	mEmpty := map[string]any{
		"spec": map[string]any{
			"kubernetes": map[string]any{
				"etcd": map[string]any{
					"hosts": []any{},
				},
			},
		},
	}

	got = etcdTopology("OnPremises", mEmpty)
	require.Equal(t, "Stacked", got, "expected Stacked when hosts empty")

	mDedicated := map[string]any{
		"spec": map[string]any{
			"kubernetes": map[string]any{
				"etcd": map[string]any{
					"hosts": []any{"h1"},
				},
			},
		},
	}

	got = etcdTopology("OnPremises", mDedicated)
	require.Equal(t, "Dedicated", got, "expected Dedicated when hosts present")

	// Immutable cases.
	got = etcdTopology("Immutable", map[string]any{})
	require.Equal(t, "Stacked", got, "expected Stacked when etcd missing for Immutable")

	mImmutableStacked := map[string]any{
		"spec": map[string]any{
			"kubernetes": map[string]any{
				"controlPlane": map[string]any{
					"members": []any{
						map[string]any{"hostname": "ctrl01"},
						map[string]any{"hostname": "ctrl02"},
					},
				},
				"etcd": map[string]any{
					"members": []any{
						map[string]any{"hostname": "ctrl01"},
					},
				},
			},
		},
	}

	got = etcdTopology("Immutable", mImmutableStacked)
	require.Equal(t, "Stacked", got, "expected Stacked when etcd members are a subset of controlPlane")

	mImmutableDedicated := map[string]any{
		"spec": map[string]any{
			"kubernetes": map[string]any{
				"controlPlane": map[string]any{
					"members": []any{
						map[string]any{"hostname": "ctrl01"},
						map[string]any{"hostname": "ctrl02"},
					},
				},
				"etcd": map[string]any{
					"members": []any{
						map[string]any{"hostname": "etcd01"},
					},
				},
			},
		},
	}

	got = etcdTopology("Immutable", mImmutableDedicated)
	require.Equal(t, "Dedicated", got, "expected Dedicated when etcd members differ from controlPlane")
}

func TestIngressType(t *testing.T) {
	t.Parallel()

	// All none/absent.
	got := ingressType(map[string]any{})
	require.Equal(t, "none", got)

	// Nginx only.
	nginx := map[string]any{
		"ingress": map[string]any{
			"nginx": map[string]any{"type": "single"},
		},
	}

	got = ingressType(nginx)
	require.Equal(t, "nginx/single", got)

	// Haproxy only.
	hap := map[string]any{
		"ingress": map[string]any{
			"haproxy": map[string]any{"type": "dual"},
		},
	}

	got = ingressType(hap)
	require.Equal(t, "haproxy/dual", got)

	// Both nginx and haproxy. Output order is deterministic because ingressType appends
	// nginx, haproxy, byoic in a fixed sequence, not by iterating the input map.
	both := map[string]any{
		"ingress": map[string]any{
			"nginx":   map[string]any{"type": "single"},
			"haproxy": map[string]any{"type": "dual"},
		},
	}

	got = ingressType(both)
	require.Equal(t, "nginx/single, haproxy/dual", got)

	// Byoic enabled with class.
	byoicWithClass := map[string]any{
		"ingress": map[string]any{
			"byoic": map[string]any{"enabled": true, "ingressClass": "custom"},
		},
	}

	got = ingressType(byoicWithClass)
	require.Equal(t, "byoic/custom", got)

	// Byoic enabled without class.
	byoicEnabled := map[string]any{
		"ingress": map[string]any{
			"byoic": map[string]any{"enabled": true},
		},
	}

	got = ingressType(byoicEnabled)
	require.Equal(t, "byoic", got)

	// Byoic disabled.
	byoicDisabled := map[string]any{
		"ingress": map[string]any{
			"byoic": map[string]any{"enabled": false, "ingressClass": "custom"},
		},
	}

	got = ingressType(byoicDisabled)
	require.Equal(t, "none", got)

	// Nginx type none excluded.
	nginxNone := map[string]any{
		"ingress": map[string]any{
			"nginx": map[string]any{"type": "none"},
		},
	}

	got = ingressType(nginxNone)
	require.Equal(t, "none", got)
}

func TestExtractPlugins(t *testing.T) {
	t.Parallel()

	// No plugins key.
	require.Nil(t, extractPlugins(map[string]any{}), "expected nil when plugins absent")

	// Present but empty.
	empty := map[string]any{
		"spec": map[string]any{
			"plugins": map[string]any{
				"kustomize": []any{},
				"helm":      map[string]any{"releases": []any{}},
			},
		},
	}

	require.Nil(t, extractPlugins(empty), "expected nil when plugins lists are empty")

	// Kustomize only.
	kus := map[string]any{
		"spec": map[string]any{
			"plugins": map[string]any{
				"kustomize": []any{
					map[string]any{"name": "a"},
					map[string]any{"name": "b"},
				},
			},
		},
	}

	got := extractPlugins(kus)

	require.NotNil(t, got, "expected non-nil for kustomize-only parse")
	require.Equal(t, []string{"a", "b"}, got.Kustomize)
	require.Nil(t, got.Helm)

	// Helm only.
	helm := map[string]any{
		"spec": map[string]any{
			"plugins": map[string]any{
				"helm": map[string]any{
					"releases": []any{
						map[string]any{"name": "x"},
						map[string]any{"name": "y"},
					},
				},
			},
		},
	}

	got = extractPlugins(helm)

	require.NotNil(t, got, "expected non-nil for helm-only parse")
	require.Equal(t, []string{"x", "y"}, got.Helm)
	require.Nil(t, got.Kustomize)

	// Both.
	both := map[string]any{
		"spec": map[string]any{
			"plugins": map[string]any{
				"kustomize": []any{map[string]any{"name": "k1"}},
				"helm": map[string]any{
					"releases": []any{map[string]any{"name": "h1"}},
				},
			},
		},
	}

	got = extractPlugins(both)

	require.NotNil(t, got, "expected non-nil for both parse")
	require.Equal(t, []string{"k1"}, got.Kustomize)
	require.Equal(t, []string{"h1"}, got.Helm)

	// Entries without name ignored.
	invalid := map[string]any{
		"spec": map[string]any{
			"plugins": map[string]any{
				"kustomize": []any{map[string]any{"foo": "bar"}},
			},
		},
	}

	require.Nil(t, extractPlugins(invalid), "expected nil when only invalid entries present")
}

func TestLatestManagedFieldTime(t *testing.T) {
	t.Parallel()

	// No metadata.
	require.True(t, latestManagedFieldTime(map[string]any{}).IsZero(), "expected zero time when no metadata present")

	// ManagedFields with multiple timestamps.
	t1, _ := time.Parse(time.RFC3339, "2024-01-01T10:00:00Z")
	t2, _ := time.Parse(time.RFC3339, "2024-01-02T10:00:00Z")
	raw := map[string]any{
		"metadata": map[string]any{
			"managedFields": []any{
				map[string]any{"time": t1.Format(time.RFC3339)},
				map[string]any{"time": t2.Format(time.RFC3339)},
			},
		},
	}

	got := latestManagedFieldTime(raw)
	require.True(t, got.Equal(t2), "expected latest managedFields time %v, got %v", t2, got)

	// Malformed managedFields, fallback to creationTimestamp.
	ct, _ := time.Parse(time.RFC3339, "2024-02-02T10:00:00Z")
	raw2 := map[string]any{
		"metadata": map[string]any{
			"managedFields":     []any{map[string]any{"time": "not-a-time"}},
			"creationTimestamp": ct.Format(time.RFC3339),
		},
	}

	got = latestManagedFieldTime(raw2)
	require.True(t, got.Equal(ct), "expected creationTimestamp fallback %v, got %v", ct, got)

	raw3 := map[string]any{"metadata": map[string]any{"creationTimestamp": "bad"}}
	require.True(t, latestManagedFieldTime(raw3).IsZero(), "expected zero when creationTimestamp is malformed")
}
