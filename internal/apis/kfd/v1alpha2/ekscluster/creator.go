// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ekscluster

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/fury-distribution/pkg/apis/ekscluster/v1alpha2/private"
	commcreate "github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/common/create"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/ekscluster/create"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/ekscluster/vpn"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/state"
	"github.com/sighupio/furyctl/internal/upgrade"
	"github.com/sighupio/furyctl/pkg/reducers"
	eksrules "github.com/sighupio/furyctl/pkg/rulesextractor"
	"github.com/sighupio/furyctl/pkg/template"
	yamlx "github.com/sighupio/furyctl/pkg/x/yaml"
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
	paths                cluster.CreatorPaths
	furyctlConf          private.EksclusterKfdV1Alpha2
	stateStore           state.Storer
	upgradeStateStore    upgrade.Storer
	kfdManifest          config.KFD
	phase                string
	skipVpn              bool
	vpnAutoConnect       bool
	dryRun               bool
	force                []string
	upgrade              bool
	externalUpgradesPath string
	postApplyPhases      []string
}

type Phases struct {
	*create.PreFlight
	Infrastructure upgrade.OperatorPhaseAsync
	Kubernetes     upgrade.OperatorPhaseAsync
	Distribution   upgrade.ReducersOperatorPhaseAsync[reducers.Reducers]
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
		if f, ok := value.([]string); ok {
			v.force = f
		}

	case cluster.CreatorPropertyUpgrade:
		if b, ok := value.(bool); ok {
			v.upgrade = b
		}

	case cluster.CreatorPropertyExternalUpgradesPath:
		if s, ok := value.(string); ok {
			v.externalUpgradesPath = s
		}

	case cluster.CreatorPropertyPostApplyPhases:
		if s, ok := value.([]string); ok {
			v.postApplyPhases = s
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

func (v *ClusterCreator) Create(startFrom string, timeout, _ int) error {
	upgr := upgrade.New(v.paths, string(v.furyctlConf.Kind))

	infra, kube, distro, plugins, preflight, err := v.setupPhases(upgr, v.upgrade)
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

			//nolint:mnd,revive // ignore magic number linters
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

	renderedConfig, err := v.RenderConfig()
	if err != nil {
		errCh <- fmt.Errorf("error while rendering config: %w", err)
	}

	status, err := phases.PreFlight.Exec(renderedConfig)
	if err != nil {
		errCh <- fmt.Errorf("error while executing preflight phase: %w", err)

		return
	}

	r, err := eksrules.NewEKSClusterRulesExtractor(v.paths.DistroPath, renderedConfig)
	if err != nil {
		if !errors.Is(err, eksrules.ErrReadingRulesFile) {
			errCh <- fmt.Errorf("error while creating rules builder: %w", err)

			return
		}
	}

	rdcs := reducers.Build(status.Diffs, r, cluster.OperationPhaseDistribution)

	unsafeReducers := r.UnsafeReducerRulesByDiffs(
		r.GetReducers(
			cluster.OperationPhaseDistribution,
		),
		status.Diffs,
	)

	if len(rdcs) > 0 {
		logrus.Infof("Differences found from previous cluster configuration, "+
			"handling the following changes:\n%s", rdcs.ToString())
	}

	if distribution.HasFeature(v.kfdManifest, distribution.FeatureClusterUpgrade) {
		preupgrade := commcreate.NewPreUpgrade(
			v.paths,
			v.kfdManifest,
			string(v.furyctlConf.Kind),
			v.dryRun,
			v.upgrade,
			v.force,
			upgr,
			rdcs,
			status.Diffs,
			v.externalUpgradesPath,
			false,
		)

		if err := preupgrade.Exec(); err != nil {
			errCh <- fmt.Errorf("error while executing preupgrade phase: %w", err)

			return
		}
	}

	switch v.phase {
	case cluster.OperationPhaseInfrastructure:
		if err := v.infraPhase(phases.Infrastructure, vpnConnector); err != nil {
			errCh <- err
		}

	case cluster.OperationPhaseKubernetes:
		if err := v.kubernetesPhase(phases.Kubernetes, vpnConnector, renderedConfig); err != nil {
			errCh <- err
		}

	case cluster.OperationPhaseDistribution:
		if len(rdcs) > 0 && len(unsafeReducers) > 0 {
			confirm, err := cluster.AskConfirmation(cluster.IsForceEnabledForFeature(v.force, cluster.ForceFeatureMigrations))
			if err != nil {
				errCh <- err
			}

			if !confirm {
				errCh <- ErrAbortedByUser
			}
		}

		if err := v.distributionPhase(phases.Distribution, vpnConnector, rdcs, renderedConfig); err != nil {
			errCh <- err
		}

	case cluster.OperationPhasePlugins:
		if !distribution.HasFeature(v.kfdManifest, distribution.FeaturePlugins) {
			errCh <- fmt.Errorf("error while executing plugins phase: %w", distribution.ErrPluginsFeatureNotSupported)
		}

		if err := phases.Plugins.Exec(); err != nil {
			errCh <- err
		}

	case cluster.OperationPhaseAll:
		if len(rdcs) > 0 && len(unsafeReducers) > 0 {
			confirm, err := cluster.AskConfirmation(cluster.IsForceEnabledForFeature(v.force, cluster.ForceFeatureMigrations))
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
			rdcs,
			upgr,
			renderedConfig,
		)

	default:
		errCh <- fmt.Errorf("%w: %s", ErrUnsupportedPhase, v.phase)
	}
}

func (v *ClusterCreator) infraPhase(infra upgrade.OperatorPhaseAsync, vpnConnector *vpn.Connector) error {
	upgradeState := upgrade.State{
		Phases: upgrade.Phases{
			PreInfrastructure:  &upgrade.Phase{Status: upgrade.PhaseStatusPending},
			Infrastructure:     &upgrade.Phase{Status: upgrade.PhaseStatusPending},
			PostInfrastructure: &upgrade.Phase{Status: upgrade.PhaseStatusPending},
		},
	}

	if v.furyctlConf.Spec.Infrastructure == nil {
		absPath, err := filepath.Abs(v.paths.ConfigPath)
		if err != nil {
			logrus.Debugf("error while getting absolute path of %s: %v", v.paths.ConfigPath, err)

			return fmt.Errorf("%w: at %s", ErrInfraNotPresent, v.paths.ConfigPath)
		}

		return fmt.Errorf("%w: check at %s", ErrInfraNotPresent, absPath)
	}

	if err := infra.Exec(StartFromFlagNotSet, &upgradeState); err != nil {
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

func (v *ClusterCreator) kubernetesPhase(
	kube upgrade.OperatorPhaseAsync,
	vpnConnector *vpn.Connector,
	renderedConfig map[string]any,
) error {
	upgradeState := upgrade.State{
		Phases: upgrade.Phases{
			PreKubernetes:  &upgrade.Phase{Status: upgrade.PhaseStatusPending},
			Kubernetes:     &upgrade.Phase{Status: upgrade.PhaseStatusPending},
			PostKubernetes: &upgrade.Phase{Status: upgrade.PhaseStatusPending},
		},
	}

	if v.furyctlConf.Spec.Kubernetes.ApiServer.PrivateAccess &&
		!v.furyctlConf.Spec.Kubernetes.ApiServer.PublicAccess &&
		!v.dryRun {
		if err := vpnConnector.Connect(); err != nil {
			return fmt.Errorf("error while connecting to the vpn: %w", err)
		}
	}

	logrus.Warn("Please make sure that the Kubernetes API is reachable before continuing" +
		" (e.g. check VPN connection is active`), otherwise the installation will fail.")

	if err := kube.Exec(StartFromFlagNotSet, &upgradeState); err != nil {
		return fmt.Errorf("error while executing kubernetes phase: %w", err)
	}

	if v.dryRun {
		logrus.Info("Kubernetes cluster created successfully (dry-run mode)")

		return nil
	}

	if err := v.stateStore.StoreConfig(renderedConfig); err != nil {
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
	distro upgrade.ReducersOperatorPhaseAsync[reducers.Reducers],
	vpnConnector *vpn.Connector,
	rdcs reducers.Reducers,
	renderedConfig map[string]any,
) error {
	upgradeState := upgrade.State{
		Phases: upgrade.Phases{
			PreDistribution:  &upgrade.Phase{Status: upgrade.PhaseStatusPending},
			Distribution:     &upgrade.Phase{Status: upgrade.PhaseStatusPending},
			PostDistribution: &upgrade.Phase{Status: upgrade.PhaseStatusPending},
		},
	}

	if v.furyctlConf.Spec.Kubernetes.ApiServer.PrivateAccess &&
		!v.furyctlConf.Spec.Kubernetes.ApiServer.PublicAccess &&
		!v.dryRun {
		if err := vpnConnector.Connect(); err != nil {
			return fmt.Errorf("error while connecting to the vpn: %w", err)
		}
	}

	if err := distro.Exec(rdcs, StartFromFlagNotSet, &upgradeState); err != nil {
		return fmt.Errorf("error while installing SIGHUP Distribution: %w", err)
	}

	if v.dryRun {
		logrus.Info("SIGHUP Distribution installed successfully (dry-run mode)")

		return nil
	}

	if err := v.stateStore.StoreConfig(renderedConfig); err != nil {
		return fmt.Errorf("error while creating secret with the cluster configuration: %w", err)
	}

	if err := v.stateStore.StoreKFD(); err != nil {
		return fmt.Errorf("error while creating secret with the distribution configuration: %w", err)
	}

	logrus.Info("SIGHUP Distribution installed successfully")

	if err := v.logVPNKill(vpnConnector); err != nil {
		return fmt.Errorf("error while logging vpn kill message: %w", err)
	}

	return nil
}

func (v *ClusterCreator) allPhases(
	startFrom string,
	phases *Phases,
	vpnConnector *vpn.Connector,
	rdcs reducers.Reducers,
	upgr *upgrade.Upgrade,
	renderedConfig map[string]any,
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
		rdcs,
		upgradeState,
	); err != nil {
		return err
	}

	if len(v.postApplyPhases) > 0 {
		logrus.Info("Executing extra phases...")

		if err := v.extraPhases(phases, upgradeState, upgr); err != nil {
			return fmt.Errorf("error while executing extra phases: %w", err)
		}
	}

	if v.dryRun {
		logrus.Info("SIGHUP Distribution cluster created successfully (dry-run mode)")

		return nil
	}

	if upgr.Enabled {
		if err := v.upgradeStateStore.Delete(); err != nil {
			return fmt.Errorf("error while deleting upgrade state: %w", err)
		}
	}

	if err := v.stateStore.StoreConfig(renderedConfig); err != nil {
		return fmt.Errorf("error while storing cluster config: %w", err)
	}

	if err := v.stateStore.StoreKFD(); err != nil {
		return fmt.Errorf("error while creating secret with the distribution configuration: %w", err)
	}

	logrus.Info("SIGHUP Distribution cluster created successfully")

	if err := v.logVPNKill(vpnConnector); err != nil {
		return fmt.Errorf("error while logging vpn kill message: %w", err)
	}

	if err := v.logKubeconfig(); err != nil {
		return fmt.Errorf("error while logging kubeconfig path: %w", err)
	}

	return nil
}

func (v *ClusterCreator) extraPhases(phases *Phases, upgradeState *upgrade.State, upgr *upgrade.Upgrade) error {
	initialUpgrade := upgr.Enabled

	defer func() {
		upgr.Enabled = initialUpgrade
	}()

	for _, phase := range v.postApplyPhases {
		switch phase {
		case cluster.OperationPhaseInfrastructure:
			phases.Infrastructure.SetUpgrade(false)

			if err := phases.Infrastructure.Exec(StartFromFlagNotSet, upgradeState); err != nil {
				return fmt.Errorf("error while executing post infrastructure phase: %w", err)
			}

		case cluster.OperationPhaseKubernetes:
			phases.Kubernetes.SetUpgrade(false)

			if err := phases.Kubernetes.Exec(StartFromFlagNotSet, upgradeState); err != nil {
				return fmt.Errorf("error while executing post kubernetes phase: %w", err)
			}

		case cluster.OperationPhaseDistribution:
			phases.Distribution.SetUpgrade(false)

			if err := phases.Distribution.Exec(
				reducers.Reducers{},
				StartFromFlagNotSet,
				upgradeState,
			); err != nil {
				return fmt.Errorf("error while executing post distribution phase: %w", err)
			}

		case cluster.OperationPhasePlugins:
			if distribution.HasFeature(v.kfdManifest, distribution.FeaturePlugins) {
				if err := phases.Plugins.Exec(); err != nil {
					return fmt.Errorf("error while executing plugins phase: %w", err)
				}
			}
		}
	}

	return nil
}

func (v *ClusterCreator) allPhasesExec(
	startFrom string,
	phases *Phases,
	vpnConnector *vpn.Connector,
	rdcs reducers.Reducers,
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
		if err := phases.Distribution.Exec(rdcs, v.getDistributionSubPhase(startFrom), upgradeState); err != nil {
			return fmt.Errorf("error while executing distribution phase: %w", err)
		}
	}

	if distribution.HasFeature(v.kfdManifest, distribution.FeaturePlugins) {
		if err := phases.Plugins.Exec(); err != nil {
			return fmt.Errorf("error while executing plugins phase: %w", err)
		}
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

func (v *ClusterCreator) RenderConfig() (map[string]any, error) {
	specMap := map[string]any{}

	phase := cluster.NewOperationPhase(
		path.Join(v.paths.WorkDir, cluster.OperationPhaseDistribution),
		v.kfdManifest.Tools,
		v.paths.BinPath,
	)

	furyctlMerger, err := phase.CreateFuryctlMerger(
		v.paths.DistroPath,
		v.paths.ConfigPath,
		"kfd-v1alpha2",
		"ekscluster",
	)
	if err != nil {
		return nil, fmt.Errorf("error while creating furyctl merger: %w", err)
	}

	tfCfg, err := template.NewConfigWithoutData(furyctlMerger, []string{})
	if err != nil {
		return nil, fmt.Errorf("error while creating template config: %w", err)
	}

	for k, v := range tfCfg.Data {
		specMap[k] = v
	}

	return specMap, nil
}

//nolint:revive // ignore maximum number of return results
func (v *ClusterCreator) setupPhases(upgr *upgrade.Upgrade, upgradeFlag bool) (
	*upgrade.OperatorPhaseAsyncDecorator,
	*upgrade.OperatorPhaseAsyncDecorator,
	*upgrade.ReducerOperatorPhaseAsyncDecorator[reducers.Reducers],
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

	distro := upgrade.NewReducerOperatorPhaseAsyncDecorator[reducers.Reducers](
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
		v.force,
		infra.Self().TerraformOutputsPath,
		v.phase,
		upgradeFlag,
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
