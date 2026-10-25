// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package create

import (
	"errors"
	"fmt"
	"os"
	"path"
	"slices"
	"strings"
	"unicode"
)

var (
	// ErrDiffScriptNotReadOnly is returned when the rendered diff.sh of the distribution could write to the cluster.
	ErrDiffScriptNotReadOnly = errors.New("the diff script of the distribution is not read-only")

	// The subcommands that write to the cluster, by tool. A kapp deploy is checked apart: it is allowed only as a
	// diff run.
	writeVerbs = map[string][]string{ //nolint:gochecknoglobals // read-only table.
		"kapp": {"delete"},
		"kubectl": {
			"apply", "create", "delete", "patch", "replace", "edit", "scale", "annotate", "label",
			"set", "rollout", "drain", "cordon", "uncordon", "taint",
		},
		"helm":     {"install", "upgrade", "uninstall", "delete", "rollback"},
		"helmfile": {"apply", "sync", "destroy", "delete"},
	}
)

// checkReadOnlyScript refuses a rendered diff.sh that could write to the cluster. The script comes from the
// templates of the distribution, so furyctl checks it before it runs it.
//
// The check is a conservative line-based scan, not a shell parser:
//   - lines ending with a backslash are joined, full-line comments are dropped; anything else, trailing comments
//     and quoted strings included, is scanned, so a tool named inside a string also counts;
//   - each line is split into commands at ; | & ( ) and backticks, and each command into words;
//   - a word is a tool when it is kapp, kubectl, helm or helmfile by name, by path (/usr/bin/kubectl,
//     "$dir/kubectl") or through a variable named after it ($kappbin, "${kubectlbin}", $KUBECTL);
//   - a command with a tool fails when any later word of it is a write subcommand of that tool, or, for kapp, when
//     it deploys without --diff-run or with -y/--yes;
//   - -y, --yes and a --diff-run set to anything but true fail anywhere in the script, so they cannot reach kapp
//     through a variable such as $additionalKappArgs.
//
// A script that cannot be read is refused.
//
// Limits: it does not follow other variables or functions that wrap a tool, eval of built strings, sourced files,
// other scripts the script calls, here-documents fed to a shell, or tools other than the four above. It may refuse
// a harmless script, for example one that only prints "kubectl delete".
func checkReadOnlyScript(script string) error {
	content, err := os.ReadFile(script)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrDiffScriptNotReadOnly, err)
	}

	if reason := readOnlyViolation(string(content)); reason != "" {
		return fmt.Errorf("%w: %s", ErrDiffScriptNotReadOnly, reason)
	}

	return nil
}

// readOnlyViolation returns why the script is not read-only, empty when the scan finds nothing.
func readOnlyViolation(script string) string {
	lines := strings.Split(strings.ReplaceAll(script, "\r\n", "\n"), "\n")

	for i := 0; i < len(lines); i++ {
		start := i
		line := strings.TrimSpace(lines[i])

		// A comment ends at the end of its line, even after a backslash.
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		for strings.HasSuffix(line, "\\") && i+1 < len(lines) {
			i++
			line = strings.TrimSuffix(line, "\\") + " " + lines[i]
		}

		for _, command := range splitCommands(line) {
			if reason := commandViolation(command); reason != "" {
				return fmt.Sprintf("line %d: %s", start+1, reason)
			}
		}
	}

	return ""
}

func splitCommands(line string) [][]string {
	segments := strings.FieldsFunc(line, func(r rune) bool {
		return strings.ContainsRune(";|&()`", r)
	})

	commands := make([][]string, 0, len(segments))

	for _, segment := range segments {
		words := strings.Fields(segment)
		for i, w := range words {
			words[i] = strings.NewReplacer(`"`, "", `'`, "").Replace(w)
		}

		commands = append(commands, words)
	}

	return commands
}

func commandViolation(words []string) string {
	for _, w := range words {
		// The value of an assignment is checked too: it can reach kapp through a variable.
		if name, value, ok := strings.Cut(w, "="); ok && isVariableName(name) {
			w = value
		}

		if isYesFlag(w) {
			return w + " confirms changes without a diff run"
		}

		if v, ok := strings.CutPrefix(w, "--diff-run="); ok && v != "true" {
			return w + " turns off the diff run"
		}
	}

	for i, w := range words {
		tool := toolOf(w)
		if tool == "" {
			continue
		}

		args := words[i+1:]

		for _, verb := range writeVerbs[tool] {
			if slices.Contains(args, verb) {
				return tool + " " + verb + " writes to the cluster"
			}
		}

		if tool == "kapp" {
			if reason := kappViolation(args); reason != "" {
				return reason
			}
		}
	}

	return ""
}

// kappViolation checks the arguments of a kapp command: a deploy must be a diff run, and no short flag group may
// hold -y.
func kappViolation(args []string) string {
	diffRun := slices.Contains(args, "--diff-run") || slices.Contains(args, "--diff-run=true")

	if slices.Contains(args, "deploy") && !diffRun {
		return "kapp deploy without --diff-run"
	}

	for _, a := range args {
		if strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "--") && strings.Contains(a, "y") {
			return "kapp with " + a + " confirms changes without a diff run"
		}
	}

	return ""
}

func isVariableName(name string) bool {
	return name != "" && strings.IndexFunc(name, notVariableRune) < 0
}

func notVariableRune(r rune) bool {
	return r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r)
}

func isYesFlag(w string) bool {
	return w == "-y" || w == "--yes" || strings.HasPrefix(w, "-y=") || strings.HasPrefix(w, "--yes=")
}

// toolOf returns the tool a word names, by name, by path or through a variable, empty when it names none.
func toolOf(word string) string {
	if base := path.Base(word); !strings.HasPrefix(base, "$") {
		if _, ok := writeVerbs[base]; ok {
			return base
		}

		return ""
	}

	name := strings.TrimLeft(word, "${")

	if end := strings.IndexFunc(name, notVariableRune); end >= 0 {
		name = name[:end]
	}

	name = strings.TrimSuffix(strings.TrimSuffix(strings.ToLower(name), "bin"), "_")

	if _, ok := writeVerbs[name]; ok {
		return name
	}

	return ""
}
