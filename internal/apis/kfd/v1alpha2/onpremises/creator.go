// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package onpremises

import (
	"errors"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/fury-distribution/pkg/apis/onpremises/v1alpha2/public"
	commcreate "github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/common/create"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/onpremises/create"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/tool/kubectl"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	iox "github.com/sighupio/furyctl/internal/x/io"
	kubex "github.com/sighupio/furyctl/internal/x/kube"
)

var ErrUnsupportedPhase = errors.New("unsupported phase")

type ClusterCreator struct {
	paths       cluster.CreatorPaths
	furyctlConf public.OnpremisesKfdV1Alpha2
	kfdManifest config.KFD
	phase       string
	dryRun      bool
}

func (c *ClusterCreator) SetProperties(props []cluster.CreatorProperty) {
	for _, prop := range props {
		c.SetProperty(prop.Name, prop.Value)
	}
}

func (c *ClusterCreator) SetProperty(name string, value any) {
	switch strings.ToLower(name) {
	case cluster.CreatorPropertyConfigPath:
		if s, ok := value.(string); ok {
			c.paths.ConfigPath = s
		}

	case cluster.CreatorPropertyDistroPath:
		if s, ok := value.(string); ok {
			c.paths.DistroPath = s
		}

	case cluster.CreatorPropertyWorkDir:
		if s, ok := value.(string); ok {
			c.paths.WorkDir = s
		}

	case cluster.CreatorPropertyBinPath:
		if s, ok := value.(string); ok {
			c.paths.BinPath = s
		}

	case cluster.CreatorPropertyKubeconfig:
		if s, ok := value.(string); ok {
			c.paths.Kubeconfig = s
		}

	case cluster.CreatorPropertyFuryctlConf:
		if s, ok := value.(public.OnpremisesKfdV1Alpha2); ok {
			c.furyctlConf = s
		}

	case cluster.CreatorPropertyKfdManifest:
		if s, ok := value.(config.KFD); ok {
			c.kfdManifest = s
		}

	case cluster.CreatorPropertyPhase:
		if s, ok := value.(string); ok {
			c.phase = s
		}

	case cluster.CreatorPropertyDryRun:
		if b, ok := value.(bool); ok {
			c.dryRun = b
		}
	}
}

func (c *ClusterCreator) Create(_ string, _ int) error {
	kubernetesPhase, err := create.NewKubernetes(
		c.furyctlConf,
		c.kfdManifest,
		c.paths,
		c.dryRun,
	)
	if err != nil {
		return fmt.Errorf("error while initiating kubernetes phase: %w", err)
	}

	distributionPhase, err := create.NewDistribution(
		c.furyctlConf,
		c.kfdManifest,
		c.paths,
		c.dryRun,
	)
	if err != nil {
		return fmt.Errorf("error while initiating distribution phase: %w", err)
	}

	pluginsPhase, err := commcreate.NewPlugins(
		c.paths,
		c.kfdManifest,
		string(c.furyctlConf.Kind),
		c.dryRun,
		c.paths.Kubeconfig,
	)
	if err != nil {
		return fmt.Errorf("error while initiating plugins phase: %w", err)
	}

	switch c.phase {
	case cluster.OperationPhaseKubernetes:
		return kubernetesPhase.Exec()

	case cluster.OperationPhaseDistribution:
		return distributionPhase.Exec()

	case cluster.OperationPhasePlugins:
		return pluginsPhase.Exec()

	case cluster.OperationPhaseAll:
		if err := kubernetesPhase.Exec(); err != nil {
			return err
		}

		if err := distributionPhase.Exec(); err != nil {
			return err
		}

		if err := pluginsPhase.Exec(); err != nil {
			return err
		}

	default:
		return ErrUnsupportedPhase
	}

	if c.dryRun {
		return nil
	}

	if err := c.storeClusterConfig(); err != nil {
		return fmt.Errorf("error while creating secret with the cluster configuration: %w", err)
	}

	if err := c.storeDistributionConfig(); err != nil {
		return fmt.Errorf("error while creating secret with the distribution configuration: %w", err)
	}

	return nil
}

func (c *ClusterCreator) storeClusterConfig() error {
	// TODO: this code is duplicated, we should refactor it.

	x, err := os.ReadFile(c.paths.ConfigPath)
	if err != nil {
		return fmt.Errorf("error while reading config file: %w", err)
	}

	secret, err := kubex.CreateSecret(x, "furyctl-config", "config", "kube-system")
	if err != nil {
		return fmt.Errorf("error while creating secret: %w", err)
	}

	secretPath := path.Join(c.paths.WorkDir, "secrets.yaml")

	if err := iox.WriteFile(secretPath, secret); err != nil {
		return fmt.Errorf("error while writing secret: %w", err)
	}

	defer os.Remove(secretPath)

	runner := kubectl.NewRunner(execx.NewStdExecutor(), kubectl.Paths{
		Kubectl:    path.Join(c.paths.BinPath, "kubectl", c.kfdManifest.Tools.Common.Kubectl.Version, "kubectl"),
		WorkDir:    c.paths.WorkDir,
		Kubeconfig: c.paths.Kubeconfig,
	}, true, true, false)

	logrus.Info("Saving furyctl configuration file in the cluster...")

	if err := runner.Apply(secretPath); err != nil {
		return fmt.Errorf("error while saving furyctl configuration file in the cluster: %w", err)
	}

	return nil
}

func (c *ClusterCreator) storeDistributionConfig() error {
	// TODO: this code is duplicated, we should refactor it.

	x, err := os.ReadFile(path.Join(c.paths.DistroPath, "kfd.yaml"))
	if err != nil {
		return fmt.Errorf("error while reading config file: %w", err)
	}

	secret, err := kubex.CreateSecret(x, "furyctl-kfd", "kfd", "kube-system")
	if err != nil {
		return fmt.Errorf("error while creating secret: %w", err)
	}

	secretPath := path.Join(c.paths.WorkDir, "secrets-kfd.yaml")

	if err := iox.WriteFile(secretPath, secret); err != nil {
		return fmt.Errorf("error while writing secret: %w", err)
	}

	defer os.Remove(secretPath)

	runner := kubectl.NewRunner(execx.NewStdExecutor(), kubectl.Paths{
		Kubectl:    path.Join(c.paths.BinPath, "kubectl", c.kfdManifest.Tools.Common.Kubectl.Version, "kubectl"),
		WorkDir:    c.paths.WorkDir,
		Kubeconfig: c.paths.Kubeconfig,
	}, true, true, false)

	logrus.Info("Saving distribution configuration file in the cluster...")

	if err := runner.Apply(secretPath); err != nil {
		return fmt.Errorf("error while saving distribution configuration file in the cluster: %w", err)
	}

	return nil
}
