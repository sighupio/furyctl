// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package statusx

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
)

var (
	// For example "10:20:30AM: ---- waiting on 3 changes [53/56 done] ----". The other forms are
	// "applying 9 changes", "applying complete" and "waiting complete".
	kappProgress = regexp.MustCompile(`---- (applying|waiting)(?: on)? (?:\d+ changes|complete) \[(\d+)/(\d+) done\] ----`)
	// For example "TASK [kubernetes-node : Install kubelet] ****".
	ansibleTask = regexp.MustCompile(`^TASK \[(.+)\]`)
)

// Scanner reads the output of a command, without colors, and reports the lines that show progress:
// the change counters of kapp and the task names of ansible. Give one to each stream of a command,
// so that a partial stdout line and a partial stderr line never join.
type Scanner struct {
	partial []byte
}

func (s *Scanner) Write(p []byte) (int, error) {
	s.partial = append(s.partial, p...)

	for {
		line, rest, ok := bytes.Cut(s.partial, []byte("\n"))
		if !ok {
			break
		}

		scanLine(string(bytes.TrimRight(line, "\r")))

		s.partial = rest
	}

	// Copy the partial line, so that the buffer of the lines already read can go.
	s.partial = bytes.Clone(s.partial)

	return len(p), nil
}

func scanLine(line string) {
	if m := kappProgress.FindStringSubmatch(line); m != nil {
		done, _ := strconv.Atoi(m[2])  //nolint:errcheck // The pattern gives digits only.
		total, _ := strconv.Atoi(m[3]) //nolint:errcheck // The pattern gives digits only.

		// The kapp apply and wait run at the same time. Only the wait counts the changes that are ready.
		if m[1] == "applying" {
			Detail(fmt.Sprintf("Applying the changes: %d of %d done", done, total), noProgress)
		} else {
			Detail(fmt.Sprintf("Waiting for the changes: %d of %d done", done, total), done*maxProgress/max(total, 1))
		}

		return
	}

	if m := ansibleTask.FindStringSubmatch(line); m != nil {
		Detail(m[1], noProgress)
	}
}
