// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package diffview

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"

	textwidth "golang.org/x/text/width"

	"github.com/sighupio/furyctl/pkg/diffs"
)

const (
	// The widest a column gets on a terminal, in characters.
	maxFieldWidth = 40
	maxValueWidth = 24

	// The narrowest a column gets on a terminal narrower than the table, before every
	// column gets down to the width of its header.
	minFieldWidth = 24
	minValueWidth = 12

	// The shortest explanation of a rule worth showing after a cut: ": x…".
	minExplanationWidth = 4

	// The columns of a table after the marker, and the width of a wide character.
	columnCount   = 4
	wideRuneWidth = 2

	ellipsis      = '…'
	columnGap     = "  "
	separatorRune = "─"

	// The symbols that open the RULE cell of a verdict.
	symbolBlocked           = "✗"
	symbolNeedsConfirmation = "!"
	symbolMigration         = "·"

	colourReset  = "\x1b[0m"
	colourBold   = "\x1b[1m"
	colourRed    = "\x1b[31m"
	colourGreen  = "\x1b[32m"
	colourYellow = "\x1b[33m"
	colourCyan   = "\x1b[36m"

	noRulesLine = "RULE   not available: the distribution has no rules file for this kind"
)

// Options are the choices of the terminal the table is painted for.
type Options struct {
	// Colour writes colour escape sequences.
	Colour bool
	// Width is the width of the terminal in columns. Zero means no terminal:
	// nothing is cut.
	Width int
}

// span is a run of characters of a line that gets a colour.
type span struct {
	start  int
	end    int
	colour string
}

// line is one line of the table: its characters, aligned and cut before any colour
// is added, so that the colour sequences never count as width.
type line struct {
	text  []rune
	spans []span
}

func plainLine(text string) line {
	return line{text: []rune(text), spans: nil}
}

// add appends text to the line, coloured when a colour is given.
func (l *line) add(text, colour string) {
	start := len(l.text)
	l.text = append(l.text, []rune(text)...)

	if colour != "" && len(l.text) > start {
		l.spans = append(l.spans, span{start: start, end: len(l.text), colour: colour})
	}
}

// fit drops the spaces a short row leaves at its end and then cuts the line to the
// width of the terminal, with an ellipsis as its last character. The spaces go first,
// so a row is cut only when a cell does not fit.
func (l *line) fit(width int) {
	for len(l.text) > 0 && l.text[len(l.text)-1] == ' ' {
		l.text = l.text[:len(l.text)-1]
	}

	if width > 0 {
		l.text = cutCell(l.text, width)
	}
}

// plain gives the line without colour.
func (l *line) plain() string {
	return string(l.text)
}

// coloured gives the line with its colour sequences. A span the cut removed, or that
// the ellipsis now covers, gets no colour.
func (l *line) coloured() string {
	var out strings.Builder

	at := 0

	for _, s := range l.spans {
		if s.end > len(l.text) || (s.end == len(l.text) && l.text[s.end-1] == ellipsis) {
			continue
		}

		out.WriteString(string(l.text[at:s.start]))
		out.WriteString(s.colour + string(l.text[s.start:s.end]) + colourReset)

		at = s.end
	}

	out.WriteString(string(l.text[at:]))

	return out.String()
}

// paintedRow is a row with its cells sanitized and its RULE cell built: the symbol
// and the label of the verdict, then the explanation, the text of the rule.
type paintedRow struct {
	marker      Marker
	field       []rune
	current     []rune
	updated     []rune
	symbol      string
	label       string
	explanation string
}

// Render paints the table. A model that breaks an invariant is not painted.
func Render(w io.Writer, model Model, options Options) error {
	if err := model.Validate(); err != nil {
		return err
	}

	var out strings.Builder

	paint := (*line).plain
	if options.Colour {
		paint = (*line).coloured
	}

	for _, l := range tableLines(model, options) {
		l.fit(options.Width)

		out.WriteString(paint(&l))
		out.WriteString("\n")
	}

	if _, err := io.WriteString(w, out.String()); err != nil {
		return fmt.Errorf("error writing the CONFIG table: %w", err)
	}

	return nil
}

func tableLines(model Model, options Options) []line {
	if model.Changes == 0 {
		return []line{plainLine("CONFIG   no changes")}
	}

	groups := make([][]paintedRow, len(model.Groups))
	widths := headerWidths()

	for i, group := range model.Groups {
		for _, row := range group.Rows {
			painted := paintRow(row)

			// Only a terminal cuts a cell.
			if options.Width > 0 {
				painted.cut(columnWidths{field: maxFieldWidth, current: maxValueWidth, updated: maxValueWidth})
			}

			widths.field = max(widths.field, textWidth(painted.field))
			widths.current = max(widths.current, textWidth(painted.current))
			widths.updated = max(widths.updated, textWidth(painted.updated))
			widths.verdict = max(widths.verdict, textWidth([]rune(painted.symbol+painted.label)))

			groups[i] = append(groups[i], painted)
		}
	}

	if options.Width > 0 {
		widths = widths.narrowed(options.Width)

		for _, rows := range groups {
			for j := range rows {
				rows[j].cut(widths)
			}
		}
	}

	header := headerLine(widths)
	header.fit(options.Width)

	lines := []line{
		summaryLine(model),
		plainLine(""),
		header,
		// The separator is as long as the header row the terminal shows.
		plainLine(strings.Repeat(separatorRune, textWidth(header.text))),
	}

	for i, group := range model.Groups {
		lines = append(lines, plainLine(group.Title))

		for _, row := range groups[i] {
			lines = append(lines, rowLine(row, widths, options.Width))
		}
	}

	if !model.RulesAvailable {
		lines = append(lines, plainLine(""), plainLine(noRulesLine))
	}

	return lines
}

// columnWidths are the widths of the columns of a table, in terminal columns. The
// verdict is the widest symbol and label of the RULE column, without explanation.
type columnWidths struct {
	field   int
	current int
	updated int
	verdict int
}

// headerWidths gives the widths of the columns of a table without rows: the widths
// of their headers.
func headerWidths() columnWidths {
	return columnWidths{field: len("FIELD"), current: len("CURRENT"), updated: len("NEW"), verdict: len("RULE")}
}

// total gives the width of a row whose RULE cell has no explanation: the marker and
// the four columns with the gaps between them.
func (w columnWidths) total() int {
	return len("~") + columnCount*len(columnGap) + w.field + w.current + w.updated + w.verdict
}

// narrowed gives the widths of the columns on a terminal narrower than the table, so
// that the symbol and the label of every verdict stay whole. CURRENT and NEW give up
// space first, down to their minimum, then FIELD down to its own; on a terminal still
// too narrow, the three get down to the width of their header. The explanation of a
// rule gets what is left of the line.
func (w columnWidths) narrowed(width int) columnWidths {
	headers := headerWidths()

	excess := w.total() - width
	excess = shrinkWidest(excess, shrinkable{&w.current, minValueWidth}, shrinkable{&w.updated, minValueWidth})
	excess = shrinkWidest(excess, shrinkable{&w.field, minFieldWidth})
	shrinkWidest(
		excess,
		shrinkable{&w.field, headers.field},
		shrinkable{&w.current, headers.current},
		shrinkable{&w.updated, headers.updated},
	)

	return w
}

// shrinkable is the width of a column and the narrowest it may get.
type shrinkable struct {
	width *int
	floor int
}

// shrinkWidest takes excess columns away from the widest of the columns, one at a
// time, without going below their floors. It gives the excess it could not take away.
func shrinkWidest(excess int, columns ...shrinkable) int {
	for excess > 0 {
		var widest *int

		for _, column := range columns {
			if *column.width > column.floor && (widest == nil || *column.width > *widest) {
				widest = column.width
			}
		}

		if widest == nil {
			return excess
		}

		*widest--
		excess--
	}

	return 0
}

func summaryLine(model Model) line {
	var l line

	noun := "changes"
	if model.Changes == 1 {
		noun = "change"
	}

	l.add("CONFIG   ", "")
	l.add(strconv.Itoa(model.Changes), colourBold)
	l.add(" "+noun, "")

	if model.Blocked > 0 {
		l.add(" · ", "")
		l.add(strconv.Itoa(model.Blocked), colourRed)
		l.add(" blocked", "")
	}

	if model.NeedsConfirmation > 0 {
		l.add(" · ", "")
		l.add(strconv.Itoa(model.NeedsConfirmation), colourYellow)
		l.add(" needs confirmation", "")
	}

	return l
}

// headerLine starts with three spaces, the width of a marker and its gap, so that
// FIELD is above the FIELD cells.
func headerLine(widths columnWidths) line {
	var l line

	l.add("   ", "")
	l.add(pad([]rune("FIELD"), widths.field)+columnGap, "")
	l.add(pad([]rune("CURRENT"), widths.current)+columnGap, "")
	l.add(pad([]rune("NEW"), widths.updated)+columnGap, "")
	l.add("RULE", "")

	return l
}

// rowLine builds the line of a row. On a terminal, the explanation of the rule is cut
// to what is left of the line, or left out when too little is left, so that a cut
// never reaches the symbol and the label of the verdict.
func rowLine(row paintedRow, widths columnWidths, width int) line {
	var l line

	l.add(string(row.marker), markerColour(row.marker))
	l.add(columnGap, "")
	l.add(pad(row.field, widths.field)+columnGap, "")
	l.add(pad(row.current, widths.current)+columnGap, "")
	l.add(pad(row.updated, widths.updated)+columnGap, "")
	l.add(row.symbol, verdictColour(row.symbol))
	l.add(row.label, "")

	explanation := []rune(row.explanation)

	if width > 0 && textWidth(l.text)+textWidth(explanation) > width {
		explanation = nil

		if left := width - textWidth(l.text); left >= minExplanationWidth {
			explanation = cutCell([]rune(row.explanation), left)
		}
	}

	l.add(string(explanation), "")

	return l
}

// pad fills a cell with spaces up to the width of its column.
func pad(cell []rune, width int) string {
	return string(cell) + strings.Repeat(" ", max(0, width-textWidth(cell)))
}

// cut cuts the cells wider than their column.
func (r *paintedRow) cut(widths columnWidths) {
	r.field = cutCell(r.field, widths.field)
	r.current = cutCell(r.current, widths.current)
	r.updated = cutCell(r.updated, widths.updated)
}

// paintRow sanitizes the cells of a row and builds its RULE cell.
func paintRow(row Row) paintedRow {
	painted := paintedRow{
		marker:      row.Marker,
		field:       []rune(sanitize(row.Field)),
		current:     []rune(sanitize(row.Current)),
		updated:     []rune(sanitize(row.New)),
		symbol:      "",
		label:       "",
		explanation: "",
	}

	if symbol, label, ok := verdictLabel(row.Verdict); ok {
		painted.symbol = symbol
		painted.label = " " + label

		if text := sanitize(row.Text); text != "" {
			painted.explanation = ": " + text
		}
	}

	return painted
}

// cutCell cuts a text wider than width so that it ends with an ellipsis and fits in
// width columns.
func cutCell(cell []rune, width int) []rune {
	if textWidth(cell) <= width {
		return cell
	}

	kept, used := 0, 0

	for kept < len(cell) && used+runeWidth(cell[kept]) <= width-1 {
		used += runeWidth(cell[kept])
		kept++
	}

	return append(cell[:kept:kept], ellipsis)
}

// textWidth gives the number of terminal columns a text takes.
func textWidth(text []rune) int {
	total := 0

	for _, r := range text {
		total += runeWidth(r)
	}

	return total
}

// runeWidth gives the number of terminal columns a character takes: none for a mark
// that combines with the character before it, two for a wide East Asian character
// and one for the others.
func runeWidth(r rune) int {
	if unicode.In(r, unicode.Mn, unicode.Me) {
		return 0
	}

	switch textwidth.LookupRune(r).Kind() {
	case textwidth.EastAsianWide, textwidth.EastAsianFullwidth:
		return wideRuneWidth

	case textwidth.Neutral, textwidth.EastAsianAmbiguous, textwidth.EastAsianNarrow, textwidth.EastAsianHalfwidth:
		fallthrough

	default:
		return 1
	}
}

// verdictLabel gives the symbol and the label of the RULE cell of a verdict. A row
// without verdict has an empty cell.
func verdictLabel(verdict diffs.Verdict) (string, string, bool) {
	switch verdict {
	case diffs.VerdictBlocked:
		return symbolBlocked, "blocked", true

	case diffs.VerdictNeedsConfirmation:
		return symbolNeedsConfirmation, "needs confirmation", true

	case diffs.VerdictMigration:
		return symbolMigration, "migration", true

	case diffs.VerdictNone:
		fallthrough

	default:
		return "", "", false
	}
}

func verdictColour(symbol string) string {
	switch symbol {
	case symbolBlocked:
		return colourRed

	case symbolNeedsConfirmation:
		return colourYellow

	case symbolMigration:
		return colourCyan

	default:
		return ""
	}
}

func markerColour(marker Marker) string {
	switch marker {
	case MarkerAdded:
		return colourGreen

	case MarkerRemoved:
		return colourRed

	case MarkerChanged:
		return colourYellow

	default:
		return ""
	}
}

// sanitize removes the control and format characters of a text that comes from a
// configuration or a rule file, so that it cannot move the cursor, change colours,
// reverse the direction of the text, hide characters or break the row: a line break
// or a tab becomes one space and the others are dropped.
func sanitize(text string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r == '\n' || r == '\t':
			return ' '

		case unicode.IsControl(r), unicode.Is(unicode.Cf, r):
			return -1

		default:
			return r
		}
	}, text)
}
