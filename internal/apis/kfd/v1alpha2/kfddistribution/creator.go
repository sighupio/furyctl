// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package kfddistribution

import (
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/fury-distribution/pkg/apis/kfddistribution/v1alpha2/public"
	commcreate "github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/common/create"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/kfddistribution/create"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/state"
	"github.com/sighupio/furyctl/internal/upgrade"
	"github.com/sighupio/furyctl/pkg/reducers"
	distrorules "github.com/sighupio/furyctl/pkg/rulesextractor"
	"github.com/sighupio/furyctl/pkg/template"
	yamlx "github.com/sighupio/furyctl/pkg/x/yaml"
)

const (
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
	furyctlConf          public.KfddistributionKfdV1Alpha2
	stateStore           state.Storer
	upgradeStateStore    upgrade.Storer
	kfdManifest          config.KFD
	phase                string
	dryRun               bool
	force                []string
	upgrade              bool
	externalUpgradesPath string
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
		if s, ok := value.(string); ok {
			c.paths.ConfigPath = s
		}

	case cluster.CreatorPropertyDistroPath:
		if s, ok := value.(string); ok {
			c.paths.DistroPath = s
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
		if s, ok := value.(public.KfddistributionKfdV1Alpha2); ok {
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

	case cluster.CreatorPropertyForce:
		if b, ok := value.([]string); ok {
			c.force = b
		}

	case cluster.CreatorPropertyUpgrade:
		if b, ok := value.(bool); ok {
			c.upgrade = b
		}

	case cluster.CreatorPropertyExternalUpgradesPath:
		if s, ok := value.(string); ok {
			c.externalUpgradesPath = s
		}

	case cluster.CreatorPropertyPostApplyPhases:
		if s, ok := value.([]string); ok {
			c.postApplyPhases = s
		}
	}
}

func (*ClusterCreator) GetPhasePath(phase string) (string, error) {
	switch phase {
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

func (c *ClusterCreator) Create(startFrom string, _, _ int) error {
	upgr := upgrade.New(c.paths, string(c.furyctlConf.Kind))
	distributionPhase := upgrade.NewReducerOperatorPhaseDecorator[reducers.Reducers](
		c.upgradeStateStore,
		create.NewDistribution(
			c.paths,
			c.furyctlConf,
			c.kfdManifest,
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

	r, err := distrorules.NewDistroClusterRulesExtractor(c.paths.DistroPath, renderedConfig)
	if err != nil {
		if !errors.Is(err, distrorules.ErrReadingRulesFile) {
			return fmt.Errorf("error while creating rules builder: %w", err)
		}
	}

	rdcs := reducers.Build(
		status.Diffs,
		r,
		cluster.OperationPhaseDistribution,
	)

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
			false,
		)

		if err := preupgradePhase.Exec(); err != nil {
			return fmt.Errorf("error while executing preupgrade phase: %w", err)
		}
	}

	switch c.phase {
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
			rdcs,
			unsafeReducers,
			distributionPhase,
			pluginsPhase,
			upgr,
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
		return fmt.Errorf("error while storing cluster config: %w", err)
	}

	if err := c.stateStore.StoreKFD(); err != nil {
		return fmt.Errorf("error while storing distribution config: %w", err)
	}

	return nil
}

func (c *ClusterCreator) allPhases(
	startFrom string,
	rdcs reducers.Reducers,
	unsafeReducers []distrorules.Rule,
	distributionPhase upgrade.ReducersOperatorPhase[reducers.Reducers],
	pluginsPhase *commcreate.Plugins,
	upgr *upgrade.Upgrade,
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

		if err := c.extraPhases(distributionPhase, pluginsPhase, upgradeState, upgr); err != nil {
			return fmt.Errorf("error while executing extra phases: %w", err)
		}
	}

	return nil
}

func (c *ClusterCreator) extraPhases(
	distributionPhase upgrade.ReducersOperatorPhase[reducers.Reducers],
	pluginsPhase *commcreate.Plugins,
	upgradeState *upgrade.State,
	upgr *upgrade.Upgrade,
) error {
	initialUpgrade := upgr.Enabled

	defer func() {
		upgr.Enabled = initialUpgrade
	}()

	for _, phase := range c.postApplyPhases {
		switch phase {
		case cluster.OperationPhaseDistribution:
			distributionPhase.SetUpgrade(false)

			if err := distributionPhase.Exec(reducers.Reducers{}, StartFromFlagNotSet, upgradeState); err != nil {
				return fmt.Errorf("error while executing distribution phase: %w", err)
			}

		case cluster.OperationPhasePlugins:
			if distribution.HasFeature(c.kfdManifest, distribution.FeaturePlugins) {
				if err := pluginsPhase.Exec(); err != nil {
					return fmt.Errorf("error while executing plugins phase: %w", err)
				}
			}
		}
	}

	return nil
}

func (*ClusterCreator) initUpgradeState() *upgrade.State {
	return &upgrade.State{
		Phases: upgrade.Phases{
			PreDistribution:  &upgrade.Phase{Status: upgrade.PhaseStatusPending},
			Distribution:     &upgrade.Phase{Status: upgrade.PhaseStatusPending},
			PostDistribution: &upgrade.Phase{Status: upgrade.PhaseStatusPending},
		},
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
		"kfddistribution",
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
