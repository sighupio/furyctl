// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package cmd //nolint:testpackage // exercises the unexported report builder.

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	commcreate "github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/common/create"
)

const configurationTitle = "Configuration"

var kappRowRe = regexp.MustCompile(`^(\S+) ([^/]+)/([^/]+)/(.+)$`)

var helmRowRe = regexp.MustCompile(`^(\S+) helm:([^,]*), (.+), (\S+) \(([^)]*)\)$`)

// parseRow reads a row of the notation of the diff-report features.
func parseRow(t *testing.T, raw string) commcreate.Row {
	t.Helper()

	if m := helmRowRe.FindStringSubmatch(raw); m != nil {
		return commcreate.Row{Op: m[1], Namespace: m[2], Name: m[3], Kind: m[4], API: m[5]}
	}

	m := kappRowRe.FindStringSubmatch(raw)
	require.NotNil(t, m, "row %q", raw)

	op := m[1]
	if op == "noop" {
		op = ""
	}

	return commcreate.Row{Op: op, Namespace: m[2], Kind: m[3], Name: m[4]}
}

// parseSections reads sections written as "<title> :: <status> :: <reason> :: <rows>" separated by " ;; ". The
// Configuration section is returned apart, with the rows as its change lines.
func parseSections(t *testing.T, notation string) (configurationResult, []commcreate.Section) {
	t.Helper()

	unescape := strings.NewReplacer(`\u001b`, "\x1b", `\u0009`, "\t")
	configuration := configurationResult{}
	sections := []commcreate.Section{}

	for _, raw := range strings.Split(unescape.Replace(notation), " ;; ") {
		fields := strings.Split(raw, "::")
		for len(fields) < 4 {
			fields = append(fields, "")
		}

		for i := range fields {
			fields[i] = strings.TrimSpace(fields[i])
		}

		rows := []string{}
		if fields[3] != "" {
			rows = strings.Split(fields[3], " & ")
		}

		if fields[0] == configurationTitle {
			if commcreate.Status(fields[1]) == commcreate.StatusFailed {
				configuration.Err = errors.New(fields[2]) //nolint:err113 // the reason of a test section.
			}

			configuration.Lines = rows

			continue
		}

		section := commcreate.Section{Title: fields[0], Status: commcreate.Status(fields[1]), Reason: fields[2]}

		for _, row := range rows {
			section.Rows = append(section.Rows, parseRow(t, row))
		}

		sections = append(sections, section)
	}

	return configuration, sections
}

func render(t *testing.T, notation, targetSection, target string) diffReport {
	t.Helper()

	configuration, sections := parseSections(t, notation)

	for i := range sections {
		if sections[i].Title == targetSection {
			sections[i].Target = target
		}
	}

	return buildDiffReport(configuration, sections)
}

// expectedLines reads the notation of the features, where a blank line is written "¶" between two spaces.
func expectedLines(notation string) string {
	pieces := strings.Split(notation, "¶")
	for i, p := range pieces {
		if i > 0 {
			p = strings.TrimPrefix(p, " ")
		}

		if i < len(pieces)-1 {
			p = strings.TrimSuffix(p, " ")
		}

		pieces[i] = p
	}

	return strings.Join(pieces, "\n")
}

// sectionLines are the blocks of the report of the notation between the header and the Not shown, Warning and
// final lines. The Configuration section is left out when the notation has none.
func sectionLines(t *testing.T, notation string) string {
	t.Helper()

	text := render(t, notation, "", "").Text
	blocks := strings.Split(strings.TrimSuffix(text, "\n"), "\n\n")
	kept := []string{}

	for _, b := range blocks[1 : len(blocks)-1] {
		if strings.HasPrefix(b, "Target cluster") || strings.HasPrefix(b, "Not shown:") || strings.HasPrefix(b, "Warning:") {
			continue
		}

		if b == "Configuration: no changes" && !strings.Contains(notation, configurationTitle) {
			continue
		}

		kept = append(kept, b)
	}

	return strings.Join(kept, "\n\n")
}

func TestDiffReportLayout01OrderOfAFullReport(t *testing.T) {
	t.Parallel()

	for _, tc := range layoutOrderCases {
		report := render(t, tc.sections, tc.targetSection, tc.target)

		require.Equal(t, expectedLines(tc.report)+"\n", report.Text)
	}
}

func TestDiffReportLayout02TargetFromTheFirstSectionThatHasOne(t *testing.T) {
	t.Parallel()

	for _, tc := range layoutTargetCases {
		report := render(t, tc.sections, tc.targetSection, tc.target)

		require.Equal(t, expectedLines(tc.report)+"\n", report.Text)
	}
}

func TestDiffReportLayout02TheFirstTargetInReportOrderWins(t *testing.T) {
	t.Parallel()

	configuration, sections := parseSections(t, "Distribution :: Compared ;; Plugin kustomize/a :: Compared ;; Plugin helm/b :: Compared")
	sections[1].Target = "Target cluster 'https://plugin'"
	sections[2].Target = "Target cluster 'https://helm'"

	report := buildDiffReport(configuration, sections)

	require.Contains(t, report.Text, "\n\nTarget cluster 'https://plugin'\n\n")
	require.NotContains(t, report.Text, "https://helm")
}

func TestDiffReportLines01AddressAndSymbol(t *testing.T) {
	t.Parallel()

	for _, tc := range linesAddressCases {
		require.Equal(t, "Distribution\n  "+tc.line, sectionLines(t, "Distribution :: Compared :: :: "+tc.rows), tc.rows)
	}
}

func TestDiffReportLines02ClusterScopedFirstThenByteOrder(t *testing.T) {
	t.Parallel()

	for _, tc := range linesOrderCases {
		first := render(t, "Distribution :: Compared :: :: "+tc.rows, "", "")
		second := render(t, "Distribution :: Compared :: :: "+tc.rows, "", "")

		require.Equal(t, first.Text, second.Text)
		require.Equal(t, expectedLines(tc.lines), sectionLines(t, "Distribution :: Compared :: :: "+tc.rows))
	}
}

func TestDiffReportLines02HelmLinesAreSortedByTheirText(t *testing.T) {
	t.Parallel()

	lines := sectionLines(t, "Plugin helm/a :: Compared :: :: create helm:b, z, Deployment (apps) & create helm:a, y, Service (v1)")

	require.Equal(t, "Plugin helm/a\n  + a, y, Service (v1)\n  + b, z, Deployment (apps)", lines)
}

func TestDiffReportLines03EveryChangeIsListed(t *testing.T) {
	t.Parallel()

	rows := make([]string, 0, 250)
	for i := range 250 {
		rows = append(rows, fmt.Sprintf("create default/ConfigMap/cm-%03d", i))
	}

	lines := strings.Split(sectionLines(t, "Distribution :: Compared :: :: "+strings.Join(rows, " & ")), "\n")

	require.Len(t, lines, 251)
	require.Equal(t, "Distribution", lines[0])
}

func TestDiffReportSections01OneLineForASectionWithoutResourceLines(t *testing.T) {
	t.Parallel()

	for _, tc := range sectionsLineCases {
		require.Equal(t, expectedLines(tc.lines), sectionLines(t, tc.sections), tc.sections)
	}
}

func TestDiffReportSections02OneSummaryLinePerPluginKind(t *testing.T) {
	t.Parallel()

	for _, tc := range sectionsPluginCases {
		require.Equal(t, expectedLines(tc.lines), sectionLines(t, tc.sections), tc.sections)
	}
}

func TestDiffReportSections03EachPluginPrintsItsOwnSection(t *testing.T) {
	t.Parallel()

	for _, tc := range sectionsEachCases {
		require.Equal(t, expectedLines(tc.lines), sectionLines(t, tc.sections), tc.sections)
	}
}

func TestDiffReportFooter01NotShownOnlyWhenSomethingWasCompared(t *testing.T) {
	t.Parallel()

	const notShown = "Not shown: kubectl cleanups, migrations and upgrade steps that apply runs outside kapp " +
		"(it deletes and recreates the minio setup Jobs); resources kapp may replace on apply (fallback-on-replace); " +
		"plugins removed from the configuration (their resources stay in the cluster)."

	for _, tc := range footerNotShownCases {
		report := render(t, tc.sections, "", "")

		if tc.present == "present" {
			require.Equal(t, 1, strings.Count(report.Text, notShown), tc.sections)
		} else {
			require.NotContains(t, report.Text, "Not shown:", tc.sections)
		}
	}
}

func TestDiffReportFooter02CascadingDeletionWarnings(t *testing.T) {
	t.Parallel()

	for _, tc := range footerWarningCases {
		report := render(t, tc.sections, "", "")

		blocks := strings.Split(strings.TrimSuffix(report.Text, "\n"), "\n\n")
		n := len(blocks)

		require.Equal(t, expectedLines(tc.warnings), blocks[n-2], tc.sections)
		require.True(t, strings.HasPrefix(blocks[n-3], "Not shown:"), tc.sections)
	}
}

func TestDiffReportFooter02NoWarningWithoutADeletedNamespaceOrCRD(t *testing.T) {
	t.Parallel()

	report := render(t, "Distribution :: Compared :: :: create (cluster)/Namespace/new & delete default/ConfigMap/a & "+
		"update (cluster)/CustomResourceDefinition/x", "", "")

	require.NotContains(t, report.Text, "Warning:")
}

func TestDiffReportFooter03FinalLinePrecedenceAndExitCode(t *testing.T) {
	t.Parallel()

	for _, tc := range footerFinalCases {
		report := render(t, tc.sections, "", "")

		lines := strings.Split(strings.TrimSuffix(report.Text, "\n"), "\n")
		code, err := strconv.Atoi(tc.code)
		require.NoError(t, err)

		require.Equal(t, tc.final, lines[len(lines)-1], tc.sections)
		require.Equal(t, code == 1, report.Failed, tc.sections)
	}
}

func TestDiffReportConfigurationDoesNotCountInThePlan(t *testing.T) {
	t.Parallel()

	report := render(t, "Configuration :: Compared :: :: .spec.a: 1 -> 2 ;; Distribution :: Compared :: :: create default/ConfigMap/a", "", "")

	require.Contains(t, report.Text, "\nPlan: 1 to create, 0 to update, 0 to delete.\n")
}

func TestDiffReportEndsWithOneNewlineAndWrapsNoLine(t *testing.T) {
	t.Parallel()

	long := strings.Repeat("x", 500)
	report := render(t, "Distribution :: Compared :: :: create default/ConfigMap/"+long, "", "")

	require.True(t, strings.HasSuffix(report.Text, "\n"))
	require.False(t, strings.HasSuffix(report.Text, "\n\n"))
	require.Contains(t, report.Text, "  + default/ConfigMap/"+long+"\n")
}

var layoutOrderCases = []struct {
	sections      string
	targetSection string
	target        string
	report        string
}{
	{"Configuration :: Compared :: :: .spec.distribution.modules.ingress.nginx.type: single -> dual ;; Distribution :: Compared :: :: create default/ConfigMap/probe ;; Plugin kustomize/alpha :: Compared :: :: update kube-system/Deployment/alpha ;; Plugin helm/poc-podinfo :: Compared :: :: delete helm:poc-helm, poc-helm, Namespace (v1)", "Distribution", "Target cluster 'https://cp1.homelab.poc:6443' (nodes: cp1.homelab.poc)", "furyctl diff: read-only comparison, nothing was applied. ¶ ¶ Target cluster 'https://cp1.homelab.poc:6443' (nodes: cp1.homelab.poc) ¶ ¶ Configuration ¶   .spec.distribution.modules.ingress.nginx.type: single -> dual ¶ ¶ Distribution ¶   + default/ConfigMap/probe ¶ ¶ Plugin kustomize/alpha ¶   ~ kube-system/Deployment/alpha ¶ ¶ Plugin helm/poc-podinfo ¶   - poc-helm, poc-helm, Namespace (v1) ¶ ¶ Not shown: kubectl cleanups, migrations and upgrade steps that apply runs outside kapp (it deletes and recreates the minio setup Jobs); resources kapp may replace on apply (fallback-on-replace); plugins removed from the configuration (their resources stay in the cluster). ¶ ¶ Warning: Namespace poc-helm will be deleted with everything in it. ¶ ¶ Plan: 1 to create, 1 to update, 1 to delete."},
}

var layoutTargetCases = []struct {
	sections      string
	targetSection string
	target        string
	report        string
}{
	{"Configuration :: Compared ;; Distribution :: NotSupported :: distribution v1.35.1 ;; Plugin kustomize/alpha :: Compared", "Plugin kustomize/alpha", "Target cluster 'https://cp1.homelab.poc:6443' (nodes: cp1.homelab.poc)", "furyctl diff: read-only comparison, nothing was applied. ¶ ¶ Target cluster 'https://cp1.homelab.poc:6443' (nodes: cp1.homelab.poc) ¶ ¶ Configuration: no changes ¶ ¶ Distribution: not supported by distribution v1.35.1 ¶ ¶ Plugins (kustomize): no changes ¶ ¶ Not shown: kubectl cleanups, migrations and upgrade steps that apply runs outside kapp (it deletes and recreates the minio setup Jobs); resources kapp may replace on apply (fallback-on-replace); plugins removed from the configuration (their resources stay in the cluster). ¶ ¶ No changes in the compared sections."},
	{"Configuration :: Compared ;; Distribution :: Failed :: preflight failed: boom", "none", "none", "furyctl diff: read-only comparison, nothing was applied. ¶ ¶ Configuration: no changes ¶ ¶ Distribution: not compared: preflight failed: boom ¶ ¶ Plan: not available (some sections were not compared)."},
}

var linesAddressCases = []struct {
	rows string
	line string
}{
	{"create (cluster)/Namespace/demo", "+ Namespace/demo"},
	{"update kube-system/Deployment/hubble-relay", "~ kube-system/Deployment/hubble-relay"},
	{"delete kube-system/ConfigMap/hubble-ui-nginx", "- kube-system/ConfigMap/hubble-ui-nginx"},
	{"create helm:poc-helm, system:metrics-server-aggregated-reader, ClusterRole (rbac.authorization.k8s.io)", "+ poc-helm, system:metrics-server-aggregated-reader, ClusterRole (rbac.authorization.k8s.io)"},
	{"update kube-system/ConfigMap/bad\\u001bname\\u0009x", "~ kube-system/ConfigMap/bad?name?x"},
}

var linesOrderCases = []struct {
	rows  string
	lines string
}{
	{"create kube-system/ConfigMap/b & create (cluster)/Namespace/zeta & update default/Deployment/a & delete (cluster)/ClusterRole/alpha & create kube-system/ConfigMap/B", "Distribution ¶   - ClusterRole/alpha ¶   + Namespace/zeta ¶   ~ default/Deployment/a ¶   + kube-system/ConfigMap/B ¶   + kube-system/ConfigMap/b"},
}

var sectionsLineCases = []struct {
	sections string
	lines    string
}{
	{"Distribution :: Compared", "Distribution: no changes"},
	{"Distribution :: Compared :: :: exists kube-system/ConfigMap/a & noop poc-probe/Deployment/b", "Distribution: no changes"},
	{"Distribution :: NotSupported :: distribution v1.35.1", "Distribution: not supported by distribution v1.35.1"},
	{"Distribution :: Failed :: preflight failed: changes to immutable fields detected", "Distribution: not compared: preflight failed: changes to immutable fields detected"},
	{"Distribution :: OutOfScope :: EKSCluster is not supported by furyctl diff yet", "Distribution: not compared: EKSCluster is not supported by furyctl diff yet"},
	{"Configuration :: Compared", "Configuration: no changes"},
	{"Configuration :: Failed :: error while reading the stored configuration", "Configuration: not compared: error while reading the stored configuration"},
	{"Configuration :: Compared :: :: .spec.a: 1 -> 2 & .spec.b: x -> y", "Configuration ¶   .spec.a: 1 -> 2 ¶   .spec.b: x -> y"},
}

var sectionsPluginCases = []struct {
	sections string
	lines    string
}{
	{"Plugin kustomize/a :: Compared ;; Plugin kustomize/b :: Compared", "Plugins (kustomize): no changes"},
	{"Plugin kustomize/a :: NotSupported :: distribution v1.35.1 ;; Plugin kustomize/b :: NotSupported :: distribution v1.35.1", "Plugins (kustomize): not supported by distribution v1.35.1"},
	{"Plugin kustomize/a :: Failed :: scripts/diff.sh exited 1: boom ;; Plugin kustomize/b :: Failed :: scripts/diff.sh exited 1: boom", "Plugins (kustomize): not compared: scripts/diff.sh exited 1: boom"},
	{"Plugin helm/poc-podinfo :: Compared ;; Plugin helm/poc-metrics :: Compared", "Plugins (helm): no changes"},
	{"Plugins (helm) :: Failed :: helmfile diff exited 1: Error: Failed to get release", "Plugins (helm): not compared: helmfile diff exited 1: Error: Failed to get release"},
}

var sectionsEachCases = []struct {
	sections string
	lines    string
}{
	{"Plugin kustomize/a :: Compared :: :: update kube-system/Deployment/a ;; Plugin kustomize/b :: Failed :: scripts/diff.sh exited 1: boom", "Plugin kustomize/a ¶   ~ kube-system/Deployment/a ¶ ¶ Plugin kustomize/b: not compared: scripts/diff.sh exited 1: boom"},
	{"Plugin kustomize/a :: Compared :: :: create default/ConfigMap/a ;; Plugin kustomize/b :: Compared", "Plugin kustomize/a ¶   + default/ConfigMap/a ¶ ¶ Plugin kustomize/b: no changes"},
	{"Plugin kustomize/a :: Compared ;; Plugin kustomize/b :: Failed :: kapp output did not match the expected format", "Plugin kustomize/a: no changes ¶ ¶ Plugin kustomize/b: not compared: kapp output did not match the expected format"},
	{"Plugin helm/poc-podinfo :: Compared ;; Plugin helm/poc-metrics :: Failed :: helmfile diff output did not match the expected format", "Plugin helm/poc-podinfo: no changes ¶ ¶ Plugin helm/poc-metrics: not compared: helmfile diff output did not match the expected format"},
}

var footerNotShownCases = []struct {
	sections string
	present  string
}{
	{"Configuration :: Compared ;; Distribution :: Compared", "present"},
	{"Configuration :: Compared ;; Distribution :: NotSupported :: distribution v1.35.1 ;; Plugin helm/poc-podinfo :: Compared", "present"},
	{"Configuration :: Compared :: :: .spec.a: 1 -> 2 ;; Distribution :: NotSupported :: distribution v1.35.1", "absent"},
	{"Configuration :: Compared ;; Distribution :: OutOfScope :: EKSCluster is not supported by furyctl diff yet", "absent"},
	{"Configuration :: Compared ;; Distribution :: Failed :: preflight failed: boom", "absent"},
}

var footerWarningCases = []struct {
	sections string
	warnings string
}{
	{"Distribution :: Compared :: :: delete (cluster)/Namespace/forecastle", "Warning: Namespace forecastle will be deleted with everything in it."},
	{"Distribution :: Compared :: :: delete (cluster)/CustomResourceDefinition/certificates.cert-manager.io & create (cluster)/Namespace/new ;; Plugin helm/poc :: Compared :: :: delete helm:poc-helm, poc-helm, Namespace (v1)", "Warning: CustomResourceDefinition certificates.cert-manager.io will be deleted with everything in it. ¶ Warning: Namespace poc-helm will be deleted with everything in it."},
	{"Plugin helm/poc :: Compared :: :: delete helm:poc-helm, widgets.example.com, CustomResourceDefinition (apiextensions.k8s.io) & update helm:poc-helm, poc, Namespace (v1)", "Warning: CustomResourceDefinition widgets.example.com will be deleted with everything in it."},
}

var footerFinalCases = []struct {
	sections string
	final    string
	code     string
}{
	{"Configuration :: Compared ;; Distribution :: Compared :: :: create default/ConfigMap/a & create default/ConfigMap/b & update kube-system/Deployment/c & delete kube-system/ConfigMap/d", "Plan: 2 to create, 1 to update, 1 to delete.", "0"},
	{"Configuration :: Compared ;; Distribution :: NotSupported :: distribution v1.35.1 ;; Plugin kustomize/a :: NotSupported :: distribution v1.35.1", "Plan: nothing was compared.", "0"},
	{"Configuration :: Compared :: :: .spec.a: 1 -> 2 ;; Distribution :: Compared :: :: exists default/ConfigMap/a & noop poc-probe/Deployment/b", "No changes in the compared sections.", "0"},
	{"Configuration :: Compared ;; Distribution :: Compared :: :: create default/ConfigMap/a ;; Plugin kustomize/b :: Failed :: scripts/diff.sh exited 1: boom", "Plan: not available (some sections were not compared).", "1"},
	{"Configuration :: Failed :: error while reading the stored configuration ;; Distribution :: Compared :: :: create default/ConfigMap/a", "Plan: not available (some sections were not compared).", "1"},
	{"Configuration :: Compared ;; Distribution :: OutOfScope :: EKSCluster is not supported by furyctl diff yet ;; Plugins (helm) :: OutOfScope :: EKSCluster is not supported by furyctl diff yet", "Plan: nothing was compared.", "0"},
	{"Configuration :: Compared ;; Distribution :: Compared ;; Plugin helm/poc-podinfo :: Compared :: :: update helm:poc-helm, poc-podinfo, Deployment (apps) & delete helm:poc-helm, poc-podinfo, HorizontalPodAutoscaler (autoscaling)", "Plan: 0 to create, 1 to update, 1 to delete.", "0"},
}
