// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package airgap //nolint:testpackage // exercises the unexported prepare/marker logic.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	iox "github.com/sighupio/furyctl/internal/x/io"
)

// makeBundle builds a minimal bundle tarball with a distro/ folder and a tool file.
func makeBundle(t *testing.T) string {
	t.Helper()

	src := t.TempDir()

	mustWrite(t, filepath.Join(src, "distro", "kfd.yaml"), "version: v1.0.0")
	mustWrite(t, filepath.Join(src, ".furyctl", "bin", "tool"), "binary")

	out := filepath.Join(t.TempDir(), "bundle.tar.gz")

	err := iox.CreateTarGz(out, []iox.TarGzEntry{
		{Src: filepath.Join(src, "distro"), Prefix: "distro"},
		{Src: filepath.Join(src, ".furyctl"), Prefix: ".furyctl"},
	})
	require.NoError(t, err, "CreateTarGz")

	return out
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()

	err := os.MkdirAll(filepath.Dir(path), 0o755)
	require.NoError(t, err, "mkdir")

	err = os.WriteFile(path, []byte(content), 0o644)
	require.NoError(t, err, "write")
}

func Test_prepare_ExtractsAndReturnsDistroLocation(t *testing.T) {
	t.Parallel()

	bundle := makeBundle(t)
	outDir := t.TempDir()

	distroLoc, err := prepare(bundle, outDir, false)
	require.NoError(t, err, "prepare")

	require.Equal(t, filepath.Join(outDir, DistroSubdir), distroLoc, "unexpected distro location")

	_, err = os.Stat(filepath.Join(outDir, "distro", "kfd.yaml"))
	require.NoError(t, err, "expected distro extracted")

	_, err = os.Stat(filepath.Join(outDir, ".furyctl", markerFile))
	require.NoError(t, err, "expected marker written")
}

func Test_prepare_SkipsWhenAlreadyExtracted(t *testing.T) {
	t.Parallel()

	bundle := makeBundle(t)
	outDir := t.TempDir()

	_, err := prepare(bundle, outDir, false)
	require.NoError(t, err, "first prepare")

	// Remove an extracted file; a skipped run (marker matches) must NOT recreate it.
	extracted := filepath.Join(outDir, "distro", "kfd.yaml")
	err = os.Remove(extracted)
	require.NoError(t, err, "remove")

	_, err = prepare(bundle, outDir, false)
	require.NoError(t, err, "second prepare")

	_, err = os.Stat(extracted)
	require.True(t, os.IsNotExist(err), "expected extraction to be skipped (file should stay removed), got err=%v", err)
}

func Test_prepare_ForceReExtracts(t *testing.T) {
	t.Parallel()

	bundle := makeBundle(t)
	outDir := t.TempDir()

	_, err := prepare(bundle, outDir, false)
	require.NoError(t, err, "first prepare")

	extracted := filepath.Join(outDir, "distro", "kfd.yaml")
	err = os.Remove(extracted)
	require.NoError(t, err, "remove")

	_, err = prepare(bundle, outDir, true)
	require.NoError(t, err, "forced prepare")

	_, err = os.Stat(extracted)
	require.NoError(t, err, "expected forced re-extraction to recreate the file")
}

func Test_prepare_MissingBundle(t *testing.T) {
	t.Parallel()

	_, err := prepare(filepath.Join(t.TempDir(), "nope.tar.gz"), t.TempDir(), false)
	require.Error(t, err, "expected error for missing bundle")
}
