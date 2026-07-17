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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			dir, err := tC.setup()
			require.NoError(t, err, "unexpected setup error")

			err = iox.CheckDirIsEmpty(dir)

			if tC.wantErr {
				assert.Error(t, err, "expected error, got none")
			} else {
				assert.NoError(t, err, "expected no error")
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
		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			file, err := tC.setup()
			require.NoError(t, err, "unexpected setup error")

			err = iox.AppendToFile("foo", file)
			require.NoError(t, err, "could not append string 'foo'")
			err = iox.AppendToFile("bar", file)
			require.NoError(t, err, "could not append string 'bar'")

			got, err := os.ReadFile(file)
			require.NoError(t, err, "could not get file content")

			assert.Equal(t, "foobar", string(got))
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
		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			dst, err := tC.setup()
			require.NoError(t, err, "unexpected setup error")

			b := bytes.NewBufferString(tC.bufContent)

			err = iox.CopyBufferToFile(*b, dst)
			require.NoError(t, err, "unexpected error")

			dstContent, err := os.ReadFile(dst)
			require.NoError(t, err, "unexpected error")

			assert.Equal(t, tC.bufContent, string(dstContent))
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
		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			src, dst, err := tC.setup()
			require.NoError(t, err, "unexpected setup error")

			err = iox.CopyFile(src, dst)

			if tC.wantErr {
				assert.Error(t, err, "expected error, got none")
			} else {
				assert.NoError(t, err, "expected no error")
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
		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			src, dst, err := tC.setup()
			require.NoError(t, err, "unexpected setup error")

			err = iox.CopyRecursive(src, dst)
			require.NoError(t, err, "unexpected error")

			for fname, f := range tC.want {
				info, err := os.Stat(filepath.Join(dst, fname))
				require.NoError(t, err, "expected no error for %s", fname)

				assert.Equal(t, f.FilePerm, info.Mode().Perm(), "expected %s to have permissions %o", fname, f.FilePerm)

				if f.FileType == "dir" {
					assert.True(t, info.IsDir(), "expected %s to be a directory", fname)
				} else {
					assert.False(t, info.IsDir(), "expected %s to be a file", fname)
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
		t.Run(tC.desc, func(t *testing.T) {
			t.Parallel()

			dir, err := tC.setup()
			require.NoError(t, err, "unexpected setup error")

			err = iox.EnsureDir(filepath.Join(dir, "to-be-created.txt"))
			assert.NoError(t, err, "expected no error")

			info, err := os.Stat(dir)
			assert.NoError(t, err, "expected no error")

			assert.True(t, info.IsDir(), "expected '%s' to be a directory", dir)
		})
	}
}
