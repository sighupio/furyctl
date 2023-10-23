// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package state

import (
	"encoding/base64"
	"fmt"
	"os"
	"path"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/furyctl/internal/tool/kubectl"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	iox "github.com/sighupio/furyctl/internal/x/io"
	kubex "github.com/sighupio/furyctl/internal/x/kube"
	yamlx "github.com/sighupio/furyctl/internal/x/yaml"
)

type Storer interface {
	StoreKFD() error
	StoreConfig() error
	GetConfig() ([]byte, error)
}

type Store struct {
	DistroPath    string
	ConfigPath    string
	WorkDir       string
	KubectlRunner *kubectl.Runner
}

func NewStore(distroPath, configPath, kubeconfigPath, workDir, kubectlVersion, binPath string) *Store {
	runner := kubectl.NewRunner(execx.NewStdExecutor(), kubectl.Paths{
		Kubectl:    path.Join(binPath, "kubectl", kubectlVersion, "kubectl"),
		WorkDir:    workDir,
		Kubeconfig: kubeconfigPath,
	}, true, true, false)

	return &Store{
		DistroPath:    distroPath,
		ConfigPath:    configPath,
		WorkDir:       workDir,
		KubectlRunner: runner,
	}
}

func (s *Store) StoreKFD() error {
	x, err := os.ReadFile(path.Join(s.DistroPath, "kfd.yaml"))
	if err != nil {
		return fmt.Errorf("error while reading config file: %w", err)
	}

	secret, err := kubex.CreateSecret(x, "furyctl-kfd", "kfd", "kube-system")
	if err != nil {
		return fmt.Errorf("error while creating secret: %w", err)
	}

	secretPath := path.Join(s.WorkDir, "secrets-kfd.yaml")

	if err := iox.WriteFile(secretPath, secret); err != nil {
		return fmt.Errorf("error while writing secret: %w", err)
	}

	defer os.Remove(secretPath)

	logrus.Info("Saving distribution configuration file in the cluster...")

	if err := s.KubectlRunner.Apply(secretPath); err != nil {
		return fmt.Errorf("error while saving distribution configuration file in the cluster: %w", err)
	}

	return nil
}

func (s *Store) StoreConfig() error {
	x, err := os.ReadFile(s.ConfigPath)
	if err != nil {
		return fmt.Errorf("error while reading config file: %w", err)
	}

	secret, err := kubex.CreateSecret(x, "furyctl-config", "config", "kube-system")
	if err != nil {
		return fmt.Errorf("error while creating secret: %w", err)
	}

	secretPath := path.Join(s.WorkDir, "secrets.yaml")

	if err := iox.WriteFile(secretPath, secret); err != nil {
		return fmt.Errorf("error while writing secret: %w", err)
	}

	defer os.Remove(secretPath)

	logrus.Info("Saving furyctl configuration file in the cluster...")

	if err := s.KubectlRunner.Apply(secretPath); err != nil {
		return fmt.Errorf("error while saving furyctl configuration file in the cluster: %w", err)
	}

	return nil
}

func (s *Store) GetConfig() ([]byte, error) {
	secret := map[string]any{}

	out, err := s.KubectlRunner.Get(true, "kube-system", "secret", "furyctl-config", "-o", "yaml")
	if err != nil {
		return nil, fmt.Errorf("error while getting current cluster config: %w", err)
	}

	if err := yamlx.UnmarshalV3([]byte(out), secret); err != nil {
		return nil, fmt.Errorf("error while unmarshalling current cluster config: %w", err)
	}

	data, ok := secret["data"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("error while getting current cluster config: %w", err)
	}

	configData, ok := data["config"].(string)
	if !ok {
		return nil, fmt.Errorf("error while getting current cluster config: %w", err)
	}

	decodedConfig, err := base64.StdEncoding.DecodeString(configData)
	if err != nil {
		return nil, fmt.Errorf("error while decoding current cluster config: %w", err)
	}

	return decodedConfig, nil
}
