// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
)

func CheckDirIsEmpty(target string) error {
	if _, err := os.Stat(target); os.IsNotExist(err) {
		return nil
	}

	return filepath.Walk(target, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("the target directory is not empty, error while checking %s: %w", path, err)
		}

		return fmt.Errorf("the target directory is not empty: %s", path)
	})
}

func AppendBufferToFile(b bytes.Buffer, target string) error {
	destination, err := os.OpenFile(target, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}

	defer destination.Close()

	_, err = b.WriteTo(destination)
	if err != nil {
		return err
	}

	return nil
}

func CopyBufferToFile(b bytes.Buffer, source, target string) error {
	if strings.TrimSpace(b.String()) == "" {
		logrus.Printf("%s --> resulted in an empty file (%d bytes). Skipping.\n", source, b.Len())
		return nil
	}

	logrus.Printf("%s --> %s\n", source, target)

	destination, err := os.Create(target)
	if err != nil {
		return err
	}

	defer destination.Close()

	_, err = b.WriteTo(destination)
	if err != nil {
		return err
	}

	return nil
}

func CopyFromSourceToTarget(src, dst string) (int64, error) {
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return 0, err
	}

	if !sourceFileStat.Mode().IsRegular() {
		return 0, fmt.Errorf("%s is not a regular file", src)
	}

	source, err := os.Open(src)
	if err != nil {
		return 0, err
	}

	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return 0, err
	}

	defer destination.Close()

	return io.Copy(destination, source)
}

// EnsureDir creates the directories to host the file.
// Example: hello/world.md will create the hello dir if it does not exists.
func EnsureDir(fileName string) (err error) {
	dirName := filepath.Dir(fileName)
	if _, serr := os.Stat(dirName); serr != nil {
		err := os.MkdirAll(dirName, os.ModePerm)
		if err != nil {
			return err
		}
	}
	return nil
}
