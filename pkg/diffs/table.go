// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package diffs

import (
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"text/tabwriter"
	"unicode"

	r3diff "github.com/r3labs/diff/v3"
)

const (
	// The CURRENT cell of a created field and the NEW cell of a deleted one.
	absentValue = "-"
	// The number of spaces between two columns.
	columnPadding = 3
)

// WriteTable writes the CONFIG table of the verdicts: a summary line, then one row per
// changed field with the columns FIELD, CURRENT, NEW and RULE, sorted by field.
// Secrets are hidden.
func WriteTable(w io.Writer, verdicts []ChangeVerdict) error {
	if len(verdicts) == 0 {
		_, err := fmt.Fprintln(w, "CONFIG   no changes")

		return wrapTableErr(err)
	}

	sorted := slices.Clone(verdicts)
	slices.SortStableFunc(sorted, func(a, b ChangeVerdict) int { return strings.Compare(a.Path(), b.Path()) })

	tw := tabwriter.NewWriter(w, 0, 0, columnPadding, ' ', 0)

	if _, err := fmt.Fprintln(w, summaryOf(sorted)); err != nil {
		return wrapTableErr(err)
	}

	if _, err := fmt.Fprintln(tw, "FIELD\tCURRENT\tNEW\tRULE"); err != nil {
		return wrapTableErr(err)
	}

	for _, v := range sorted {
		current, updated := cellOf(v.Change.Path, v.Change.From), cellOf(v.Change.Path, v.Change.To)

		switch v.Change.Type {
		case r3diff.CREATE:
			current = absentValue

		case r3diff.DELETE:
			updated = absentValue

		default:
			// An update shows both values.
		}

		if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", v.Path(), current, updated, ruleOf(v)); err != nil {
			return wrapTableErr(err)
		}
	}

	return wrapTableErr(tw.Flush())
}

func wrapTableErr(err error) error {
	if err != nil {
		return fmt.Errorf("error writing the CONFIG table: %w", err)
	}

	return nil
}

func summaryOf(verdicts []ChangeVerdict) string {
	counts := make(map[Verdict]int)
	for _, v := range verdicts {
		counts[v.Verdict]++
	}

	noun := "changes"
	if len(verdicts) == 1 {
		noun = "change"
	}

	var line strings.Builder

	fmt.Fprintf(&line, "CONFIG   %d %s", len(verdicts), noun)

	for _, verdict := range []Verdict{VerdictBlocked, VerdictNeedsConfirmation} {
		if counts[verdict] > 0 {
			fmt.Fprintf(&line, " · %d %s", counts[verdict], verdict)
		}
	}

	return line.String()
}

func ruleOf(v ChangeVerdict) string {
	if v.Verdict == VerdictNone || v.Verdict == "" {
		return ""
	}

	if v.Text == "" {
		return string(v.Verdict)
	}

	return fmt.Sprintf("%s: %s", v.Verdict, oneLine(v.Text))
}

// cellOf gives the text of a value with its secrets hidden: compact JSON for an object
// or a list, the value as written otherwise.
func cellOf(path []string, value any) string {
	if IsSensitivePath(path) {
		return HiddenValue
	}

	if value == nil {
		return "null"
	}

	masked := MaskSensitive(path, value)

	switch reflect.ValueOf(masked).Kind() {
	case reflect.Map, reflect.Slice, reflect.Array:
		if out, err := json.Marshal(masked); err == nil {
			return string(out)
		}

	default:
		// A scalar is printed as written.
	}

	return oneLine(fmt.Sprint(masked))
}

// oneLine quotes a text with a control or non-printable character, so that a cell keeps
// its row and cannot send escape sequences to the terminal.
func oneLine(text string) string {
	if strings.IndexFunc(text, func(r rune) bool { return !unicode.IsPrint(r) }) >= 0 {
		return strconv.Quote(text)
	}

	return text
}
