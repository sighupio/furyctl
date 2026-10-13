// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package osx

import (
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/sirupsen/logrus"
)

//nolint:gochecknoglobals // The temporary directories are shared between all the command instances.
var (
	tempDirsMu sync.Mutex
	tempDirs   []string
)

func CleanupTempDir(dir string) error {
	if err := os.RemoveAll(dir); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("error removing dir: %w", err)
	}

	return nil
}

// RegisterTempDir adds dir to the directories that RemoveTempDirs removes. Use it for a directory
// that must stay until furyctl exits.
func RegisterTempDir(dir string) {
	tempDirsMu.Lock()
	defer tempDirsMu.Unlock()

	tempDirs = append(tempDirs, dir)
}

// RemoveTempDirs removes the directories that RegisterTempDir adds. The main function and the signal
// handler call it before furyctl exits.
func RemoveTempDirs() {
	tempDirsMu.Lock()
	defer tempDirsMu.Unlock()

	for _, dir := range tempDirs {
		if err := CleanupTempDir(dir); err != nil {
			logrus.Debugf("furyctl cannot remove the temporary folder %s: %v", dir, err)
		}
	}

	tempDirs = nil
}
