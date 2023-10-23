// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package eks

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/fury-distribution/pkg/apis/ekscluster/v1alpha2/private"
	commcreate "github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/common/create"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/eks/create"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/eks/vpn"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/state"
)

var (
	ErrUnsupportedPhase = errors.New("unsupported phase")
	ErrInfraNotPresent  = errors.New("the configuration file does not contain an infrastructure section")
	ErrTimeout          = errors.New("timeout reached")
)

type ClusterCreator struct {
	paths          cluster.CreatorPaths
	furyctlConf    private.EksclusterKfdV1Alpha2
	stateStore     state.Storer
	kfdManifest    config.KFD
	phase          string
	skipVpn        bool
	vpnAutoConnect bool
	dryRun         bool
}

func (v *ClusterCreator) SetProperties(props []cluster.CreatorProperty) {
	for _, prop := range props {
		v.SetProperty(prop.Name, prop.Value)
	}

	v.stateStore = state.NewStore(
		v.paths.DistroPath,
		v.paths.ConfigPath,
		v.paths.Kubeconfig,
		v.paths.WorkDir,
		v.kfdManifest.Tools.Common.Kubectl.Version,
		v.paths.BinPath,
	)
}

func (v *ClusterCreator) SetProperty(name string, value any) {
	lcName := strings.ToLower(name)

	switch lcName {
	case cluster.CreatorPropertyFuryctlConf:
		if s, ok := value.(private.EksclusterKfdV1Alpha2); ok {
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

	case cluster.CreatorPropertySkipVpn:
		if b, ok := value.(bool); ok {
			v.skipVpn = b
		}

	case cluster.CreatorPropertyVpnAutoConnect:
		if b, ok := value.(bool); ok {
			v.vpnAutoConnect = b
		}

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

	case cluster.CreatorPropertyDryRun:
		if b, ok := value.(bool); ok {
			v.dryRun = b
		}
	}
}

func (v *ClusterCreator) Create(skipPhase string, timeout int) error {
	infra, kube, distro, plugins, preflight, err := v.setupPhases()
	if err != nil {
		return err
	}

	var vpnConfig *private.SpecInfrastructureVpn

	if v.furyctlConf.Spec.Infrastructure != nil {
		vpnConfig = v.furyctlConf.Spec.Infrastructure.Vpn
	}

	vpnConnector, err := vpn.NewConnector(
		v.furyctlConf.Metadata.Name,
		infra.TerraformSecretsPath,
		v.paths.BinPath,
		v.kfdManifest.Tools.Common.Furyagent.Version,
		v.vpnAutoConnect,
		v.skipVpn,
		vpnConfig,
	)
	if err != nil {
		return fmt.Errorf("error while creating vpn connector: %w", err)
	}

	errCh := make(chan error)
	doneCh := make(chan bool)

	go func() {
		defer close(doneCh)

		if err := preflight.Exec(); err != nil {
			errCh <- fmt.Errorf("error while executing preflight phase: %w", err)

			return
		}

		switch v.phase {
		case cluster.OperationPhaseInfrastructure:
			if err := v.infraPhase(infra, vpnConnector); err != nil {
				errCh <- err
			}

		case cluster.OperationPhaseKubernetes:
			if err := v.kubernetesPhase(kube, vpnConnector); err != nil {
				errCh <- err
			}

		case cluster.OperationPhaseDistribution:
			if err := v.distributionPhase(distro, vpnConnector); err != nil {
				errCh <- err
			}

		case cluster.OperationPhasePlugins:
			if err := plugins.Exec(); err != nil {
				errCh <- err
			}

		case cluster.OperationPhaseAll:
			errCh <- v.allPhases(skipPhase, infra, kube, distro, plugins, vpnConnector)

		default:
			errCh <- ErrUnsupportedPhase
		}
	}()

	select {
	case <-time.After(time.Duration(timeout) * time.Second):
		switch v.phase {
		case cluster.OperationPhaseInfrastructure:
			if err := infra.Stop(); err != nil {
				return fmt.Errorf("error stopping infrastructure phase: %w", err)
			}

		case cluster.OperationPhaseKubernetes:
			if err := kube.Stop(); err != nil {
				return fmt.Errorf("error stopping kubernetes phase: %w", err)
			}

		case cluster.OperationPhaseDistribution:
			if err := distro.Stop(); err != nil {
				return fmt.Errorf("error stopping distribution phase: %w", err)
			}

		case cluster.OperationPhaseAll:
			var stopWg sync.WaitGroup

			//nolint:gomnd,revive // ignore magic number linters
			stopWg.Add(3)

			go func() {
				if err := infra.Stop(); err != nil {
					logrus.Error(err)
				}

				stopWg.Done()
			}()

			go func() {
				if err := kube.Stop(); err != nil {
					logrus.Error(err)
				}

				stopWg.Done()
			}()

			go func() {
				if err := distro.Stop(); err != nil {
					logrus.Error(err)
				}

				stopWg.Done()
			}()

			stopWg.Wait()
		}

		return ErrTimeout

	case <-doneCh:

	case err := <-errCh:
		close(errCh)

		return err
	}

	return nil
}

func (v *ClusterCreator) infraPhase(infra *create.Infrastructure, vpnConnector *vpn.Connector) error {
	if v.furyctlConf.Spec.Infrastructure == nil {
		absPath, err := filepath.Abs(v.paths.ConfigPath)
		if err != nil {
			logrus.Debugf("error while getting absolute path of %s: %v", v.paths.ConfigPath, err)

			return fmt.Errorf("%w: at %s", ErrInfraNotPresent, v.paths.ConfigPath)
		}

		return fmt.Errorf("%w: check at %s", ErrInfraNotPresent, absPath)
	}

	if err := infra.Exec(); err != nil {
		return fmt.Errorf("error while executing infrastructure phase: %w", err)
	}

	if v.dryRun {
		logrus.Info("Infrastructure created successfully (dry-run mode)")

		return nil
	}

	logrus.Info("Infrastructure created successfully")

	if vpnConnector.IsConfigured() {
		if err := vpnConnector.GenerateCertificates(); err != nil {
			return fmt.Errorf("error while generating vpn certificates: %w", err)
		}
	}

	return nil
}

func (v *ClusterCreator) kubernetesPhase(kube *create.Kubernetes, vpnConnector *vpn.Connector) error {
	if v.furyctlConf.Spec.Kubernetes.ApiServer.PrivateAccess &&
		!v.furyctlConf.Spec.Kubernetes.ApiServer.PublicAccess &&
		!v.dryRun {
		if err := vpnConnector.Connect(); err != nil {
			return fmt.Errorf("error while connecting to the vpn: %w", err)
		}
	}

	logrus.Warn("Please make sure that the Kubernetes API is reachable before continuing" +
		" (e.g. check VPN connection is active`), otherwise the installation will fail.")

	if err := kube.Exec(); err != nil {
		return fmt.Errorf("error while executing kubernetes phase: %w", err)
	}

	if v.dryRun {
		logrus.Info("Kubernetes cluster created successfully (dry-run mode)")

		return nil
	}

	if err := v.stateStore.StoreConfig(); err != nil {
		return fmt.Errorf("error while creating secret with the cluster configuration: %w", err)
	}

	if err := v.stateStore.StoreKFD(); err != nil {
		return fmt.Errorf("error while creating secret with the distribution configuration: %w", err)
	}

	logrus.Info("Kubernetes cluster created successfully")

	if err := v.logKubeconfig(); err != nil {
		return fmt.Errorf("error while logging kubeconfig path: %w", err)
	}

	if err := v.logVPNKill(vpnConnector); err != nil {
		return fmt.Errorf("error while logging vpn kill message: %w", err)
	}

	return nil
}

func (v *ClusterCreator) distributionPhase(distro *create.Distribution, vpnConnector *vpn.Connector) error {
	if v.furyctlConf.Spec.Kubernetes.ApiServer.PrivateAccess &&
		!v.furyctlConf.Spec.Kubernetes.ApiServer.PublicAccess &&
		!v.dryRun {
		if err := vpnConnector.Connect(); err != nil {
			return fmt.Errorf("error while connecting to the vpn: %w", err)
		}
	}

	if err := distro.Exec(); err != nil {
		return fmt.Errorf("error while installing Kubernetes Fury Distribution: %w", err)
	}

	if v.dryRun {
		logrus.Info("Kubernetes Fury Distribution installed successfully (dry-run mode)")

		return nil
	}

	if err := v.stateStore.StoreConfig(); err != nil {
		return fmt.Errorf("error while creating secret with the cluster configuration: %w", err)
	}

	if err := v.stateStore.StoreKFD(); err != nil {
		return fmt.Errorf("error while creating secret with the distribution configuration: %w", err)
	}

	logrus.Info("Kubernetes Fury Distribution installed successfully")

	if err := v.logVPNKill(vpnConnector); err != nil {
		return fmt.Errorf("error while logging vpn kill message: %w", err)
	}

	return nil
}

func (v *ClusterCreator) allPhases(
	skipPhase string,
	infra *create.Infrastructure,
	kube *create.Kubernetes,
	distro *create.Distribution,
	plugins *commcreate.Plugins,
	vpnConnector *vpn.Connector,
) error {
	if v.dryRun {
		logrus.Info("furcytl will try its best to calculate what would have changed. " +
			"Sometimes this is not possible, for better results limit the scope with the --phase flag.")
	}

	logrus.Info("Creating cluster...")

	if v.furyctlConf.Spec.Infrastructure != nil &&
		(skipPhase == "" || skipPhase == cluster.OperationPhaseDistribution) {
		if err := infra.Exec(); err != nil {
			return fmt.Errorf("error while executing infrastructure phase: %w", err)
		}

		if !v.dryRun && vpnConnector.IsConfigured() {
			if err := vpnConnector.GenerateCertificates(); err != nil {
				return fmt.Errorf("error while generating vpn certificates: %w", err)
			}
		}
	}

	if v.furyctlConf.Spec.Kubernetes.ApiServer.PrivateAccess &&
		!v.furyctlConf.Spec.Kubernetes.ApiServer.PublicAccess &&
		!v.dryRun {
		if err := vpnConnector.Connect(); err != nil {
			return fmt.Errorf("error while connecting to the vpn: %w", err)
		}
	}

	if skipPhase != cluster.OperationPhaseKubernetes {
		if err := kube.Exec(); err != nil {
			return fmt.Errorf("error while executing kubernetes phase: %w", err)
		}

		if !v.dryRun {
			if err := v.stateStore.StoreConfig(); err != nil {
				return fmt.Errorf("error while storing cluster config: %w", err)
			}

			if err := v.stateStore.StoreKFD(); err != nil {
				return fmt.Errorf("error while creating secret with the distribution configuration: %w", err)
			}
		}
	}

	if skipPhase != cluster.OperationPhaseDistribution {
		if err := distro.Exec(); err != nil {
			return fmt.Errorf("error while executing distribution phase: %w", err)
		}

		if !v.dryRun {
			if err := v.stateStore.StoreConfig(); err != nil {
				return fmt.Errorf("error while storing cluster config: %w", err)
			}

			if err := v.stateStore.StoreKFD(); err != nil {
				return fmt.Errorf("error while creating secret with the distribution configuration: %w", err)
			}
		}
	}

	if skipPhase != cluster.OperationPhasePlugins {
		if err := plugins.Exec(); err != nil {
			return fmt.Errorf("error while executing plugins phase: %w", err)
		}
	}

	if v.dryRun {
		logrus.Info("Kubernetes Fury cluster created successfully (dry-run mode)")

		return nil
	}

	logrus.Info("Kubernetes Fury cluster created successfully")

	if err := v.logVPNKill(vpnConnector); err != nil {
		return fmt.Errorf("error while logging vpn kill message: %w", err)
	}

	if err := v.logKubeconfig(); err != nil {
		return fmt.Errorf("error while logging kubeconfig path: %w", err)
	}

	return nil
}

//nolint:revive // ignore maximum number of return results
func (v *ClusterCreator) setupPhases() (
	*create.Infrastructure,
	*create.Kubernetes,
	*create.Distribution,
	*commcreate.Plugins,
	*create.PreFlight,
	error,
) {
	infra, err := create.NewInfrastructure(v.furyctlConf, v.kfdManifest, v.paths, v.dryRun)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("error while initiating infrastructure phase: %w", err)
	}

	kube, err := create.NewKubernetes(
		v.furyctlConf,
		v.kfdManifest,
		infra.TerraformOutputsPath,
		v.paths,
		v.dryRun,
	)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("error while initiating kubernetes phase: %w", err)
	}

	distro, err := create.NewDistribution(
		v.paths,
		v.furyctlConf,
		v.kfdManifest,
		infra.TerraformOutputsPath,
		v.dryRun,
		v.phase,
		v.paths.Kubeconfig,
	)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("error while initiating distribution phase: %w", err)
	}

	plugins, err := commcreate.NewPlugins(
		v.paths,
		v.kfdManifest,
		string(v.furyctlConf.Kind),
		v.dryRun,
		v.paths.Kubeconfig,
	)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("error while initiating plugins phase: %w", err)
	}

	preflight, err := create.NewPreFlight(
		v.furyctlConf,
		v.kfdManifest,
		v.paths,
		v.dryRun,
		v.vpnAutoConnect,
		v.skipVpn,
	)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("error while initiating preflight phase: %w", err)
	}

	return infra, kube, distro, plugins, preflight, nil
}

func (*ClusterCreator) logKubeconfig() error {
	currentDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("error getting current dir: %w", err)
	}

	kubeconfigPath := filepath.Join(currentDir, "kubeconfig")

	logrus.Infof("To connect to the cluster, set the path to your kubeconfig with 'export KUBECONFIG=%s'"+
		" or use the '--kubeconfig %s' flag in following executions", kubeconfigPath, kubeconfigPath)

	return nil
}

func (*ClusterCreator) logVPNKill(vpnConnector *vpn.Connector) error {
	if vpnConnector.IsConfigured() {
		killVpnMsg, err := vpnConnector.GetKillMessage()
		if err != nil {
			return fmt.Errorf("error while getting vpn kill message: %w", err)
		}

		logrus.Info(killVpnMsg)
	}

	return nil
}
