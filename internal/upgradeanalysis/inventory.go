// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package upgradeanalysis

import (
	"fmt"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/sighupio/furyctl/internal/clusterhealth"
	"github.com/sighupio/furyctl/internal/clusterinfo"
)

// inventoryRows returns the facts the inventory table of an upgrade analysis opens with,
// in the order they are written.
func inventoryRows(a *Analysis) [][2]string {
	rows := [][2]string{
		{"Cluster", a.ClusterName},
		{"Kind", a.Kind},
		{"Current version", a.From},
		{"Target version", a.To},
	}

	if len(a.Hops) > 1 {
		intermediate := make([]string, 0, len(a.Hops)-1)
		for _, hop := range a.Hops[:len(a.Hops)-1] {
			intermediate = append(intermediate, hop.To)
		}

		rows = append(rows, [2]string{"Intermediate versions", strings.Join(intermediate, ", ")})
	}

	if len(a.Hops) > 0 {
		last := a.Hops[len(a.Hops)-1]
		if a.Hops[0].KubernetesFrom != "" {
			rows = append(rows,
				[2]string{"Current Kubernetes", a.Hops[0].KubernetesFrom},
				[2]string{"Target Kubernetes", last.KubernetesTo},
			)
		}

		if a.Hops[0].InstallerFrom != "" {
			rows = append(rows,
				[2]string{"Current installer", a.Hops[0].InstallerFrom},
				[2]string{"Target installer", last.InstallerTo},
			)
		}
	}

	return append(rows, clusterRows(a.Cluster)...)
}

// clusterRows adds the facts that come from the cluster itself rather than from the plan.
func clusterRows(info *clusterinfo.Info) [][2]string {
	if info == nil {
		return nil
	}

	var rows [][2]string

	if info.EtcdTopology != "" {
		rows = append(rows, [2]string{"Etcd topology", info.EtcdTopology})
	}

	// Custom patches are the part of a configuration most likely to break on a version
	// bump, so their presence belongs in the inventory rather than buried somewhere.
	rows = append(rows, [2]string{"Custom patches", customPatchesLabel(info)})

	if !info.LastConfigurationChange.IsZero() {
		rows = append(rows, [2]string{
			"Last configuration change",
			info.LastConfigurationChange.UTC().Format("2006-01-02 15:04:05 (UTC)"),
		})
	}

	if info.Nodes != nil {
		rows = append(rows, [2]string{"Nodes", nodeSummaryLine(info.Nodes)})
	}

	if plugins := pluginNames(info.Plugins); plugins != "" {
		rows = append(rows, [2]string{"Plugins", plugins})
	}

	return rows
}

// customPatchesLabel renders whether a configuration carries custom patches.
func customPatchesLabel(info *clusterinfo.Info) string {
	if info.CustomPatchesPresent {
		return "Yes"
	}

	return "None"
}

// nodeSummaryLine renders the node count per role, for example "10 (3 control-plane, 3 infra...)".
func nodeSummaryLine(nodes *clusterinfo.NodesSummary) string {
	parts := make([]string, 0, len(nodes.Roles))
	for _, role := range nodes.Roles {
		parts = append(parts, fmt.Sprintf("%d %s", role.Quantity, role.Role))
	}

	if len(parts) == 0 {
		return strconv.Itoa(nodes.Totals.Quantity)
	}

	return fmt.Sprintf("%d (%s)", nodes.Totals.Quantity, strings.Join(parts, ", "))
}

// pluginNames lists the plugins deployed on the cluster. They are customer workloads the
// distribution does not manage, and are exactly what an upgrade tends to disturb.
func pluginNames(plugins *clusterinfo.PluginsInfo) string {
	if plugins == nil {
		return ""
	}

	names := make([]string, 0, len(plugins.Kustomize)+len(plugins.Helm))
	names = append(names, plugins.Kustomize...)
	names = append(names, plugins.Helm...)

	return strings.Join(names, ", ")
}

// writeNodeTable renders one row per node, the table an analysis document opens with.
func writeNodeTable(sb *strings.Builder, nodes *clusterinfo.NodesSummary) {
	if nodes == nil || len(nodes.Nodes) == 0 {
		return
	}

	const tabPadding = 2

	var buf strings.Builder

	w := tabwriter.NewWriter(&buf, 0, 0, tabPadding, ' ', 0)

	_, _ = fmt.Fprintln(w, "  Name\tStatus\tRole\tVersion\tOS Image\tKernel\tContainer Runtime")

	for _, node := range nodes.Nodes {
		_, _ = fmt.Fprintf(w, "  %s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			node.Name, node.Status, node.Role, node.KubeletVersion,
			node.OSImage, node.KernelVersion, node.ContainerRuntime)
	}

	_ = w.Flush()

	_, _ = sb.WriteString(buf.String())
}

// writeChecks renders a list of checks, stating for each whether it ran.
func writeChecks(sb *strings.Builder, checks []clusterhealth.Check) {
	for i := range checks {
		check := &checks[i]

		switch {
		case !check.Ran():
			_, _ = fmt.Fprintf(sb, "  %s: could not be checked: %s\n", check.Name, check.Err)

		case check.Clean():
			_, _ = fmt.Fprintf(sb, "  %s: checked, nothing found (%s)\n", check.Name, check.Description)

		default:
			_, _ = fmt.Fprintf(sb, "  %s: %d found (%s)\n", check.Name, len(check.Issues), check.Description)

			for _, issue := range check.Issues {
				_, _ = fmt.Fprintf(sb, "    [%s] %s: %s\n", issue.Severity, issue.Subject, issue.Detail)
			}
		}
	}
}
