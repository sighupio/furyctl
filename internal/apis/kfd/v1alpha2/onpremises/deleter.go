// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package onpremises

import (
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/fury-distribution/pkg/apis/onpremises/v1alpha2/public"
	del "github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/onpremises/delete"
	"github.com/sighupio/furyctl/internal/cluster"
)

type ClusterDeleter struct {
	paths       cluster.DeleterPaths
	furyctlConf public.OnpremisesKfdV1Alpha2
	kfdManifest config.KFD
	phase       string
	dryRun      bool
}

func (d *ClusterDeleter) SetProperties(props []cluster.DeleterProperty) {
	for _, prop := range props {
		d.SetProperty(prop.Name, prop.Value)
	}
}

func (d *ClusterDeleter) SetProperty(name string, value any) {
	switch strings.ToLower(name) {
	case cluster.DeleterPropertyDistroPath:
		if s, ok := value.(string); ok {
			d.paths.DistroPath = s
		}

	case cluster.DeleterPropertyWorkDir:
		if s, ok := value.(string); ok {
			d.paths.WorkDir = s
		}

	case cluster.DeleterPropertyBinPath:
		if s, ok := value.(string); ok {
			d.paths.BinPath = s
		}

	case cluster.DeleterPropertyConfigPath:
		if s, ok := value.(string); ok {
			d.paths.ConfigPath = s
		}

	case cluster.CreatorPropertyFuryctlConf:
		if s, ok := value.(public.OnpremisesKfdV1Alpha2); ok {
			d.furyctlConf = s
		}

	case cluster.CreatorPropertyKfdManifest:
		if s, ok := value.(config.KFD); ok {
			d.kfdManifest = s
		}

	case cluster.CreatorPropertyPhase:
		if s, ok := value.(string); ok {
			d.phase = s
		}

	case cluster.CreatorPropertyDryRun:
		if b, ok := value.(bool); ok {
			d.dryRun = b
		}
	}
}

func (d *ClusterDeleter) Delete() error {
	logrus.Warn("This process will only reset the Kubernetes cluster " +
		"and will not uninstall all the packages installed on the nodes.")

	kubernetesPhase := del.NewKubernetes(
		d.furyctlConf,
		d.kfdManifest,
		d.paths,
		d.dryRun,
	)

	distributionPhase := del.NewDistribution(
		d.furyctlConf,
		d.kfdManifest,
		d.paths,
		d.dryRun,
	)

	preflight := del.NewPreFlight(d.furyctlConf, d.kfdManifest, d.paths, d.dryRun)

	if err := preflight.Exec(); err != nil {
		return fmt.Errorf("error while executing preflight phase: %w", err)
	}

	switch d.phase {
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
