// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package create

import (
	"regexp"
	"strings"
)

// Status is the outcome of one resource section of `furyctl diff`.
type Status string

const (
	StatusCompared     Status = "Compared"
	StatusFailed       Status = "Failed"
	StatusOutOfScope   Status = "OutOfScope"
	StatusNotSupported Status = "NotSupported"

	// KustomizeGroup is the title of the summary of the kustomize plugin sections.
	KustomizeGroup = "Plugins (kustomize)"
	// HelmGroup is the title of the summary of the helm release sections.
	HelmGroup = "Plugins (helm)"

	kustomizePrefix = "Plugin kustomize/"
	helmPrefix      = "Plugin helm/"
)

// Row is one change of a section. API is only set for the rows of helm releases.
type Row struct {
	Namespace string
	Name      string
	Kind      string
	API       string
	Op        string
}

// Section is the comparison result of one resource section. The Reason of a NotSupported section is the
// distribution it is not supported by, for example `distribution v1.35.1`.
type Section struct {
	Title  string
	Status Status
	Reason string
	Target string
	Rows   []Row
}

// Group is the summary title of the plugins the section belongs to: KustomizeGroup or HelmGroup, empty for a
// section that is not about plugins.
func (s Section) Group() string {
	switch {
	case s.Title == KustomizeGroup || strings.HasPrefix(s.Title, kustomizePrefix):
		return KustomizeGroup
	case s.Title == HelmGroup || strings.HasPrefix(s.Title, helmPrefix):
		return HelmGroup
	}

	return ""
}

func kustomizeTitle(name string) string {
	return kustomizePrefix + name
}

func helmTitle(name string) string {
	return helmPrefix + name
}

var whitespaceRe = regexp.MustCompile(`\s*\n\s*`)

// uniformSections gives every title the same status and reason.
func uniformSections(titles []string, status Status, reason string) []Section {
	sections := make([]Section, 0, len(titles))

	for _, title := range titles {
		sections = append(sections, Section{Title: title, Status: status, Reason: reason})
	}

	return sections
}

// collapseLines joins a multi-line message into one line.
func collapseLines(s string) string {
	return whitespaceRe.ReplaceAllString(s, " ")
}
