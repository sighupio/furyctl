// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package lockfile

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	iox "github.com/sighupio/furyctl/internal/x/io"
)

var ErrLockFileExists = errors.New(
	"lock file exists. Last execution finished abnormally or there may be another instance of furyctl running with PID",
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
	pid, err := os.ReadFile(l.Path)
	if err == nil {
		return fmt.Errorf("%w %s", ErrLockFileExists, pid)
	}

	if !os.IsNotExist(err) {
		return fmt.Errorf("error while checking lock file: \"%w\"", err)
	}

	return nil
}

func (l *LockFile) Create() error {
	if err := os.WriteFile(l.Path, []byte(strconv.Itoa(os.Getpid())), iox.RWPermAccessPermissive); err != nil {
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
