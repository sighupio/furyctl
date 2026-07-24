// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package kfddistribution

import (
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/furyctl/internal/apis/config"
	del "github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/kfddistribution/delete"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/kfddistribution/public"
	"github.com/sighupio/furyctl/internal/cluster"
)

type ClusterDeleter struct {
	paths       cluster.DeleterPaths
	kfdManifest config.KFD
	furyctlConf public.KfddistributionKfdV1Alpha2
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
		cluster.SetPropertyValue(value, &d.paths.DistroPath)
	case cluster.DeleterPropertyWorkDir:
		cluster.SetPropertyValue(value, &d.paths.WorkDir)
	case cluster.DeleterPropertyBinPath:
		cluster.SetPropertyValue(value, &d.paths.BinPath)
	case cluster.DeleterPropertyConfigPath:
		cluster.SetPropertyValue(value, &d.paths.ConfigPath)
	case cluster.DeleterPropertyKfdManifest:
		cluster.SetPropertyValue(value, &d.kfdManifest)
	case cluster.DeleterPropertyFuryctlConf:
		cluster.SetPropertyValue(value, &d.furyctlConf)
	case cluster.DeleterPropertyPhase:
		cluster.SetPropertyValue(value, &d.phase)
	case cluster.DeleterPropertyDryRun:
		cluster.SetPropertyValue(value, &d.dryRun)
	default:
		logrus.Debugf("ignoring unknown property %q", name)
	}
}

func (d *ClusterDeleter) Delete() error {
	if d.phase != "" && d.phase != cluster.OperationPhaseDistribution {
		return ErrUnsupportedPhase
	}

	preflight := del.NewPreFlight(d.furyctlConf, d.kfdManifest, d.paths)

	if err := preflight.Exec(); err != nil {
		return fmt.Errorf("error while executing preflight phase: %w", err)
	}

	distro := del.NewDistribution(d.furyctlConf, d.dryRun, d.kfdManifest, d.paths)

	if err := distro.Exec(); err != nil {
		return fmt.Errorf("error while deleting distribution: %w", err)
	}

	return nil
}
