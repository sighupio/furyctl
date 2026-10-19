// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ansible

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	execx "github.com/sighupio/furyctl/internal/x/exec"
)

var (
	//go:embed furyctl_failures.py
	failuresPlugin []byte

	// The first line of a block of ansible output, and of a warning block.
	blockStart   = regexp.MustCompile(`^(PLAY \[|PLAY RECAP|TASK \[|RUNNING HANDLER \[|\[[A-Z][A-Z ]*\]:|ERROR! )`)
	warningStart = regexp.MustCompile(`^\[(WARNING|DEPRECATION WARNING)\]:`)
)

// hostFailure is the last failure of a host, as the furyctl_failures callback writes it.
type hostFailure struct {
	Host        string `json:"host"`
	Unreachable bool   `json:"unreachable"`
	Play        string `json:"play"`
	Task        string `json:"task"`
	Msg         string `json:"msg"`
	RC          *int   `json:"rc"`
	Stderr      string `json:"stderr"`
}

// cause is the reason of the failure in one line.
func (f hostFailure) cause() string {
	// The msg of command and shell is only "non-zero return code".
	if stderr := strings.TrimSpace(f.Stderr); f.RC != nil && stderr != "" {
		return fmt.Sprintf("rc=%d: %s", *f.RC, stderr[strings.LastIndex(stderr, "\n")+1:])
	}

	msg, _, _ := strings.Cut(strings.TrimSpace(f.Msg), "\n")

	for _, p := range []string{
		"Task failed: ",
		"Unexpected AnsibleActionFail error: ",
		"Failed to connect to the host via ssh: ",
	} {
		msg = strings.TrimPrefix(msg, p)
	}

	return msg
}

// readFailures reads the file of the furyctl_failures callback, sorted by host. The file is missing
// when ansible fails before the end of the playbook stats, for example on a YAML error.
func readFailures(file string) []hostFailure {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil
	}

	var failures []hostFailure
	if err := json.Unmarshal(data, &failures); err != nil {
		return nil
	}

	slices.SortFunc(failures, func(a, b hostFailure) int { return strings.Compare(a.Host, b.Host) })

	return failures
}

// sshSummary names the hosts that do not answer, when no host fails for another reason.
func sshSummary(failures []hostFailure) string {
	if slices.ContainsFunc(failures, func(f hostFailure) bool { return !f.Unreachable }) {
		return ""
	}

	var b strings.Builder

	b.WriteString("these hosts do not answer on ssh:")

	for _, f := range failures {
		fmt.Fprintf(&b, "\n  %s: %s", f.Host, f.cause())
	}

	return b.String()
}

func summary(failures []hostFailure) string {
	var b strings.Builder

	fmt.Fprintf(&b, "ansible failed on %d host(s):", len(failures))

	for _, f := range failures {
		kind := "failed"
		if f.Unreachable {
			kind = "unreachable"
		}

		fmt.Fprintf(&b, "\n  %s: %s at %q > %q: %s", f.Host, kind, f.Play, f.Task, f.cause())
	}

	return b.String()
}

// describe sets what the error of a failed ansible command shows: a summary of the failed hosts and the
// play recap, or the last ansible blocks when no play ran. The warnings become a count.
func describe(e *execx.CmdError, entrypoint string, args []string, failuresFile string) {
	e.Name = filepath.Base(entrypoint)
	e.Args = args

	adHoc := e.Name == "ansible"

	if m := module(args); adHoc && m != "" {
		e.LogName = "ansible-" + m
	}

	lines, warnings := withoutWarnings(e.Lines)

	switch warnings {
	case 0:
	case 1:
		e.Note = "(1 Ansible warning not shown, the full output has it)"
	default:
		e.Note = fmt.Sprintf("(%d Ansible warnings not shown, the full output has them)", warnings)
	}

	failures := readFailures(failuresFile)
	if len(failures) == 0 {
		e.Block = execx.Tail(lines, execx.OutputLines)
		if i := slices.IndexFunc(e.Block, blockStart.MatchString); i >= 0 {
			e.Block = e.Block[i:]
		}

		return
	}

	if adHoc {
		e.Summary = sshSummary(failures)
	}

	if e.Summary == "" {
		e.Summary = summary(failures)
	}

	e.LeadIn = "Recap of the playbook:"
	e.Block = nil

	for i, l := range slices.Backward(lines) {
		if strings.HasPrefix(l, "PLAY RECAP") {
			// Without the "PLAY RECAP ****" header: the lead-in already names the recap.
			e.Block = execx.Tail(lines[i+1:], len(lines))

			break
		}
	}
}

// withoutWarnings returns the text of the lines without the warning blocks on stderr, and the number
// of warning blocks.
func withoutWarnings(lines []execx.Line) ([]string, int) {
	text := make([]string, 0, len(lines))
	warnings := 0
	inWarning := false

	for _, l := range lines {
		if l.Stderr {
			if blockStart.MatchString(l.Text) {
				inWarning = warningStart.MatchString(l.Text)
				if inWarning {
					warnings++
				}
			}

			if inWarning {
				continue
			}
		}

		text = append(text, l.Text)
	}

	return text, warnings
}

// module gives the module of an ad-hoc command, from -m, --module-name or --module-name=.
func module(args []string) string {
	for i, a := range args {
		if v, ok := strings.CutPrefix(a, "--module-name="); ok {
			return v
		}

		if (a == "-m" || a == "--module-name") && i+1 < len(args) {
			return args[i+1]
		}
	}

	return ""
}
