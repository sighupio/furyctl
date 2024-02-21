// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package iox

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const (
	FullPermAccess   = 0o755
	UserGroupPerm    = 0o750
	FullRWPermAccess = 0o600
	RWPermAccess     = 0o644
)

var (
	ErrEmptyFile         = errors.New("trimming buffer resulted in an empty file")
	errTargetDirNotEmpty = errors.New("the target directory is not empty")
	errNotRegularFile    = errors.New("is not a regular file")
)

func CheckDirIsEmpty(target string) error {
	if _, err := os.Stat(target); os.IsNotExist(err) {
		return nil
	}

	err := filepath.Walk(target, func(path string, _ os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("the target directory is not empty, error while checking %s: %w", path, err)
		}

		if target == path {
			return nil
		}

		return fmt.Errorf("%w: %s", errTargetDirNotEmpty, path)
	})
	if err != nil {
		return fmt.Errorf("error while checking path %s: %w", target, err)
	}

	return nil
}

func WriteFile(target string, data []byte) error {
	if err := os.WriteFile(target, data, FullRWPermAccess); err != nil {
		return fmt.Errorf("error while writing file %s: %w", target, err)
	}

	return nil
}

func AppendToFile(s, target string) error {
	destination, err := os.OpenFile(target, os.O_APPEND|os.O_CREATE|os.O_WRONLY, RWPermAccess)
	if err != nil {
		return fmt.Errorf("error while opening file %s: %w", target, err)
	}

	defer destination.Close()

	_, err = destination.WriteString(s)
	if err != nil {
		return fmt.Errorf("error while writing to file %s: %w", target, err)
	}

	return nil
}

func CopyBufferToFile(b bytes.Buffer, target string) error {
	if strings.TrimSpace(b.String()) == "" {
		return nil
	}

	destination, err := os.Create(target)
	if err != nil {
		return fmt.Errorf("error while creating file %s: %w", target, err)
	}

	defer destination.Close()

	_, err = b.WriteTo(destination)
	if err != nil {
		return fmt.Errorf("error while writing to file %s: %w", target, err)
	}

	return nil
}

func CopyFile(src, dst string) error {
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("error while getting file info %s: %w", src, err)
	}

	if !sourceFileStat.Mode().IsRegular() {
		return fmt.Errorf("%s %w", src, errNotRegularFile)
	}

	source, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("error while opening file %s: %w", src, err)
	}

	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("error while creating file %s: %w", dst, err)
	}

	defer destination.Close()

	_, err = io.Copy(destination, source)
	if err != nil {
		return fmt.Errorf("error while copying file %s to %s: %w", src, dst, err)
	}

	return nil
}

func CopyRecursive(src fs.FS, dest string) error {
	stuff, err := fs.ReadDir(src, ".")
	if err != nil {
		return fmt.Errorf("error while reading directory %s: %w", src, err)
	}

	for _, file := range stuff {
		if file.IsDir() {
			sub, err := fs.Sub(src, file.Name())
			if err != nil {
				return fmt.Errorf("error while converting sub directory %s to fs.FS: %w", file.Name(), err)
			}

			if err := os.Mkdir(path.Join(dest, file.Name()), FullPermAccess); err != nil && !os.IsExist(err) {
				return fmt.Errorf("error while creating directory %s: %w", file.Name(), err)
			}

			if err := CopyRecursive(sub, path.Join(dest, file.Name())); err != nil {
				return err
			}

			continue
		}

		fileContent, err := fs.ReadFile(src, file.Name())
		if err != nil {
			return fmt.Errorf("error while reading file %s: %w", file.Name(), err)
		}

		if err := EnsureDir(path.Join(dest, file.Name())); err != nil {
			return err
		}

		if err := os.WriteFile(path.Join(dest, file.Name()), fileContent, RWPermAccess); err != nil {
			return fmt.Errorf("error while writing file %s: %w", file.Name(), err)
		}

		si, err := fs.Stat(src, file.Name())
		if err != nil {
			return fmt.Errorf("error while getting file info %s: %w", file.Name(), err)
		}

		err = os.Chmod(path.Join(dest, file.Name()), si.Mode())
		if err != nil {
			return fmt.Errorf("error while changing file mode %s: %w", file.Name(), err)
		}
	}

	return nil
}

// EnsureDir creates the directories to host the file.
// Example: hello/world.md will create the hello dir if it does not exist.
func EnsureDir(fileName string) error {
	dirName := filepath.Dir(fileName)
	if _, serr := os.Stat(dirName); serr != nil {
		if !os.IsNotExist(serr) {
			return fmt.Errorf("error while checking if directory %s exists: %w", dirName, serr)
		}

		if err := os.MkdirAll(dirName, os.ModePerm); err != nil {
			return fmt.Errorf("error while creating directory %s: %w", dirName, err)
		}
	}

	return nil
}
