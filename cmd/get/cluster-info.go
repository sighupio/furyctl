// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package get

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	v3 "gopkg.in/yaml.v3"

	"github.com/sighupio/furyctl/internal/analytics"
	"github.com/sighupio/furyctl/internal/app"
	"github.com/sighupio/furyctl/internal/clusterinfo"
	"github.com/sighupio/furyctl/internal/flags"
	cobrax "github.com/sighupio/furyctl/internal/x/cobra"
	execx "github.com/sighupio/furyctl/internal/x/exec"
)

const (
	outputFormatText = "text"
	outputFormatJSON = "json"
	outputFormatYAML = "yaml"
	minTableLines    = 3
)

var errInvalidOutputFormat = errors.New("invalid output format, supported values are: text, json, yaml")

func NewClusterInfoCmd() *cobra.Command {
	var cmdEvent analytics.Event

	clusterInfoCmd := &cobra.Command{
		Args:  cobra.NoArgs,
		Use:   "cluster-info",
		Short: "Display cluster information.",
		Long:  `Display information about the cluster, Kubernetes, and SD status. The command provides a quick overview of the cluster configuration and its current state; its output can be used for analysis and troubleshooting.`,
		Example: `  furyctl get cluster-info                 display cluster info in text format (default)
  furyctl get cluster-info --format json   display cluster info as JSON
  furyctl get cluster-info --format yaml   display cluster info as YAML
 `,
		PreRun: func(cmd *cobra.Command, _ []string) {
			cmdEvent = analytics.NewCommandEvent(cobrax.GetFullname(cmd))

			if err := flags.LoadAndMergeCommandFlags("get"); err != nil {
				logrus.Fatalf("failed to load flags from configuration: %v", err)
			}

			if err := viper.BindPFlags(cmd.Flags()); err != nil {
				logrus.Fatalf("error while binding flags: %v", err)
			}
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			ctn := app.GetContainerInstance()

			tracker := ctn.Tracker()
			tracker.Flush()

			binPath := viper.GetString("bin-path")
			currentDir := viper.GetString("workdir")
			debug := viper.GetBool("debug")
			format := viper.GetString("format")
			outDir := viper.GetString("outdir")

			execx.Debug = debug

			if format != outputFormatText && format != outputFormatJSON && format != outputFormatYAML {
				cmdEvent.AddErrorMessage(errInvalidOutputFormat)
				tracker.Track(cmdEvent)

				return errInvalidOutputFormat
			}

			kubectlBin := resolveKubectlBin(binPath, outDir)

			collector := clusterinfo.NewCollector(kubectlBin, currentDir)

			info, err := collector.Collect()
			if err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while collecting cluster information: %w", err)
			}

			if err := printInfo(info, format); err != nil {
				cmdEvent.AddErrorMessage(err)
				tracker.Track(cmdEvent)

				return fmt.Errorf("error while printing cluster information: %w", err)
			}

			cmdEvent.AddSuccessMessage("cluster info successfully retrieved")
			tracker.Track(cmdEvent)

			return nil
		},
	}

	clusterInfoCmd.Flags().StringP(
		"bin-path",
		"b",
		"",
		"Path to the folder where all the dependencies' binaries are installed. "+
			"When set, furyctl looks for kubectl inside this folder. "+
			"If not set, kubectl is resolved from PATH.",
	)

	clusterInfoCmd.Flags().StringP(
		"format",
		"f",
		outputFormatText,
		"Output format. Supported values: text, json, yaml",
	)

	// Tab-completion for the "format" flag.
	if err := clusterInfoCmd.RegisterFlagCompletionFunc("format", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return []string{outputFormatText, outputFormatJSON, outputFormatYAML}, cobra.ShellCompDirectiveDefault
	}); err != nil {
		logrus.Fatalf("error while registering flag completion: %v", err)
	}

	return clusterInfoCmd
}

func resolveKubectlBin(binPath, outDir string) string {
	searchDir := binPath

	if searchDir == "" && outDir != "" {
		searchDir = path.Join(outDir, ".furyctl", "bin")
	}

	if searchDir != "" {
		pattern := filepath.Join(searchDir, "kubectl", "*", "kubectl")

		matches, err := filepath.Glob(pattern)
		if err == nil && len(matches) > 0 {
			return matches[len(matches)-1]
		}
	}

	return "kubectl"
}

func printInfo(info *clusterinfo.Info, format string) error {
	switch format {
	case outputFormatJSON:
		return printJSON(info)

	case outputFormatYAML:
		return printYAML(info)

	default:
		// Print plain text directly to stdout to avoid log prefixes and match
		// the JSON/YAML behavior (clean pipeable output).
		if _, err := fmt.Fprint(os.Stdout, formatText(info)); err != nil {
			return fmt.Errorf("error writing output: %w", err)
		}

		return nil
	}
}

func printJSON(info *clusterinfo.Info) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")

	if err := enc.Encode(info); err != nil {
		return fmt.Errorf("error encoding JSON: %w", err)
	}

	return nil
}

func printYAML(info *clusterinfo.Info) error {
	enc := v3.NewEncoder(os.Stdout)
	defer enc.Close()

	if err := enc.Encode(info); err != nil {
		return fmt.Errorf("error encoding YAML: %w", err)
	}

	return nil
}

func formatText(info *clusterinfo.Info) string {
	const tabPadding = 2

	var sb strings.Builder

	w := tabwriter.NewWriter(&sb, 0, 0, tabPadding, ' ', 0)

	_, _ = fmt.Fprintf(w, "%s\t%s\n", "Cluster Name:", info.ClusterName)
	_, _ = fmt.Fprintf(w, "%s\t%s\n", "SD Version:", info.SDVersion)
	_, _ = fmt.Fprintf(w, "%s\t%s\n", "SD Kind:", info.SDKind)

	if info.SDInstallerVersion != "" {
		_, _ = fmt.Fprintf(w, "%s\t%s\n", "SD Installer version:", info.SDInstallerVersion)
	}

	if len(info.SDUpgradePaths) > 0 {
		_, _ = fmt.Fprintf(w, "%s\t%s\n", "SD Upgrade paths:", strings.Join(info.SDUpgradePaths, ", "))
	} else {
		_, _ = fmt.Fprintf(w, "%s\t%s\n", "SD Upgrade paths:", "None")
	}

	if info.SDOngoingUpgrade != nil {
		u := info.SDOngoingUpgrade
		_, _ = fmt.Fprintf(w, "%s\t%s\n", "SD Ongoing upgrade:", fmt.Sprintf("Yes (%s: %s)", u.Phase, u.Status))
	} else {
		_, _ = fmt.Fprintf(w, "%s\t%s\n", "SD Ongoing upgrade:", "None")
	}

	if info.KubernetesVersion != "" {
		_, _ = fmt.Fprintf(w, "%s\t%s\n", "Kubernetes version:", info.KubernetesVersion)
	}

	if info.EtcdTopology != "" {
		_, _ = fmt.Fprintf(w, "%s\t%s\n", "Etcd topology:", info.EtcdTopology)
	}

	if !info.LastConfigurationChange.IsZero() {
		_, _ = fmt.Fprintf(w, "%s\t%s\n", "Last configuration change:",
			info.LastConfigurationChange.UTC().Format("2006-01-02 15:04:05 (UTC)"))
	}

	if info.CustomPatchesPresent {
		_, _ = fmt.Fprintf(w, "%s\t%s\n", "Custom Patches present:", "Yes")
	} else {
		_, _ = fmt.Fprintf(w, "%s\t%s\n", "Custom Patches present:", "None")
	}

	_ = w.Flush()

	if len(info.Modules) > 0 {
		_, _ = sb.WriteString("\n")
		writeModulesTable(&sb, info.Modules)
	}

	if info.Plugins != nil {
		_, _ = sb.WriteString("\n")
		writePluginsTable(&sb, info.Plugins)
	}

	if info.Nodes != nil {
		_, _ = sb.WriteString("\n")
		writeNodesTable(&sb, info.Nodes)
	}

	return sb.String()
}

func writeModulesTable(sb *strings.Builder, modules []clusterinfo.ModuleInfo) {
	const tabPadding = 2

	var buf strings.Builder

	w := tabwriter.NewWriter(&buf, 0, 0, tabPadding, ' ', 0)

	_, _ = fmt.Fprintln(w, "Module\tVersion\tType")

	for _, m := range modules {
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", m.Name, m.Version, m.Type)
	}

	_ = w.Flush()

	_, _ = sb.WriteString(insertHeaderSeparator(buf.String(), "-"))
}

func writePluginsTable(sb *strings.Builder, plugins *clusterinfo.PluginsInfo) {
	const tabPadding = 2

	var buf strings.Builder

	w := tabwriter.NewWriter(&buf, 0, 0, tabPadding, ' ', 0)

	_, _ = fmt.Fprintln(w, "Plugin Name\tType")

	for _, name := range plugins.Kustomize {
		_, _ = fmt.Fprintf(w, "%s\t%s\n", name, "Kustomize")
	}

	for _, name := range plugins.Helm {
		_, _ = fmt.Fprintf(w, "%s\t%s\n", name, "Helm")
	}

	_ = w.Flush()

	_, _ = sb.WriteString(insertHeaderSeparator(buf.String(), "-"))
}

func writeNodesTable(sb *strings.Builder, nodes *clusterinfo.NodesSummary) {
	const tabPadding = 2

	var buf strings.Builder

	w := tabwriter.NewWriter(&buf, 0, 0, tabPadding, ' ', 0)

	_, _ = fmt.Fprintln(w, "Node Role\tQty\tvCPU\tRAM(GiB)")

	for _, g := range nodes.Roles {
		_, _ = fmt.Fprintf(w, "%s\t%d\t%d\t%s\n", g.Role, g.Quantity, g.VCPU, formatRAM(g.RAMGb))
	}

	_, _ = fmt.Fprintf(w, "Total\t%d\t%d\t%s\n", nodes.Totals.Quantity, nodes.Totals.VCPU, formatRAM(nodes.Totals.RAMGb))

	_ = w.Flush()

	_, _ = sb.WriteString(insertSeparator(buf.String(), "-", "="))
}

//

func insertHeaderSeparator(table, char string) string {
	lines := strings.SplitN(table, "\n", 2) //nolint:mnd // split into header + rest
	if len(lines) < 2 {                     //nolint:mnd // need at least header + data
		return table
	}

	sepLen := len(strings.TrimRight(lines[0], " "))

	return lines[0] + "\n" + strings.Repeat(char, sepLen) + "\n" + lines[1]
}

func insertSeparator(table, headerChar, footerChar string) string {
	lines := strings.Split(strings.TrimRight(table, "\n"), "\n")
	if len(lines) < minTableLines {
		return insertHeaderSeparator(table, headerChar)
	}

	sepLen := len(strings.TrimRight(lines[0], " "))

	var sb strings.Builder

	_, _ = sb.WriteString(lines[0] + "\n")
	_, _ = sb.WriteString(strings.Repeat(headerChar, sepLen) + "\n")

	for _, line := range lines[1 : len(lines)-1] {
		_, _ = sb.WriteString(line + "\n")
	}

	_, _ = sb.WriteString(strings.Repeat(footerChar, sepLen) + "\n")
	_, _ = sb.WriteString(lines[len(lines)-1] + "\n")

	return sb.String()
}

// formatRAM returns a compact string representation of a RAM value in GiB.
// Whole-number GiB values are printed without decimals.
func formatRAM(gb float64) string {
	if gb == math.Trunc(gb) {
		return fmt.Sprintf("%.0f", gb)
	}

	return fmt.Sprintf("%.1f", gb)
}
