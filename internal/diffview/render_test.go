// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package diffview_test

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/google/go-cmp/cmp"
	r3diff "github.com/r3labs/diff/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sighupio/furyctl/internal/diffview"
	"github.com/sighupio/furyctl/pkg/diffs"
)

// update rewrites the golden files with the current output:
// go test -tags=unit ./internal/diffview/ -run Golden -update.
var (
	update = flag.Bool("update", false, "rewrite the golden files")

	colourSequence = regexp.MustCompile(esc + `\[[0-9;]*m`)
)

const (
	sixtyCharacters = "0123456789abcdefghij0123456789abcdefghij0123456789abcdefghij"
	esc             = "\x1b"

	// The line of the first row of a table with one group: after the summary, an
	// empty line, the header, the separator and the title of the group.
	rowLineIndex = 5

	// The marker of a changed row with its colour.
	yellowMarker = esc + "[33m~" + esc + "[0m"
)

func render(t *testing.T, model diffview.Model, options diffview.Options) string {
	t.Helper()

	var out bytes.Buffer

	require.NoError(t, diffview.Render(&out, model, options))

	return out.String()
}

// tableRow is a row of a rendered table, cut at the columns of its header.
type tableRow struct {
	group   string
	marker  string
	field   string
	current string
	updated string
	rule    string
	line    string
}

// table is a rendered table read back: its lines and its rows.
type table struct {
	lines  []string
	header string
	rows   []tableRow
	// The widths of the FIELD, CURRENT and NEW columns, in characters.
	widths [3]int
}

func cut(line []rune, from, to int) string {
	from, to = min(from, len(line)), min(to, len(line))

	return string(line[from:to])
}

// parse reads a rendered table without colour. The columns are the ones of the
// header row, counted in characters.
func parse(t *testing.T, output string) table {
	t.Helper()

	require.True(t, strings.HasSuffix(output, "\n"), "the output ends with a line feed")

	parsed := table{lines: strings.Split(strings.TrimSuffix(output, "\n"), "\n")}

	if len(parsed.lines) < 4 {
		return parsed
	}

	parsed.header = parsed.lines[2]
	header := []rune(parsed.header)

	column := func(name string) int {
		index := strings.Index(parsed.header, name)
		require.GreaterOrEqual(t, index, 0, "the header has the column %s", name)

		return utf8.RuneCountInString(parsed.header[:index])
	}

	field, current, updated := column("FIELD"), column("CURRENT"), column("NEW")

	rule := len(header)
	if strings.Contains(parsed.header, "RULE") {
		rule = column("RULE")
	}

	parsed.widths = [3]int{current - field - 2, updated - current - 2, rule - updated - 2}

	group := ""

	for _, line := range parsed.lines[4:] {
		runes := []rune(line)

		if line == "" || strings.HasPrefix(line, "RULE   ") {
			break
		}

		if !strings.ContainsAny(string(runes[:1]), "+-~") {
			group = line

			continue
		}

		parsed.rows = append(parsed.rows, tableRow{
			group:   group,
			marker:  string(runes[:1]),
			field:   strings.TrimRight(cut(runes, field, current-2), " "),
			current: strings.TrimRight(cut(runes, current, updated-2), " "),
			updated: strings.TrimRight(cut(runes, updated, rule-2), " "),
			rule:    cut(runes, rule, len(runes)),
			line:    line,
		})
	}

	return parsed
}

func (tb table) row(t *testing.T, field string) tableRow {
	t.Helper()

	found := []tableRow{}

	for _, row := range tb.rows {
		if row.field == field {
			found = append(found, row)
		}
	}

	require.Len(t, found, 1, "rows with FIELD %s in:\n%s", field, strings.Join(tb.lines, "\n"))

	return found[0]
}

// builtRow is a row of a table built without rule files.
type builtRow struct {
	field   string
	current string
	updated string
	verdict diffs.Verdict
	text    string
}

// built gives a model of ~ rows under Distribution, as the summary and the RULE cell
// are specified: apart from any rule file.
func built(t *testing.T, rows ...builtRow) diffview.Model {
	t.Helper()

	verdicts := make([]diffs.ChangeVerdict, 0, len(rows))

	for _, row := range rows {
		change := changeOf(".spec.distribution."+row.field, row.current, row.updated)

		verdicts = append(verdicts, diffs.ChangeVerdict{
			Change:   change,
			Reported: change,
			Verdict:  row.verdict,
			Text:     row.text,
		})
	}

	model := diffview.NewModel(verdicts, nil, nil)

	require.NoError(t, model.Validate())

	return model
}

func assertGolden(t *testing.T, name, got string) {
	t.Helper()

	path := filepath.Join("testdata", name+".golden")

	if *update {
		require.NoError(t, os.WriteFile(path, []byte(got), 0o600))
	}

	want, err := os.ReadFile(path)
	require.NoError(t, err)

	if diff := cmp.Diff(strings.Split(string(want), "\n"), strings.Split(got, "\n")); diff != "" {
		t.Errorf("%s differs from the output (-golden +output):\n%s", path, diff)
	}
}

func TestRender_Golden(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		golden  string
		options diffview.Options
	}{
		{
			name:    "the simulation inputs give the expected table",
			golden:  "simulation",
			options: diffview.Options{Colour: false, Width: 0},
		},
		{
			name:    "the simulation table with colour",
			golden:  "simulation-colour",
			options: diffview.Options{Colour: true, Width: 0},
		},
		{
			name:    "the simulation table on a terminal 120 columns wide",
			golden:  "simulation-120",
			options: diffview.Options{Colour: false, Width: 120},
		},
		{
			name:    "the simulation table on a terminal 80 columns wide",
			golden:  "simulation-80",
			options: diffview.Options{Colour: false, Width: 80},
		},
	}

	for _, tC := range testCases {
		t.Run(tC.name, func(t *testing.T) {
			t.Parallel()

			assertGolden(t, tC.golden, render(t, simulationModel(t), tC.options))
		})
	}
}

func TestRender_Layout(t *testing.T) {
	t.Parallel()

	t.Run("the layout of the simulation table", func(t *testing.T) {
		t.Parallel()

		output := render(t, simulationModel(t), diffview.Options{})
		tb := parse(t, output)

		assert.Equal(t, "CONFIG   10 changes · 2 blocked · 1 needs confirmation", tb.lines[0])
		assert.Empty(t, tb.lines[1])
		assert.Regexp(t, `^   FIELD +  CURRENT +  NEW +  RULE$`, tb.header)
		assert.Equal(t, strings.Repeat("─", utf8.RuneCountInString(tb.header)), tb.lines[3])
		assert.Equal(t, "Infrastructure", tb.lines[4])
		assert.Len(t, tb.rows, 10)

		for _, line := range tb.lines {
			assert.False(t, strings.HasSuffix(line, " "), "no line ends with a space: %q", line)
		}

		// Every row is the marker, two spaces and the cells two spaces apart.
		tracing := tb.row(t, "modules.tracing.type")
		assert.Equal(t, "-", tracing.marker)
		assert.Equal(t, "Distribution", tracing.group)
		assert.Equal(t, "none", tracing.current)
		assert.Equal(t, "–", tracing.updated)
		assert.Empty(t, tracing.rule)
		assert.True(t, strings.HasPrefix(tracing.line, "-  modules.tracing.type  "), tracing.line)
		assert.True(t, strings.HasSuffix(tracing.line, "  –"), tracing.line)

		podCIDR := tb.row(t, "networking.podCIDR")
		assert.Equal(t, "✗ blocked: Pod CIDR cannot be changed after cluster creation", podCIDR.rule)

		migrations, empty := 0, 0

		for _, row := range tb.rows {
			if strings.HasPrefix(row.rule, "· migration") {
				migrations++
			}

			if row.rule == "" {
				empty++
			}
		}

		assert.Equal(t, 2, migrations)
		assert.Equal(t, 5, empty)
	})

	t.Run("a table without rows is one line", func(t *testing.T) {
		t.Parallel()

		current, _ := simulation(t)

		assert.Equal(t, "CONFIG   no changes\n", render(t, modelOf(t, current, current), diffview.Options{}))
		assert.Equal(
			t,
			"CONFIG   no changes\n",
			render(t, modelOf(t, current, current), diffview.Options{Colour: true, Width: 120}),
		)
	})

	t.Run("summary line counts changes and verdicts", func(t *testing.T) {
		t.Parallel()

		rows := make([]builtRow, 0, 10)

		for i := range 10 {
			row := builtRow{field: "modules.m" + string(rune('a'+i)) + ".type", current: "a", updated: "b", verdict: diffs.VerdictNone}

			switch i {
			case 0, 1:
				row.verdict = diffs.VerdictBlocked
			case 2:
				row.verdict = diffs.VerdictNeedsConfirmation
			}

			rows = append(rows, row)
		}

		tb := parse(t, render(t, built(t, rows...), diffview.Options{}))

		assert.Equal(t, "CONFIG   10 changes · 2 blocked · 1 needs confirmation", tb.lines[0])
	})

	t.Run("summary omits zero counts and uses the singular", func(t *testing.T) {
		t.Parallel()

		output := render(
			t,
			built(t, builtRow{field: "modules.logging.type", current: "a", updated: "b", verdict: diffs.VerdictNone}),
			diffview.Options{},
		)

		assert.Equal(
			t,
			"CONFIG   1 change\n"+
				"\n"+
				"   FIELD                 CURRENT  NEW  RULE\n"+
				"───────────────────────────────────────────\n"+
				"Distribution\n"+
				"~  modules.logging.type  a        b\n",
			output,
		)
	})

	t.Run("a model that is not valid is not painted", func(t *testing.T) {
		t.Parallel()

		model := simulationModel(t)
		model.Changes = 3

		var out bytes.Buffer

		require.ErrorIs(t, diffview.Render(&out, model, diffview.Options{}), diffview.ErrInvalidModel)
		assert.Empty(t, out.String())
	})

	t.Run("repeated runs give identical output", func(t *testing.T) {
		t.Parallel()

		first := render(t, simulationModel(t), diffview.Options{})

		for range 50 {
			require.Equal(t, first, render(t, simulationModel(t), diffview.Options{}))
		}
	})
}

func TestRender_RuleCell(t *testing.T) {
	t.Parallel()

	t.Run("the cell is symbol, label and the text of the rule", func(t *testing.T) {
		t.Parallel()

		output := render(t, built(
			t,
			builtRow{"modules.dr.type", "a", "b", diffs.VerdictBlocked, "cannot change"},
			builtRow{"modules.logging.type", "a", "b", diffs.VerdictNeedsConfirmation, "review this"},
			builtRow{"modules.tracing.type", "a", "b", diffs.VerdictMigration, "moves data"},
		), diffview.Options{})

		assert.Equal(
			t,
			"CONFIG   3 changes · 1 blocked · 1 needs confirmation\n"+
				"\n"+
				"   FIELD                 CURRENT  NEW  RULE\n"+
				"───────────────────────────────────────────\n"+
				"Distribution\n"+
				"~  modules.dr.type       a        b    ✗ blocked: cannot change\n"+
				"~  modules.logging.type  a        b    ! needs confirmation: review this\n"+
				"~  modules.tracing.type  a        b    · migration: moves data\n",
			output,
		)
	})

	t.Run("a verdict without text gives the label alone and no verdict an empty cell", func(t *testing.T) {
		t.Parallel()

		tb := parse(t, render(t, built(
			t,
			builtRow{"modules.dr.type", "a", "b", diffs.VerdictNeedsConfirmation, ""},
			builtRow{"modules.logging.type", "a", "b", diffs.VerdictBlocked, ""},
			builtRow{"modules.monitoring.type", "a", "b", diffs.VerdictNone, ""},
			builtRow{"modules.tracing.type", "a", "b", diffs.VerdictMigration, ""},
		), diffview.Options{}))

		assert.Equal(t, "! needs confirmation", tb.row(t, "modules.dr.type").rule)
		assert.Equal(t, "✗ blocked", tb.row(t, "modules.logging.type").rule)
		assert.Equal(t, "· migration", tb.row(t, "modules.tracing.type").rule)
		assert.Empty(t, tb.row(t, "modules.monitoring.type").rule)
		assert.Equal(t, "~  modules.monitoring.type  a        b", tb.row(t, "modules.monitoring.type").line)
	})

	t.Run("rules not available", func(t *testing.T) {
		t.Parallel()

		current := config(t, `{spec: {distribution: {modules: {logging: {type: none}, monitoring: {type: none}}}}}`)
		updated := config(t, `{spec: {distribution: {modules: {logging: {type: loki}, monitoring: {type: prometheus}}}}}`)

		output := render(
			t,
			diffview.NewModelWithoutRules(changelogOf(t, current, updated), current, updated),
			diffview.Options{},
		)

		assert.Equal(
			t,
			"CONFIG   2 changes\n"+
				"\n"+
				"   FIELD                    CURRENT  NEW         RULE\n"+
				"─────────────────────────────────────────────────────\n"+
				"Distribution\n"+
				"~  modules.logging.type     none     loki\n"+
				"~  modules.monitoring.type  none     prometheus\n"+
				"\n"+
				"RULE   not available: the distribution has no rules file for this kind\n",
			output,
		)
	})
}

func TestRender_Width(t *testing.T) {
	t.Parallel()

	baseDomain := func(t *testing.T, from, to string) diffview.Model {
		t.Helper()

		return modelOfYAML(
			t,
			`{distribution: {modules: {ingress: {baseDomain: `+from+`}}}}`,
			`{distribution: {modules: {ingress: {baseDomain: `+to+`}}}}`,
		)
	}

	assertNoLineLongerThan := func(t *testing.T, tb table, width int) {
		t.Helper()

		for _, line := range tb.lines {
			assert.LessOrEqual(t, utf8.RuneCountInString(line), width, line)
		}
	}

	t.Run("a long NEW value is cut on a terminal", func(t *testing.T) {
		t.Parallel()

		tb := parse(t, render(t, baseDomain(t, "example.test", sixtyCharacters), diffview.Options{Width: 120}))
		row := tb.row(t, "modules.ingress.baseDomain")

		assert.Equal(t, "0123456789abcdefghij012…", row.updated)
		assert.Equal(t, 24, utf8.RuneCountInString(row.updated))
		assert.Equal(t, "example.test", row.current)
		assertNoLineLongerThan(t, tb, 120)
	})

	t.Run("nothing is cut without a terminal", func(t *testing.T) {
		t.Parallel()

		tb := parse(t, render(t, baseDomain(t, "example.test", sixtyCharacters), diffview.Options{}))
		row := tb.row(t, "modules.ingress.baseDomain")

		assert.Equal(t, sixtyCharacters, row.updated)
		assert.Equal(t, "example.test", row.current)
		assert.Equal(t, 60, tb.widths[2])
	})

	t.Run("a long FIELD is cut at 40 characters on a terminal", func(t *testing.T) {
		t.Parallel()

		const longField = "modules.monitoring.prometheus.remoteWrite[0].remoteName"

		model := modelOfYAML(
			t,
			`{distribution: {modules: {monitoring: {prometheus: {remoteWrite: [{remoteName: alpha}]}}}}}`,
			`{distribution: {modules: {monitoring: {prometheus: {remoteWrite: [{remoteName: beta}]}}}}}`,
		)

		require.Equal(t, 55, utf8.RuneCountInString(longField))

		onTerminal := parse(t, render(t, model, diffview.Options{Width: 120}))
		row := onTerminal.row(t, "modules.monitoring.prometheus.remoteWri…")

		assert.Equal(t, 40, utf8.RuneCountInString(row.field))
		assert.Equal(t, 40, onTerminal.widths[0])
		assert.Equal(t, "alpha", row.current)
		assert.Equal(t, "beta", row.updated)

		withoutTerminal := parse(t, render(t, model, diffview.Options{}))

		assert.Equal(t, "alpha", withoutTerminal.row(t, longField).current)
		assert.Equal(t, 55, withoutTerminal.widths[0])
	})

	t.Run("a long CURRENT value is cut at 24 characters on a terminal", func(t *testing.T) {
		t.Parallel()

		tb := parse(t, render(t, baseDomain(t, sixtyCharacters, "abcd"), diffview.Options{Width: 120}))
		row := tb.row(t, "modules.ingress.baseDomain")

		assert.Equal(t, "0123456789abcdefghij012…", row.current)
		assert.Equal(t, "abcd", row.updated)
		assert.Equal(t, 24, tb.widths[1])
		assert.Equal(t, 4, tb.widths[2])
	})

	t.Run("a cell as long as its maximum is not cut", func(t *testing.T) {
		t.Parallel()

		exact := sixtyCharacters[:24]

		tb := parse(t, render(t, baseDomain(t, exact, exact+"x"), diffview.Options{Width: 120}))
		row := tb.row(t, "modules.ingress.baseDomain")

		assert.Equal(t, exact, row.current)
		assert.Equal(t, exact[:23]+"…", row.updated)
	})

	t.Run("width is counted in characters, never in bytes", func(t *testing.T) {
		t.Parallel()

		accents := strings.Repeat("é", 30)

		tb := parse(t, render(t, baseDomain(t, "example.test", accents), diffview.Options{Width: 120}))

		assert.Equal(t, strings.Repeat("é", 23)+"…", tb.row(t, "modules.ingress.baseDomain").updated)
		assert.Equal(t, 24, tb.widths[2])
	})

	t.Run("a narrow terminal cuts the explanation of the rule at the end of the line", func(t *testing.T) {
		t.Parallel()

		model := modelOfYAML(
			t,
			`{distribution: {modules: {ingress: {nginx: {type: single}}}}}`,
			`{distribution: {modules: {ingress: {nginx: {type: dual}}}}}`,
		)

		full := parse(t, render(t, model, diffview.Options{})).row(t, "modules.ingress.nginx.type")
		require.Greater(t, utf8.RuneCountInString(full.line), 140)

		tb := parse(t, render(t, model, diffview.Options{Width: 80}))
		row := tb.row(t, "modules.ingress.nginx.type")

		assert.Equal(t, 80, utf8.RuneCountInString(row.line))
		assert.True(t, strings.HasSuffix(row.line, "…"), row.line)
		assert.Equal(t, "single", row.current)
		assert.Equal(t, "dual", row.updated)
		assert.Equal(t, []rune(full.line)[:79], []rune(row.line)[:79])
		assertNoLineLongerThan(t, tb, 80)
	})

	t.Run("the columns are narrowed to keep the header and its separator within a narrow terminal", func(t *testing.T) {
		t.Parallel()

		model := modelOfYAML(
			t,
			`{distribution: {modules: {monitoring: {prometheus: {remoteWrite: [{remoteName: `+sixtyCharacters+`}]}}}}}`,
			`{distribution: {modules: {monitoring: {prometheus: {remoteWrite: [{remoteName: x`+sixtyCharacters+`}]}}}}}`,
		)

		lines := strings.Split(render(t, model, diffview.Options{Width: 60}), "\n")

		require.Len(t, lines, 7)

		assert.Equal(t, 60, utf8.RuneCountInString(lines[2]))
		assert.True(t, strings.HasSuffix(lines[2], "  RULE"), lines[2])
		assert.Equal(t, strings.Repeat("─", 60), lines[3])
		assert.LessOrEqual(t, utf8.RuneCountInString(lines[5]), 60)

		tb := parse(t, strings.Join(lines, "\n"))
		row := tb.rows[0]

		assert.Equal(t, sixtyCharacters[:11]+"…", row.current)
		assert.Equal(t, "x"+sixtyCharacters[:10]+"…", row.updated)
	})
}

// narrowModel is a table with a long FIELD, values as long as a MAC address and the
// three verdicts with long explanations, as a cluster with nodes gives.
func narrowModel(t *testing.T) diffview.Model {
	t.Helper()

	return built(
		t,
		builtRow{
			"modules.monitoring.prometheus.remoteWrite.endpoint.macAddress",
			"BC:24:11:CC:DD:08", "BC:24:11:CC:DD:99",
			diffs.VerdictNeedsConfirmation, "changing the address of a node recreates the machine",
		},
		builtRow{
			"networking.podCIDR", "10.32.0.0/16", "10.33.0.0/16",
			diffs.VerdictBlocked, "Pod CIDR cannot be changed after cluster creation",
		},
		builtRow{
			"modules.tracing.type", "none", "tempo",
			diffs.VerdictMigration, "changes to the tracing module move data to a new backend",
		},
		builtRow{"modules.ingress.baseDomain", "homelab.poc", "homelab2.poc", diffs.VerdictNone, ""},
	)
}

func TestRender_NarrowTerminal(t *testing.T) {
	t.Parallel()

	verdictWords := map[string]string{
		"modules.monitoring.prometheus.remoteWrite.endpoint.macAddress": "! needs confirmation",
		"networking.podCIDR":   "✗ blocked",
		"modules.tracing.type": "· migration",
	}

	for _, width := range []int{80, 100, 120} {
		t.Run("every verdict word stays whole at "+strconv.Itoa(width)+" columns", func(t *testing.T) {
			t.Parallel()

			output := render(t, narrowModel(t), diffview.Options{Width: width})
			tb := parse(t, output)

			assert.True(t, strings.HasSuffix(tb.header, "  RULE"), tb.header)
			require.Len(t, tb.rows, 4)

			for _, line := range tb.lines {
				assert.LessOrEqual(t, displayWidth(line), width, line)
			}

			for _, row := range tb.rows {
				full := strings.TrimSuffix(row.field, "…")
				word := ""

				for field, w := range verdictWords {
					if strings.HasPrefix(field, full) {
						word = w
					}
				}

				if word == "" {
					assert.Empty(t, row.rule, row.line)

					continue
				}

				assert.True(t, strings.HasPrefix(row.rule, word), "%q has no %q", row.line, word)
				assert.NotContains(t, row.rule, word[:len(word)-1]+"…", row.line)
			}

			assert.Equal(t, "networking.podCIDR", tb.row(t, "networking.podCIDR").field)
			assert.Equal(t, "10.32.0.0/16", tb.row(t, "networking.podCIDR").current)
		})
	}

	t.Run("the explanation is cut, never the verdict word", func(t *testing.T) {
		t.Parallel()

		tb := parse(t, render(t, narrowModel(t), diffview.Options{Width: 80}))

		assert.Equal(t, "✗ blocked: Pod CIDR…", tb.row(t, "networking.podCIDR").rule)
		assert.Equal(t, "· migration: change…", tb.row(t, "modules.tracing.type").rule)
		macAddress := tb.row(t, "modules.monitoring.prometh…")
		assert.Equal(t, "! needs confirmation", macAddress.rule)
		assert.Equal(t, "BC:24:11:CC…", macAddress.current)
	})

	t.Run("an explanation with too little room left is left out", func(t *testing.T) {
		t.Parallel()

		model := built(t, builtRow{"modules.dr.type", "a", "b", diffs.VerdictBlocked, "cannot change"})
		whole := parse(t, render(t, model, diffview.Options{})).rows[0].line
		labelEnd := utf8.RuneCountInString(whole) - utf8.RuneCountInString(": cannot change")

		for width, want := range map[int]string{
			labelEnd:     "✗ blocked",
			labelEnd + 3: "✗ blocked",
			labelEnd + 4: "✗ blocked: c…",
			labelEnd + 6: "✗ blocked: can…",
		} {
			assert.Equal(t, want, parse(t, render(t, model, diffview.Options{Width: width})).rows[0].rule, "width %d", width)
		}
	})

	t.Run("a pipe gets every column and every explanation in full", func(t *testing.T) {
		t.Parallel()

		tb := parse(t, render(t, narrowModel(t), diffview.Options{}))

		macAddress := tb.row(t, "modules.monitoring.prometheus.remoteWrite.endpoint.macAddress")
		assert.Equal(t, "BC:24:11:CC:DD:08", macAddress.current)
		assert.Equal(t, "! needs confirmation: changing the address of a node recreates the machine", macAddress.rule)
	})

	t.Run("a terminal too narrow for the verdict word cuts the line", func(t *testing.T) {
		t.Parallel()

		lines := strings.SplitSeq(strings.TrimSuffix(render(t, narrowModel(t), diffview.Options{Width: 30}), "\n"), "\n")

		for line := range lines {
			assert.LessOrEqual(t, displayWidth(line), 30, line)
		}
	})
}

func TestRender_DisplayWidth(t *testing.T) {
	t.Parallel()

	t.Run("wide characters count two columns when cutting and aligning", func(t *testing.T) {
		t.Parallel()

		model := built(
			t,
			builtRow{"modules.a.type", strings.Repeat("\u754c", 30), "b", diffs.VerdictBlocked, "x"},
			builtRow{"modules.b.type", "e\u0301", "c", diffs.VerdictBlocked, "y"},
			builtRow{"modules.c.type", "plain", "d", diffs.VerdictBlocked, "z"},
		)

		for _, width := range []int{0, 120, 60} {
			lines := strings.Split(strings.TrimSuffix(render(t, model, diffview.Options{Width: width}), "\n"), "\n")
			rows := lines[5:]

			require.Len(t, rows, 3)

			// The RULE cell starts at the same column on every row.
			before, _, found := strings.Cut(rows[0], "✗")
			require.True(t, found, rows[0])

			column := displayWidth(before)

			for _, row := range rows {
				before, _, found := strings.Cut(row, "✗")
				require.True(t, found, row)
				assert.Equal(t, column, displayWidth(before), "width %d: %q", width, row)

				if width > 0 {
					assert.LessOrEqual(t, displayWidth(row), width, row)
				}
			}

			if width > 0 {
				assert.Contains(t, rows[0], strings.Repeat("\u754c", 11)+"…", "width %d", width)
			}
		}
	})

	t.Run("format characters that reorder or hide text are removed", func(t *testing.T) {
		t.Parallel()

		model := built(t, builtRow{
			"modules.a\u200b.type", "ab\u202ecd", "\u2066x\u2069\ufeff", diffs.VerdictBlocked, "do\u202enot",
		})

		output := render(t, model, diffview.Options{})
		row := parse(t, output).rows[0]

		assert.Equal(t, "modules.a.type", row.field)
		assert.Equal(t, "abcd", row.current)
		assert.Equal(t, "x", row.updated)
		assert.Equal(t, "✗ blocked: donot", row.rule)

		for _, r := range output {
			assert.False(t, unicode.Is(unicode.Cf, r), "format character %U in the output", r)
		}
	})
}

func TestRender_ColourAndSanitizing(t *testing.T) {
	t.Parallel()

	withVerdicts := func(t *testing.T) diffview.Model {
		t.Helper()

		return built(
			t,
			builtRow{"modules.dr.type", "a", "b", diffs.VerdictBlocked, "cannot change"},
			builtRow{"modules.logging.type", "a", "b", diffs.VerdictNeedsConfirmation, "review this"},
			builtRow{"modules.tracing.type", "a", "b", diffs.VerdictMigration, "moves data"},
		)
	}

	t.Run("no colour when colour is disabled", func(t *testing.T) {
		t.Parallel()

		assert.NotContains(t, render(t, withVerdicts(t), diffview.Options{Colour: false}), esc)
		assert.NotContains(t, render(t, simulationModel(t), diffview.Options{Colour: false, Width: 80}), esc)
	})

	t.Run("colour does not change the alignment", func(t *testing.T) {
		t.Parallel()

		for _, model := range []diffview.Model{withVerdicts(t), simulationModel(t)} {
			for _, width := range []int{0, 120, 80, 30} {
				plain := render(t, model, diffview.Options{Colour: false, Width: width})
				coloured := render(t, model, diffview.Options{Colour: true, Width: width})

				assert.Contains(t, coloured, esc)
				assert.Equal(t, plain, colourSequence.ReplaceAllString(coloured, ""), "width %d", width)
			}
		}
	})

	t.Run("colour is only on the markers, the RULE symbols and the numbers of the summary", func(t *testing.T) {
		t.Parallel()

		coloured := render(t, withVerdicts(t), diffview.Options{Colour: true})

		// What is left when every coloured piece is taken out with its sequences.
		piece := regexp.MustCompile(esc + `\[[0-9;]*m([^` + esc + `]*)` + esc + `\[0m`)

		matches := piece.FindAllStringSubmatch(coloured, -1)

		pieces := make([]string, 0, len(matches))
		for _, m := range matches {
			pieces = append(pieces, m[1])
		}

		assert.Equal(t, []string{"3", "1", "1", "~", "✗", "~", "!", "~", "·"}, pieces)
		assert.NotContains(t, piece.ReplaceAllString(coloured, ""), esc)
	})

	t.Run("control characters in values are removed", func(t *testing.T) {
		t.Parallel()

		current := config(t, `{spec: {distribution: {modules: {ingress: {baseDomain: example.test}}}}}`)
		updated := config(t, `{spec: {distribution: {modules: {ingress: {baseDomain: "a\e[31mb\nc"}}}}}`)

		output := render(t, modelOf(t, current, updated), diffview.Options{})
		tb := parse(t, output)

		assert.Equal(t, "a[31mb c", tb.row(t, "modules.ingress.baseDomain").updated)
		assert.Len(t, tb.lines, 6)
		assert.NotContains(t, output, esc)
	})

	t.Run("control characters in a field name, a label and a rule text are removed", func(t *testing.T) {
		t.Parallel()

		current := config(t, `{spec: {kubernetes: {nodeGroups: [{name: "wor\tker\r", nodes: 1}]}}}`)
		updated := config(t, `{spec: {kubernetes: {nodeGroups: [{name: "wor\tker\r", nodes: 2}]}}}`)

		change := changeOf(".spec.kubernetes.nodeGroups.0.nodes", 1, 2)
		keyChange := changeOf(".spec.kubernetes.a\x07b\nc", "x\x00y", "z\x7f")

		model := diffview.NewModel([]diffs.ChangeVerdict{
			{Change: keyChange, Reported: keyChange, Verdict: diffs.VerdictNone},
			{Change: change, Reported: change, Verdict: diffs.VerdictBlocked, Text: "do\x1b[2Jnot\r\nchange\tit"},
		}, current, updated)

		output := render(t, model, diffview.Options{})
		tb := parse(t, output)

		assert.Len(t, tb.lines, 7)
		assert.Equal(t, "CONFIG   2 changes · 1 blocked", tb.lines[0])

		key := tb.row(t, "ab c")
		assert.Equal(t, "xy", key.current)
		assert.Equal(t, "z", key.updated)

		labelled := tb.row(t, "nodeGroups[wor ker].nodes")
		assert.Equal(t, "✗ blocked: do[2Jnot change it", labelled.rule)

		for _, r := range strings.ReplaceAll(output, "\n", "") {
			assert.False(t, unicode.IsControl(r), "control character %U in the output", r)
		}
	})
}

func changeOf(path string, from, to any) r3diff.Change {
	return r3diff.Change{
		Type: r3diff.UPDATE,
		Path: strings.Split(strings.TrimPrefix(path, "."), "."),
		From: from,
		To:   to,
	}
}

// The table tests compare the coloured output with the plain one after removing the
// colour sequences, which holds wherever the sequences are. These tests check where
// they are at the end of a line: no colour on a symbol the ellipsis replaces, and a
// cell that ends its line keeps its colour.
func TestRender_ColourAtTheEndOfALine(t *testing.T) {
	t.Parallel()

	t.Run("a symbol the ellipsis replaces gets no colour", func(t *testing.T) {
		t.Parallel()

		model := built(t, builtRow{"modules.logging.type", "loki", "none", diffs.VerdictBlocked, "not allowed"})

		// On a terminal too narrow for the columns at the width of their headers, the
		// line is cut where the symbol is.
		const width = 25

		plain := strings.Split(render(t, model, diffview.Options{Width: width}), "\n")[rowLineIndex]
		coloured := strings.Split(render(t, model, diffview.Options{Colour: true, Width: width}), "\n")[rowLineIndex]

		require.True(t, strings.HasSuffix(plain, "  …"), plain)
		assert.Equal(t, yellowMarker+plain[1:], coloured)
	})

	t.Run("a coloured cell that ends its line keeps its colour", func(t *testing.T) {
		t.Parallel()

		// A row with empty cells and no verdict is its marker alone.
		model := diffview.Model{
			Groups: []diffview.Group{{
				Title: titleDistribution,
				Rows:  []diffview.Row{{Marker: diffview.MarkerChanged, Path: []string{"spec", "distribution", "a"}}},
			}},
			Changes:        1,
			RulesAvailable: true,
		}

		coloured := strings.Split(render(t, model, diffview.Options{Colour: true}), "\n")

		assert.Equal(t, yellowMarker, coloured[rowLineIndex])
	})
}
