// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package kubex

import (
	"fmt"
	"os"
	"path"
	"path/filepath"

	iox "github.com/sighupio/furyctl/internal/x/io"
)

func CreateConfig(data []byte, p string) (string, error) {
	err := iox.WriteFile(path.Join(p, "kubeconfig"), data)
	if err != nil {
		return "", fmt.Errorf("error writing kubeconfig file: %w", err)
	}

	return path.Join(p, "kubeconfig"), nil
}

func SetConfigEnv(p string) error {
	kubePath, err := filepath.Abs(p)
	if err != nil {
		return fmt.Errorf("error getting kubeconfig absolute path: %w", err)
	}

	err = os.Setenv("KUBECONFIG", kubePath)
	if err != nil {
		return fmt.Errorf("error setting kubeconfig env: %w", err)
	}

	return nil
}

func CopyToWorkDir(p string, n string) error {
	currentDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("error getting current dir: %w", err)
	}

	absPath, err := filepath.Abs(p)
	if err != nil {
		return fmt.Errorf("error getting absolute path: %w", err)
	}

	fileBytes, err := os.ReadFile(absPath)
	if err != nil {
		return fmt.Errorf("error reading file: %w", err)
	}

	err = iox.WriteFile(path.Join(currentDir, n), fileBytes)
	if err != nil {
		return fmt.Errorf("error writing file: %w", err)
	}

	return nil
}
