// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package lockfile

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

var ErrLockFileExists = errors.New(
	"lock file exists. This usually means that there is another instance of furyctl running",
)

type LockFile struct {
	Path string
}

func NewLockFile(clusterName string) *LockFile {
	fileName := "furyctl-" + clusterName

	path := filepath.Join(os.TempDir(), fileName)

	return &LockFile{Path: path}
}

func (l *LockFile) Verify() error {
	_, err := os.Stat(l.Path)
	if err == nil {
		return ErrLockFileExists
	}

	if !os.IsNotExist(err) {
		return fmt.Errorf("error while checking lock file: %w", err)
	}

	return nil
}

func (l *LockFile) Create() error {
	_, err := os.Create(l.Path)
	if err != nil {
		return fmt.Errorf("error while creating lock file: %w", err)
	}

	return nil
}

func (l *LockFile) Remove() error {
	err := os.Remove(l.Path)
	if err != nil {
		return fmt.Errorf("error while removing lock file: %w", err)
	}

	return nil
}
