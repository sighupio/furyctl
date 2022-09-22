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
