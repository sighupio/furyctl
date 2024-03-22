// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package onpremises

import (
	"errors"
	"fmt"
	"path"
	"strings"

	r3diff "github.com/r3labs/diff/v3"
	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/fury-distribution/pkg/apis/onpremises/v1alpha2/public"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2"
	commcreate "github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/common/create"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/onpremises/create"
	premrules "github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/onpremises/rules"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/rules"
	"github.com/sighupio/furyctl/internal/state"
	"github.com/sighupio/furyctl/internal/template"
	"github.com/sighupio/furyctl/internal/upgrade"
	yamlx "github.com/sighupio/furyctl/internal/x/yaml"
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
		if s, ok := value.(public.OnpremisesKfdV1Alpha2); ok {
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

	case cluster.CreatorPropertySkipNodesUpgrade:
		if b, ok := value.(bool); ok {
			c.skipNodesUpgrade = b
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

	case cluster.CreatorPropertyUpgradeNode:
		if s, ok := value.(string); ok {
			c.upgradeNode = s
		}
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

	kubernetesPhase := upgrade.NewOperatorPhaseDecorator(
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

	distributionPhase := upgrade.NewReducerOperatorPhaseDecorator[v1alpha2.Reducers](
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
	)

	renderedConfig, err := c.RenderConfig()
	if err != nil {
		return fmt.Errorf("error while rendering config: %w", err)
	}

	status, err := preflight.Exec(renderedConfig)
	if err != nil {
		return fmt.Errorf("error while executing preflight phase: %w", err)
	}

	r, err := premrules.NewOnPremClusterRulesExtractor(c.paths.DistroPath)
	if err != nil {
		if !errors.Is(err, premrules.ErrReadingRulesFile) {
			return fmt.Errorf("error while creating rules builder: %w", err)
		}
	}

	reducers := c.buildReducers(
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

	if len(reducers) > 0 {
		logrus.Infof("Differences found from previous cluster configuration, "+
			"handling the following changes:\n%s", reducers.ToString())
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
			reducers,
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
		upgradeState := upgrade.State{
			Phases: upgrade.Phases{
				PreKubernetes:  &upgrade.Phase{Status: upgrade.PhaseStatusPending},
				Kubernetes:     &upgrade.Phase{Status: upgrade.PhaseStatusPending},
				PostKubernetes: &upgrade.Phase{Status: upgrade.PhaseStatusPending},
			},
		}

		if err := kubernetesPhase.Exec(StartFromFlagNotSet, &upgradeState); err != nil {
			return fmt.Errorf("error while executing kubernetes phase: %w", err)
		}

	case cluster.OperationPhaseDistribution:
		if len(reducers) > 0 && len(unsafeReducers) > 0 {
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

		if err := distributionPhase.Exec(reducers, StartFromFlagNotSet, &upgradeState); err != nil {
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
			reducers,
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

func (c *ClusterCreator) allPhases(
	startFrom string,
	kubernetesPhase upgrade.OperatorPhase,
	distributionPhase upgrade.ReducersOperatorPhase[v1alpha2.Reducers],
	pluginsPhase *commcreate.Plugins,
	upgr *upgrade.Upgrade,
	reducers v1alpha2.Reducers,
	unsafeReducers []rules.Rule,
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
		if err := kubernetesPhase.Exec(c.getKubernetesSubPhase(startFrom), upgradeState); err != nil {
			return fmt.Errorf("error while executing kubernetes phase: %w", err)
		}

		if c.upgradeNode != "" {
			return nil
		}
	}

	if startFrom != cluster.OperationPhasePlugins {
		if len(reducers) > 0 && len(unsafeReducers) > 0 {
			confirm, err := cluster.AskConfirmation(cluster.IsForceEnabledForFeature(c.force, cluster.ForceFeatureMigrations))
			if err != nil {
				return fmt.Errorf("error while asking for confirmation: %w", err)
			}

			if !confirm {
				return ErrAbortedByUser
			}
		}

		if err := distributionPhase.Exec(reducers, c.getDistributionSubPhase(startFrom), upgradeState); err != nil {
			return fmt.Errorf("error while executing distribution phase: %w", err)
		}
	}

	if distribution.HasFeature(c.kfdManifest, distribution.FeaturePlugins) {
		if err := pluginsPhase.Exec(); err != nil {
			return fmt.Errorf("error while executing plugins phase: %w", err)
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
					reducers = append(
						reducers,
						v1alpha2.NewBaseReducer(
							red.Key,
							red.From,
							red.To,
							red.Lifecycle,
							reducer.Path,
						),
					)
				}
			}
		}
	}

	return reducers
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

	tfCfg, err := template.NewConfigWithoutData(furyctlMerger, []string{})
	if err != nil {
		return nil, fmt.Errorf("error while creating template config: %w", err)
	}

	for k, v := range tfCfg.Data {
		specMap[k] = v
	}

	return specMap, nil
}
