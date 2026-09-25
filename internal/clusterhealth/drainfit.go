// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package clusterhealth

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
)

const (
	bytesPerKi = int64(1) << 10
	bytesPerMi = int64(1) << 20
	bytesPerGi = int64(1) << 30
	bytesPerTi = int64(1) << 40

	bytesPerK = int64(1_000)
	bytesPerM = int64(1_000_000)
	bytesPerG = int64(1_000_000_000)
	bytesPerT = int64(1_000_000_000_000)

	millisPerCore = 1000

	// A role served by fewer nodes than this has nowhere to move its pods during a drain.
	minPeersForDrain = 2
)

// ErrNoUsageData is returned when `kubectl top` produced nothing usable, which normally
// means metrics-server is not installed.
var ErrNoUsageData = errors.New("no usage data, metrics-server may not be installed")

// NodeCapacity is what one node can hold and what is already asked of it.
type NodeCapacity struct {
	Name string
	Role string
	// AllocatableCPU is in millicores, AllocatableMemory in bytes.
	AllocatableCPU    int64
	AllocatableMemory int64
	RequestedCPU      int64
	RequestedMemory   int64
}

// Free returns the capacity not yet requested on this node.
func (n NodeCapacity) Free() (int64, int64) {
	return n.AllocatableCPU - n.RequestedCPU, n.AllocatableMemory - n.RequestedMemory
}

// DrainFit reports, for every node, whether the pods it runs would fit on the other nodes
// that share its role.
//
// Nodes are upgraded one at a time, draining each in turn, so a node whose requests do
// not fit elsewhere leaves pods pending for the length of the upgrade. The check works on
// requests rather than on current usage, because requests are what the scheduler enforces.
// DaemonSet pods are excluded: they are not rescheduled by a drain.
func DrainFit(nodes []NodeCapacity) []Issue {
	byRole := map[string][]NodeCapacity{}
	for _, node := range nodes {
		byRole[node.Role] = append(byRole[node.Role], node)
	}

	roles := make([]string, 0, len(byRole))
	for role := range byRole {
		roles = append(roles, role)
	}

	slices.Sort(roles)

	var issues []Issue

	for _, role := range roles {
		peers := byRole[role]

		if len(peers) < minPeersForDrain {
			issues = append(issues, singleNodeRoleIssue(peers)...)

			continue
		}

		for _, node := range peers {
			if issue, tight := fitElsewhere(node, peers); tight {
				issues = append(issues, issue)
			}
		}
	}

	return issues
}

// singleNodeRoleIssue reports a role served by a single node, whose pods have no peer to
// move to during a drain.
func singleNodeRoleIssue(peers []NodeCapacity) []Issue {
	if len(peers) != 1 || peers[0].RequestedCPU == 0 && peers[0].RequestedMemory == 0 {
		return nil
	}

	return []Issue{{
		Severity: SeverityWarning,
		Subject:  "node " + peers[0].Name,
		Detail: fmt.Sprintf(
			"the only node with role %q: draining it stops everything it runs", peers[0].Role,
		),
	}}
}

// fitElsewhere checks one node's requests against the free capacity of its peers.
func fitElsewhere(node NodeCapacity, peers []NodeCapacity) (Issue, bool) {
	var freeCPU, freeMemory int64

	for _, peer := range peers {
		if peer.Name == node.Name {
			continue
		}

		cpu, memory := peer.Free()
		freeCPU += cpu
		freeMemory += memory
	}

	var shortfalls []string

	if node.RequestedCPU > freeCPU {
		shortfalls = append(shortfalls, fmt.Sprintf(
			"CPU short by %dm", node.RequestedCPU-freeCPU,
		))
	}

	if node.RequestedMemory > freeMemory {
		shortfalls = append(shortfalls, fmt.Sprintf(
			"memory short by %dMi", (node.RequestedMemory-freeMemory)/bytesPerMi,
		))
	}

	if len(shortfalls) == 0 {
		return Issue{}, false
	}

	return Issue{
		Severity: SeverityWarning,
		Subject:  "node " + node.Name,
		Detail: fmt.Sprintf(
			"its pod requests do not fit on the other %q nodes (%s); a drain would leave pods pending",
			node.Role, strings.Join(shortfalls, ", "),
		),
	}, true
}

// parseCapacity combines the node list and the pod list into the per-node capacity picture
// DrainFit works on.
func parseCapacity(nodesJSON, podsJSON string) ([]NodeCapacity, error) {
	var nodes nodeCapacityList
	if err := json.Unmarshal([]byte(nodesJSON), &nodes); err != nil {
		return nil, fmt.Errorf("cannot parse the node list: %w", err)
	}

	var pods podList
	if err := json.Unmarshal([]byte(podsJSON), &pods); err != nil {
		return nil, fmt.Errorf("cannot parse the pod list: %w", err)
	}

	capacities := make(map[string]*NodeCapacity, len(nodes.Items))
	order := make([]string, 0, len(nodes.Items))

	for _, node := range nodes.Items {
		capacities[node.Metadata.Name] = &NodeCapacity{
			Name:              node.Metadata.Name,
			Role:              nodeRole(node.Metadata.Labels),
			AllocatableCPU:    parseCPUMillis(node.Status.Allocatable.CPU),
			AllocatableMemory: parseMemoryBytes(node.Status.Allocatable.Memory),
		}
		order = append(order, node.Metadata.Name)
	}

	for _, pod := range pods.Items {
		capacity, known := capacities[pod.Spec.NodeName]
		if !known || isDaemonSetPod(pod) {
			continue
		}

		for _, container := range pod.Spec.Containers {
			capacity.RequestedCPU += parseCPUMillis(container.Resources.Requests.CPU)
			capacity.RequestedMemory += parseMemoryBytes(container.Resources.Requests.Memory)
		}
	}

	out := make([]NodeCapacity, 0, len(order))
	for _, name := range order {
		out = append(out, *capacities[name])
	}

	return out, nil
}

// isDaemonSetPod reports whether a pod belongs to a DaemonSet, whose pods a drain does not
// move to another node.
func isDaemonSetPod(pod podItem) bool {
	for _, owner := range pod.Metadata.OwnerReferences {
		if owner.Kind == "DaemonSet" {
			return true
		}
	}

	return false
}

// nodeRole returns the role a node serves, so that a drain is only compared against the
// nodes a pod could actually move to.
func nodeRole(labels map[string]string) string {
	const prefix = "node-role.kubernetes.io/"

	roles := make([]string, 0, 1)

	for label := range labels {
		if role, found := strings.CutPrefix(label, prefix); found && role != "" {
			roles = append(roles, role)
		}
	}

	if len(roles) == 0 {
		return "<none>"
	}

	slices.Sort(roles)

	return roles[0]
}

// parseCPUMillis reads a Kubernetes CPU quantity as millicores.
func parseCPUMillis(cpu string) int64 {
	if cpu == "" {
		return 0
	}

	if millis, found := strings.CutSuffix(cpu, "m"); found {
		var value int64
		if _, err := fmt.Sscanf(millis, "%d", &value); err != nil {
			return 0
		}

		return value
	}

	var cores float64
	if _, err := fmt.Sscanf(cpu, "%f", &cores); err != nil {
		return 0
	}

	return int64(cores * millisPerCore)
}

// parseMemoryBytes reads a Kubernetes memory quantity as bytes.
func parseMemoryBytes(memory string) int64 {
	if memory == "" {
		return 0
	}

	suffixes := []struct {
		suffix string
		factor int64
	}{
		{"Ki", bytesPerKi},
		{"Mi", bytesPerMi},
		{"Gi", bytesPerGi},
		{"Ti", bytesPerTi},
		{"K", bytesPerK},
		{"M", bytesPerM},
		{"G", bytesPerG},
		{"T", bytesPerT},
	}

	for _, s := range suffixes {
		if trimmed, found := strings.CutSuffix(memory, s.suffix); found {
			var value int64
			if _, err := fmt.Sscanf(trimmed, "%d", &value); err != nil {
				return 0
			}

			return value * s.factor
		}
	}

	var value int64
	if _, err := fmt.Sscanf(memory, "%d", &value); err != nil {
		return 0
	}

	return value
}
