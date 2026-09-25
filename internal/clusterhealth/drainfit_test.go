// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

//nolint:testpackage // white-box tests for the capacity parser
package clusterhealth

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const gib = int64(1) << 30

func TestDrainFitDetectsANodeThatWouldNotFit(t *testing.T) {
	t.Parallel()

	// The real QA shape: two workers of the same size, one carrying far more than the
	// other has free. Draining worker01 would leave pods pending for the whole upgrade.
	nodes := []NodeCapacity{
		{
			Name: "worker01", Role: "worker",
			AllocatableCPU: 8000, AllocatableMemory: 24 * gib,
			RequestedCPU: 4000, RequestedMemory: 20 * gib,
		},
		{
			Name: "worker02", Role: "worker",
			AllocatableCPU: 8000, AllocatableMemory: 24 * gib,
			RequestedCPU: 1000, RequestedMemory: 7 * gib,
		},
	}

	issues := DrainFit(nodes)

	// Both nodes are reported, and that is the right answer rather than noise: with two
	// nodes of the same size the condition is symmetric. 20Gi does not fit in worker02's
	// 17Gi of free space, and 7Gi does not fit in worker01's 4Gi either. The worker pool is
	// simply too tight to drain either node without leaving pods pending.
	require.Len(t, issues, 2, "neither worker can absorb the other")

	subjects := make([]string, 0, len(issues))
	for _, issue := range issues {
		subjects = append(subjects, issue.Subject)
		assert.Contains(t, issue.Detail, "memory short by", "the shortfall is quantified")
		assert.Contains(t, issue.Detail, "pods pending", "and its consequence spelled out")
	}

	assert.Contains(t, subjects, "node worker01", "worker01")
	assert.Contains(t, subjects, "node worker02", "worker02")
}

func TestDrainFitWithAThirdNodeToAbsorbTheLoad(t *testing.T) {
	t.Parallel()

	// The same two workers, plus an empty third one. Now every node's pods have somewhere
	// to go, so the pool is drainable and nothing is reported.
	nodes := []NodeCapacity{
		{
			Name: "worker01", Role: "worker",
			AllocatableCPU: 8000, AllocatableMemory: 24 * gib,
			RequestedCPU: 4000, RequestedMemory: 20 * gib,
		},
		{
			Name: "worker02", Role: "worker",
			AllocatableCPU: 8000, AllocatableMemory: 24 * gib,
			RequestedCPU: 1000, RequestedMemory: 7 * gib,
		},
		{
			Name: "worker03", Role: "worker",
			AllocatableCPU: 8000, AllocatableMemory: 24 * gib,
		},
	}

	assert.Empty(t, DrainFit(nodes), "a spare node makes the pool drainable")
}

func TestDrainFitWhenEverythingFits(t *testing.T) {
	t.Parallel()

	nodes := []NodeCapacity{
		{Name: "a", Role: "worker", AllocatableCPU: 8000, AllocatableMemory: 24 * gib, RequestedCPU: 1000, RequestedMemory: 4 * gib},
		{Name: "b", Role: "worker", AllocatableCPU: 8000, AllocatableMemory: 24 * gib, RequestedCPU: 1000, RequestedMemory: 4 * gib},
		{Name: "c", Role: "worker", AllocatableCPU: 8000, AllocatableMemory: 24 * gib, RequestedCPU: 1000, RequestedMemory: 4 * gib},
	}

	assert.Empty(t, DrainFit(nodes), "a cluster with room to spare has nothing to report")
}

func TestDrainFitSingleNodeRole(t *testing.T) {
	t.Parallel()

	// The elastic and postgres nodes of a real cluster: one node for the role, so a drain
	// stops the service outright. That is worth saying even though no peer can overflow.
	nodes := []NodeCapacity{
		{Name: "postgres01", Role: "postgres", AllocatableCPU: 4000, AllocatableMemory: 8 * gib, RequestedCPU: 500, RequestedMemory: 2 * gib},
	}

	issues := DrainFit(nodes)
	require.Len(t, issues, 1, "the single-node role is reported")
	assert.Contains(t, issues[0].Detail, "the only node with role", "why it matters")
}

func TestDrainFitComparesWithinARoleOnly(t *testing.T) {
	t.Parallel()

	// An overloaded worker must not be told it can spill onto the masters.
	nodes := []NodeCapacity{
		{Name: "master01", Role: "control-plane", AllocatableCPU: 8000, AllocatableMemory: 64 * gib},
		{Name: "master02", Role: "control-plane", AllocatableCPU: 8000, AllocatableMemory: 64 * gib},
		{Name: "worker01", Role: "worker", AllocatableCPU: 8000, AllocatableMemory: 8 * gib, RequestedCPU: 1000, RequestedMemory: 7 * gib},
		{Name: "worker02", Role: "worker", AllocatableCPU: 8000, AllocatableMemory: 8 * gib, RequestedCPU: 1000, RequestedMemory: 7 * gib},
	}

	issues := DrainFit(nodes)
	require.NotEmpty(t, issues, "the workers do not fit on each other")

	for _, issue := range issues {
		assert.NotContains(t, issue.Subject, "master", "masters have room, but are a different role")
	}
}

func TestParseCapacityExcludesDaemonSetPods(t *testing.T) {
	t.Parallel()

	nodesJSON := `{"items":[
      {"metadata":{"name":"worker01","labels":{"node-role.kubernetes.io/worker":""}},
       "status":{"allocatable":{"cpu":"8","memory":"24Gi"}}}
    ]}`

	podsJSON := `{"items":[
      {"metadata":{"name":"app","namespace":"a"},
       "spec":{"nodeName":"worker01","containers":[{"resources":{"requests":{"cpu":"500m","memory":"1Gi"}}}]}},
      {"metadata":{"name":"agent","namespace":"kube-system","ownerReferences":[{"kind":"DaemonSet"}]},
       "spec":{"nodeName":"worker01","containers":[{"resources":{"requests":{"cpu":"2","memory":"4Gi"}}}]}}
    ]}`

	capacities, err := parseCapacity(nodesJSON, podsJSON)
	require.NoError(t, err, "parseCapacity")
	require.Len(t, capacities, 1, "one node")

	node := capacities[0]
	assert.Equal(t, "worker", node.Role, "role from the label")
	assert.Equal(t, int64(8000), node.AllocatableCPU, "allocatable cpu in millicores")
	assert.Equal(t, 24*gib, node.AllocatableMemory, "allocatable memory in bytes")

	// A drain does not move DaemonSet pods, so counting them would overstate what has to
	// be rescheduled and produce false warnings.
	assert.Equal(t, int64(500), node.RequestedCPU, "only the non-daemonset pod counts")
	assert.Equal(t, gib, node.RequestedMemory, "only the non-daemonset pod counts")
}

func TestParseQuantities(t *testing.T) {
	t.Parallel()

	cpu := map[string]int64{"8": 8000, "500m": 500, "1500m": 1500, "0": 0, "": 0, "bad": 0}
	for in, want := range cpu {
		assert.Equal(t, want, parseCPUMillis(in), "parseCPUMillis(%q)", in)
	}

	memory := map[string]int64{
		"24Gi":  24 * gib,
		"512Mi": 512 << 20,
		"1Ki":   1024,
		"1G":    1_000_000_000,
		"1024":  1024,
		"":      0,
		"bad":   0,
	}
	for in, want := range memory {
		assert.Equal(t, want, parseMemoryBytes(in), "parseMemoryBytes(%q)", in)
	}
}
