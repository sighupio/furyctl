// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

//nolint:testpackage // white-box tests for the unexported text renderer
package get

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sighupio/furyctl/internal/clusterinfo"
)

func testInfo(nodes []clusterinfo.NodeDetail) *clusterinfo.Info {
	return &clusterinfo.Info{
		ClusterName: "test",
		SDVersion:   "v1.34.0",
		SDKind:      "OnPremises",
		Nodes: &clusterinfo.NodesSummary{
			Roles:  []clusterinfo.NodeRoleGroup{{Role: "worker", Quantity: len(nodes), VCPU: 8, RAMGb: 16}},
			Totals: clusterinfo.NodeTotals{Quantity: len(nodes), VCPU: 8, RAMGb: 16},
			Nodes:  nodes,
		},
	}
}

func TestFormatTextNodeDetailIsOptIn(t *testing.T) {
	t.Parallel()

	info := testInfo([]clusterinfo.NodeDetail{
		{
			Name:             "worker01",
			Role:             "worker",
			Status:           "Ready",
			KubeletVersion:   "v1.34.4",
			OSImage:          "Ubuntu 24.04.2 LTS",
			KernelVersion:    "6.11.0-1018-azure",
			ContainerRuntime: "containerd://1.7.29",
		},
	})

	// The role summary is always present; it is the default output.
	withoutDetail := formatText(info, textOptions{})
	assert.Contains(t, withoutDetail, "Node Role", "role summary header")
	assert.NotContains(t, withoutDetail, "worker01", "node names must not appear without --detail")

	withDetail := formatText(info, textOptions{nodeDetail: true})
	assert.Contains(t, withDetail, "Node Role", "role summary is kept alongside the detail table")
	assert.Contains(t, withDetail, "worker01", "node name")
	assert.Contains(t, withDetail, "Ubuntu 24.04.2 LTS", "os image")
	assert.Contains(t, withDetail, "6.11.0-1018-azure", "kernel version")
	assert.Contains(t, withDetail, "containerd://1.7.29", "container runtime")
}

func TestFormatTextPressuresColumn(t *testing.T) {
	t.Parallel()

	healthy := []clusterinfo.NodeDetail{
		{Name: "worker01", Role: "worker", Status: "Ready", KubeletVersion: "v1.34.4"},
	}

	underPressure := []clusterinfo.NodeDetail{
		{Name: "worker01", Role: "worker", Status: "Ready", KubeletVersion: "v1.34.4"},
		{
			Name:           "worker02",
			Role:           "worker",
			Status:         "Ready",
			KubeletVersion: "v1.34.4",
			Pressures:      []string{"MemoryPressure"},
		},
	}

	out := formatText(testInfo(healthy), textOptions{nodeDetail: true})
	assert.NotContains(t, out, "Pressures", "column is hidden when no node reports pressure")

	out = formatText(testInfo(underPressure), textOptions{nodeDetail: true})
	assert.Contains(t, out, "Pressures", "column appears when a node reports pressure")
	assert.Contains(t, out, "MemoryPressure", "the pressure itself")

	// The healthy node in the same table gets a placeholder rather than a blank cell.
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "worker01") {
			assert.Contains(t, line, "-", "healthy node placeholder in the pressures column")
		}
	}
}

func TestFormatTextNoNodes(t *testing.T) {
	t.Parallel()

	info := &clusterinfo.Info{ClusterName: "test", SDVersion: "v1.34.0", SDKind: "OnPremises"}

	// Must not panic and must not emit an empty detail table.
	out := formatText(info, textOptions{nodeDetail: true})
	assert.Contains(t, out, "test", "cluster name still rendered")
	assert.NotContains(t, out, "Container Runtime", "no detail table without nodes")
}
