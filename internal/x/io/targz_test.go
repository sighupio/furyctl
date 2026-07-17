// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package iox_test

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	iox "github.com/sighupio/furyctl/internal/x/io"
)

func Test_CreateTarGz_PreservesSymlinkAndContent(t *testing.T) {
	t.Parallel()

	src := t.TempDir()

	dataDir := filepath.Join(src, "data")
	err := os.MkdirAll(dataDir, 0o755)
	require.NoError(t, err, "mkdir")

	err = os.WriteFile(filepath.Join(dataDir, "real.txt"), []byte("hello"), 0o644)
	require.NoError(t, err, "write file")

	// Relative symlink mirroring the materialized tool layout.
	err = os.Symlink(filepath.Join("data", "real.txt"), filepath.Join(src, "link"))
	require.NoError(t, err, "symlink")

	out := filepath.Join(t.TempDir(), "bundle.tar.gz")

	err = iox.CreateTarGz(out, []iox.TarGzEntry{{Src: src, Prefix: "root"}})
	require.NoError(t, err, "CreateTarGz")

	links, contents := readTarGz(t, out)

	require.Equal(t, filepath.Join("data", "real.txt"), links["root/link"], "expected symlink target preserved")
	require.Equal(t, "hello", contents["root/data/real.txt"], "expected file content 'hello'")
}

func Test_ExtractTarGz_RoundTripPreservesSymlink(t *testing.T) {
	t.Parallel()

	src := t.TempDir()

	dataDir := filepath.Join(src, "data")
	err := os.MkdirAll(dataDir, 0o755)
	require.NoError(t, err, "mkdir")

	err = os.WriteFile(filepath.Join(dataDir, "real.txt"), []byte("payload"), 0o644)
	require.NoError(t, err, "write")

	err = os.Symlink(filepath.Join("data", "real.txt"), filepath.Join(src, "link"))
	require.NoError(t, err, "symlink")

	archive := filepath.Join(t.TempDir(), "b.tar.gz")
	err = iox.CreateTarGz(archive, []iox.TarGzEntry{{Src: src, Prefix: "root"}})
	require.NoError(t, err, "CreateTarGz")

	dst := t.TempDir()
	err = iox.ExtractTarGz(archive, dst)
	require.NoError(t, err, "ExtractTarGz")

	// The symlink must survive extraction and still resolve to the real file content.
	content, err := os.ReadFile(filepath.Join(dst, "root", "link"))
	require.NoError(t, err, "read via extracted symlink")
	require.Equal(t, "payload", string(content))

	target, err := os.Readlink(filepath.Join(dst, "root", "link"))
	require.NoError(t, err, "readlink")
	require.Equal(t, filepath.Join("data", "real.txt"), target, "expected relative symlink preserved")
}

func readTarGz(t *testing.T, path string) (map[string]string, map[string]string) {
	t.Helper()

	f, err := os.Open(path)
	require.NoError(t, err, "open archive")

	defer f.Close()

	gr, err := gzip.NewReader(f)
	require.NoError(t, err, "gzip reader")

	defer gr.Close()

	tr := tar.NewReader(gr)

	links := map[string]string{}
	contents := map[string]string{}

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}

		require.NoError(t, err, "tar next")

		switch hdr.Typeflag {
		case tar.TypeSymlink:
			links[hdr.Name] = hdr.Linkname

		case tar.TypeReg:
			b, err := io.ReadAll(tr)
			require.NoError(t, err, "read tar entry")

			contents[hdr.Name] = string(b)
		}
	}

	return links, contents
}
