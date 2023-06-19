// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package bash_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/sighupio/furyctl/internal/tool/bash"
	execx "github.com/sighupio/furyctl/internal/x/exec"
)

func Test_Runner_Version(t *testing.T) {
	r := bash.NewRunner(execx.NewFakeExecutor(), bash.Paths{
		Bash:    "bash",
		WorkDir: os.TempDir(),
	})

	got, err := r.Version()
	if err != nil {
		t.Fatal(err)
	}

	want := "GNU bash, version 3.2.57(1)-release (arm64-apple-darwin22)\nCopyright (C) 2007 Free Software Foundation, Inc."

	if got != want {
		t.Errorf("expected version '%s', got '%s'", want, got)
	}
}

func TestHelperProcess(t *testing.T) {
	args := os.Args

	if len(args) < 3 || args[1] != "-test.run=TestHelperProcess" {
		return
	}

	cmd, subcmd := args[3], args[4]

	switch cmd {
	case "bash":
		switch subcmd {
		case "--version":
			fmt.Fprintf(os.Stdout, "GNU bash, version 3.2.57(1)-release (arm64-apple-darwin22)\nCopyright (C) 2007 Free Software Foundation, Inc.")
		default:
			fmt.Fprintf(os.Stdout, "subcommand '%s' not found", subcmd)
		}
	default:
		fmt.Fprintf(os.Stdout, "command '%s' not found", cmd)
	}

	os.Exit(0)
}
