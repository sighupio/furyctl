// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package yq_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/sighupio/furyctl/internal/tool/yq"
	execx "github.com/sighupio/furyctl/internal/x/exec"
)

func Test_Runner_Version(t *testing.T) {
	r := yq.NewRunner(execx.NewFakeExecutor("TestHelperProcess"), yq.Paths{
		Yq:      "yq",
		WorkDir: os.TempDir(),
	})

	got, err := r.Version()
	if err != nil {
		t.Fatal(err)
	}

	want := "yq (https://github.com/mikefarah/yq/) version v4.34.1"

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
	case "yq":
		switch subcmd {
		case "--version":
			fmt.Fprintf(os.Stdout, "yq (https://github.com/mikefarah/yq/) version v4.34.1")
		default:
			fmt.Fprintf(os.Stdout, "subcommand '%s' not found", subcmd)
		}
	default:
		fmt.Fprintf(os.Stdout, "command '%s' not found", cmd)
	}

	os.Exit(0)
}
