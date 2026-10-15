// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package kapp

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"
)

const (
	targetPrefix    = "Target cluster"
	kappErrorPrefix = "kapp: Error:"
)

var (
	ErrInvalidDiff = errors.New("kapp output did not match the expected format")

	whitespaceAfterNewlineRe = regexp.MustCompile(`\s*\n\s*`)
)

// Row is one resource row of a kapp diff table.
type Row struct {
	Namespace string
	Name      string
	Kind      string
	Op        string
}

// DiffResult is the content of a `kapp deploy --diff-run --json` file. KappError and Target are filled even when
// the file is not valid.
type DiffResult struct {
	Rows      []Row
	Target    string
	KappError string
}

// Changes lists the rows that create, update or delete a resource. Rows of resources that exist or that kapp
// leaves untouched are not changes.
func (r DiffResult) Changes() []Row {
	changes := []Row{}

	for _, row := range r.Rows {
		if slices.Contains([]string{"create", "update", "delete"}, row.Op) {
			changes = append(changes, row)
		}
	}

	return changes
}

type diffTable struct {
	Rows []map[string]any `json:"Rows"` //nolint:tagliatelle // kapp prints its keys capitalized.
}

type diffFile struct {
	Tables []diffTable `json:"Tables"` //nolint:tagliatelle // kapp prints its keys capitalized.
	Lines  []string    `json:"Lines"`  //nolint:tagliatelle // kapp prints its keys capitalized.
}

// ParseDiff reads a kapp diff file. It returns ErrInvalidDiff, together with the kapp error and target it found,
// when the file is missing or does not match the expected format.
func ParseDiff(path string) (DiffResult, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return DiffResult{}, fmt.Errorf("%w: %w", ErrInvalidDiff, err)
	}

	var file diffFile

	if err := json.Unmarshal(content, &file); err != nil {
		return DiffResult{}, fmt.Errorf("%w: %w", ErrInvalidDiff, err)
	}

	res := DiffResult{}
	res.Target, res.KappError = readLines(file.Lines)

	if len(file.Tables) == 0 {
		return res, fmt.Errorf("%w: no table", ErrInvalidDiff)
	}

	for _, table := range file.Tables {
		for _, raw := range table.Rows {
			row, err := readRow(raw)
			if err != nil {
				return res, err
			}

			res.Rows = append(res.Rows, row)
		}
	}

	return res, nil
}

// readLines reads the target cluster line and the kapp error out of the output lines of the file.
func readLines(lines []string) (string, string) {
	target, kappError := "", ""

	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, targetPrefix):
			target = line
		case strings.HasPrefix(line, kappErrorPrefix):
			kappError = whitespaceAfterNewlineRe.ReplaceAllString(line, " ")
		default:
			// The other lines of the output are not read.
		}
	}

	return target, kappError
}

func readRow(raw map[string]any) (Row, error) {
	fields := map[string]string{}

	for _, key := range []string{"namespace", "name", "kind", "op"} {
		value, ok := raw[key].(string)
		if !ok {
			return Row{}, fmt.Errorf("%w: field %s is not a string", ErrInvalidDiff, key)
		}

		fields[key] = value
	}

	if fields["name"] == "" || fields["kind"] == "" {
		return Row{}, fmt.Errorf("%w: row without name or kind", ErrInvalidDiff)
	}

	if !slices.Contains([]string{"create", "update", "delete", "exists", ""}, fields["op"]) {
		return Row{}, fmt.Errorf("%w: unknown op %q", ErrInvalidDiff, fields["op"])
	}

	return Row{Namespace: fields["namespace"], Name: fields["name"], Kind: fields["kind"], Op: fields["op"]}, nil
}
