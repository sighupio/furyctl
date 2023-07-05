// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package git_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/sighupio/furyctl/internal/tool/git"
	execx "github.com/sighupio/furyctl/internal/x/exec"
)

func Test_Runner_Version(t *testing.T) {
	r := git.NewRunner(execx.NewFakeExecutor("TestHelperProcess"), git.Paths{
		Git:     "git",
		WorkDir: os.TempDir(),
	})

	got, err := r.Version()
	if err != nil {
		t.Fatal(err)
	}

	want := "git version 2.39.0"

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
	case "git":
		switch subcmd {
		case "version":
			fmt.Fprintf(os.Stdout, "git version 2.39.0")
		default:
			fmt.Fprintf(os.Stdout, "subcommand '%s' not found", subcmd)
		}
	default:
		fmt.Fprintf(os.Stdout, "command '%s' not found", cmd)
	}

	os.Exit(0)
}
