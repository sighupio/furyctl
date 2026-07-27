// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package onpremises

import (
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/furyctl/internal/apis/config"
	commcreate "github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/common/create"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/onpremises/create"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/onpremises/public"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/state"
	"github.com/sighupio/furyctl/internal/upgrade"
	"github.com/sighupio/furyctl/pkg/diffs"
	"github.com/sighupio/furyctl/pkg/reducers"
	premrules "github.com/sighupio/furyctl/pkg/rulesextractor"
	templatex "github.com/sighupio/furyctl/pkg/template"
	yamlx "github.com/sighupio/furyctl/pkg/x/yaml"
)

const (
	KubernetesPhaseSchemaPath   = ".spec.kubernetes"
	DistributionPhaseSchemaPath = ".spec.distribution"
	PluginsPhaseSchemaPath      = ".spec.plugins"
	AllPhaseSchemaPath          = ""
	StartFromFlagNotSet         = ""
)

var (
	ErrUnsupportedPhase = errors.New("unsupported phase")
	ErrAbortedByUser    = errors.New("operation aborted by user")
)

type ClusterCreator struct {
	paths                cluster.CreatorPaths
	furyctlConf          public.OnpremisesKfdV1Alpha2
	stateStore           state.Storer
	upgradeStateStore    upgrade.Storer
	skipNodesUpgrade     bool
	kfdManifest          config.KFD
	phase                string
	dryRun               bool
	force                []string
	upgrade              bool
	externalUpgradesPath string
	upgradeNode          string
	postApplyPhases      []string
}

func (c *ClusterCreator) SetProperties(props []cluster.CreatorProperty) {
	for _, prop := range props {
		c.SetProperty(prop.Name, prop.Value)
	}

	c.stateStore = state.NewStore(
		c.paths.DistroPath,
		c.paths.ConfigPath,
		c.paths.WorkDir,
		c.kfdManifest.Tools.Common.Kubectl.Version,
		c.paths.BinPath,
	)

	c.upgradeStateStore = upgrade.NewStateStore(
		c.paths.WorkDir,
		c.kfdManifest.Tools.Common.Kubectl.Version,
		c.paths.BinPath,
	)
}

func (c *ClusterCreator) SetProperty(name string, value any) {
	switch strings.ToLower(name) {
	case cluster.CreatorPropertyConfigPath:
		cluster.SetPropertyValue(value, &c.paths.ConfigPath)
	case cluster.CreatorPropertyDistroPath:
		cluster.SetPropertyValue(value, &c.paths.DistroPath)
	case cluster.CreatorPropertyWorkDir:
		cluster.SetPropertyValue(value, &c.paths.WorkDir)
	case cluster.CreatorPropertyBinPath:
		cluster.SetPropertyValue(value, &c.paths.BinPath)
	case cluster.CreatorPropertyFuryctlConf:
		cluster.SetPropertyValue(value, &c.furyctlConf)
	case cluster.CreatorPropertyKfdManifest:
		cluster.SetPropertyValue(value, &c.kfdManifest)
	case cluster.CreatorPropertyPhase:
		cluster.SetPropertyValue(value, &c.phase)
	case cluster.CreatorPropertySkipNodesUpgrade:
		cluster.SetPropertyValue(value, &c.skipNodesUpgrade)
	case cluster.CreatorPropertyDryRun:
		cluster.SetPropertyValue(value, &c.dryRun)
	case cluster.CreatorPropertyForce:
		cluster.SetPropertyValue(value, &c.force)
	case cluster.CreatorPropertyUpgrade:
		cluster.SetPropertyValue(value, &c.upgrade)
	case cluster.CreatorPropertyExternalUpgradesPath:
		cluster.SetPropertyValue(value, &c.externalUpgradesPath)
	case cluster.CreatorPropertyUpgradeNode:
		cluster.SetPropertyValue(value, &c.upgradeNode)
	case cluster.CreatorPropertyPostApplyPhases:
		cluster.SetPropertyValue(value, &c.postApplyPhases)
	default:
		logrus.Debugf("ignoring unknown property %q", name)
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
		return "", fmt.Errorf("%w: %s", ErrUnsupportedPhase, phase)
	}
}

func (c *ClusterCreator) Create(startFrom string, _, podRunningCheckTimeout int) error {
	upgr := upgrade.New(c.paths, string(c.furyctlConf.Kind))

	kubernetesPhase := upgrade.NewReducerOperatorPhaseDecorator[reducers.Reducers](
		c.upgradeStateStore,
		create.NewKubernetes(
			c.furyctlConf,
			c.kfdManifest,
			c.paths,
			c.dryRun,
			upgr,
			c.upgradeNode,
			c.force,
			podRunningCheckTimeout,
		),
		c.dryRun,
		upgr,
	)

	distributionPhase := upgrade.NewReducerOperatorPhaseDecorator[reducers.Reducers](
		c.upgradeStateStore,
		create.NewDistribution(
			c.furyctlConf,
			c.kfdManifest,
			c.paths,
			c.dryRun,
			upgr,
		),
		c.dryRun,
		upgr,
	)

	pluginsPhase := commcreate.NewPlugins(
		c.paths,
		c.kfdManifest,
		string(c.furyctlConf.Kind),
		c.dryRun,
	)

	preflight := create.NewPreFlight(
		c.furyctlConf,
		c.kfdManifest,
		c.paths,
		c.dryRun,
		c.stateStore,
		c.force,
		c.phase,
		c.upgrade,
	)

	renderedConfig, err := c.RenderConfig()
	if err != nil {
		return fmt.Errorf("error while rendering config: %w", err)
	}

	status, err := preflight.Exec(renderedConfig)
	if err != nil {
		return fmt.Errorf("error while executing preflight phase: %w", err)
	}

	r, err := premrules.NewOnPremClusterRulesExtractor(c.paths.DistroPath, renderedConfig)
	if err != nil {
		if !errors.Is(err, premrules.ErrReadingRulesFile) {
			return fmt.Errorf("error while creating rules builder: %w", err)
		}
	}

	rdcs := reducers.Build(
		status.Diffs,
		r,
		cluster.OperationPhaseDistribution,
	)

	// The kube-proxy.type field can be added to a config that never had it (the
	// parent object is born), so the diff lands on the parent path. Expand it to
	// per-leaf changes so reducers targeting the leaf (e.g. kubeProxy.type) match
	// nil -> value transitions too.
	kubeRdcs := reducers.Build(
		diffs.ExpandMapChanges(status.Diffs),
		r,
		cluster.OperationPhaseKubernetes,
	)

	unsafeReducers := r.UnsafeReducerRulesByDiffs(
		r.GetReducers(
			cluster.OperationPhaseDistribution,
		),
		status.Diffs,
	)

	unsafeKubeReducers := r.UnsafeReducerRulesByDiffs(
		r.GetReducers(
			cluster.OperationPhaseKubernetes,
		),
		diffs.ExpandMapChanges(status.Diffs),
	)

	if len(rdcs) > 0 {
		logrus.Infof("Differences found from previous cluster configuration, "+
			"handling the following changes:\n%s", rdcs.ToString())
	}

	if distribution.HasFeature(c.kfdManifest, distribution.FeatureClusterUpgrade) {
		preupgradePhase := commcreate.NewPreUpgrade(
			c.paths,
			c.kfdManifest,
			string(c.furyctlConf.Kind),
			c.dryRun,
			c.upgrade,
			c.force,
			upgr,
			rdcs,
			status.Diffs,
			c.externalUpgradesPath,
			c.skipNodesUpgrade,
		)

		if err := preupgradePhase.Exec(); err != nil {
			return fmt.Errorf("error while executing preupgrade phase: %w", err)
		}
	}

	switch c.phase {
	case cluster.OperationPhaseKubernetes:
		if len(kubeRdcs) > 0 && len(unsafeKubeReducers) > 0 {
			confirm, err := cluster.AskConfirmation(cluster.IsForceEnabledForFeature(c.force, cluster.ForceFeatureMigrations))
			if err != nil {
				return fmt.Errorf("error while asking for confirmation: %w", err)
			}

			if !confirm {
				return ErrAbortedByUser
			}
		}

		upgradeState := upgrade.State{
			Phases: upgrade.Phases{
				PreKubernetes:  &upgrade.Phase{Status: upgrade.PhaseStatusPending},
				Kubernetes:     &upgrade.Phase{Status: upgrade.PhaseStatusPending},
				PostKubernetes: &upgrade.Phase{Status: upgrade.PhaseStatusPending},
			},
		}

		if err := kubernetesPhase.Exec(kubeRdcs, StartFromFlagNotSet, &upgradeState); err != nil {
			return fmt.Errorf("error while executing kubernetes phase: %w", err)
		}

	case cluster.OperationPhaseDistribution:
		if len(rdcs) > 0 && len(unsafeReducers) > 0 {
			confirm, err := cluster.AskConfirmation(cluster.IsForceEnabledForFeature(c.force, cluster.ForceFeatureMigrations))
			if err != nil {
				return fmt.Errorf("error while asking for confirmation: %w", err)
			}

			if !confirm {
				return ErrAbortedByUser
			}
		}

		upgradeState := upgrade.State{
			Phases: upgrade.Phases{
				PreDistribution:  &upgrade.Phase{Status: upgrade.PhaseStatusPending},
				Distribution:     &upgrade.Phase{Status: upgrade.PhaseStatusPending},
				PostDistribution: &upgrade.Phase{Status: upgrade.PhaseStatusPending},
			},
		}

		if err := distributionPhase.Exec(rdcs, StartFromFlagNotSet, &upgradeState); err != nil {
			return fmt.Errorf("error while executing distribution phase: %w", err)
		}

	case cluster.OperationPhasePlugins:
		if !distribution.HasFeature(c.kfdManifest, distribution.FeaturePlugins) {
			return fmt.Errorf("error while executing plugins phase: %w", distribution.ErrPluginsFeatureNotSupported)
		}

		if err := pluginsPhase.Exec(); err != nil {
			return fmt.Errorf("error while executing plugins phase: %w", err)
		}

	case cluster.OperationPhaseAll:
		if err := c.allPhases(
			startFrom,
			kubernetesPhase,
			distributionPhase,
			pluginsPhase,
			upgr,
			kubeRdcs,
			rdcs,
			unsafeKubeReducers,
			unsafeReducers,
		); err != nil {
			return fmt.Errorf("error while executing cluster creation: %w", err)
		}

	default:
		return fmt.Errorf("%w: %s", ErrUnsupportedPhase, c.phase)
	}

	if c.dryRun {
		return nil
	}

	if upgr.Enabled {
		if err := c.upgradeStateStore.Delete(); err != nil {
			return fmt.Errorf("error while deleting upgrade state: %w", err)
		}
	}

	if err := c.stateStore.StoreConfig(renderedConfig); err != nil {
		return fmt.Errorf("error while creating secret with the cluster configuration: %w", err)
	}

	if err := c.stateStore.StoreKFD(); err != nil {
		return fmt.Errorf("error while creating secret with the distribution configuration: %w", err)
	}

	return nil
}

func (c *ClusterCreator) RenderConfig() (map[string]any, error) {
	specMap := map[string]any{}

	phase := cluster.NewOperationPhase(
		path.Join(c.paths.WorkDir, cluster.OperationPhaseDistribution),
		c.kfdManifest.Tools,
		c.paths.BinPath,
	)

	furyctlMerger, err := phase.CreateFuryctlMerger(
		c.paths.DistroPath,
		c.paths.ConfigPath,
		"kfd-v1alpha2",
		"onpremises",
	)
	if err != nil {
		return nil, fmt.Errorf("error while creating furyctl merger: %w", err)
	}

	tfCfg, err := templatex.NewConfigWithoutData(furyctlMerger, []string{})
	if err != nil {
		return nil, fmt.Errorf("error while creating template config: %w", err)
	}

	for k, v := range tfCfg.Data {
		specMap[k] = v
	}

	return specMap, nil
}

func (c *ClusterCreator) allPhases(
	startFrom string,
	kubernetesPhase upgrade.ReducersOperatorPhase[reducers.Reducers],
	distributionPhase upgrade.ReducersOperatorPhase[reducers.Reducers],
	pluginsPhase *commcreate.Plugins,
	upgr *upgrade.Upgrade,
	kubeRdcs reducers.Reducers,
	rdcs reducers.Reducers,
	unsafeKubeReducers []premrules.Rule,
	unsafeReducers []premrules.Rule,
) error {
	upgradeState := &upgrade.State{}

	if upgr.Enabled && !c.dryRun {
		s, err := c.upgradeStateStore.Get()
		if err == nil {
			if err := yamlx.UnmarshalV3(s, &upgradeState); err != nil {
				return fmt.Errorf("error while unmarshalling upgrade state: %w", err)
			}

			if startFrom == "" {
				resumableState := c.upgradeStateStore.GetLatestResumablePhase(upgradeState)

				logrus.Infof("An upgrade is already in progress, resuming from %s phase.\n"+
					"If you wish to start from a different phase, you can use the --start-from "+
					"flag to select the desired phase to resume.", resumableState)

				startFrom = resumableState
			}
		} else {
			logrus.Debugf("error while getting upgrade state: %v", err)
			logrus.Debugf("creating a new upgrade state on the cluster...")

			upgradeState = c.initUpgradeState()

			if err := c.upgradeStateStore.Store(upgradeState); err != nil {
				return fmt.Errorf("error while storing upgrade state: %w", err)
			}
		}
	}

	if startFrom != cluster.OperationSubPhasePreDistribution &&
		startFrom != cluster.OperationPhaseDistribution &&
		startFrom != cluster.OperationSubPhasePostDistribution &&
		startFrom != cluster.OperationPhasePlugins {
		if len(kubeRdcs) > 0 && len(unsafeKubeReducers) > 0 {
			confirm, err := cluster.AskConfirmation(cluster.IsForceEnabledForFeature(c.force, cluster.ForceFeatureMigrations))
			if err != nil {
				return fmt.Errorf("error while asking for confirmation: %w", err)
			}

			if !confirm {
				return ErrAbortedByUser
			}
		}

		if err := kubernetesPhase.Exec(kubeRdcs, c.getKubernetesSubPhase(startFrom), upgradeState); err != nil {
			return fmt.Errorf("error while executing kubernetes phase: %w", err)
		}

		if c.upgradeNode != "" {
			return nil
		}
	}

	if startFrom != cluster.OperationPhasePlugins {
		if len(rdcs) > 0 && len(unsafeReducers) > 0 {
			confirm, err := cluster.AskConfirmation(cluster.IsForceEnabledForFeature(c.force, cluster.ForceFeatureMigrations))
			if err != nil {
				return fmt.Errorf("error while asking for confirmation: %w", err)
			}

			if !confirm {
				return ErrAbortedByUser
			}
		}

		if err := distributionPhase.Exec(rdcs, c.getDistributionSubPhase(startFrom), upgradeState); err != nil {
			return fmt.Errorf("error while executing distribution phase: %w", err)
		}
	}

	if distribution.HasFeature(c.kfdManifest, distribution.FeaturePlugins) {
		if err := pluginsPhase.Exec(); err != nil {
			return fmt.Errorf("error while executing plugins phase: %w", err)
		}
	}

	if len(c.postApplyPhases) > 0 {
		logrus.Info("Executing extra phases...")

		if err := c.extraPhases(
			kubernetesPhase,
			distributionPhase,
			pluginsPhase,
			upgr,
			upgradeState,
		); err != nil {
			return fmt.Errorf("error while executing extra phases: %w", err)
		}
	}

	return nil
}

func (c *ClusterCreator) extraPhases(
	kubernetesPhase upgrade.ReducersOperatorPhase[reducers.Reducers],
	distributionPhase upgrade.ReducersOperatorPhase[reducers.Reducers],
	pluginsPhase *commcreate.Plugins,
	upgr *upgrade.Upgrade,
	upgradeState *upgrade.State,
) error {
	initialUpgrade := upgr.Enabled

	defer func() {
		upgr.Enabled = initialUpgrade
	}()

	for _, phase := range c.postApplyPhases {
		switch phase {
		case cluster.OperationPhaseKubernetes:
			kubernetesPhase.SetUpgrade(false)

			if err := kubernetesPhase.Exec(nil, StartFromFlagNotSet, upgradeState); err != nil {
				return fmt.Errorf("error while executing kubernetes phase: %w", err)
			}

		case cluster.OperationPhaseDistribution:
			distributionPhase.SetUpgrade(false)

			if err := distributionPhase.Exec(nil, StartFromFlagNotSet, upgradeState); err != nil {
				return fmt.Errorf("error while executing distribution phase: %w", err)
			}

		case cluster.OperationPhasePlugins:
			if distribution.HasFeature(c.kfdManifest, distribution.FeaturePlugins) {
				if err := pluginsPhase.Exec(); err != nil {
					return fmt.Errorf("error while executing plugins phase: %w", err)
				}
			}

		default:
			logrus.Debugf("ignoring unknown post-apply phase %q", phase)
		}
	}

	return nil
}

func (*ClusterCreator) initUpgradeState() *upgrade.State {
	return &upgrade.State{
		Phases: upgrade.Phases{
			PreKubernetes:    &upgrade.Phase{Status: upgrade.PhaseStatusPending},
			Kubernetes:       &upgrade.Phase{Status: upgrade.PhaseStatusPending},
			PostKubernetes:   &upgrade.Phase{Status: upgrade.PhaseStatusPending},
			PreDistribution:  &upgrade.Phase{Status: upgrade.PhaseStatusPending},
			Distribution:     &upgrade.Phase{Status: upgrade.PhaseStatusPending},
			PostDistribution: &upgrade.Phase{Status: upgrade.PhaseStatusPending},
		},
	}
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
