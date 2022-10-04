// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package execx_test

import (
	"testing"

	execx "github.com/sighupio/furyctl/internal/x/exec"
)

func Test_StdExecutor_Command(t *testing.T) {
	e := execx.NewStdExecutor()

	cmd := e.Command("echo", "hello go world")
	if cmd == nil {
		t.Fatalf("expected command to be not nil")
	}

	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("expected command to be executed without errors: %v", err)
	}

	if string(out) != "hello go world\n" {
		t.Errorf("want = 'hello go world', got = '%s'", string(out))
	}
}
