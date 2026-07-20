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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCopyDirSkipsFuryctlWorkdir(t *testing.T) {
	t.Parallel()

	src := t.TempDir()
	dst := t.TempDir()

	// A regular distribution file that must be copied.
	err := os.WriteFile(filepath.Join(src, "kfd.yaml"), []byte("version: vX"), 0o600)
	require.NoError(t, err, "failed to create source file")

	// furyctl's own working directory with a tool shim that would otherwise
	// break the copy (here a symlink pointing to a directory, which makes a
	// plain file copy fail with "is a directory").
	binDir := filepath.Join(src, ".furyctl", "bin")
	err = os.MkdirAll(binDir, 0o750)
	require.NoError(t, err, "failed to create .furyctl/bin")

	err = os.Symlink(t.TempDir(), filepath.Join(binDir, "kubectl"))
	require.NoError(t, err, "failed to create dir symlink")

	err = copyDir(context.Background(), dst, src, false, false, 0)
	require.NoError(t, err, "copyDir must skip the .furyctl workdir")

	// The distribution file must have been copied.
	_, err = os.Stat(filepath.Join(dst, "kfd.yaml"))
	assert.NoError(t, err, "expected kfd.yaml to be copied")

	// The .furyctl workdir must not have been copied at all.
	_, err = os.Stat(filepath.Join(dst, ".furyctl"))
	assert.True(t, os.IsNotExist(err), "expected .furyctl to be skipped, but it exists in dst")
}

func TestCopyDirSkipsDanglingSymlinks(t *testing.T) {
	t.Parallel()

	src := t.TempDir()
	dst := t.TempDir()

	// A regular file that must be copied.
	err := os.WriteFile(filepath.Join(src, "real.txt"), []byte("hello"), 0o600)
	require.NoError(t, err, "failed to create source file")

	// A dangling symlink (target does not exist), like furyctl's own
	// `.furyctl/bin` tool shims pointing into a cache that is not present.
	err = os.Symlink(filepath.Join(src, "does-not-exist"), filepath.Join(src, "broken"))
	require.NoError(t, err, "failed to create dangling symlink")

	err = copyDir(context.Background(), dst, src, false, false, 0)
	require.NoError(t, err, "copyDir must not fail on a dangling symlink")

	// The regular file must have been copied.
	_, err = os.Stat(filepath.Join(dst, "real.txt"))
	assert.NoError(t, err, "expected real.txt to be copied")

	// The dangling symlink must have been skipped (not copied).
	_, err = os.Lstat(filepath.Join(dst, "broken"))
	assert.True(t, os.IsNotExist(err), "expected dangling symlink to be skipped, but it exists in dst")
}
