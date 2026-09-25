// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package execx

import (
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
)

//nolint:gochecknoglobals // The running commands are shared between all the command instances.
var (
	runningMu sync.Mutex
	running   = map[*exec.Cmd]struct{}{}
	runningWg sync.WaitGroup
	stopping  bool
	onStop    func()

	leftoverCheckInterval = 500 * time.Millisecond
	leftoverGracePeriod   = 10 * time.Second
)

// HandleSignals installs the handler for SIGINT and SIGTERM. The handler stops the running commands
// and their descendants. Then it runs the OnStop function and exits furyctl with code 1.
// A second signal kills the commands.
func HandleSignals() {
	sigs := make(chan os.Signal, 1)

	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := syscall.SIGTERM
		if <-sigs == os.Interrupt {
			sig = syscall.SIGINT
		}

		stopAll(sig, sigs)

		runningMu.Lock()
		f := onStop
		runningMu.Unlock()

		if f != nil {
			f()
		}

		os.Exit(1) //nolint:revive // deep-exit is acceptable in the signal handler.
	}()
}

// OnStop sets f to run when a signal stops furyctl, after the running commands exit.
// A nil f removes the function.
func OnStop(f func()) {
	runningMu.Lock()
	defer runningMu.Unlock()

	onStop = f
}

// stopAll sends sig to every running command, blocks new ones and waits for them to exit.
// If force receives a value, stopAll kills the commands and returns immediately.
func stopAll(sig syscall.Signal, force <-chan os.Signal) {
	runningMu.Lock()
	stopping = true
	runningMu.Unlock()

	if names := signalAll(sig); len(names) > 0 {
		logrus.Warnf("Stopping the running commands: %s. To kill them now, send the signal again (Ctrl-C).",
			strings.Join(names, ", "))
	}

	done := make(chan struct{})

	go func() {
		runningWg.Wait()
		close(done)
	}()

	ticker := time.NewTicker(leftoverCheckInterval)
	defer ticker.Stop()

	leftovers := map[*exec.Cmd]time.Time{}

	for {
		select {
		case <-done:
			return

		case <-force:
			if names := signalAll(syscall.SIGKILL); len(names) > 0 {
				logrus.Warnf("Killing the running commands: %s.", strings.Join(names, ", "))
			}

			return

		case <-ticker.C:
			// Wait also waits for the output pipes. A descendant that ignores sig (for example a
			// shell job started with &) keeps them open after its command exits.
			stopLeftovers(leftovers)
		}
	}
}

// signalAll sends sig to the running commands and returns their names.
func signalAll(sig syscall.Signal) []string {
	runningMu.Lock()
	defer runningMu.Unlock()

	names := make([]string, 0, len(running))

	for c := range running {
		names = append(names, cmdName(c))

		// On Ctrl-C the terminal already sent SIGINT to the commands in the process group of furyctl.
		if sig == syscall.SIGINT && !ownGroup(c) {
			continue
		}

		_ = kill(c, sig) //nolint:errcheck // The command can exit in the meantime.
	}

	return names
}

// stopLeftovers stops the descendants of each command that exited while its descendants still run.
// It sends SIGTERM first, then SIGKILL after leftoverGracePeriod. The leftovers map holds the time
// of the SIGTERM for each command, or the zero time after the SIGKILL.
func stopLeftovers(leftovers map[*exec.Cmd]time.Time) {
	runningMu.Lock()
	defer runningMu.Unlock()

	for c := range running {
		if !ownGroup(c) || !errors.Is(syscall.Kill(c.Process.Pid, 0), syscall.ESRCH) {
			continue
		}

		name := cmdName(c)
		termAt, ok := leftovers[c]

		if !ok {
			logrus.Warnf("%s exited, but its child processes still run. Sending SIGTERM to them.", name)

			leftovers[c] = time.Now()
			_ = kill(c, syscall.SIGTERM) //nolint:errcheck // The group can exit in the meantime.
		} else if !termAt.IsZero() && time.Since(termAt) > leftoverGracePeriod {
			logrus.Warnf("The child processes of %s did not stop in %s. Killing them.", name, leftoverGracePeriod)

			leftovers[c] = time.Time{}
			_ = kill(c, syscall.SIGKILL) //nolint:errcheck // The group can exit in the meantime.
		}
	}
}

// run runs c and tracks it for stopAll. After stopAll starts, run never returns: the signal
// handler exits furyctl, so callers do not start other commands or exit before the cleanup.
func run(c *exec.Cmd) error {
	runningMu.Lock()

	if stopping {
		runningMu.Unlock()
		select {}
	}

	if err := c.Start(); err != nil {
		runningMu.Unlock()

		return err //nolint:wrapcheck // The caller wraps it.
	}

	running[c] = struct{}{}
	runningWg.Add(1)
	runningMu.Unlock()

	err := c.Wait()

	runningMu.Lock()
	delete(running, c)
	runningWg.Done()
	blocked := stopping
	runningMu.Unlock()

	if blocked {
		select {}
	}

	return err //nolint:wrapcheck // The caller wraps it.
}

// kill sends sig to the process group of c. If c stays in the process group of furyctl, kill sends
// sig only to the process of c.
func kill(c *exec.Cmd, sig syscall.Signal) error {
	pid := c.Process.Pid
	if ownGroup(c) {
		pid = -pid
	}

	return syscall.Kill(pid, sig) //nolint:wrapcheck // The caller wraps it.
}

func ownGroup(c *exec.Cmd) bool {
	return c.SysProcAttr != nil && c.SysProcAttr.Setpgid
}

// cmdName returns the program of c and its first argument, for example "kapp deploy" or
// "python ansible-playbook". A program name alone is not clear for interpreters.
func cmdName(c *exec.Cmd) string {
	name := filepath.Base(c.Path)
	if len(c.Args) > 1 && !strings.HasPrefix(c.Args[1], "-") {
		name += " " + filepath.Base(c.Args[1])
	}

	return name
}
