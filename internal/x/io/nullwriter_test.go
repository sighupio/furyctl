// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package iox_test

import (
	"testing"

	iox "github.com/sighupio/furyctl/internal/x/io"
)

func Test_NullWriter_Write(t *testing.T) {
	nw := iox.NewNullWriter()

	n, err := nw.Write([]byte("hello world"))
	if err != nil {
		t.Fatalf("expected to write without errors: %v", err)
	}

	if n != 0 {
		t.Errorf("want = 0, got = %d", n)
	}
}
