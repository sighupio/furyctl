// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"cmp"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode"

	commcreate "github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/common/create"
)

const (
	opCreate = "create"
	opUpdate = "update"
	opDelete = "delete"

	clusterScoped = "(cluster)"
)

var lineBreakRe = regexp.MustCompile(`\s*\n\s*`)

// resourceLine is one change of a section, ready to print.
type resourceLine struct {
	text    string
	address string
	scoped  bool
	op      string
	kind    string
	name    string
}

// resourceLines lists the changes of a section: cluster-scoped lines first, then byte order of the address.
func resourceLines(section commcreate.Section) []resourceLine {
	lines := []resourceLine{}

	for _, row := range section.Rows {
		symbol, ok := map[string]string{opCreate: "+", opUpdate: "~", opDelete: "-"}[row.Op]
		if !ok {
			continue
		}

		address := rowAddress(row)

		lines = append(lines, resourceLine{
			text:    symbol + " " + address,
			address: address,
			scoped:  row.API == "" && row.Namespace == clusterScoped,
			op:      row.Op,
			kind:    printable(row.Kind),
			name:    printable(row.Name),
		})
	}

	slices.SortStableFunc(lines, func(a, b resourceLine) int {
		return cmp.Or(cmp.Compare(a.scopeRank(), b.scopeRank()), strings.Compare(a.address, b.address))
	})

	return lines
}

// scopeRank puts the cluster-scoped lines before the others.
func (l resourceLine) scopeRank() int {
	if l.scoped {
		return 0
	}

	return 1
}

// rowAddress is how a change is named: as helmfile printed it for a helm release, else namespace/Kind/name.
func rowAddress(row commcreate.Row) string {
	switch {
	case row.API != "":
		return printable(fmt.Sprintf("%s, %s, %s (%s)", row.Namespace, row.Name, row.Kind, row.API))
	case row.Namespace == clusterScoped:
		return printable(row.Kind + "/" + row.Name)
	}

	return printable(row.Namespace + "/" + row.Kind + "/" + row.Name)
}

// oneLine puts a message on one line, so that no line of the report is wrapped.
func oneLine(s string) string {
	return printable(lineBreakRe.ReplaceAllString(s, " "))
}

// printable replaces the control characters by ?.
func printable(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return '?'
		}

		return r
	}, s)
}
