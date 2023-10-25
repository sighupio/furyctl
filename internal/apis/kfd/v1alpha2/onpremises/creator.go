// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package onpremises

import (
	"errors"
	"fmt"
	"strings"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/fury-distribution/pkg/apis/onpremises/v1alpha2/public"
	commcreate "github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/common/create"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/onpremises/create"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/state"
)

const (
	KubernetesPhaseSchemaPath   = ".spec.kubernetes"
	DistributionPhaseSchemaPath = ".spec.distribution"
	PluginsPhaseSchemaPath      = ".spec.plugins"
	AllPhaseSchemaPath          = ""
)

var ErrUnsupportedPhase = errors.New("unsupported phase")

type ClusterCreator struct {
	paths       cluster.CreatorPaths
	furyctlConf public.OnpremisesKfdV1Alpha2
	stateStore  state.Storer
	kfdManifest config.KFD
	phase       string
	dryRun      bool
}

func (c *ClusterCreator) SetProperties(props []cluster.CreatorProperty) {
	for _, prop := range props {
		c.SetProperty(prop.Name, prop.Value)
	}

	c.stateStore = state.NewStore(
		c.paths.DistroPath,
		c.paths.ConfigPath,
		c.paths.Kubeconfig,
		c.paths.WorkDir,
		c.kfdManifest.Tools.Common.Kubectl.Version,
		c.paths.BinPath,
	)
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

func (*ClusterCreator) GetPhasePath(phase string) (string, error) {
	switch phase {
	case cluster.OperationPhaseKubernetes:
		return KubernetesPhaseSchemaPath, nil

	case cluster.OperationPhaseDistribution:
		return DistributionPhaseSchemaPath, nil

	case cluster.OperationPhasePlugins:
		return PluginsPhaseSchemaPath, nil

	case cluster.OperationPhaseAll:
		return AllPhaseSchemaPath, nil

	default:
		return "", ErrUnsupportedPhase
	}
}

func (c *ClusterCreator) Create(skipPhase string, _ int) error {
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

	preflight, err := create.NewPreFlight(
		c.furyctlConf,
		c.kfdManifest,
		c.paths,
		c.dryRun,
		c.paths.Kubeconfig,
		c.stateStore,
	)
	if err != nil {
		return fmt.Errorf("error while initiating preflight phase: %w", err)
	}

	if err := preflight.Exec(); err != nil {
		return fmt.Errorf("error while executing preflight phase: %w", err)
	}

	switch c.phase {
	case cluster.OperationPhaseKubernetes:
		if err := kubernetesPhase.Exec(); err != nil {
			return fmt.Errorf("error while executing kubernetes phase: %w", err)
		}

	case cluster.OperationPhaseDistribution:
		if err := distributionPhase.Exec(); err != nil {
			return fmt.Errorf("error while executing distribution phase: %w", err)
		}

	case cluster.OperationPhasePlugins:
		if err := pluginsPhase.Exec(); err != nil {
			return fmt.Errorf("error while executing plugins phase: %w", err)
		}

	case cluster.OperationPhaseAll:
		if skipPhase != cluster.OperationPhaseKubernetes {
			if err := kubernetesPhase.Exec(); err != nil {
				return fmt.Errorf("error while executing kubernetes phase: %w", err)
			}
		}

		if skipPhase != cluster.OperationPhaseDistribution {
			if err := distributionPhase.Exec(); err != nil {
				return fmt.Errorf("error while executing distribution phase: %w", err)
			}
		}

		if skipPhase != cluster.OperationPhasePlugins {
			if err := pluginsPhase.Exec(); err != nil {
				return fmt.Errorf("error while executing plugins phase: %w", err)
			}
		}

	default:
		return ErrUnsupportedPhase
	}

	if c.dryRun {
		return nil
	}

	if err := c.stateStore.StoreConfig(); err != nil {
		return fmt.Errorf("error while creating secret with the cluster configuration: %w", err)
	}

	if err := c.stateStore.StoreKFD(); err != nil {
		return fmt.Errorf("error while creating secret with the distribution configuration: %w", err)
	}

	return nil
}
