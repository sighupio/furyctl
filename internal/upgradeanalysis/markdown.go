// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package upgradeanalysis

import (
	"fmt"
	"strings"

	"github.com/sighupio/furyctl/internal/clusterhealth"
)

// Markdown renders the analysis as the skeleton of an upgrade analysis document: the
// sections in the order they are written, with the collected facts already in place and the
// judgement left to whoever runs the upgrade.
func Markdown(a *Analysis) string {
	var sb strings.Builder

	_, _ = fmt.Fprintf(&sb, "# Upgrade Analysis — %s — SD %s → %s\n\n", a.ClusterName, a.From, a.To)

	writeInventoryMD(&sb, a)

	if a.AlreadyAtTarget() {
		_, _ = sb.WriteString("\nThe cluster already runs the target version, nothing to plan.\n")

		return sb.String()
	}

	writeHealthMD(&sb, a.Health)
	writePlanMD(&sb, a)
	writeWarningsMD(&sb, a)

	return sb.String()
}

func writeInventoryMD(sb *strings.Builder, a *Analysis) {
	_, _ = sb.WriteString("# Cluster Inventory\n\n")
	_, _ = sb.WriteString("| | |\n| --- | --- |\n")
	_, _ = fmt.Fprintf(sb, "| Cluster | %s |\n", a.ClusterName)
	_, _ = fmt.Fprintf(sb, "| Kind | %s |\n", a.Kind)
	_, _ = fmt.Fprintf(sb, "| Current version | %s |\n", a.From)
	_, _ = fmt.Fprintf(sb, "| Target version | %s |\n", a.To)

	if len(a.Hops) > 1 {
		intermediate := make([]string, 0, len(a.Hops)-1)
		for _, hop := range a.Hops[:len(a.Hops)-1] {
			intermediate = append(intermediate, hop.To)
		}

		_, _ = fmt.Fprintf(sb, "| Intermediate versions | %s |\n", strings.Join(intermediate, ", "))
	}

	if len(a.Deployed) > 0 {
		_, _ = fmt.Fprintf(sb, "| Modules deployed | %s |\n", strings.Join(a.Deployed, ", "))
	}

	if len(a.Skipped) > 0 {
		_, _ = fmt.Fprintf(sb, "| Not deployed | %s |\n", strings.Join(a.Skipped, ", "))
	}
}

func writeHealthMD(sb *strings.Builder, report *clusterhealth.Report) {
	if report == nil {
		return
	}

	_, _ = sb.WriteString("\n# Cluster health Analysis\n\n")

	for i := range report.Checks {
		check := &report.Checks[i]

		switch {
		case !check.Ran():
			_, _ = fmt.Fprintf(sb, "**%s** — could not be checked: %s\n\n", check.Name, check.Err)

		case check.Clean():
			_, _ = fmt.Fprintf(sb, "**%s** — checked, nothing found (%s).\n\n", check.Name, check.Description)

		default:
			_, _ = fmt.Fprintf(sb, "**%s** — %s\n\n", check.Name, check.Description)

			for _, issue := range check.Issues {
				_, _ = fmt.Fprintf(sb, "- **%s** %s: %s\n", issue.Severity, issue.Subject, issue.Detail)
			}

			_, _ = sb.WriteString("\n")
		}
	}
}

func writePlanMD(sb *strings.Builder, a *Analysis) {
	_, _ = sb.WriteString("\n# Upgrade Plan\n\n")

	versions := make([]string, 0, len(a.Hops)+1)
	versions = append(versions, a.Hops[0].From)

	for i := range a.Hops {
		versions = append(versions, a.Hops[i].To)
	}

	_, _ = fmt.Fprintf(sb, "Upgrade path (%s):\n\n```\n%s\n```\n",
		pluralHops(len(a.Hops)), strings.Join(versions, " -> "))

	for i := range a.Hops {
		writeHopMD(sb, a, &a.Hops[i])
	}
}

func writeHopMD(sb *strings.Builder, a *Analysis, hop *Hop) {
	_, _ = fmt.Fprintf(sb, "\n## Hop %s → %s\n\n", hop.From, hop.To)

	if hop.KubernetesFrom != "" {
		kubernetes := hop.KubernetesFrom + " (unchanged)"
		if hop.KubernetesChanged() {
			kubernetes = hop.KubernetesFrom + " → " + hop.KubernetesTo
		}

		_, _ = fmt.Fprintf(sb, "- Kubernetes: %s\n", kubernetes)
	}

	if hop.InstallerFrom != "" {
		installer := hop.InstallerFrom + " (unchanged)"
		if hop.InstallerFrom != hop.InstallerTo {
			installer = hop.InstallerFrom + " → " + hop.InstallerTo
		}

		_, _ = fmt.Fprintf(sb, "- Installer: %s\n", installer)
	}

	if hop.DistributionOnly {
		_, _ = sb.WriteString("- Distribution phase only, Kubernetes is not touched\n")
	}

	writeHopModulesMD(sb, hop)
	writeHopFindingsMD(sb, a, hop)
}

func writeHopModulesMD(sb *strings.Builder, hop *Hop) {
	changed := hop.ChangedModules()
	if len(changed) == 0 {
		_, _ = sb.WriteString("\nNo deployed module changes version in this hop.\n")

		return
	}

	_, _ = sb.WriteString("\n| Module | Type | Current version | New version | Update Notes |\n")
	_, _ = sb.WriteString("| --- | --- | --- | --- | --- |\n")

	for _, module := range changed {
		_, _ = fmt.Fprintf(sb, "| %s | %s | %s | %s | |\n", module.Name, module.Type, module.From, module.To)
	}

	if unchanged := hop.UnchangedModules(); len(unchanged) > 0 {
		_, _ = fmt.Fprintf(sb, "\nUnchanged: %s\n", strings.Join(unchanged, ", "))
	}
}

func writeHopFindingsMD(sb *strings.Builder, a *Analysis, hop *Hop) {
	switch {
	case !a.ConfigChecked:
		_, _ = sb.WriteString("\nConfiguration: not checked, the stored configuration could not be read.\n")

	case hop.ConfigCheckError != "":
		_, _ = fmt.Fprintf(sb, "\nConfiguration: could not be checked: %s\n", hop.ConfigCheckError)

	case len(hop.Findings) == 0:
		_, _ = sb.WriteString("\nConfiguration: checked, no changes required.\n")

	default:
		_, _ = sb.WriteString("\n**Configuration changes required**\n\n")

		for _, finding := range hop.Findings {
			_, _ = fmt.Fprintf(sb, "- **%s** %s\n", finding.Severity, finding.Message)
		}
	}
}

func writeWarningsMD(sb *strings.Builder, a *Analysis) {
	if len(a.Warnings) == 0 {
		return
	}

	_, _ = sb.WriteString("\n# Warnings\n\n")

	for _, warning := range a.Warnings {
		_, _ = fmt.Fprintf(sb, "- %s\n", warning)
	}
}
