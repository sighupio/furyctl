// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package gogetterx

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestCopyDirSkipsDanglingSymlinks(t *testing.T) {
	t.Parallel()

	src := t.TempDir()
	dst := t.TempDir()

	// A regular file that must be copied.
	if err := os.WriteFile(filepath.Join(src, "real.txt"), []byte("hello"), 0o600); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}

	// A dangling symlink (target does not exist), like furyctl's own
	// `.furyctl/bin` tool shims pointing into a cache that is not present.
	if err := os.Symlink(filepath.Join(src, "does-not-exist"), filepath.Join(src, "broken")); err != nil {
		t.Fatalf("failed to create dangling symlink: %v", err)
	}

	if err := copyDir(context.Background(), dst, src, false, false, 0); err != nil {
		t.Fatalf("copyDir must not fail on a dangling symlink, got: %v", err)
	}

	// The regular file must have been copied.
	if _, err := os.Stat(filepath.Join(dst, "real.txt")); err != nil {
		t.Errorf("expected real.txt to be copied, got: %v", err)
	}

	// The dangling symlink must have been skipped (not copied).
	if _, err := os.Lstat(filepath.Join(dst, "broken")); !os.IsNotExist(err) {
		t.Errorf("expected dangling symlink to be skipped, but it exists in dst")
	}
}
