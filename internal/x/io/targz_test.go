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

	iox "github.com/sighupio/furyctl/internal/x/io"
)

func Test_CreateTarGz_PreservesSymlinkAndContent(t *testing.T) {
	t.Parallel()

	src := t.TempDir()

	dataDir := filepath.Join(src, "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(dataDir, "real.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	// Relative symlink mirroring the materialized tool layout.
	if err := os.Symlink(filepath.Join("data", "real.txt"), filepath.Join(src, "link")); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	out := filepath.Join(t.TempDir(), "bundle.tar.gz")

	if err := iox.CreateTarGz(out, []iox.TarGzEntry{{Src: src, Prefix: "root"}}); err != nil {
		t.Fatalf("CreateTarGz: %v", err)
	}

	links, contents := readTarGz(t, out)

	if got := links["root/link"]; got != filepath.Join("data", "real.txt") {
		t.Fatalf("expected symlink target preserved, got %q", got)
	}

	if got := contents["root/data/real.txt"]; got != "hello" {
		t.Fatalf("expected file content 'hello', got %q", got)
	}
}

func Test_ExtractTarGz_RoundTripPreservesSymlink(t *testing.T) {
	t.Parallel()

	src := t.TempDir()

	dataDir := filepath.Join(src, "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(dataDir, "real.txt"), []byte("payload"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := os.Symlink(filepath.Join("data", "real.txt"), filepath.Join(src, "link")); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	archive := filepath.Join(t.TempDir(), "b.tar.gz")
	if err := iox.CreateTarGz(archive, []iox.TarGzEntry{{Src: src, Prefix: "root"}}); err != nil {
		t.Fatalf("CreateTarGz: %v", err)
	}

	dst := t.TempDir()
	if err := iox.ExtractTarGz(archive, dst); err != nil {
		t.Fatalf("ExtractTarGz: %v", err)
	}

	// The symlink must survive extraction and still resolve to the real file content.
	content, err := os.ReadFile(filepath.Join(dst, "root", "link"))
	if err != nil {
		t.Fatalf("read via extracted symlink: %v", err)
	}

	if string(content) != "payload" {
		t.Fatalf("expected 'payload' via symlink, got %q", content)
	}

	target, err := os.Readlink(filepath.Join(dst, "root", "link"))
	if err != nil {
		t.Fatalf("readlink: %v", err)
	}

	if target != filepath.Join("data", "real.txt") {
		t.Fatalf("expected relative symlink preserved, got %q", target)
	}
}

func readTarGz(t *testing.T, path string) (map[string]string, map[string]string) {
	t.Helper()

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}

	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("gzip reader: %v", err)
	}

	defer gr.Close()

	tr := tar.NewReader(gr)

	links := map[string]string{}
	contents := map[string]string{}

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			t.Fatalf("tar next: %v", err)
		}

		switch hdr.Typeflag {
		case tar.TypeSymlink:
			links[hdr.Name] = hdr.Linkname

		case tar.TypeReg:
			b, err := io.ReadAll(tr)
			if err != nil {
				t.Fatalf("read tar entry: %v", err)
			}

			contents[hdr.Name] = string(b)
		}
	}

	return links, contents
}
