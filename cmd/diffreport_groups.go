// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import commcreate "github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/common/create"

// groupPluginSections replaces the plugins of a kind by one summary section when none of them has changes and
// they share the same status and reason.
func groupPluginSections(sections []commcreate.Section) []commcreate.Section {
	grouped := make([]commcreate.Section, 0, len(sections))
	done := map[string]bool{}

	for _, section := range sections {
		kind := section.Group()
		if kind == "" {
			grouped = append(grouped, section)

			continue
		}

		if done[kind] {
			continue
		}

		same := sectionsOfKind(sections, kind)

		if len(same) > 1 || len(resourceLines(same[0])) == 0 {
			if summary, ok := summarize(kind, same); ok {
				done[kind] = true

				grouped = append(grouped, summary)

				continue
			}
		}

		grouped = append(grouped, section)
	}

	return grouped
}

func summarize(title string, sections []commcreate.Section) (commcreate.Section, bool) {
	first := sections[0]

	for _, section := range sections {
		if len(resourceLines(section)) > 0 || section.Status != first.Status || section.Reason != first.Reason {
			return commcreate.Section{}, false
		}
	}

	return commcreate.Section{Title: title, Status: first.Status, Reason: first.Reason, Target: firstTarget(sections)}, true
}

func sectionsOfKind(sections []commcreate.Section, kind string) []commcreate.Section {
	same := []commcreate.Section{}

	for _, section := range sections {
		if section.Group() == kind {
			same = append(same, section)
		}
	}

	return same
}
