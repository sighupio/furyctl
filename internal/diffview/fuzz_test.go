// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package diffview_test

import (
	"bytes"
	"cmp"
	"math/rand/v2"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	r3diff "github.com/r3labs/diff/v3"
	"golang.org/x/text/width"

	"github.com/sighupio/furyctl/internal/diffview"
	"github.com/sighupio/furyctl/pkg/diffs"
)

// arbitraryModel builds a table of three rows from texts nobody chose: a field name,
// the name of a list item, two values and a rule text. The rows are in two groups and
// one of them is a list item, so the name reaches FIELD as a label.
func arbitraryModel(name, label, current, updated, text string) diffview.Model {
	item := map[string]any{"name": label, "value": current}
	newItem := map[string]any{"name": label, "value": updated}

	currentConfig := map[string]any{"spec": map[string]any{"kubernetes": map[string]any{"items": []any{item}}}}
	updatedConfig := map[string]any{"spec": map[string]any{"kubernetes": map[string]any{"items": []any{newItem}}}}

	verdict := func(change r3diff.Change, v diffs.Verdict) diffs.ChangeVerdict {
		return diffs.ChangeVerdict{Change: change, Reported: change, Verdict: v, Text: text}
	}

	return diffview.NewModel([]diffs.ChangeVerdict{
		verdict(r3diff.Change{Type: r3diff.UPDATE, Path: []string{name}, From: current, To: updated}, diffs.VerdictBlocked),
		verdict(r3diff.Change{
			Type: r3diff.CREATE,
			Path: []string{"spec", "distribution", name},
			From: nil,
			To:   []any{map[string]any{name: updated}},
		}, diffs.VerdictNeedsConfirmation),
		verdict(r3diff.Change{
			Type: r3diff.UPDATE,
			Path: []string{"spec", "kubernetes", "items", "0", "value"},
			From: current,
			To:   updated,
		}, diffs.VerdictMigration),
	}, currentConfig, updatedConfig)
}

// displayWidth gives the number of terminal columns a line takes: none for a combining
// mark, two for a wide East Asian character, one for the others.
func displayWidth(line string) int {
	total := 0

	for _, r := range line {
		switch {
		case unicode.In(r, unicode.Mn, unicode.Me):
		case width.LookupRune(r).Kind() == width.EastAsianWide, width.LookupRune(r).Kind() == width.EastAsianFullwidth:
			total += 2
		default:
			total++
		}
	}

	return total
}

// assertSafeOutput checks what holds for every table painted without colour: the only
// control character is the line feed that ends each line, no format character such as
// a direction override is left, no line is wider than the terminal and, without a
// terminal, the summary counts the rows.
func assertSafeOutput(t *testing.T, model diffview.Model, width int) {
	t.Helper()

	var out bytes.Buffer

	if err := diffview.Render(&out, model, diffview.Options{Colour: false, Width: width}); err != nil {
		t.Fatalf("rendering: %v", err)
	}

	output := out.String()

	if !utf8.ValidString(output) || !strings.HasSuffix(output, "\n") {
		t.Fatalf("the output is not text that ends with a line feed: %q", output)
	}

	lines := strings.Split(strings.TrimSuffix(output, "\n"), "\n")
	rows := 0

	for i, line := range lines {
		for _, r := range line {
			if unicode.IsControl(r) {
				t.Fatalf("control character %U in line %d: %q", r, i, line)
			}

			if unicode.Is(unicode.Cf, r) {
				t.Fatalf("format character %U in line %d: %q", r, i, line)
			}
		}

		if width > 0 && displayWidth(line) > width {
			t.Fatalf("line %d is wider than %d columns: %q", i, width, line)
		}

		if strings.HasSuffix(line, " ") {
			t.Fatalf("line %d ends with a space: %q", i, line)
		}

		// A row is a marker and the gap before FIELD; a group title is one word.
		if i > 3 && len(line) > 1 && strings.ContainsAny(line[:1], "+-~") {
			rows++
		}
	}

	// A terminal may cut the summary line and the rows, so they are counted without one.
	if width > 0 {
		return
	}

	summary := strings.Fields(strings.TrimPrefix(lines[0], "CONFIG"))
	if len(summary) == 0 {
		t.Fatalf("no summary in %q", lines[0])
	}

	if counted, err := strconv.Atoi(summary[0]); err != nil || counted != rows || counted != model.Changes {
		t.Fatalf("the summary %q does not count the %d rows of the table (%d in the model)", lines[0], rows, model.Changes)
	}
}

// randomText gives a text of up to twelve characters from an alphabet of control
// characters, format characters, wide characters, combining marks and plain ones.
func randomText(rnd *rand.Rand) string {
	alphabet := []rune("ab.Z9 é–…✗\x00\x01\a\b\t\n\v\f\r\x1b\x7f\u0085\u009b[31m\u202e\u2066\u200b\ufeff\u754cｗ🙂\u0301")

	var text strings.Builder

	for range rnd.IntN(13) {
		text.WriteRune(alphabet[rnd.IntN(len(alphabet))])
	}

	return text.String()
}

func TestRender_ArbitraryTexts(t *testing.T) {
	t.Parallel()

	t.Run("no output contains a control character", func(t *testing.T) {
		t.Parallel()

		rnd := rand.New(rand.NewPCG(5, 2026))

		for range 200 {
			model := arbitraryModel(randomText(rnd), randomText(rnd), randomText(rnd), randomText(rnd), randomText(rnd))

			assertSafeOutput(t, model, 0)
			assertSafeOutput(t, model, 1+rnd.IntN(130))
		}
	})

	t.Run("texts made only of control characters", func(t *testing.T) {
		t.Parallel()

		controls := "\x1b\x00\r\x7f"

		assertSafeOutput(t, arbitraryModel(controls, controls, controls, controls, controls), 0)
		assertSafeOutput(t, arbitraryModel("\n", "\t", "\n\n", "\t\t", "\n"), 0)
	})
}

// FuzzRender looks for a text that breaks the table: a panic, a control character in
// the output, a summary that does not count the rows or a line longer than the
// terminal.
func FuzzRender(f *testing.F) {
	f.Add("modules.logging.type", "worker", "none", "loki", "cannot change", 0)
	f.Add("a\x1b[31mb\nc", "wor\tker", "x\x00y", "z\x7f", "do\x1b[2Jnot\r\nchange", 80)
	f.Add("", "", "", "", "", 1)
	f.Add("passphrase", "\u009b", strings.Repeat("é", 60), strings.Repeat("…", 30), strings.Repeat("0123456789", 20), 120)
	f.Add("a\u202eb\u200bc", "\u754c\u2066", strings.Repeat("\u754c", 30), "e\u0301"+strings.Repeat("🙂", 20), "\ufeff\u898f\u5247", 80)

	f.Fuzz(func(t *testing.T, name, label, current, updated, text string, width int) {
		if width < 0 || width > 1000 {
			width = 0
		}

		assertSafeOutput(t, arbitraryModel(name, label, current, updated, text), width)
	})
}

// propertySentinel is the value every secret of a random table holds: it must never
// reach the output.
const propertySentinel = "sentinel-7f3a9c"

// propertySegments are what random paths are made of: phase keys, list positions,
// sensitive keys and plain keys.
func propertySegments() []string {
	return []string{
		"spec", "distribution", "kubernetes", "infrastructure", "plugins", "modules", "nodes",
		"0", "1", "2", "10", "name", "type", "token", "clientSecret", "passphrase", "a", "b", "é",
	}
}

// propertyValue gives a random value: the sentinel on a sensitive path, otherwise a
// scalar or an object or list that holds the sentinel under a sensitive key.
func propertyValue(rnd *rand.Rand, path []string) any {
	if diffs.IsSensitivePath(path) {
		return propertySentinel
	}

	switch rnd.IntN(5) {
	case 0:
		return "x"
	case 1:
		return 7
	case 2:
		return true
	case 3:
		return map[string]any{"apiToken": propertySentinel, "k": "v"}
	default:
		return []any{"a", map[string]any{"secretRef": propertySentinel}}
	}
}

// propertyVerdicts gives one to eight leaf changes at distinct random paths, with
// random types, values, verdicts and texts.
func propertyVerdicts(rnd *rand.Rand) []diffs.ChangeVerdict {
	segments := propertySegments()
	allVerdicts := []diffs.Verdict{
		diffs.VerdictNone, diffs.VerdictMigration, diffs.VerdictNeedsConfirmation, diffs.VerdictBlocked,
	}
	texts := []string{"", "cannot change", "do\tnot"}
	seen := map[string]bool{}

	var verdicts []diffs.ChangeVerdict

	for want := 1 + rnd.IntN(8); len(verdicts) < want; {
		path := make([]string, 1+rnd.IntN(5))
		for i := range path {
			path[i] = segments[rnd.IntN(len(segments))]
		}

		key := strings.Join(path, "\x00")
		if seen[key] {
			continue
		}

		seen[key] = true

		change := r3diff.Change{Type: r3diff.UPDATE, Path: path, From: propertyValue(rnd, path), To: propertyValue(rnd, path)}

		switch rnd.IntN(3) {
		case 0:
			change.Type, change.From = r3diff.CREATE, nil
		case 1:
			change.Type, change.To = r3diff.DELETE, nil
		}

		verdicts = append(verdicts, diffs.ChangeVerdict{
			Change:   change,
			Reported: change,
			Verdict:  allVerdicts[rnd.IntN(len(allVerdicts))],
			Text:     texts[rnd.IntN(len(texts))],
		})
	}

	return verdicts
}

// expectedRowOrder is the documented order of the rows, written again here so that
// the model is checked against an independent copy and not against itself: segment
// by segment, two list positions as numbers, anything else in byte order, and a path
// that is a prefix of another first.
func expectedRowOrder(a, b []string) int {
	for i := range min(len(a), len(b)) {
		x, errX := strconv.Atoi(a[i])
		y, errY := strconv.Atoi(b[i])

		if errX == nil && errY == nil {
			if x != y {
				return cmp.Compare(x, y)
			}

			continue
		}

		if order := strings.Compare(a[i], b[i]); order != 0 {
			return order
		}
	}

	return cmp.Compare(len(a), len(b))
}

// The invariants of the table hold for random changes: the model validates and its
// summary counts the verdicts, the groups come in the fixed order and their rows in
// the expected order, the same changes in any order give the same bytes, colour
// changes no alignment and no secret reaches the output.
func TestModel_Properties(t *testing.T) {
	t.Parallel()

	rnd := rand.New(rand.NewPCG(11, 2026))
	fixedTitles := []string{"General", "Infrastructure", "Kubernetes", "Distribution", "Plugins"}

	for run := range 150 {
		verdicts := propertyVerdicts(rnd)
		model := diffview.NewModel(verdicts, nil, nil)

		if err := model.Validate(); err != nil {
			t.Fatalf("run %d: %v", run, err)
		}

		counted := func(verdict diffs.Verdict) int {
			count := 0

			for _, v := range verdicts {
				if v.Verdict == verdict {
					count++
				}
			}

			return count
		}

		if model.Changes != len(verdicts) || model.Blocked != counted(diffs.VerdictBlocked) ||
			model.NeedsConfirmation != counted(diffs.VerdictNeedsConfirmation) {
			t.Fatalf("run %d: the summary %d, %d, %d does not count the verdicts", run, model.Changes, model.Blocked, model.NeedsConfirmation)
		}

		next := 0

		for _, group := range model.Groups {
			at := slices.Index(fixedTitles, group.Title)
			if at < next || len(group.Rows) == 0 {
				t.Fatalf("run %d: group %q out of order or empty", run, group.Title)
			}

			next = at + 1

			for i := 1; i < len(group.Rows); i++ {
				if expectedRowOrder(group.Rows[i-1].Path, group.Rows[i].Path) >= 0 {
					t.Fatalf("run %d: rows %v and %v of %s are not in the expected order", run, group.Rows[i-1].Path, group.Rows[i].Path, group.Title)
				}
			}
		}

		plain := render(t, model, diffview.Options{Colour: false, Width: 0})

		shuffled := slices.Clone(verdicts)
		rnd.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })

		if again := render(t, diffview.NewModel(shuffled, nil, nil), diffview.Options{Colour: false, Width: 0}); again != plain {
			t.Fatalf("run %d: the same changes in another order give another table:\n%s\n%s", run, plain, again)
		}

		coloured := render(t, model, diffview.Options{Colour: true, Width: 0})
		if stripped := colourSequence.ReplaceAllString(coloured, ""); stripped != plain {
			t.Fatalf("run %d: colour changes the alignment:\n%s\n%s", run, plain, stripped)
		}

		narrow := render(t, model, diffview.Options{Colour: true, Width: 80})

		for _, output := range []string{plain, coloured, narrow} {
			if strings.Contains(output, propertySentinel) {
				t.Fatalf("run %d: a secret reaches the output:\n%s", run, output)
			}
		}

		assertSafeOutput(t, model, 1+rnd.IntN(130))
	}
}
