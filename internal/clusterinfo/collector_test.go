// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

//nolint:testpackage // white-box tests for internal helpers
package clusterinfo

import (
	"reflect"
	"sort"
	"testing"
	"time"
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
		if got != tt.want {
			t.Fatalf("%s: primaryRole() = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestRoleSort(t *testing.T) {
	t.Parallel()

	input := []string{"<none>", "worker", "master", "infra", "control-plane", "b", "a"}
	want := []string{"control-plane", "master", "a", "b", "infra", "worker", "<none>"}

	sort.Slice(input, func(i, j int) bool { return roleSort(input[i], input[j]) })

	if !reflect.DeepEqual(input, want) {
		t.Fatalf("roleSort order = %v, want %v", input, want)
	}
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

			if got := parseCPU(tt.in); got != tt.want {
				t.Errorf("parseCPU(%q) = %d, want %d", tt.in, got, tt.want)
			}
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
	if hasCustomPatches(map[string]any{}) {
		t.Fatal("expected false when customPatches absent")
	}

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

	if hasCustomPatches(m1) {
		t.Fatal("expected false when all arrays are empty")
	}

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

	if !hasCustomPatches(m2) {
		t.Fatal("expected true when one array has elements")
	}

	// Not a map.
	m3 := map[string]any{
		"spec": map[string]any{
			"distribution": map[string]any{
				"customPatches": []any{"oops"},
			},
		},
	}

	if hasCustomPatches(m3) {
		t.Fatal("expected false when customPatches is not a map")
	}
}

func TestEtcdTopology(t *testing.T) {
	t.Parallel()

	// Non OnPremises.
	if got := etcdTopology("EKSCluster", nil); got != "" {
		t.Fatalf("expected empty for EKSCluster, got %q", got)
	}

	if got := etcdTopology("KFDDistribution", nil); got != "" {
		t.Fatalf("expected empty for KFDDistribution, got %q", got)
	}

	// OnPremises cases.
	if got := etcdTopology("OnPremises", map[string]any{}); got != "Stacked" {
		t.Fatalf("expected Stacked when etcd missing, got %q", got)
	}

	mEmpty := map[string]any{
		"spec": map[string]any{
			"kubernetes": map[string]any{
				"etcd": map[string]any{
					"hosts": []any{},
				},
			},
		},
	}

	if got := etcdTopology("OnPremises", mEmpty); got != "Stacked" {
		t.Fatalf("expected Stacked when hosts empty, got %q", got)
	}

	mDedicated := map[string]any{
		"spec": map[string]any{
			"kubernetes": map[string]any{
				"etcd": map[string]any{
					"hosts": []any{"h1"},
				},
			},
		},
	}

	if got := etcdTopology("OnPremises", mDedicated); got != "Dedicated" {
		t.Fatalf("expected Dedicated when hosts present, got %q", got)
	}

	// Immutable cases.
	if got := etcdTopology("Immutable", map[string]any{}); got != "Stacked" {
		t.Fatalf("expected Stacked when etcd missing for Immutable, got %q", got)
	}

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

	if got := etcdTopology("Immutable", mImmutableStacked); got != "Stacked" {
		t.Fatalf("expected Stacked when etcd members are a subset of controlPlane, got %q", got)
	}

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

	if got := etcdTopology("Immutable", mImmutableDedicated); got != "Dedicated" {
		t.Fatalf("expected Dedicated when etcd members differ from controlPlane, got %q", got)
	}
}

func TestIngressType(t *testing.T) {
	t.Parallel()

	// All none/absent.
	if got := ingressType(map[string]any{}); got != "none" {
		t.Fatalf("expected none, got %q", got)
	}

	// Nginx only.
	nginx := map[string]any{
		"ingress": map[string]any{
			"nginx": map[string]any{"type": "single"},
		},
	}

	if got := ingressType(nginx); got != "nginx/single" {
		t.Fatalf("expected nginx/single, got %q", got)
	}

	// Haproxy only.
	hap := map[string]any{
		"ingress": map[string]any{
			"haproxy": map[string]any{"type": "dual"},
		},
	}

	if got := ingressType(hap); got != "haproxy/dual" {
		t.Fatalf("expected haproxy/dual, got %q", got)
	}

	// Both nginx and haproxy. Output order is deterministic because ingressType appends
	// nginx, haproxy, byoic in a fixed sequence, not by iterating the input map.
	both := map[string]any{
		"ingress": map[string]any{
			"nginx":   map[string]any{"type": "single"},
			"haproxy": map[string]any{"type": "dual"},
		},
	}

	if got := ingressType(both); got != "nginx/single, haproxy/dual" {
		t.Fatalf("expected combined output, got %q", got)
	}

	// Byoic enabled with class.
	byoicWithClass := map[string]any{
		"ingress": map[string]any{
			"byoic": map[string]any{"enabled": true, "ingressClass": "custom"},
		},
	}

	if got := ingressType(byoicWithClass); got != "byoic/custom" {
		t.Fatalf("expected byoic/custom, got %q", got)
	}

	// Byoic enabled without class.
	byoicEnabled := map[string]any{
		"ingress": map[string]any{
			"byoic": map[string]any{"enabled": true},
		},
	}

	if got := ingressType(byoicEnabled); got != "byoic" {
		t.Fatalf("expected byoic, got %q", got)
	}

	// Byoic disabled.
	byoicDisabled := map[string]any{
		"ingress": map[string]any{
			"byoic": map[string]any{"enabled": false, "ingressClass": "custom"},
		},
	}

	if got := ingressType(byoicDisabled); got != "none" {
		t.Fatalf("expected none when byoic disabled, got %q", got)
	}

	// Nginx type none excluded.
	nginxNone := map[string]any{
		"ingress": map[string]any{
			"nginx": map[string]any{"type": "none"},
		},
	}

	if got := ingressType(nginxNone); got != "none" {
		t.Fatalf("expected none when nginx type is none, got %q", got)
	}
}

func TestExtractPlugins(t *testing.T) {
	t.Parallel()

	// No plugins key.
	if extractPlugins(map[string]any{}) != nil {
		t.Fatal("expected nil when plugins absent")
	}

	// Present but empty.
	empty := map[string]any{
		"spec": map[string]any{
			"plugins": map[string]any{
				"kustomize": []any{},
				"helm":      map[string]any{"releases": []any{}},
			},
		},
	}

	if extractPlugins(empty) != nil {
		t.Fatal("expected nil when plugins lists are empty")
	}

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

	if got == nil || !reflect.DeepEqual(got.Kustomize, []string{"a", "b"}) || got.Helm != nil {
		t.Fatalf("unexpected kustomize-only parse: %+v", got)
	}

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

	if got == nil || !reflect.DeepEqual(got.Helm, []string{"x", "y"}) || got.Kustomize != nil {
		t.Fatalf("unexpected helm-only parse: %+v", got)
	}

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

	if got == nil || !reflect.DeepEqual(got.Kustomize, []string{"k1"}) || !reflect.DeepEqual(got.Helm, []string{"h1"}) {
		t.Fatalf("unexpected both parse: %+v", got)
	}

	// Entries without name ignored.
	invalid := map[string]any{
		"spec": map[string]any{
			"plugins": map[string]any{
				"kustomize": []any{map[string]any{"foo": "bar"}},
			},
		},
	}

	if got := extractPlugins(invalid); got != nil {
		t.Fatalf("expected nil when only invalid entries present, got: %+v", got)
	}
}

func TestLatestManagedFieldTime(t *testing.T) {
	t.Parallel()

	// No metadata.
	if !latestManagedFieldTime(map[string]any{}).IsZero() {
		t.Fatal("expected zero time when no metadata present")
	}

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

	if !got.Equal(t2) {
		t.Fatalf("expected latest managedFields time %v, got %v", t2, got)
	}

	// Malformed managedFields, fallback to creationTimestamp.
	ct, _ := time.Parse(time.RFC3339, "2024-02-02T10:00:00Z")
	raw2 := map[string]any{
		"metadata": map[string]any{
			"managedFields":     []any{map[string]any{"time": "not-a-time"}},
			"creationTimestamp": ct.Format(time.RFC3339),
		},
	}

	got = latestManagedFieldTime(raw2)

	if !got.Equal(ct) {
		t.Fatalf("expected creationTimestamp fallback %v, got %v", ct, got)
	}

	raw3 := map[string]any{"metadata": map[string]any{"creationTimestamp": "bad"}}

	if !latestManagedFieldTime(raw3).IsZero() {
		t.Fatal("expected zero when creationTimestamp is malformed")
	}
}
