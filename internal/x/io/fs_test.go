// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unit

package iox_test

import (
	"bytes"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/exp/rand"

	iox "github.com/sighupio/furyctl/internal/x/io"
)

func TestCheckDirIsEmpty(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc    string
		setup   func() (string, error)
		wantErr bool
	}{
		{
			desc: "directory does not exist",
			setup: func() (string, error) {
				dir, err := os.MkdirTemp("", "furyctl")
				if err != nil {
					return "", err
				}

				return filepath.Join(dir, "does-not-exist"), nil
			},
		},
		{
			desc: "directory is empty",
			setup: func() (string, error) {
				return os.MkdirTemp("", "furyctl")
			},
		},
		{
			desc: "directory is not empty",
			setup: func() (string, error) {
				dir, err := os.MkdirTemp("", "furyctl")
				if err != nil {
					return "", err
				}

				f, err := os.CreateTemp(dir, "furyctl")
				defer f.Close()

				return dir, err
			},
			wantErr: true,
		},
	}
	for _, tC := range testCases {
		tC := tC

		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			dir, err := tC.setup()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			err = iox.CheckDirIsEmpty(dir)

			if !tC.wantErr && err != nil {
				t.Errorf("expected no error, got %v", err)
			}

			if tC.wantErr && err == nil {
				t.Errorf("expected error, got none")
			}
		})
	}
}

func TestAppendBufferToFile(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc  string
		setup func() (string, error)
	}{
		{
			desc: "existing file",
			setup: func() (string, error) {
				f, err := os.CreateTemp("", "furyctl")
				if err != nil {
					return "", err
				}
				defer f.Close()

				return f.Name(), nil
			},
		},
		{
			desc: "not existing file",
			setup: func() (string, error) {
				dir, err := os.MkdirTemp("", "furyctl")
				if err != nil {
					return "", err
				}

				return filepath.Join(dir, "does-not-exist"), nil
			},
		},
	}
	for _, tC := range testCases {
		tC := tC

		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			file, err := tC.setup()
			if err != nil {
				t.Fatalf("unexpected setup error: %v", err)
			}

			err = iox.AppendToFile("foo", file)
			if err != nil {
				t.Fatalf("could not append string 'foo': %v", err)
			}
			err = iox.AppendToFile("bar", file)
			if err != nil {
				t.Fatalf("could not append string 'bar': %v", err)
			}

			got, err := os.ReadFile(file)
			if err != nil {
				t.Fatalf("could not get file content: %v", err)
			}

			want := "foobar"
			if string(got) != want {
				t.Errorf("expected %s, got %s", want, got)
			}
		})
	}
}

func TestCopyBufferToFile(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc       string
		bufContent string
		setup      func() (string, error)
	}{
		{
			desc:       "empty buffer",
			bufContent: "",
			setup: func() (string, error) {
				f, err := os.CreateTemp("", "furyctl")
				if err != nil {
					return "", err
				}

				return f.Name(), nil
			},
		},
		{
			desc:       "filled buffer",
			bufContent: "foo bar baz quux",
			setup: func() (string, error) {
				f, err := os.CreateTemp("", "furyctl")
				if err != nil {
					return "", err
				}

				return f.Name(), nil
			},
		},
	}
	for _, tC := range testCases {
		tC := tC

		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			dst, err := tC.setup()
			if err != nil {
				t.Fatalf("unexpected setup error: %v", err)
			}

			b := bytes.NewBufferString(tC.bufContent)

			if err := iox.CopyBufferToFile(*b, dst); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			dstContent, err := os.ReadFile(dst)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if string(dstContent) != tC.bufContent {
				t.Errorf("expected %s, got %s", tC.bufContent, dstContent)
			}
		})
	}
}

func TestCopyFile(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc    string
		setup   func() (string, string, error)
		wantErr bool
	}{
		{
			desc: "existing file",
			setup: func() (string, string, error) {
				dir, err := os.MkdirTemp("", "furyctl")
				if err != nil {
					return "", "", err
				}

				src := filepath.Join(dir, "src")
				dst := filepath.Join(dir, "dst")

				f, err := os.Create(src)
				if err != nil {
					return "", "", err
				}
				defer f.Close()

				return src, dst, nil
			},
		},
		{
			desc: "not existing source file",
			setup: func() (string, string, error) {
				dir, err := os.MkdirTemp("", "furyctl")
				if err != nil {
					return "", "", err
				}

				src := filepath.Join(dir, "not-existing", "src")
				dst := filepath.Join(dir, "dst")

				return src, dst, nil
			},
			wantErr: true,
		},
	}
	for _, tC := range testCases {
		tC := tC

		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			src, dst, err := tC.setup()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			err = iox.CopyFile(src, dst)

			if !tC.wantErr && err != nil {
				t.Errorf("expected no error, got %v", err)
			}

			if tC.wantErr && err == nil {
				t.Errorf("expected error, got none")
			}
		})
	}
}

func TestCopyRecursive(t *testing.T) {
	t.Parallel()

	type File struct {
		FileType string
		FilePerm fs.FileMode
	}

	testCases := []struct {
		desc  string
		setup func() (fs.FS, string, error)
		want  map[string]File
	}{
		{
			desc: "dir containing other dirs and files",
			setup: func() (fs.FS, string, error) {
				src, err := os.MkdirTemp("", "furyctl")
				if err != nil {
					return nil, "", err
				}

				if err := os.Mkdir(filepath.Join(src, "foo"), 0o755); err != nil {
					return nil, "", err
				}

				f1, err := os.Create(filepath.Join(src, "bar.txt"))
				if err != nil {
					return nil, "", err
				}
				defer f1.Close()

				f2, err := os.Create(filepath.Join(src, "foo", "bar.txt"))
				if err != nil {
					return nil, "", err
				}
				defer f2.Close()

				if err := os.Chmod(filepath.Join(src, "foo", "bar.txt"), 0o664); err != nil {
					return nil, "", err
				}

				if err := os.Mkdir(filepath.Join(src, "foo", "bar"), 0o755); err != nil {
					return nil, "", err
				}

				f3, err := os.Create(filepath.Join(src, "foo", "bar", "baz.txt"))
				if err != nil {
					return nil, "", err
				}
				defer f3.Close()

				if err := os.Chmod(filepath.Join(src, "foo", "bar", "baz.txt"), 0o666); err != nil {
					return nil, "", err
				}

				if err := os.Mkdir(filepath.Join(src, "foo", "bar", "baz"), 0o755); err != nil {
					return nil, "", err
				}

				dst := filepath.Join(
					os.TempDir(),
					fmt.Sprintf("furyctl-iox-copy-recursive-test-%d-%d", time.Now().Unix(), rand.Intn(math.MaxInt)),
				)

				return os.DirFS(src), dst, nil
			},
			want: map[string]File{
				"foo": {
					FileType: "dir",
					FilePerm: 0o755,
				},
				"bar.txt": {
					FileType: "file",
					FilePerm: 0o644,
				},
				"foo/bar.txt": {
					FileType: "file",
					FilePerm: 0o664,
				},
				"foo/bar": {
					FileType: "dir",
					FilePerm: 0o755,
				},
				"foo/bar/baz.txt": {
					FileType: "file",
					FilePerm: 0o666,
				},
				"foo/bar/baz": {
					FileType: "dir",
					FilePerm: 0o755,
				},
			},
		},
	}
	for _, tC := range testCases {
		tC := tC

		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			src, dst, err := tC.setup()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if err := iox.CopyRecursive(src, dst); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			for fname, f := range tC.want {
				info, err := os.Stat(filepath.Join(dst, fname))
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}

				if f.FilePerm != info.Mode().Perm() {
					t.Errorf("expected %s to have permissions %o, got %o", fname, f.FilePerm, info.Mode().Perm())
				}

				if f.FileType == "dir" && !info.IsDir() {
					t.Errorf("expected %s to be a directory", fname)
				}
				if f.FileType == "file" && info.IsDir() {
					t.Errorf("expected %s to be a file", fname)
				}
			}
		})
	}
}

func TestEnsureDir(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc  string
		setup func() (string, error)
	}{
		{
			desc: "directory does not exist",
			setup: func() (string, error) {
				dir, err := os.MkdirTemp("", "furyctl")
				if err != nil {
					return "", err
				}

				return filepath.Join(dir, "does", "not", "exist"), nil
			},
		},
		{
			desc: "directory already exists",
			setup: func() (string, error) {
				return os.MkdirTemp("", "furyctl")
			},
		},
	}
	for _, tC := range testCases {
		tC := tC

		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			dir, err := tC.setup()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if err := iox.EnsureDir(filepath.Join(dir, "to-be-created.txt")); err != nil {
				t.Errorf("expected no error, got %v", err)
			}

			info, err := os.Stat(dir)
			if err != nil {
				t.Errorf("expected no error, got %v", err)
			}

			if !info.IsDir() {
				t.Errorf("expected '%s' to be a directory", dir)
			}
		})
	}
}
