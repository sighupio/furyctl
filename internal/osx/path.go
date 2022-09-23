// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package osx

import (
	"errors"
	"os"
)

func CleanupTempDir(dir string) error {
	if err := os.RemoveAll(dir); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}

	return nil
}
