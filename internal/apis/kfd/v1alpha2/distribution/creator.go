// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package distribution

import (
	"errors"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/fury-distribution/pkg/apis/kfddistribution/v1alpha2/public"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/distribution/create"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/tool/kubectl"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	iox "github.com/sighupio/furyctl/internal/x/io"
	kubex "github.com/sighupio/furyctl/internal/x/kube"
)

var ErrUnsupportedPhase = errors.New("unsupported phase")

type ClusterCreator struct {
	paths       cluster.CreatorPaths
	furyctlConf public.KfddistributionKfdV1Alpha2
	kfdManifest config.KFD
	phase       string
	dryRun      bool
}

func (v *ClusterCreator) SetProperties(props []cluster.CreatorProperty) {
	for _, prop := range props {
		v.SetProperty(prop.Name, prop.Value)
	}
}

func (v *ClusterCreator) SetProperty(name string, value any) {
	switch strings.ToLower(name) {
	case cluster.CreatorPropertyConfigPath:
		if s, ok := value.(string); ok {
			v.paths.ConfigPath = s
		}

	case cluster.CreatorPropertyDistroPath:
		if s, ok := value.(string); ok {
			v.paths.DistroPath = s
		}

	case cluster.CreatorPropertyWorkDir:
		if s, ok := value.(string); ok {
			v.paths.WorkDir = s
		}

	case cluster.CreatorPropertyBinPath:
		if s, ok := value.(string); ok {
			v.paths.BinPath = s
		}

	case cluster.CreatorPropertyKubeconfig:
		if s, ok := value.(string); ok {
			v.paths.Kubeconfig = s
		}

	case cluster.CreatorPropertyFuryctlConf:
		if s, ok := value.(public.KfddistributionKfdV1Alpha2); ok {
			v.furyctlConf = s
		}

	case cluster.CreatorPropertyKfdManifest:
		if s, ok := value.(config.KFD); ok {
			v.kfdManifest = s
		}

	case cluster.CreatorPropertyPhase:
		if s, ok := value.(string); ok {
			v.phase = s
		}

	case cluster.CreatorPropertyDryRun:
		if b, ok := value.(bool); ok {
			v.dryRun = b
		}
	}
}

func (v *ClusterCreator) Create(_ string, _ int) error {
	if v.phase != "" && v.phase != cluster.OperationPhaseDistribution {
		return ErrUnsupportedPhase
	}

	distro, err := create.NewDistribution(
		v.paths,
		v.furyctlConf,
		v.kfdManifest,
		v.dryRun,
		v.paths.Kubeconfig,
	)
	if err != nil {
		return fmt.Errorf("error while initiating distribution phase: %w", err)
	}

	if err := distro.Exec(); err != nil {
		return fmt.Errorf("error while installing Kubernetes Fury Distribution: %w", err)
	}

	if v.dryRun {
		logrus.Info("Kubernetes Fury Distribution installed successfully (dry-run mode)")

		return nil
	}

	if err := v.storeClusterConfig(); err != nil {
		return fmt.Errorf("error while storing cluster config: %w", err)
	}

	if err := v.storeDistributionConfig(); err != nil {
		return fmt.Errorf("error while storing distribution config: %w", err)
	}

	logrus.Info("Kubernetes Fury Distribution installed successfully")

	return nil
}

func (v *ClusterCreator) storeClusterConfig() error {
	x, err := os.ReadFile(v.paths.ConfigPath)
	if err != nil {
		return fmt.Errorf("error while reading config file: %w", err)
	}

	secret, err := kubex.CreateSecret(x, "furyctl-config", "config", "kube-system")
	if err != nil {
		return fmt.Errorf("error while creating secret: %w", err)
	}

	secretPath := path.Join(v.paths.WorkDir, "secrets.yaml")

	if err := iox.WriteFile(secretPath, secret); err != nil {
		return fmt.Errorf("error while writing secret: %w", err)
	}

	defer os.Remove(secretPath)

	runner := kubectl.NewRunner(execx.NewStdExecutor(), kubectl.Paths{
		Kubectl:    path.Join(v.paths.BinPath, "kubectl", v.kfdManifest.Tools.Common.Kubectl.Version, "kubectl"),
		WorkDir:    v.paths.WorkDir,
		Kubeconfig: v.paths.Kubeconfig,
	}, true, true, false)

	logrus.Info("Saving furyctl configuration file in the cluster...")

	if err := runner.Apply(secretPath); err != nil {
		return fmt.Errorf("error while saving furyctl configuration file in the cluster: %w", err)
	}

	return nil
}

func (v *ClusterCreator) storeDistributionConfig() error {
	x, err := os.ReadFile(path.Join(v.paths.DistroPath, "kfd.yaml"))
	if err != nil {
		return fmt.Errorf("error while reading config file: %w", err)
	}

	secret, err := kubex.CreateSecret(x, "furyctl-kfd", "kfd", "kube-system")
	if err != nil {
		return fmt.Errorf("error while creating secret: %w", err)
	}

	secretPath := path.Join(v.paths.WorkDir, "secrets-kfd.yaml")

	if err := iox.WriteFile(secretPath, secret); err != nil {
		return fmt.Errorf("error while writing secret: %w", err)
	}

	defer os.Remove(secretPath)

	runner := kubectl.NewRunner(execx.NewStdExecutor(), kubectl.Paths{
		Kubectl:    path.Join(v.paths.BinPath, "kubectl", v.kfdManifest.Tools.Common.Kubectl.Version, "kubectl"),
		WorkDir:    v.paths.WorkDir,
		Kubeconfig: v.paths.Kubeconfig,
	}, true, true, false)

	logrus.Info("Saving distribution configuration file in the cluster...")

	if err := runner.Apply(secretPath); err != nil {
		return fmt.Errorf("error while saving distribution configuration file in the cluster: %w", err)
	}

	return nil
}
