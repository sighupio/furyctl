// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package iox

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	dirPerm   = 0o755
	copyChunk = 1 << 20 // 1 MiB per io.CopyN step when extracting regular files.
)

var (
	// ErrIllegalArchivePath is returned when a tar entry would extract outside the destination dir.
	ErrIllegalArchivePath = errors.New("illegal path in archive (escapes destination)")

	// ErrUnsupportedTarEntryType is returned when a tar entry has an unsupported type flag.
	ErrUnsupportedTarEntryType = errors.New("unsupported tar entry type")
)

// TarGzEntry maps a file or directory on disk to the path prefix it should appear under inside the
// archive.
type TarGzEntry struct {
	Src    string
	Prefix string
}

// CreateTarGz writes a gzip-compressed tarball at output containing the given entries. Symlinks are
// archived as symlinks (not followed), so the relative links of the materialized tool layout keep
// resolving after the bundle is extracted on another machine (air-gapped transfer).
func CreateTarGz(output string, entries []TarGzEntry) (err error) {
	out, err := os.Create(output)
	if err != nil {
		return fmt.Errorf("error creating archive %s: %w", output, err)
	}

	defer func() {
		if cerr := out.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("error closing archive %s: %w", output, cerr)
		}

		if err != nil {
			_ = os.Remove(output)
		}
	}()

	gw := gzip.NewWriter(out)
	tw := tar.NewWriter(gw)

	for _, e := range entries {
		if werr := addToTar(tw, e.Src, e.Prefix); werr != nil {
			return werr
		}
	}

	if cerr := tw.Close(); cerr != nil {
		return fmt.Errorf("error finalizing tar: %w", cerr)
	}

	if cerr := gw.Close(); cerr != nil {
		return fmt.Errorf("error finalizing gzip: %w", cerr)
	}

	return nil
}

func addToTar(tw *tar.Writer, src, prefix string) error {
	err := filepath.Walk(src, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("error walking %s: %w", path, walkErr)
		}

		name := prefix

		if rel, err := filepath.Rel(src, path); err == nil && rel != "." {
			name = filepath.Join(prefix, rel)
		}

		link := ""

		if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err != nil {
				return fmt.Errorf("error reading symlink %s: %w", path, err)
			}

			link = target
		}

		hdr, err := tar.FileInfoHeader(info, link)
		if err != nil {
			return fmt.Errorf("error building tar header for %s: %w", path, err)
		}

		hdr.Name = name

		if err := tw.WriteHeader(hdr); err != nil {
			return fmt.Errorf("error writing tar header for %s: %w", name, err)
		}

		if !info.Mode().IsRegular() {
			return nil
		}

		return copyFileToTar(tw, path, name)
	})
	if err != nil {
		return fmt.Errorf("error archiving %s: %w", src, err)
	}

	return nil
}

// ExtractTarGz extracts a gzip-compressed tarball into dst, recreating directories, regular files
// and symlinks. It rejects entries whose path would escape dst (zip-slip protection).
func ExtractTarGz(src, dst string) (err error) {
	f, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("error opening archive %s: %w", src, err)
	}

	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("error reading gzip %s: %w", src, err)
	}

	defer func() {
		if cerr := gr.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("error closing gzip: %w", cerr)
		}
	}()

	cleanDst := filepath.Clean(dst)
	tr := tar.NewReader(gr)

	for {
		hdr, nerr := tr.Next()
		if errors.Is(nerr, io.EOF) {
			return nil
		}

		if nerr != nil {
			return fmt.Errorf("error reading tar entry: %w", nerr)
		}

		target := filepath.Join(cleanDst, hdr.Name) //nolint:gosec // path validated below against dst.

		if target != cleanDst && !strings.HasPrefix(target, cleanDst+string(os.PathSeparator)) {
			return fmt.Errorf("%w: %s", ErrIllegalArchivePath, hdr.Name)
		}

		if eerr := extractEntry(tr, hdr, target); eerr != nil {
			return eerr
		}
	}
}

func extractEntry(tr *tar.Reader, hdr *tar.Header, target string) error {
	switch hdr.Typeflag {
	case tar.TypeDir:
		if err := os.MkdirAll(target, hdr.FileInfo().Mode()); err != nil {
			return fmt.Errorf("error creating dir %s: %w", target, err)
		}

	case tar.TypeReg:
		if err := os.MkdirAll(filepath.Dir(target), dirPerm); err != nil {
			return fmt.Errorf("error creating parent of %s: %w", target, err)
		}

		return writeRegularFile(tr, hdr, target)

	case tar.TypeSymlink:
		if err := os.MkdirAll(filepath.Dir(target), dirPerm); err != nil {
			return fmt.Errorf("error creating parent of %s: %w", target, err)
		}

		_ = os.Remove(target)

		if err := os.Symlink(hdr.Linkname, target); err != nil {
			return fmt.Errorf("error creating symlink %s: %w", target, err)
		}

	default:
		return fmt.Errorf("%w: %b", ErrUnsupportedTarEntryType, hdr.Typeflag)
	}

	return nil
}

func writeRegularFile(tr *tar.Reader, hdr *tar.Header, target string) error {
	// Remove any existing file first: re-extracting (e.g. --force-extract) over read-only files such
	// as git objects (mode 0444) would otherwise fail to reopen them for writing.
	_ = os.Remove(target)

	out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, hdr.FileInfo().Mode())
	if err != nil {
		return fmt.Errorf("error creating file %s: %w", target, err)
	}

	defer out.Close()

	for {
		_, cerr := io.CopyN(out, tr, copyChunk)
		if errors.Is(cerr, io.EOF) {
			return nil
		}

		if cerr != nil {
			return fmt.Errorf("error writing file %s: %w", target, cerr)
		}
	}
}

func copyFileToTar(tw *tar.Writer, path, name string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("error opening %s: %w", path, err)
	}

	defer f.Close()

	if _, err := io.Copy(tw, f); err != nil {
		return fmt.Errorf("error writing %s to tar: %w", name, err)
	}

	return nil
}
