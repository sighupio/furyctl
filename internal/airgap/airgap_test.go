// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package airgap //nolint:testpackage // exercises the unexported prepare/marker logic.

import (
	"os"
	"path/filepath"
	"testing"

	iox "github.com/sighupio/furyctl/internal/x/io"
)

// makeBundle builds a minimal bundle tarball with a distro/ folder and a tool file.
func makeBundle(t *testing.T) string {
	t.Helper()

	src := t.TempDir()

	mustWrite(t, filepath.Join(src, "distro", "kfd.yaml"), "version: v1.0.0")
	mustWrite(t, filepath.Join(src, ".furyctl", "bin", "tool"), "binary")

	out := filepath.Join(t.TempDir(), "bundle.tar.gz")

	if err := iox.CreateTarGz(out, []iox.TarGzEntry{
		{Src: filepath.Join(src, "distro"), Prefix: "distro"},
		{Src: filepath.Join(src, ".furyctl"), Prefix: ".furyctl"},
	}); err != nil {
		t.Fatalf("CreateTarGz: %v", err)
	}

	return out
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func Test_prepare_ExtractsAndReturnsDistroLocation(t *testing.T) {
	t.Parallel()

	bundle := makeBundle(t)
	outDir := t.TempDir()

	distroLoc, err := prepare(bundle, outDir, false)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}

	if distroLoc != filepath.Join(outDir, DistroSubdir) {
		t.Fatalf("unexpected distro location: %s", distroLoc)
	}

	if _, err := os.Stat(filepath.Join(outDir, "distro", "kfd.yaml")); err != nil {
		t.Fatalf("expected distro extracted: %v", err)
	}

	if _, err := os.Stat(filepath.Join(outDir, ".furyctl", markerFile)); err != nil {
		t.Fatalf("expected marker written: %v", err)
	}
}

func Test_prepare_SkipsWhenAlreadyExtracted(t *testing.T) {
	t.Parallel()

	bundle := makeBundle(t)
	outDir := t.TempDir()

	if _, err := prepare(bundle, outDir, false); err != nil {
		t.Fatalf("first prepare: %v", err)
	}

	// Remove an extracted file; a skipped run (marker matches) must NOT recreate it.
	extracted := filepath.Join(outDir, "distro", "kfd.yaml")
	if err := os.Remove(extracted); err != nil {
		t.Fatalf("remove: %v", err)
	}

	if _, err := prepare(bundle, outDir, false); err != nil {
		t.Fatalf("second prepare: %v", err)
	}

	if _, err := os.Stat(extracted); !os.IsNotExist(err) {
		t.Fatalf("expected extraction to be skipped (file should stay removed), got err=%v", err)
	}
}

func Test_prepare_ForceReExtracts(t *testing.T) {
	t.Parallel()

	bundle := makeBundle(t)
	outDir := t.TempDir()

	if _, err := prepare(bundle, outDir, false); err != nil {
		t.Fatalf("first prepare: %v", err)
	}

	extracted := filepath.Join(outDir, "distro", "kfd.yaml")
	if err := os.Remove(extracted); err != nil {
		t.Fatalf("remove: %v", err)
	}

	if _, err := prepare(bundle, outDir, true); err != nil {
		t.Fatalf("forced prepare: %v", err)
	}

	if _, err := os.Stat(extracted); err != nil {
		t.Fatalf("expected forced re-extraction to recreate the file: %v", err)
	}
}

func Test_prepare_MissingBundle(t *testing.T) {
	t.Parallel()

	_, err := prepare(filepath.Join(t.TempDir(), "nope.tar.gz"), t.TempDir(), false)
	if err == nil {
		t.Fatal("expected error for missing bundle")
	}
}
