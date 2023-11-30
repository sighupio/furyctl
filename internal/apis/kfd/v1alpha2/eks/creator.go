// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package eks

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	r3diff "github.com/r3labs/diff/v3"
	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/fury-distribution/pkg/apis/ekscluster/v1alpha2/private"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2"
	commcreate "github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/common/create"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/eks/create"
	eksrules "github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/eks/rules"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/eks/vpn"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/rules"
	"github.com/sighupio/furyctl/internal/state"
	"github.com/sighupio/furyctl/internal/upgrade"
	iox "github.com/sighupio/furyctl/internal/x/io"
	yamlx "github.com/sighupio/furyctl/internal/x/yaml"
)

const (
	InfrastructurePhaseSchemaPath = ".spec.infrastructure"
	KubernetesPhaseSchemaPath     = ".spec.kubernetes"
	DistributionPhaseSchemaPath   = ".spec.distribution"
	PluginsPhaseSchemaPath        = ".spec.plugins"
	AllPhaseSchemaPath            = ""
	StartFromFlagNotSet           = ""
)

var (
	ErrUnsupportedPhase = errors.New("unsupported phase")
	ErrInfraNotPresent  = errors.New("the configuration file does not contain an infrastructure section")
	ErrTimeout          = errors.New("timeout reached")
	ErrAbortedByUser    = errors.New("operation aborted by user")
)

type ClusterCreator struct {
	paths             cluster.CreatorPaths
	furyctlConf       private.EksclusterKfdV1Alpha2
	stateStore        state.Storer
	upgradeStateStore upgrade.Storer
	kfdManifest       config.KFD
	phase             string
	skipVpn           bool
	vpnAutoConnect    bool
	dryRun            bool
	force             bool
	upgrade           bool
}

type Phases struct {
	*create.PreFlight
	Infrastructure upgrade.OperatorPhaseAsync
	Kubernetes     upgrade.OperatorPhaseAsync
	Distribution   upgrade.ReducersOperatorPhaseAsync[v1alpha2.Reducers]
	*commcreate.Plugins
}

func (v *ClusterCreator) SetProperties(props []cluster.CreatorProperty) {
	for _, prop := range props {
		v.SetProperty(prop.Name, prop.Value)
	}

	v.stateStore = state.NewStore(
		v.paths.DistroPath,
		v.paths.ConfigPath,
		v.paths.WorkDir,
		v.kfdManifest.Tools.Common.Kubectl.Version,
		v.paths.BinPath,
	)

	v.upgradeStateStore = upgrade.NewStateStore(
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

	case cluster.CreatorPropertyDryRun:
		if b, ok := value.(bool); ok {
			v.dryRun = b
		}

	case cluster.CreatorPropertyForce:
		if f, ok := value.(bool); ok {
			v.force = f
		}

	case cluster.CreatorPropertyUpgrade:
		if b, ok := value.(bool); ok {
			v.upgrade = b
		}
	}
}

func (*ClusterCreator) GetPhasePath(phase string) (string, error) {
	switch phase {
	case cluster.OperationPhaseInfrastructure:
		return InfrastructurePhaseSchemaPath, nil

	case cluster.OperationPhaseKubernetes:
		return KubernetesPhaseSchemaPath, nil

	case cluster.OperationPhaseDistribution:
		return DistributionPhaseSchemaPath, nil

	case cluster.OperationPhasePlugins:
		return PluginsPhaseSchemaPath, nil

	case cluster.OperationPhaseAll:
		return AllPhaseSchemaPath, nil

	default:
		return "", fmt.Errorf("%w: %s", ErrUnsupportedPhase, phase)
	}
}

func (v *ClusterCreator) Create(startFrom string, timeout int) error {
	upgr := upgrade.New(v.paths, string(v.furyctlConf.Kind))

	infra, kube, distro, plugins, preflight, err := v.setupPhases(upgr)
	if err != nil {
		return err
	}

	var vpnConfig *private.SpecInfrastructureVpn

	if v.furyctlConf.Spec.Infrastructure != nil {
		vpnConfig = v.furyctlConf.Spec.Infrastructure.Vpn
	}

	vpnConnector, err := vpn.NewConnector(
		v.furyctlConf.Metadata.Name,
		infra.Self().TerraformSecretsPath,
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

	go v.CreateAsync(
		&Phases{
			PreFlight:      preflight,
			Infrastructure: infra,
			Kubernetes:     kube,
			Distribution:   distro,
			Plugins:        plugins,
		},
		startFrom,
		vpnConnector,
		upgr,
		errCh,
		doneCh,
	)

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

func (*ClusterCreator) initUpgradeState() *upgrade.State {
	return &upgrade.State{
		Phases: upgrade.Phases{
			PreInfrastructure:  &upgrade.Phase{Status: upgrade.PhaseStatusPending},
			Infrastructure:     &upgrade.Phase{Status: upgrade.PhaseStatusPending},
			PostInfrastructure: &upgrade.Phase{Status: upgrade.PhaseStatusPending},
			PreKubernetes:      &upgrade.Phase{Status: upgrade.PhaseStatusPending},
			Kubernetes:         &upgrade.Phase{Status: upgrade.PhaseStatusPending},
			PostKubernetes:     &upgrade.Phase{Status: upgrade.PhaseStatusPending},
			PreDistribution:    &upgrade.Phase{Status: upgrade.PhaseStatusPending},
			Distribution:       &upgrade.Phase{Status: upgrade.PhaseStatusPending},
			PostDistribution:   &upgrade.Phase{Status: upgrade.PhaseStatusPending},
		},
	}
}

func (v *ClusterCreator) CreateAsync(
	phases *Phases,
	startFrom string,
	vpnConnector *vpn.Connector,
	upgr *upgrade.Upgrade,
	errCh chan error,
	doneCh chan bool,
) {
	defer close(doneCh)

	status, err := phases.PreFlight.Exec()
	if err != nil {
		errCh <- fmt.Errorf("error while executing preflight phase: %w", err)

		return
	}

	r, err := eksrules.NewEKSClusterRulesExtractor(v.paths.DistroPath)
	if err != nil {
		if !errors.Is(err, eksrules.ErrReadingRulesFile) {
			errCh <- fmt.Errorf("error while creating rules builder: %w", err)

			return
		}
	}

	reducers := v.buildReducers(status.Diffs, r, cluster.OperationPhaseDistribution)

	preupgrade := commcreate.NewPreUpgrade(
		v.paths,
		v.kfdManifest,
		string(v.furyctlConf.Kind),
		v.dryRun,
		v.upgrade,
		v.force,
		upgr,
		reducers,
		status.Diffs,
	)

	if err := preupgrade.Exec(); err != nil {
		errCh <- fmt.Errorf("error while executing preupgrade phase: %w", err)

		return
	}

	switch v.phase {
	case cluster.OperationPhaseInfrastructure:
		if err := v.infraPhase(phases.Infrastructure, vpnConnector); err != nil {
			errCh <- err
		}

	case cluster.OperationPhaseKubernetes:
		if err := v.kubernetesPhase(phases.Kubernetes, vpnConnector); err != nil {
			errCh <- err
		}

	case cluster.OperationPhaseDistribution:
		if len(reducers) > 0 {
			confirm, err := v.AskConfirmation()
			if err != nil {
				errCh <- err
			}

			if !confirm {
				errCh <- ErrAbortedByUser
			}
		}

		if err := v.distributionPhase(phases.Distribution, vpnConnector, reducers); err != nil {
			errCh <- err
		}

	case cluster.OperationPhasePlugins:
		if err := phases.Plugins.Exec(); err != nil {
			errCh <- err
		}

	case cluster.OperationPhaseAll:
		if len(reducers) > 0 {
			confirm, err := v.AskConfirmation()
			if err != nil {
				errCh <- err
			}

			if !confirm {
				errCh <- ErrAbortedByUser
			}
		}

		errCh <- v.allPhases(
			startFrom,
			phases,
			vpnConnector,
			reducers,
			upgr,
		)

	default:
		errCh <- fmt.Errorf("%w: %s", ErrUnsupportedPhase, v.phase)
	}
}

func (v *ClusterCreator) infraPhase(infra upgrade.OperatorPhaseAsync, vpnConnector *vpn.Connector) error {
	if v.furyctlConf.Spec.Infrastructure == nil {
		absPath, err := filepath.Abs(v.paths.ConfigPath)
		if err != nil {
			logrus.Debugf("error while getting absolute path of %s: %v", v.paths.ConfigPath, err)

			return fmt.Errorf("%w: at %s", ErrInfraNotPresent, v.paths.ConfigPath)
		}

		return fmt.Errorf("%w: check at %s", ErrInfraNotPresent, absPath)
	}

	if err := infra.Exec(StartFromFlagNotSet, &upgrade.State{}); err != nil {
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

func (v *ClusterCreator) kubernetesPhase(kube upgrade.OperatorPhaseAsync, vpnConnector *vpn.Connector) error {
	if v.furyctlConf.Spec.Kubernetes.ApiServer.PrivateAccess &&
		!v.furyctlConf.Spec.Kubernetes.ApiServer.PublicAccess &&
		!v.dryRun {
		if err := vpnConnector.Connect(); err != nil {
			return fmt.Errorf("error while connecting to the vpn: %w", err)
		}
	}

	logrus.Warn("Please make sure that the Kubernetes API is reachable before continuing" +
		" (e.g. check VPN connection is active`), otherwise the installation will fail.")

	if err := kube.Exec(StartFromFlagNotSet, &upgrade.State{}); err != nil {
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

func (v *ClusterCreator) distributionPhase(
	distro upgrade.ReducersOperatorPhaseAsync[v1alpha2.Reducers],
	vpnConnector *vpn.Connector,
	reducers v1alpha2.Reducers,
) error {
	if v.furyctlConf.Spec.Kubernetes.ApiServer.PrivateAccess &&
		!v.furyctlConf.Spec.Kubernetes.ApiServer.PublicAccess &&
		!v.dryRun {
		if err := vpnConnector.Connect(); err != nil {
			return fmt.Errorf("error while connecting to the vpn: %w", err)
		}
	}

	if err := distro.Exec(reducers, StartFromFlagNotSet, &upgrade.State{}); err != nil {
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
	startFrom string,
	phases *Phases,
	vpnConnector *vpn.Connector,
	reducers v1alpha2.Reducers,
	upgr *upgrade.Upgrade,
) error {
	if v.dryRun {
		logrus.Info("furcytl will try its best to calculate what would have changed. " +
			"Sometimes this is not possible, for better results limit the scope with the --phase flag.")
	}

	upgradeState := &upgrade.State{}

	if upgr.Enabled && !v.dryRun {
		s, err := v.upgradeStateStore.Get()
		if err == nil {
			if err := yamlx.UnmarshalV3(s, &upgradeState); err != nil {
				return fmt.Errorf("error while unmarshalling upgrade state: %w", err)
			}

			if startFrom == "" {
				resumableState := v.upgradeStateStore.GetLatestResumablePhase(upgradeState)

				logrus.Infof("An upgrade is already in progress, resuming from %s phase.\n"+
					"If you wish to start from a different phase, you can use the --start-from "+
					"flag to select the desired phase to resume.", resumableState)

				startFrom = resumableState
			}
		} else {
			logrus.Debugf("error while getting upgrade state: %v", err)
			logrus.Debugf("creating a new upgrade state on the cluster...")

			upgradeState = v.initUpgradeState()

			if err := v.upgradeStateStore.Store(upgradeState); err != nil {
				return fmt.Errorf("error while storing upgrade state: %w", err)
			}
		}
	}

	logrus.Info("Creating cluster...")

	if err := v.allPhasesExec(
		startFrom,
		phases,
		vpnConnector,
		reducers,
		upgradeState,
	); err != nil {
		return err
	}

	if v.dryRun {
		logrus.Info("Kubernetes Fury cluster created successfully (dry-run mode)")

		return nil
	}

	if upgr.Enabled {
		if err := v.upgradeStateStore.Delete(); err != nil {
			return fmt.Errorf("error while deleting upgrade state: %w", err)
		}
	}

	if err := v.stateStore.StoreConfig(); err != nil {
		return fmt.Errorf("error while storing cluster config: %w", err)
	}

	if err := v.stateStore.StoreKFD(); err != nil {
		return fmt.Errorf("error while creating secret with the distribution configuration: %w", err)
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

func (v *ClusterCreator) allPhasesExec(
	startFrom string,
	phases *Phases,
	vpnConnector *vpn.Connector,
	reducers v1alpha2.Reducers,
	upgradeState *upgrade.State,
) error {
	if v.furyctlConf.Spec.Infrastructure != nil &&
		(startFrom == "" ||
			startFrom == cluster.OperationPhaseInfrastructure ||
			startFrom == cluster.OperationSubPhasePreInfrastructure ||
			startFrom == cluster.OperationSubPhasePostInfrastructure) {
		if err := phases.Infrastructure.Exec(v.getInfrastructureSubPhase(startFrom), upgradeState); err != nil {
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

	if startFrom != cluster.OperationSubPhasePreDistribution &&
		startFrom != cluster.OperationPhaseDistribution &&
		startFrom != cluster.OperationSubPhasePostDistribution &&
		startFrom != cluster.OperationPhasePlugins {
		if err := phases.Kubernetes.Exec(v.getKubernetesSubPhase(startFrom), upgradeState); err != nil {
			return fmt.Errorf("error while executing kubernetes phase: %w", err)
		}
	}

	if startFrom != cluster.OperationPhasePlugins {
		if err := phases.Distribution.Exec(reducers, v.getDistributionSubPhase(startFrom), upgradeState); err != nil {
			return fmt.Errorf("error while executing distribution phase: %w", err)
		}
	}

	if err := phases.Plugins.Exec(); err != nil {
		return fmt.Errorf("error while executing plugins phase: %w", err)
	}

	return nil
}

func (*ClusterCreator) getInfrastructureSubPhase(startFrom string) string {
	return startFrom
}

func (*ClusterCreator) getKubernetesSubPhase(startFrom string) string {
	switch startFrom {
	case cluster.OperationPhaseKubernetes,
		cluster.OperationSubPhasePreKubernetes,
		cluster.OperationSubPhasePostKubernetes:
		return startFrom

	default:
		return ""
	}
}

func (*ClusterCreator) getDistributionSubPhase(startFrom string) string {
	switch startFrom {
	case cluster.OperationPhaseDistribution,
		cluster.OperationSubPhasePreDistribution,
		cluster.OperationSubPhasePostDistribution:
		return startFrom

	default:
		return ""
	}
}

func (*ClusterCreator) buildReducers(
	statusDiffs r3diff.Changelog,
	rulesExtractor rules.Extractor,
	phase string,
) v1alpha2.Reducers {
	reducersRules := rulesExtractor.GetReducers(phase)

	filteredReducers := rulesExtractor.ReducerRulesByDiffs(reducersRules, statusDiffs)

	reducers := make(v1alpha2.Reducers, len(filteredReducers))

	if len(filteredReducers) > 0 {
		for _, reducer := range filteredReducers {
			if reducer.Reducers != nil {
				if reducer.Description != nil {
					logrus.Infof("%s", *reducer.Description)
				}

				for _, red := range *reducer.Reducers {
					reducers = append(reducers, v1alpha2.NewBaseReducer(
						red.Key,
						red.From,
						red.To,
						red.Lifecycle,
					),
					)
				}
			}
		}
	}

	return reducers
}

//nolint:revive // ignore maximum number of return results
func (v *ClusterCreator) setupPhases(upgr *upgrade.Upgrade) (
	*upgrade.OperatorPhaseAsyncDecorator,
	*upgrade.OperatorPhaseAsyncDecorator,
	*upgrade.ReducerOperatorPhaseAsyncDecorator[v1alpha2.Reducers],
	*commcreate.Plugins,
	*create.PreFlight,
	error,
) {
	infra := upgrade.NewOperatorPhaseAsyncDecorator(
		v.upgradeStateStore,
		create.NewInfrastructure(
			v.furyctlConf,
			v.kfdManifest,
			v.paths,
			v.dryRun,
			upgr,
		),
		v.dryRun,
		upgr,
	)

	kube := upgrade.NewOperatorPhaseAsyncDecorator(
		v.upgradeStateStore,
		create.NewKubernetes(
			v.furyctlConf,
			v.kfdManifest,
			infra.Self().TerraformOutputsPath,
			v.paths,
			v.dryRun,
			upgr,
		),
		v.dryRun,
		upgr,
	)

	distro := upgrade.NewReducerOperatorPhaseAsyncDecorator(
		v.upgradeStateStore,
		create.NewDistribution(
			v.paths,
			v.furyctlConf,
			v.kfdManifest,
			infra.Self().TerraformOutputsPath,
			v.dryRun,
			v.phase,
			upgr,
		),
		v.dryRun,
		upgr,
	)

	plugins := commcreate.NewPlugins(
		v.paths,
		v.kfdManifest,
		string(v.furyctlConf.Kind),
		v.dryRun,
	)

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

func (v *ClusterCreator) AskConfirmation() (bool, error) {
	if !v.force {
		if _, err := fmt.Println("WARNING: You are about to apply changes to the cluster configuration."); err != nil {
			return false, fmt.Errorf("error while printing to stdout: %w", err)
		}

		if _, err := fmt.Println("Are you sure you want to continue? Only 'yes' will be accepted to confirm."); err != nil {
			return false, fmt.Errorf("error while printing to stdout: %w", err)
		}

		prompter := iox.NewPrompter(bufio.NewReader(os.Stdin))

		prompt, err := prompter.Ask("yes")
		if err != nil {
			return false, fmt.Errorf("error reading user input: %w", err)
		}

		if !prompt {
			return false, nil
		}
	}

	return true, nil
}
