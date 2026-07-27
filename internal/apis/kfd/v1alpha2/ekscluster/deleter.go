// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ekscluster

import (
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/furyctl/internal/apis/config"
	del "github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/ekscluster/delete"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/ekscluster/private"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/ekscluster/vpn"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/distribution"
)

type ClusterDeleter struct {
	paths          cluster.DeleterPaths
	kfdManifest    config.KFD
	furyctlConf    private.EksclusterKfdV1Alpha2
	phase          string
	skipVpn        bool
	vpnAutoConnect bool
	dryRun         bool
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
	case cluster.DeleterPropertySkipVpn:
		cluster.SetPropertyValue(value, &d.skipVpn)
	case cluster.DeleterPropertyVpnAutoConnect:
		cluster.SetPropertyValue(value, &d.vpnAutoConnect)
	case cluster.DeleterPropertyDryRun:
		cluster.SetPropertyValue(value, &d.dryRun)
	default:
		logrus.Debugf("ignoring unknown property %q", name)
	}
}

func (d *ClusterDeleter) Delete() error {
	infra := del.NewInfrastructure(
		d.furyctlConf,
		d.dryRun,
		d.kfdManifest,
		d.paths,
	)

	distro := del.NewDistribution(d.dryRun,
		d.kfdManifest,
		infra.Self().TerraformOutputsPath,
		d.paths,
		d.furyctlConf,
	)

	kube := del.NewKubernetes(d.furyctlConf,
		d.dryRun,
		d.kfdManifest,
		infra.Self().TerraformOutputsPath,
		d.paths,
	)

	var vpnConfig *private.SpecInfrastructureVpn

	if d.furyctlConf.Spec.Infrastructure != nil {
		vpnConfig = d.furyctlConf.Spec.Infrastructure.Vpn
	}

	vpnConnector, err := vpn.NewConnector(
		d.furyctlConf.Metadata.Name,
		infra.TerraformSecretsPath,
		d.paths.BinPath,
		distribution.EffectiveFuryagentVersion(d.kfdManifest.Tools),
		d.vpnAutoConnect,
		d.skipVpn,
		vpnConfig,
	)
	if err != nil {
		return fmt.Errorf("error while creating vpn connector: %w", err)
	}

	if err := vpnConnector.ValidateConfig(); err != nil {
		return fmt.Errorf("error while validating VPN configuration: %w", err)
	}

	preflight, err := del.NewPreFlight(
		d.furyctlConf,
		d.kfdManifest,
		d.paths,
		d.vpnAutoConnect,
		d.skipVpn,
		infra.Self().TerraformOutputsPath,
	)
	if err != nil {
		return fmt.Errorf("error while creating preflight phase: %w", err)
	}

	if err := preflight.Exec(); err != nil {
		return fmt.Errorf("error while executing preflight phase: %w", err)
	}

	switch d.phase {
	case cluster.OperationPhaseInfrastructure:
		if err := infra.Exec(); err != nil {
			return fmt.Errorf("error while deleting infrastructure phase: %w", err)
		}

		return nil

	case cluster.OperationPhaseKubernetes:
		if d.furyctlConf.Spec.Kubernetes.ApiServer.PrivateAccess && !d.dryRun {
			if err = vpnConnector.Connect(); err != nil {
				return fmt.Errorf("error while connecting to the vpn: %w", err)
			}
		}

		logrus.Warn("Please make sure that the Kubernetes API is reachable before continuing" +
			" (e.g. check VPN connection is active`), otherwise the deletion will fail.")

		if err := kube.Exec(); err != nil {
			return fmt.Errorf("error while deleting kubernetes phase: %w", err)
		}

		return nil

	case cluster.OperationPhaseDistribution:
		if d.furyctlConf.Spec.Kubernetes.ApiServer.PrivateAccess && !d.dryRun {
			if err = vpnConnector.Connect(); err != nil {
				return fmt.Errorf("error while connecting to the vpn: %w", err)
			}
		}

		if err := distro.Exec(); err != nil {
			return fmt.Errorf("error while deleting distribution phase: %w", err)
		}

		return nil

	case cluster.OperationPhaseAll:
		if d.dryRun {
			logrus.Info("furcytl will try its best to calculate what would have changed. " +
				"Sometimes this is not possible, for better results limit the scope with the --phase flag.")
		}

		if d.furyctlConf.Spec.Kubernetes.ApiServer.PrivateAccess &&
			!d.furyctlConf.Spec.Kubernetes.ApiServer.PublicAccess &&
			!d.dryRun {
			if err := vpnConnector.Connect(); err != nil {
				return fmt.Errorf("error while connecting to the vpn: %w", err)
			}
		}

		if err := distro.Exec(); err != nil {
			return fmt.Errorf("error while deleting distribution phase: %w", err)
		}

		if err := kube.Exec(); err != nil {
			return fmt.Errorf("error while deleting kubernetes phase: %w", err)
		}

		if d.furyctlConf.Spec.Infrastructure != nil {
			if err := infra.Exec(); err != nil {
				return fmt.Errorf("error while deleting infrastructure phase: %w", err)
			}
		}

		return nil

	default:
		return ErrUnsupportedPhase
	}
}
