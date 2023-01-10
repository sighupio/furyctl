// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package kube

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	iox "github.com/sighupio/furyctl/internal/x/io"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func CreateConfig(data []byte, p string) (string, error) {
	err := os.WriteFile(path.Join(p, "kubeconfig"), data, iox.FullRWPermAccess)
	if err != nil {
		return "", fmt.Errorf("error writing kubeconfig file: %w", err)
	}

	return path.Join(p, "kubeconfig"), nil
}

func SetConfigEnv(p string) error {
	kubePath, err := filepath.Abs(path.Join(p, "kubeconfig"))
	if err != nil {
		return fmt.Errorf("error getting kubeconfig absolute path: %w", err)
	}

	err = os.Setenv("KUBECONFIG", kubePath)
	if err != nil {
		return fmt.Errorf("error setting kubeconfig env: %w", err)
	}

	return nil
}

func CopyConfigToWorkDir(p string) error {
	currentDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("error getting current dir: %w", err)
	}

	kubePath, err := filepath.Abs(path.Join(p, "kubeconfig"))
	if err != nil {
		return fmt.Errorf("error getting kubeconfig absolute path: %w", err)
	}

	kubeconfig, err := os.ReadFile(kubePath)
	if err != nil {
		return fmt.Errorf("error reading kubeconfig file: %w", err)
	}

	err = os.WriteFile(path.Join(currentDir, "kubeconfig"), kubeconfig, iox.FullRWPermAccess)
	if err != nil {
		return fmt.Errorf("error writing kubeconfig file: %w", err)
	}

	return nil
}

func GetConfigFromWorkdir(kubeConfigPath string) (*rest.Config, error) {
	kubeConfigContent, err := ioutil.ReadFile(kubeConfigPath)
	if err != nil {
		return nil, err
	}

	c, err := clientcmd.RESTConfigFromKubeConfig(kubeConfigContent)
	if err != nil {
		return nil, fmt.Errorf("unexpected error while getting config from file: %w", err)
	}

	return c, nil
}
