// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ekscluster

import (
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/fury-distribution/pkg/apis/ekscluster/v1alpha2/private"
	del "github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/ekscluster/delete"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/ekscluster/vpn"
	"github.com/sighupio/furyctl/internal/cluster"
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
	lcName := strings.ToLower(name)

	switch lcName {
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

	case cluster.DeleterPropertyKfdManifest:
		if kfdManifest, ok := value.(config.KFD); ok {
			d.kfdManifest = kfdManifest
		}

	case cluster.DeleterPropertyFuryctlConf:
		if s, ok := value.(private.EksclusterKfdV1Alpha2); ok {
			d.furyctlConf = s
		}

	case cluster.DeleterPropertyPhase:
		if s, ok := value.(string); ok {
			d.phase = s
		}

	case cluster.DeleterPropertySkipVpn:
		if b, ok := value.(bool); ok {
			d.skipVpn = b
		}

	case cluster.DeleterPropertyVpnAutoConnect:
		if b, ok := value.(bool); ok {
			d.vpnAutoConnect = b
		}

	case cluster.DeleterPropertyDryRun:
		if b, ok := value.(bool); ok {
			d.dryRun = b
		}
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
		d.kfdManifest.Tools.Common.Furyagent.Version,
		d.vpnAutoConnect,
		d.skipVpn,
		vpnConfig,
	)
	if err != nil {
		return fmt.Errorf("error while creating vpn connector: %w", err)
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
