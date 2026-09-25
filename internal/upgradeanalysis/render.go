// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package upgradeanalysis

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/sighupio/furyctl/internal/clusterhealth"
)

const tabPadding = 2

// Text renders the analysis for a terminal, in the order an upgrade is planned: what the
// chain looks like, then what each hop changes.
func Text(a *Analysis) string {
	var sb strings.Builder

	w := tabwriter.NewWriter(&sb, 0, 0, tabPadding, ' ', 0)

	for _, row := range inventoryRows(a) {
		_, _ = fmt.Fprintf(w, "%s:\t%s\n", row[0], row[1])
	}

	_ = w.Flush()

	if a.Cluster != nil && a.Cluster.Nodes != nil && len(a.Cluster.Nodes.Nodes) > 0 {
		_, _ = sb.WriteString("\n")
		writeNodeTable(&sb, a.Cluster.Nodes)
	}

	if a.AlreadyAtTarget() {
		_, _ = sb.WriteString("\nThe cluster already runs the target version, nothing to plan.\n")

		return sb.String()
	}

	_, _ = sb.WriteString("\n" + chainLine(a) + "\n")

	if len(a.Skipped) > 0 {
		_, _ = fmt.Fprintf(&sb,
			"\nNot deployed on this cluster, left out of the report: %s\n",
			strings.Join(a.Skipped, ", "),
		)
	}

	for i := range a.Hops {
		writeHop(&sb, a, &a.Hops[i])
	}

	if len(a.Preflight) > 0 {
		_, _ = sb.WriteString("\nPreflight\n")
		writeChecks(&sb, a.Preflight)
	}

	writeHealth(&sb, a.Health)

	if len(a.Warnings) > 0 {
		_, _ = sb.WriteString("\nWarnings\n")

		for _, warning := range a.Warnings {
			_, _ = sb.WriteString("  - " + warning + "\n")
		}
	}

	return sb.String()
}

// chainLine renders the hop chain as a single arrow-separated line.
func chainLine(a *Analysis) string {
	versions := make([]string, 0, len(a.Hops)+1)
	versions = append(versions, a.Hops[0].From)

	for i := range a.Hops {
		versions = append(versions, a.Hops[i].To)
	}

	return fmt.Sprintf(
		"Upgrade path (%s): %s",
		pluralHops(len(a.Hops)),
		strings.Join(versions, " -> "),
	)
}

func pluralHops(n int) string {
	if n == 1 {
		return "1 hop"
	}

	return fmt.Sprintf("%d hops", n)
}

func writeHop(sb *strings.Builder, a *Analysis, hop *Hop) {
	_, _ = fmt.Fprintf(sb, "\n%s -> %s\n", hop.From, hop.To)

	if hop.KubernetesFrom != "" || hop.KubernetesTo != "" {
		if hop.KubernetesChanged() {
			_, _ = fmt.Fprintf(sb, "  Kubernetes:  %s -> %s\n", hop.KubernetesFrom, hop.KubernetesTo)
		} else {
			_, _ = fmt.Fprintf(sb, "  Kubernetes:  %s (unchanged)\n", hop.KubernetesFrom)
		}
	}

	if hop.InstallerFrom != "" || hop.InstallerTo != "" {
		installer := fmt.Sprintf("%s -> %s", hop.InstallerFrom, hop.InstallerTo)
		if hop.InstallerFrom == hop.InstallerTo {
			installer = hop.InstallerFrom + " (unchanged)"
		}

		_, _ = sb.WriteString("  Installer:   " + installer + "\n")
	}

	phase := "Kubernetes and distribution"
	if hop.DistributionOnly {
		phase = "distribution only, Kubernetes is not touched"
	}

	_, _ = fmt.Fprintf(sb, "  Phase:       %s\n", phase)

	writeModuleTable(sb, hop)
	writeFindings(sb, a, hop)
	writeBreakingChanges(sb, hop)
}

// writeBreakingChanges shows what every release this hop passes through declares breaking.
// A release that ships no such section is named as well, so that its silence is visible
// rather than mistaken for a release with nothing to declare.
func writeBreakingChanges(sb *strings.Builder, hop *Hop) {
	if hop.BreakingChangesError != "" {
		_, _ = fmt.Fprintf(sb, "  Breaking changes: could not be read: %s\n", hop.BreakingChangesError)

		return
	}

	if len(hop.BreakingChanges) == 0 {
		_, _ = sb.WriteString("  Breaking changes: no release notes cover this hop\n")

		return
	}

	_, _ = sb.WriteString("  Breaking changes declared by the releases in this hop:\n")

	for _, release := range hop.BreakingChanges {
		if release.Section == "" {
			_, _ = fmt.Fprintf(sb,
				"    %s: the release notes list no breaking-changes section\n", release.Version)

			continue
		}

		_, _ = fmt.Fprintf(sb, "    %s:\n", release.Version)

		for line := range strings.SplitSeq(release.Section, "\n") {
			_, _ = fmt.Fprintf(sb, "      %s\n", line)
		}
	}
}

func writeModuleTable(sb *strings.Builder, hop *Hop) {
	changed := hop.ChangedModules()

	if len(changed) == 0 {
		_, _ = sb.WriteString("  No deployed module changes version in this hop.\n")

		return
	}

	var buf strings.Builder

	w := tabwriter.NewWriter(&buf, 0, 0, tabPadding, ' ', 0)

	_, _ = fmt.Fprintln(w, "  Module\tType\tFrom\tTo")

	for _, module := range changed {
		_, _ = fmt.Fprintf(w, "  %s\t%s\t%s\t%s\n", module.Name, module.Type, module.From, module.To)
	}

	_ = w.Flush()

	_, _ = sb.WriteString(buf.String())

	if unchanged := hop.UnchangedModules(); len(unchanged) > 0 {
		_, _ = sb.WriteString("  Unchanged: " + strings.Join(unchanged, ", ") + "\n")
	}
}

// writeHealth renders the state of the running cluster. Each check says whether it ran, so
// that a check which failed is never mistaken for a cluster that is fine.
func writeHealth(sb *strings.Builder, report *clusterhealth.Report) {
	if report == nil {
		return
	}

	_, _ = sb.WriteString("\nCluster health\n")
	writeChecks(sb, report.Checks)
}

// writeFindings lists the configuration changes a hop requires. When the configuration was
// checked and nothing came up it says so explicitly: an empty section would otherwise read
// the same as a check that never ran.
func writeFindings(sb *strings.Builder, a *Analysis, hop *Hop) {
	if !a.ConfigChecked {
		_, _ = sb.WriteString("  Configuration: not checked, the stored configuration could not be read\n")

		return
	}

	if hop.ConfigCheckError != "" {
		_, _ = fmt.Fprintf(sb, "  Configuration: could not be checked: %s\n", hop.ConfigCheckError)

		return
	}

	if len(hop.Findings) == 0 {
		_, _ = sb.WriteString("  Configuration: checked, no changes required\n")

		return
	}

	_, _ = sb.WriteString("  Configuration changes required:\n")

	for _, finding := range hop.Findings {
		_, _ = fmt.Fprintf(sb, "    [%s] %s\n", finding.Severity, finding.Message)
	}
}
