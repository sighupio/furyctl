// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package eks

import (
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/config"
	"github.com/sighupio/fury-distribution/pkg/schema"
	"github.com/sighupio/furyctl/internal/cluster"
)

var ErrUnsupportedPhase = fmt.Errorf("unsupported phase")

type ClusterCreator struct {
	configPath     string
	furyctlConf    schema.EksclusterKfdV1Alpha2
	kfdManifest    config.KFD
	phase          string
	vpnAutoConnect bool
}

func (v *ClusterCreator) SetProperties(props []cluster.CreatorProperty) {
	for _, prop := range props {
		v.SetProperty(prop.Name, prop.Value)
	}
}

func (v *ClusterCreator) SetProperty(name string, value any) {
	lcName := strings.ToLower(name)

	switch lcName {
	case cluster.CreatorPropertyConfigPath:
		v.configPath = value.(string)
	case cluster.CreatorPropertyFuryctlConf:
		v.furyctlConf = value.(schema.EksclusterKfdV1Alpha2)
	case cluster.CreatorPropertyKfdManifest:
		v.kfdManifest = value.(config.KFD)
	case cluster.CreatorPropertyPhase:
		v.phase = value.(string)
	case cluster.CreatorPropertyVpnAutoConnect:
		v.vpnAutoConnect = value.(bool)
	}
}

func (v *ClusterCreator) Create(dryRun bool) error {
	logrus.Infof("Running phase: %s", v.phase)

	infra, err := NewInfrastructure(v.furyctlConf, v.kfdManifest)
	if err != nil {
		return err
	}

	kube, err := NewKubernetes(v.furyctlConf, v.kfdManifest, infra.OutputsPath)
	if err != nil {
		return err
	}

	distro, err := NewDistribution(v.furyctlConf, v.kfdManifest)
	if err != nil {
		return err
	}

	infraOpts := []cluster.CreationPhaseOption{
		{Name: cluster.CreationPhaseOptionVPNAutoConnect, Value: v.vpnAutoConnect},
	}

	switch v.phase {
	case cluster.CreationPhaseInfrastructure:
		return infra.Exec(dryRun, infraOpts)
	case cluster.CreationPhaseKubernetes:
		return kube.Exec(dryRun)
	case cluster.CreationPhaseDistribution:
		return distro.Exec(dryRun)
	case cluster.CreationPhaseAll:
		if v.furyctlConf.Spec.Infrastructure != nil {
			if err := infra.Exec(dryRun, infraOpts); err != nil {
				return err
			}
		}

		if err := kube.Exec(dryRun); err != nil {
			return err
		}

		return distro.Exec(dryRun)
	default:
		return ErrUnsupportedPhase
	}
}
