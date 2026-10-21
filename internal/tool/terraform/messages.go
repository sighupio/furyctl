// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package terraform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// jsonLine is a line that `terraform -json` prints.
type jsonLine struct {
	Level      string         `json:"@level"`
	Message    string         `json:"@message"`
	Diagnostic jsonDiagnostic `json:"diagnostic"`
}

type jsonDiagnostic struct {
	Detail string `json:"detail"`
}

// jsonText converts the lines that `terraform -json` prints to their messages. An error also gives the
// detail of its diagnostic, indented. Lines that are not JSON, or have no message, stay as they are.
func jsonText(lines []string) []string {
	text := make([]string, 0, len(lines))

	for _, l := range lines {
		var j jsonLine

		if err := json.Unmarshal([]byte(l), &j); err != nil || j.Message == "" {
			text = append(text, l)

			continue
		}

		text = append(text, strings.Split(j.Message, "\n")...)

		if j.Level == "error" && j.Diagnostic.Detail != "" {
			for d := range strings.SplitSeq(strings.TrimRight(j.Diagnostic.Detail, "\n"), "\n") {
				text = append(text, "  "+d)
			}
		}
	}

	return text
}

// jsonMessageWriter forwards the "@message" field of each JSON line that `terraform -json` prints.
// Lines that are not JSON, or have no message, go through unchanged. Use one for each stream.
type jsonMessageWriter struct {
	w       io.Writer
	partial []byte
}

func (j *jsonMessageWriter) Write(p []byte) (int, error) {
	j.partial = append(j.partial, p...)

	for {
		i := bytes.IndexByte(j.partial, '\n')
		if i < 0 {
			return len(p), nil
		}

		line := j.partial[:i+1]

		var msg jsonLine

		if err := json.Unmarshal(line, &msg); err == nil && msg.Message != "" {
			line = []byte(msg.Message + "\n")
		}

		if _, err := j.w.Write(line); err != nil {
			return len(p), fmt.Errorf("error writing terraform message: %w", err)
		}

		j.partial = j.partial[i+1:]
	}
}
