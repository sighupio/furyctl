// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"
	"strings"

	commcreate "github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/common/create"
)

const (
	reportHeader = "furyctl diff: read-only comparison, nothing was applied."

	reportNotShown = "Not shown: kubectl cleanups, migrations and upgrade steps that apply runs outside kapp " +
		"(it deletes and recreates the minio setup Jobs); resources kapp may replace on apply (fallback-on-replace); " +
		"plugins removed from the configuration (their resources stay in the cluster)."

	reportConfigurationTitle = "Configuration"

	indent = "  "
)

// configurationResult is the comparison of the furyctl configuration with the one stored in the cluster: the
// change lines of the base output, or the error that stopped it.
type configurationResult struct {
	Err   error
	Lines []string
}

// diffReport is the text `furyctl diff` prints on the standard output and whether it must exit with 1.
type diffReport struct {
	Text   string
	Failed bool
}

type counts struct {
	create, update, destroy int
}

// buildDiffReport lays out the report of the command: header, target, configuration, resource sections, the
// Not shown line, the cascading deletion warnings and the final line.
func buildDiffReport(configuration configurationResult, sections []commcreate.Section) diffReport {
	grouped := groupPluginSections(sections)
	blocks := []string{reportHeader}

	if target := firstTarget(grouped); target != "" {
		blocks = append(blocks, printable(target))
	}

	blocks = append(blocks, configurationBlock(configuration))

	failed := configuration.Err != nil
	compared := false
	total := counts{}
	warnings := []string{}

	for _, section := range grouped {
		lines := resourceLines(section)

		blocks = append(blocks, sectionBlock(section, lines))

		failed = failed || section.Status == commcreate.StatusFailed

		if section.Status != commcreate.StatusCompared {
			continue
		}

		compared = true
		total.add(lines)
		warnings = append(warnings, deletionWarnings(lines)...)
	}

	if compared {
		blocks = append(blocks, reportNotShown)
	}

	if len(warnings) > 0 {
		blocks = append(blocks, strings.Join(warnings, "\n"))
	}

	blocks = append(blocks, finalLine(failed, compared, total))

	return diffReport{Text: strings.Join(blocks, "\n\n") + "\n", Failed: failed}
}

func configurationBlock(c configurationResult) string {
	switch {
	case c.Err != nil:
		return reportConfigurationTitle + ": not compared: " + oneLine(c.Err.Error())
	case len(c.Lines) == 0:
		return reportConfigurationTitle + ": no changes"
	}

	lines := []string{reportConfigurationTitle}

	for _, line := range c.Lines {
		lines = append(lines, indent+printable(line))
	}

	return strings.Join(lines, "\n")
}

func sectionBlock(section commcreate.Section, lines []resourceLine) string {
	title := printable(section.Title)

	if len(lines) > 0 {
		out := make([]string, 0, 1+len(lines))
		out = append(out, title)

		for _, line := range lines {
			out = append(out, indent+line.text)
		}

		return strings.Join(out, "\n")
	}

	switch section.Status {
	case commcreate.StatusCompared:
		return title + ": no changes"
	case commcreate.StatusNotSupported:
		return title + ": not supported by " + oneLine(section.Reason)
	default:
		return title + ": not compared: " + oneLine(section.Reason)
	}
}

func (c *counts) add(lines []resourceLine) {
	for _, line := range lines {
		switch line.op {
		case opCreate:
			c.create++
		case opUpdate:
			c.update++
		case opDelete:
			c.destroy++
		default:
			// Only the three operations change a resource.
		}
	}
}

func deletionWarnings(lines []resourceLine) []string {
	warnings := []string{}

	for _, line := range lines {
		if line.op == opDelete && (line.kind == "Namespace" || line.kind == "CustomResourceDefinition") {
			warnings = append(warnings, fmt.Sprintf("Warning: %s %s will be deleted with everything in it.", line.kind, line.name))
		}
	}

	return warnings
}

func finalLine(failed, compared bool, total counts) string {
	switch {
	case failed:
		return "Plan: not available (some sections were not compared)."
	case !compared:
		return "Plan: nothing was compared."
	case total.create+total.update+total.destroy > 0:
		return fmt.Sprintf("Plan: %d to create, %d to update, %d to delete.", total.create, total.update, total.destroy)
	}

	return "No changes in the compared sections."
}

func firstTarget(sections []commcreate.Section) string {
	for _, section := range sections {
		if section.Target != "" {
			return section.Target
		}
	}

	return ""
}
