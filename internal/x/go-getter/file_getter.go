// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gogetterx

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	gogetter "github.com/hashicorp/go-getter"
)

type getter struct {
	client *gogetter.Client
}

func (g *getter) SetClient(c *gogetter.Client) { g.client = c }

func (g *getter) Context() context.Context {
	if g == nil || g.client == nil {
		return context.Background()
	}

	return g.client.Ctx
}

// umask returns the effective umask for the Client, defaulting to the process umask.
func (g *getter) umask() os.FileMode {
	if g == nil {
		return 0
	}

	return g.client.Umask
}

// mode returns file mode umasked by the Client umask.
func (g *getter) mode(mode os.FileMode) os.FileMode {
	m := mode & ^g.umask()

	return m
}

type FileGetter struct {
	getter

	Copy bool
}

func (g *FileGetter) ClientMode(u *url.URL) (gogetter.ClientMode, error) {
	path := u.Path
	if u.RawPath != "" {
		path = u.RawPath
	}

	fi, err := os.Stat(path)
	if err != nil {
		return 0, err
	}

	// Check if the source is a directory.
	if fi.IsDir() {
		return gogetter.ClientModeDir, nil
	}

	return gogetter.ClientModeFile, nil
}

func (g *FileGetter) Get(dst string, u *url.URL) error {
	path := u.Path
	if u.RawPath != "" {
		path = u.RawPath
	}

	// The source path must exist and be a directory to be usable.
	if fi, err := os.Stat(path); err != nil {
		return fmt.Errorf("source path error: %s", err)
	} else if !fi.IsDir() {
		return fmt.Errorf("source path must be a directory")
	}

	fi, err := os.Lstat(dst)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	// If the destination already exists, it must be a symlink.
	if err == nil {
		mode := fi.Mode()
		if mode&os.ModeSymlink == 0 && !g.Copy {
			return fmt.Errorf("destination exists and is not a symlink")
		}

		// Remove the destination
		if err := os.RemoveAll(dst); err != nil {
			return err
		}
	}

	// Create all the parent directories.
	if err := os.MkdirAll(filepath.Dir(dst), g.mode(0o755)); err != nil {
		return err
	}

	if !g.Copy {
		return os.Symlink(path, dst)
	}

	if err := os.Mkdir(dst, g.mode(0o755)); err != nil {
		return err
	}

	return copyDir(g.Context(), dst, path, false, g.client.DisableSymlinks, g.umask())
}

func (g *FileGetter) GetFile(dst string, u *url.URL) error {
	ctx := g.Context()
	path := u.Path

	if u.RawPath != "" {
		path = u.RawPath
	}

	// The source path must exist and be a file to be usable.
	var fi os.FileInfo

	var err error

	if fi, err = os.Stat(path); err != nil {
		return fmt.Errorf("source path error: %s", err)
	} else if fi.IsDir() {
		return fmt.Errorf("source path must be a file")
	}

	_, err = os.Lstat(dst)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	// If the destination already exists, it must be a symlink.
	if err == nil {
		// Remove the destination.
		if err := os.Remove(dst); err != nil {
			return err
		}
	}

	// Create all the parent directories.
	if err = os.MkdirAll(filepath.Dir(dst), g.mode(0o755)); err != nil {
		return err
	}

	// If we're not copying, just symlink and we're done.
	if !g.Copy {
		return os.Symlink(path, dst)
	}

	var disableSymlinks bool

	if g.client != nil && g.client.DisableSymlinks {
		disableSymlinks = true
	}

	// Copy.
	_, err = copyFile(ctx, dst, path, disableSymlinks, fi.Mode(), g.umask())

	return err
}

// readerFunc is syntactic sugar for read interface.
type readerFunc func(p []byte) (n int, err error)

func (rf readerFunc) Read(p []byte) (n int, err error) { return rf(p) }

// Copy is a io.Copy cancellable by context.
func Copy(ctx context.Context, dst io.Writer, src io.Reader) (int64, error) {
	// Copy will call the Reader and Writer interface multiple time, in order
	// to copy by chunk (avoiding loading the whole file in memory).
	return io.Copy(dst, readerFunc(func(p []byte) (int, error) {
		select {
		case <-ctx.Done():
			// Context has been canceled
			// stop process and propagate "context canceled" error.
			return 0, ctx.Err()

		default:
			// Otherwise just run default io.Reader implementation.
			return src.Read(p)
		}
	}))
}

// mode returns the file mode masked by the umask.
func mode(mode, umask os.FileMode) os.FileMode {
	return mode & ^umask
}

// copyFile copies a file in chunks from src path to dst path, using umask to create the dst file.
func copyFile(ctx context.Context, dst, src string, disableSymlinks bool, fmode, umask os.FileMode) (int64, error) {
	if disableSymlinks {
		fileInfo, err := os.Lstat(src)
		if err != nil {
			return 0, fmt.Errorf("failed to check copy file source for symlinks: %w", err)
		}
		if fileInfo.Mode()&os.ModeSymlink == os.ModeSymlink {
			return 0, gogetter.ErrSymlinkCopy
		}
	}

	srcF, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer srcF.Close()

	dstF, err := os.OpenFile(dst, os.O_RDWR|os.O_CREATE|os.O_TRUNC, fmode)
	if err != nil {
		return 0, err
	}
	defer dstF.Close()

	count, err := Copy(ctx, dstF, srcF)
	if err != nil {
		return 0, err
	}

	// Explicitly chmod; the process umask is unconditionally applied otherwise.
	// We'll mask the mode with our own umask, but that may be different than
	// the process umask.
	err = os.Chmod(dst, mode(fmode, umask))

	return count, err
}

// copyDir copies the src directory contents into dst. Both directories
// should already exist.
//
// If ignoreDot is set to true, then dot-prefixed files/folders are ignored.
func copyDir(ctx context.Context, dst, src string, ignoreDot, disableSymlinks bool, umask os.FileMode) error {
	// We can safely evaluate the symlinks here, even if disabled, because they
	// will be checked before actual use in walkFn and copyFile.
	var err error

	src, err = filepath.EvalSymlinks(src)
	if err != nil {
		return err
	}

	walkFn := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if disableSymlinks {
			fileInfo, err := os.Lstat(path)
			if err != nil {
				return fmt.Errorf("failed to check copy file source for symlinks: %w", err)
			}

			if fileInfo.Mode()&os.ModeSymlink == os.ModeSymlink {
				return gogetter.ErrSymlinkCopy
			}

			if info.Mode()&os.ModeSymlink == os.ModeSymlink {
				return gogetter.ErrSymlinkCopy
			}
		}

		if path == src {
			return nil
		}

		if ignoreDot && strings.HasPrefix(filepath.Base(path), ".") {
			// Skip any dot files.
			if info.IsDir() {
				return filepath.SkipDir
			} else {
				return nil
			}
		}

		// The "path" has the src prefixed to it. We need to join our
		// destination with the path without the src on it.
		dstPath := filepath.Join(dst, path[len(src):])

		// If we have a directory, make that subdirectory, then continue
		// the walk.
		if info.IsDir() {
			if path == filepath.Join(src, dst) {
				// Dst is in src; don't walk it.
				return nil
			}

			if err := os.MkdirAll(dstPath, mode(0o755, umask)); err != nil {
				return err
			}

			return nil
		}

		// If we have a file, copy the contents.
		_, err = copyFile(ctx, dstPath, path, disableSymlinks, info.Mode(), umask)

		return err
	}

	return filepath.Walk(src, walkFn)
}
