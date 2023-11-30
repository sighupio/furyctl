// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package onpremises

import (
	"fmt"
	"path"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/fury-distribution/pkg/apis/onpremises/v1alpha2/public"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/onpremises/delete"
	"github.com/sighupio/furyctl/internal/cluster"
	kubex "github.com/sighupio/furyctl/internal/x/kube"
)

type ClusterDeleter struct {
	paths       cluster.DeleterPaths
	furyctlConf public.OnpremisesKfdV1Alpha2
	kfdManifest config.KFD
	phase       string
	dryRun      bool
}

func (c *ClusterDeleter) SetProperties(props []cluster.DeleterProperty) {
	for _, prop := range props {
		c.SetProperty(prop.Name, prop.Value)
	}
}

func (c *ClusterDeleter) SetProperty(name string, value any) {
	switch strings.ToLower(name) {
	case cluster.CreatorPropertyConfigPath:
		if s, ok := value.(string); ok {
			c.paths.ConfigPath = s
		}

	case cluster.CreatorPropertyWorkDir:
		if s, ok := value.(string); ok {
			c.paths.WorkDir = s
		}

	case cluster.CreatorPropertyBinPath:
		if s, ok := value.(string); ok {
			c.paths.BinPath = s
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

func (c *ClusterDeleter) Delete() error {
	logrus.Warn("This process will only reset the Kubernetes cluster " +
		"and will not uninstall all the packages installed on the nodes.")

	kubernetesPhase := delete.NewKubernetes(
		c.furyctlConf,
		c.kfdManifest,
		c.paths,
		c.dryRun,
	)

	distributionPhase := delete.NewDistribution(
		c.furyctlConf,
		c.kfdManifest,
		c.paths,
		c.dryRun,
	)

	// Move this code to delete preflight.
	if err := kubex.SetConfigEnv(path.Join(c.paths.WorkDir, cluster.OperationPhasePreFlight, "admin.conf")); err != nil {
		return fmt.Errorf("error setting kubeconfig env: %w", err)
	}

	switch c.phase {
	case cluster.OperationPhaseKubernetes:
		if err := kubernetesPhase.Exec(); err != nil {
			return fmt.Errorf("error while deleting kubernetes phase: %w", err)
		}

	case cluster.OperationPhaseDistribution:
		if err := distributionPhase.Exec(); err != nil {
			return fmt.Errorf("error while deleting distribution phase: %w", err)
		}

	case cluster.OperationPhaseAll:
		if err := distributionPhase.Exec(); err != nil {
			return fmt.Errorf("error while deleting distribution phase: %w", err)
		}

		if err := kubernetesPhase.Exec(); err != nil {
			return fmt.Errorf("error while deleting kubernetes phase: %w", err)
		}

	default:
		return ErrUnsupportedPhase
	}

	return nil
}
