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

func TestCopyDirSkipsFuryctlWorkdir(t *testing.T) {
	t.Parallel()

	src := t.TempDir()
	dst := t.TempDir()

	// A regular distribution file that must be copied.
	if err := os.WriteFile(filepath.Join(src, "kfd.yaml"), []byte("version: vX"), 0o600); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}

	// furyctl's own working directory with a tool shim that would otherwise
	// break the copy (here a symlink pointing to a directory, which makes a
	// plain file copy fail with "is a directory").
	binDir := filepath.Join(src, ".furyctl", "bin")
	if err := os.MkdirAll(binDir, 0o750); err != nil {
		t.Fatalf("failed to create .furyctl/bin: %v", err)
	}

	if err := os.Symlink(t.TempDir(), filepath.Join(binDir, "kubectl")); err != nil {
		t.Fatalf("failed to create dir symlink: %v", err)
	}

	if err := copyDir(context.Background(), dst, src, false, false, 0); err != nil {
		t.Fatalf("copyDir must skip the .furyctl workdir, got: %v", err)
	}

	// The distribution file must have been copied.
	if _, err := os.Stat(filepath.Join(dst, "kfd.yaml")); err != nil {
		t.Errorf("expected kfd.yaml to be copied, got: %v", err)
	}

	// The .furyctl workdir must not have been copied at all.
	if _, err := os.Stat(filepath.Join(dst, ".furyctl")); !os.IsNotExist(err) {
		t.Errorf("expected .furyctl to be skipped, but it exists in dst")
	}
}

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
