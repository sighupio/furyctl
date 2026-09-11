// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package secrets

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ErrConfigHoldsNoFields reports a configuration file that furyctl cannot write a field in. The
// file is empty, or it holds no fields.
var ErrConfigHoldsNoFields = errors.New("the configuration file holds no fields")

// fieldState tells what furyctl did with one field of the configuration file.
type fieldState int

const (
	// A field that furyctl wrote.
	fieldWritten fieldState = iota

	// A field that already reads the file of the secret.
	fieldSet

	// A field that furyctl did not change, because it holds another value or because its parent
	// cannot hold it, for example a flow mapping.
	fieldKept
)

// UpdateConfig writes a reference to the file of each secret in the configuration file. It returns
// the number of fields that it wrote, and the fields that it did not change because they already
// hold another value.
//
// It edits only the lines of the fields that it writes, and adds the lines of the fields that the
// file does not have yet. It never writes the file again from the parsed YAML. Thus each comment,
// each empty line, and the format of the rest of the file stay as they are.
func UpdateConfig(configPath string, results []Result) (int, []string, error) {
	info, err := os.Stat(configPath)
	if err != nil {
		return 0, nil, fmt.Errorf("error while reading the configuration file %s: %w", configPath, err)
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		return 0, nil, fmt.Errorf("error while reading the configuration file %s: %w", configPath, err)
	}

	written := 0
	kept := []string{}
	lines := strings.Split(string(content), "\n")
	configDir := filepath.Dir(configPath)

	for _, result := range results {
		edited, state, err := setField(lines, result.Path, Reference(result.File, configDir))
		if err != nil {
			return 0, nil, fmt.Errorf("%w: %s", err, configPath)
		}

		switch state {
		case fieldWritten:
			lines = edited
			written++

		case fieldKept:
			kept = append(kept, result.Path)

		// The field reads the file of the secret already (fieldSet), thus nothing changes. A run
		// after the first one ends here.
		default:
		}
	}

	if written == 0 {
		return 0, kept, nil
	}

	// One write for the whole run: the file on disk is either the file from before the command, or
	// the file with each new field in it.
	if err := replaceFile(configPath, strings.Join(lines, "\n"), info.Mode().Perm()); err != nil {
		return 0, nil, err
	}

	return written, kept, nil
}

// setField returns the lines with the value in the field of the path. It parses the lines again for
// each secret, because the edit of the field before this one moved them.
func setField(lines []string, path, value string) ([]string, fieldState, error) {
	var doc yaml.Node

	if err := yaml.Unmarshal([]byte(strings.Join(lines, "\n")), &doc); err != nil {
		return nil, fieldKept, fmt.Errorf("error while parsing the configuration file: %w", err)
	}

	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, fieldKept, ErrConfigHoldsNoFields
	}

	edited, state := editLines(lines, doc.Content[0], strings.Split(path, "."), value)

	return edited, state, nil
}

// replaceFile writes the content to a new file in the folder of the file, then moves that new file
// over it. A command that stops in the middle of the write thus keeps the file as it was.
func replaceFile(path, content string, perm os.FileMode) error {
	// A configuration file can be a symbolic link to a file that more clusters share. The move must
	// replace the file at the end of the link, and keep the link as it is.
	target, err := filepath.EvalSymlinks(path)
	if err != nil {
		return fmt.Errorf("error while resolving the path of the configuration file %s: %w", path, err)
	}

	path = target

	temp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".furyctl-*")
	if err != nil {
		return fmt.Errorf("error while creating a temporary file next to %s: %w", path, err)
	}

	name := temp.Name()

	if _, err := temp.WriteString(content); err != nil {
		return errors.Join(
			fmt.Errorf("error while writing the temporary file %s: %w", name, err),
			temp.Close(),
			os.Remove(name),
		)
	}

	// Close reports the errors that the write kept in the buffer, for example a full disk.
	if err := temp.Close(); err != nil {
		return errors.Join(
			fmt.Errorf("error while closing the temporary file %s: %w", name, err),
			os.Remove(name),
		)
	}

	// A temporary file gets 0600. The configuration file keeps the permissions that it had.
	if err := os.Chmod(name, perm); err != nil {
		return errors.Join(
			fmt.Errorf("error while setting the permissions of %s: %w", name, err),
			os.Remove(name),
		)
	}

	if err := os.Rename(name, path); err != nil {
		return errors.Join(
			fmt.Errorf("error while moving %s over %s: %w", name, path, err),
			os.Remove(name),
		)
	}

	return nil
}

// target tells where the field of a path is in the file, or where it has to go.
type target struct {
	// The value of the field. It is nil when the field is not in the file yet.
	leaf *yaml.Node

	// The deepest value of the path that the file has, with its key.
	parent    *yaml.Node
	parentKey *yaml.Node

	// The fields that furyctl must add under the parent.
	missing []string
}

// editLines returns the lines of the configuration file with the value in the field of the path.
func editLines(lines []string, root *yaml.Node, fields []string, value string) ([]string, fieldState) {
	found := walk(root, fields)

	if found.leaf != nil {
		switch {
		case found.leaf.Value == value:
			return nil, fieldSet

		case isEmpty(found.leaf):
			return replaceValue(lines, found.leaf, value), fieldWritten

		default:
			return nil, fieldKept
		}
	}

	return insertFields(lines, found, value)
}

// walk follows the fields from the root of the file.
func walk(root *yaml.Node, fields []string) target {
	found := target{parent: root}

	for i, field := range fields {
		if found.parent.Kind != yaml.MappingNode {
			found.missing = fields[i:]

			return found
		}

		key, value := child(found.parent, field)
		if value == nil {
			found.missing = fields[i:]

			return found
		}

		if i == len(fields)-1 {
			found.leaf = value

			return found
		}

		found.parentKey, found.parent = key, value
	}

	return found
}

// child returns the key and the value of the field of a mapping. A mapping holds its keys and its
// values in one list, one after the other.
func child(mapping *yaml.Node, field string) (*yaml.Node, *yaml.Node) {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == field {
			return mapping.Content[i], mapping.Content[i+1]
		}
	}

	return nil, nil
}

// isEmpty reports whether the field has no value: an empty string, or nothing at all.
func isEmpty(node *yaml.Node) bool {
	return node.Kind == yaml.ScalarNode && (node.Value == "" || node.Tag == "!!null")
}

// replaceValue writes the value in the line of the field, and keeps the comment of that line.
func replaceValue(lines []string, node *yaml.Node, value string) []string {
	index := node.Line - 1
	if index < 0 || index >= len(lines) {
		return lines
	}

	line := lines[index]
	column := min(max(node.Column-1, 0), len(line))
	head, rest := line[:column], line[column:]

	// The field holds an empty value. Thus the rest of the line is that value, and a comment after
	// it when there is one. Only the value changes: the spaces and the comment stay as they are.
	old := ""
	if fields := strings.Fields(rest); len(fields) > 0 && !strings.HasPrefix(fields[0], "#") {
		old = fields[0]
	}

	tail := rest[len(old):]

	// A comment needs a space before it, and a field with no value at all ends with the colon.
	if strings.HasPrefix(tail, "#") {
		tail = " " + tail
	}

	if !strings.HasSuffix(head, " ") {
		head += " "
	}

	lines[index] = head + quote(value) + tail

	return lines
}

// insertFields adds the fields that the configuration file does not have yet, under the deepest
// field that it has. It reports fieldKept when that field cannot hold them.
func insertFields(lines []string, found target, value string) ([]string, fieldState) {
	if len(found.missing) == 0 {
		return nil, fieldKept
	}

	after, indent, ok := insertionPoint(found)
	if !ok {
		return nil, fieldKept
	}

	after = blockEnd(lines, after, indent)

	added := make([]string, 0, len(found.missing))

	for i, field := range found.missing[:len(found.missing)-1] {
		added = append(added, strings.Repeat(" ", indent+i*snippetIndent)+field+":")
	}

	last := strings.Repeat(" ", indent+(len(found.missing)-1)*snippetIndent) + found.missing[len(found.missing)-1]
	added = append(added, last+": "+quote(value))

	out := make([]string, 0, len(lines)+len(added))
	out = append(out, lines[:after]...)
	out = append(out, added...)
	out = append(out, lines[after:]...)

	return out, fieldWritten
}

// insertionPoint returns the line to add the new fields after, the indentation that they need, and
// whether the parent can hold them.
func insertionPoint(found target) (int, int, bool) {
	switch {
	// The field has no value yet, for example `pomerium:`. The new fields go under it, one level
	// deeper than its key.
	case isEmpty(found.parent):
		if found.parentKey == nil {
			return 0, 0, false
		}

		return found.parent.Line, found.parentKey.Column - 1 + snippetIndent, true

	// The field is a block mapping with fields in it. The new fields go after the last line of the
	// mapping, with the same indentation as the fields that are already there.
	case found.parent.Kind == yaml.MappingNode && found.parent.Style == 0 && len(found.parent.Content) > 0:
		return lastLine(found.parent), found.parent.Content[0].Column - 1, true

	// A flow mapping, a sequence or a value: furyctl cannot add a field to it.
	default:
		return 0, 0, false
	}
}

// blockEnd returns the first line that the new fields can go before. The parser gives no line for
// the text of a block scalar, for example the lines of a certificate or of a script. Such a line
// belongs to the value before the new fields, thus its indentation is deeper than theirs.
func blockEnd(lines []string, after, indent int) int {
	end := after

	for i := after; i < len(lines); i++ {
		// An empty line does not end the block: the text of a block scalar can hold one.
		if strings.TrimSpace(lines[i]) == "" {
			continue
		}

		if indentOf(lines[i]) <= indent {
			break
		}

		end = i + 1
	}

	return end
}

// indentOf returns the number of spaces at the start of the line. YAML rejects a tab there.
func indentOf(line string) int {
	return len(line) - len(strings.TrimLeft(line, " "))
}

// lastLine returns the last line of the value, and of each value in it.
func lastLine(node *yaml.Node) int {
	last := node.Line

	for _, item := range node.Content {
		if line := lastLine(item); line > last {
			last = line
		}
	}

	return last
}

// quote returns the value between double quotes. The value starts with a brace, thus YAML reads a
// value with no quotes as a mapping.
func quote(value string) string {
	return `"` + value + `"`
}
