// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package terraform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

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

		var msg struct {
			Message string `json:"@message"`
		}

		if err := json.Unmarshal(line, &msg); err == nil && msg.Message != "" {
			line = []byte(msg.Message + "\n")
		}

		if _, err := j.w.Write(line); err != nil {
			return len(p), fmt.Errorf("error writing terraform message: %w", err)
		}

		j.partial = j.partial[i+1:]
	}
}
