// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package diffview holds the CONFIG table of furyctl diff: the model built from the
// configuration changes and their rule verdicts, and the renderer that paints it.
package diffview

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"

	r3diff "github.com/r3labs/diff/v3"

	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/pkg/diffs"
)

// ErrInvalidModel is returned when a model breaks an invariant of the table.
var ErrInvalidModel = errors.New("invalid CONFIG table")

// Marker tells on which side of the comparison a field exists.
type Marker string

const (
	// MarkerAdded is a field that exists only in the new configuration.
	MarkerAdded Marker = "+"
	// MarkerRemoved is a field that exists only in the current configuration.
	MarkerRemoved Marker = "-"
	// MarkerChanged is a field that exists in both with different values.
	MarkerChanged Marker = "~"

	// AbsentValue is the cell of a side where the field does not exist (U+2013).
	AbsentValue = "–"

	generalTitle = "General"
	specKey      = "spec"
)

// Row is one changed leaf field.
type Row struct {
	Marker Marker
	// Path is the path of the field as the changelog reports it, with list positions.
	// The rows are sorted by it.
	Path []string
	// Field is the FIELD cell: the path without the phase prefix and with the list
	// items labelled.
	Field string
	// Current and New are the cells of the two sides: the value, AbsentValue or
	// diffs.HiddenValue. A secret value never reaches the model.
	Current   string
	New       string
	Sensitive bool
	Verdict   diffs.Verdict
	// Text is the text of the rule behind the verdict.
	Text string
}

// Group is the rows of one phase, under its title.
type Group struct {
	Title string
	Rows  []Row
}

// Model is the CONFIG table before it is painted.
type Model struct {
	// Groups holds the groups that have rows, in the fixed order of the table.
	Groups []Group
	// Changes, Blocked and NeedsConfirmation are the counts of the summary line.
	Changes           int
	Blocked           int
	NeedsConfirmation int
	// RulesAvailable is false when the distribution has no rules file for the kind.
	RulesAvailable bool
}

// phaseGroup is a phase whose changes have their own group.
type phaseGroup struct {
	phase string
	title string
}

// phaseGroups gives the groups of the phases, in the order of the table. The group
// General comes before them.
func phaseGroups() []phaseGroup {
	return []phaseGroup{
		{cluster.OperationPhaseInfrastructure, "Infrastructure"},
		{cluster.OperationPhaseKubernetes, "Kubernetes"},
		{cluster.OperationPhaseDistribution, "Distribution"},
		{cluster.OperationPhasePlugins, "Plugins"},
	}
}

// NewModel builds the table from the verdicts of the changes and the two
// configurations, which give the labels of the list items.
func NewModel(verdicts []diffs.ChangeVerdict, current, updated map[string]any) Model {
	return modelOf(verdicts, current, updated, true)
}

// NewModelWithoutRules builds the table from the changelog when the rules of the
// kind are not available: no row has a verdict.
func NewModelWithoutRules(changelog r3diff.Changelog, current, updated map[string]any) Model {
	return modelOf(diffs.Undecided(changelog), current, updated, false)
}

func modelOf(verdicts []diffs.ChangeVerdict, current, updated map[string]any, rulesAvailable bool) Model {
	rowsByTitle := map[string][]Row{}

	for _, verdict := range verdicts {
		title, prefix := groupOf(verdict.Change.Path)

		rowsByTitle[title] = append(rowsByTitle[title], newRow(verdict, prefix, current, updated))
	}

	var groups []Group

	for _, title := range groupTitles() {
		rows := rowsByTitle[title]
		if len(rows) == 0 {
			continue
		}

		slices.SortStableFunc(rows, func(a, b Row) int {
			return comparePaths(a.Path, b.Path)
		})

		groups = append(groups, Group{Title: title, Rows: rows})
	}

	changes, blocked, needsConfirmation := summaryCounts(groups)

	return Model{
		Groups:            groups,
		Changes:           changes,
		Blocked:           blocked,
		NeedsConfirmation: needsConfirmation,
		RulesAvailable:    rulesAvailable,
	}
}

// summaryCounts gives the counts of the summary line for the rows of the groups:
// the changes, the blocked ones and the ones that need confirmation.
func summaryCounts(groups []Group) (int, int, int) {
	changes, blocked, needsConfirmation := 0, 0, 0

	for _, group := range groups {
		changes += len(group.Rows)

		for _, row := range group.Rows {
			if row.Verdict == diffs.VerdictBlocked {
				blocked++
			}

			if row.Verdict == diffs.VerdictNeedsConfirmation {
				needsConfirmation++
			}
		}
	}

	return changes, blocked, needsConfirmation
}

func groupTitles() []string {
	groups := phaseGroups()

	titles := make([]string, 0, len(groups)+1)
	titles = append(titles, generalTitle)

	for _, group := range groups {
		titles = append(titles, group.title)
	}

	return titles
}

// groupOf gives the title of the group of a path and how many leading segments its
// FIELD omits: the two of .spec.<phase> for a phase, none for General.
func groupOf(path []string) (string, int) {
	const phasePrefix = 2

	if len(path) > phasePrefix && path[0] == specKey {
		for _, group := range phaseGroups() {
			if path[1] == group.phase {
				return group.title, phasePrefix
			}
		}
	}

	return generalTitle, 0
}

func newRow(verdict diffs.ChangeVerdict, prefix int, current, updated map[string]any) Row {
	change := verdict.Change
	sensitive := diffs.IsSensitivePath(change.Path)

	cellOf := func(value any) string {
		return valueCell(change.Path, value)
	}
	if sensitive {
		cellOf = hiddenCell
	}

	// The diff library reports a removed list item at its position in the current
	// list, where the new list may hold another item, and any other change at its
	// position in the new list. The labels are read from the list the position is of.
	labels := updated
	if change.Type == r3diff.DELETE {
		labels = current
	}

	row := Row{
		Marker:    MarkerChanged,
		Path:      change.Path,
		Field:     fieldOf(change.Path, prefix, labels),
		Current:   cellOf(change.From),
		New:       cellOf(change.To),
		Sensitive: sensitive,
		Verdict:   verdict.Verdict,
		Text:      verdict.Text,
	}

	// A field that is created has no current side and one that is deleted has no
	// new side; the type of the change says it, also for the leaves of a whole object.
	if change.Type == r3diff.CREATE {
		row.Marker, row.Current = MarkerAdded, AbsentValue
	}

	if change.Type == r3diff.DELETE {
		row.Marker, row.New = MarkerRemoved, AbsentValue
	}

	return row
}

// hiddenCell is the cell of a value of a sensitive field.
func hiddenCell(any) string {
	return diffs.HiddenValue
}

// valueCell gives the text of the value of the field at the path, with its secrets
// hidden: compact JSON when it is an object or a list, and the value as furyctl.yaml
// writes it otherwise.
func valueCell(path []string, value any) string {
	if value == nil {
		return "null"
	}

	masked := diffs.MaskSensitive(path, value)

	switch reflect.ValueOf(masked).Kind() {
	case reflect.Map, reflect.Slice, reflect.Array:
		return compactJSON(masked)

	default:
		return fmt.Sprint(masked)
	}
}

// compactJSON prints a value on one line with no spaces between tokens. The encoder
// sorts the keys of an object. The characters <, > and & stay as they are: the text
// is read on a terminal, not by a browser.
func compactJSON(value any) string {
	var buf bytes.Buffer

	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)

	if err := encoder.Encode(value); err != nil {
		// A value JSON cannot hold (a number that is not finite) is still shown.
		return fmt.Sprint(value)
	}

	return strings.TrimSuffix(buf.String(), "\n")
}

// fieldOf gives the FIELD cell of a path: the segments after the prefix, with a list
// position printed as [label] attached to the key of the list. The lists and their
// items are read from the configuration given.
func fieldOf(path []string, prefix int, config map[string]any) string {
	var field strings.Builder

	node := any(config)

	for i, segment := range path {
		list, isList := node.([]any)
		position, err := strconv.Atoi(segment)
		isPosition := isList && err == nil && position >= 0

		node = child(node, segment)

		if i < prefix {
			continue
		}

		switch {
		case isPosition && diffs.IsSensitivePath(path[:i]):
			// The name of an item of a secret list may be part of the secret.
			field.WriteString("[" + strconv.Itoa(position) + "]")

		case isPosition:
			field.WriteString("[" + itemLabel(position, itemAt(list, position)) + "]")

		case field.Len() == 0:
			field.WriteString(segment)

		default:
			field.WriteString("." + segment)
		}
	}

	return field.String()
}

func itemAt(list []any, position int) any {
	if position >= len(list) {
		return nil
	}

	return list[position]
}

// child gives the value of a key of an object or of a position of a list, or nil
// when the node does not have it.
func child(node any, segment string) any {
	switch typed := node.(type) {
	case map[string]any:
		return typed[segment]

	case []any:
		position, err := strconv.Atoi(segment)
		if err != nil || position < 0 {
			return nil
		}

		return itemAt(typed, position)

	default:
		return nil
	}
}

// itemLabel gives the label of a list item: its name, else its hostname, else its
// position.
func itemLabel(position int, item any) string {
	if object, ok := item.(map[string]any); ok {
		for _, key := range []string{"name", "hostname"} {
			if label, ok := object[key].(string); ok && label != "" {
				return label
			}
		}
	}

	return strconv.Itoa(position)
}

// comparePaths orders two paths segment by segment: two list positions as numbers,
// any other pair in byte order, and a path that is a prefix of another first. The
// labels are not part of the order, so it does not depend on the names of the items.
func comparePaths(a, b []string) int {
	for i := range min(len(a), len(b)) {
		if order := compareSegments(a[i], b[i]); order != 0 {
			return order
		}
	}

	return len(a) - len(b)
}

func compareSegments(a, b string) int {
	if diffs.IsListPosition(a) && diffs.IsListPosition(b) {
		// Without the leading zeros, the longer number is the greater one.
		a, b = strings.TrimLeft(a, "0"), strings.TrimLeft(b, "0")

		if len(a) != len(b) {
			return len(a) - len(b)
		}
	}

	return strings.Compare(a, b)
}

// Validate checks the invariants the renderer relies on: the counts of the summary
// line are the ones of the rows, and the rows of each group are sorted.
func (m Model) Validate() error {
	for _, group := range m.Groups {
		for i, row := range group.Rows {
			if i > 0 && comparePaths(group.Rows[i-1].Path, row.Path) > 0 {
				return fmt.Errorf("%w: the rows of %s are not sorted at %s", ErrInvalidModel, group.Title, row.Field)
			}
		}
	}

	changes, blocked, needsConfirmation := summaryCounts(m.Groups)

	if changes != m.Changes || blocked != m.Blocked || needsConfirmation != m.NeedsConfirmation {
		return fmt.Errorf(
			"%w: the summary counts %d changes, %d blocked and %d that need confirmation, the rows are %d, %d and %d",
			ErrInvalidModel,
			m.Changes, m.Blocked, m.NeedsConfirmation,
			changes, blocked, needsConfirmation,
		)
	}

	return nil
}
