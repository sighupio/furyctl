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

var ErrEmptyFile = errors.New("trimming buffer resulted in an empty file")

func CheckDirIsEmpty(target string) error {
	if _, err := os.Stat(target); os.IsNotExist(err) {
		return nil
	}

	return filepath.Walk(target, func(path string, _ os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("the target directory is not empty, error while checking %s: %w", path, err)
		}

		if target == path {
			return nil
		}

		return fmt.Errorf("the target directory is not empty: %s", path)
	})
}

func AppendToFile(s, target string) error {
	destination, err := os.OpenFile(target, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}

	defer destination.Close()

	_, err = destination.Write([]byte(s))

	return err
}

func CopyBufferToFile(b bytes.Buffer, target string) error {
	if strings.TrimSpace(b.String()) == "" {
		return nil
	}

	destination, err := os.Create(target)
	if err != nil {
		return err
	}

	defer destination.Close()

	_, err = b.WriteTo(destination)

	return err
}

func CopyFile(src, dst string) error {
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return err
	}

	if !sourceFileStat.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", src)
	}

	source, err := os.Open(src)
	if err != nil {
		return err
	}

	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return err
	}

	defer destination.Close()

	_, err = io.Copy(destination, source)

	return err
}

func CopyRecursive(src fs.FS, dest string) error {
	stuff, err := fs.ReadDir(src, ".")
	if err != nil {
		return err
	}

	for _, file := range stuff {
		if file.IsDir() {
			sub, err := fs.Sub(src, file.Name())
			if err != nil {
				return err
			}

			if err := os.Mkdir(path.Join(dest, file.Name()), 0o755); err != nil && !os.IsExist(err) {
				return err
			}

			if err := CopyRecursive(sub, path.Join(dest, file.Name())); err != nil {
				return err
			}

			continue
		}

		fileContent, err := fs.ReadFile(src, file.Name())
		if err != nil {
			return err
		}

		if err := EnsureDir(path.Join(dest, file.Name())); err != nil {
			return err
		}

		if err := os.WriteFile(path.Join(dest, file.Name()), fileContent, 0o600); err != nil {
			return err
		}
	}

	return nil
}

// EnsureDir creates the directories to host the file.
// Example: hello/world.md will create the hello dir if it does not exists.
func EnsureDir(fileName string) (err error) {
	dirName := filepath.Dir(fileName)
	if _, serr := os.Stat(dirName); serr != nil {
		if !os.IsNotExist(serr) {
			return serr
		}

		if err := os.MkdirAll(dirName, os.ModePerm); err != nil {
			return err
		}
	}
	return nil
}
