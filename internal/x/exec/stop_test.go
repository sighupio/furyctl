// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package execx

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestStopAllReachesDescendants(t *testing.T) {
	testCases := map[string]struct {
		script string
		sig    syscall.Signal
	}{
		"descendant gets the signal": {script: "sleep 60 & echo $! > %s; wait", sig: syscall.SIGTERM},
		// A shell job started with & ignores SIGINT and keeps the output pipes open after sh exits.
		"leftover that ignores the signal": {script: "sleep 60 & echo $! > %s", sig: syscall.SIGINT},
		// Ignored signals are inherited, so the job ignores SIGTERM too and needs the SIGKILL.
		"leftover that ignores SIGTERM": {script: "trap '' TERM; sleep 60 & echo $! > %s", sig: syscall.SIGINT},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			leftoverGracePeriod = time.Second

			t.Cleanup(func() {
				leftoverGracePeriod = 10 * time.Second

				runningMu.Lock()
				stopping = false
				runningMu.Unlock()
			})

			pidFile := filepath.Join(t.TempDir(), "pid")
			cmd := NewCmd("sh", CmdOptions{Args: []string{"-c", fmt.Sprintf(tc.script, pidFile)}})

			go cmd.Run() //nolint:errcheck // Run never returns after stopAll.

			var pid int

			waitFor(t, func() bool {
				out, err := os.ReadFile(pidFile)
				pid, _ = strconv.Atoi(strings.TrimSpace(string(out)))

				return err == nil && pid > 0
			})

			done := make(chan struct{})

			go func() {
				stopAll(tc.sig, nil)
				close(done)
			}()

			select {
			case <-done:
			case <-time.After(10 * time.Second):
				t.Fatal("stopAll did not return")
			}

			waitFor(t, func() bool { return syscall.Kill(pid, 0) != nil })
		})
	}
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()

	for range 100 {
		if cond() {
			return
		}

		time.Sleep(50 * time.Millisecond)
	}

	t.Fatal("the condition did not become true")
}
