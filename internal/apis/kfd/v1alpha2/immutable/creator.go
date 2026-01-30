// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package immutable

import (
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/fury-distribution/pkg/apis/immutable/v1alpha2/public"
	commcreate "github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/common/create"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/immutable/create"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/immutable/supported"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/state"
	"github.com/sighupio/furyctl/internal/upgrade"
	"github.com/sighupio/furyctl/pkg/reducers"
	premrules "github.com/sighupio/furyctl/pkg/rulesextractor"
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
	ErrUnsupportedPhase              = errors.New("unsupported phase")
	ErrAbortedByUser                 = errors.New("operation aborted by user")
	ErrClusterCreationNotImplemented = errors.New("cluster creation not implemented for Immutable kind")
)

type ClusterCreator struct {
	paths                cluster.CreatorPaths
	furyctlConf          public.ImmutableKfdV1Alpha2
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
		if s, ok := value.(public.ImmutableKfdV1Alpha2); ok {
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

	case cluster.CreatorPropertyPostApplyPhases:
		if s, ok := value.([]string); ok {
			c.postApplyPhases = s
		}
	}
}

func (*ClusterCreator) GetPhasePath(phase string) (string, error) {
	schemaPath, ok := supported.GetSchemaPath(phase)
	if !ok {
		return "", fmt.Errorf("%w: %s", ErrUnsupportedPhase, phase)
	}

	return schemaPath, nil
}

func createInfrastructurePhase(c *ClusterCreator) (*create.Infrastructure, error) {
	// Render merged configuration (defaults + user config).
	mergedConfig, err := c.RenderConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to render config: %w", err)
	}

	infraPath := filepath.Join(c.paths.WorkDir, "infrastructure")

	phase := cluster.NewOperationPhase(
		infraPath,
		c.kfdManifest.Tools,
		c.paths.BinPath,
	)

	infra := create.NewInfrastructure(phase, c.paths.ConfigPath, mergedConfig, c.paths.DistroPath)

	return infra, nil
}

func (c *ClusterCreator) Create(startFrom string, _, podRunningCheckTimeout int) error {
	upgr := upgrade.New(c.paths, string(c.furyctlConf.Kind))

	infra, err := createInfrastructurePhase(c)
	if err != nil {
		return fmt.Errorf("failed to create infrastructure phase: %w", err)
	}

	infrastructurePhase := upgrade.NewOperatorPhaseDecorator(
		c.upgradeStateStore,
		infra,
		c.dryRun,
		upgr,
	)

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

	r, err := premrules.NewImmutableClusterRulesExtractor(c.paths.DistroPath, renderedConfig)
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
			c.skipNodesUpgrade,
		)

		if err := preupgradePhase.Exec(); err != nil {
			return fmt.Errorf("error while executing preupgrade phase: %w", err)
		}
	}

	switch c.phase {
	case cluster.OperationPhaseInfrastructure:
		upgradeState := upgrade.State{
			Phases: upgrade.Phases{
				PreInfrastructure:  &upgrade.Phase{Status: upgrade.PhaseStatusPending},
				Infrastructure:     &upgrade.Phase{Status: upgrade.PhaseStatusPending},
				PostInfrastructure: &upgrade.Phase{Status: upgrade.PhaseStatusPending},
			},
		}

		if err := infrastructurePhase.Exec(StartFromFlagNotSet, &upgradeState); err != nil {
			return fmt.Errorf("error while executing infrastructure phase: %w", err)
		}

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
			infrastructurePhase,
			kubernetesPhase,
			distributionPhase,
			pluginsPhase,
			upgr,
			rdcs,
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

// convertValue recursively converts any value, handling maps and slices.
func convertValue(v any) any {
	switch val := v.(type) {
	case map[any]any:
		// Convert map[any]any to map[string]any.
		result := make(map[string]any)

		for k, v := range val {
			keyStr, ok := k.(string)
			if !ok {
				continue
			}

			result[keyStr] = convertValue(v)
		}

		return result

	case map[string]any:
		// Already correct type, but check nested values.
		result := make(map[string]any)
		for k, v := range val {
			result[k] = convertValue(v)
		}

		return result

	case []any:
		result := make([]any, len(val))
		for i, item := range val {
			result[i] = convertValue(item)
		}

		return result

	default:
		return val
	}
}

func (c *ClusterCreator) allPhases(
	startFrom string,
	infrastructurePhase upgrade.OperatorPhase,
	kubernetesPhase upgrade.OperatorPhase,
	distributionPhase upgrade.ReducersOperatorPhase[reducers.Reducers],
	pluginsPhase *commcreate.Plugins,
	upgr *upgrade.Upgrade,
	rdcs reducers.Reducers,
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

	if startFrom == "" ||
		startFrom == cluster.OperationPhaseInfrastructure ||
		startFrom == cluster.OperationSubPhasePreInfrastructure ||
		startFrom == cluster.OperationSubPhasePostInfrastructure {
		if err := infrastructurePhase.Exec(c.getInfrastructureSubPhase(startFrom), upgradeState); err != nil {
			return fmt.Errorf("error while executing infrastructure phase: %w", err)
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
	kubernetesPhase upgrade.OperatorPhase,
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

			if err := kubernetesPhase.Exec(StartFromFlagNotSet, upgradeState); err != nil {
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

// RenderConfig loads the complete furyctl configuration merged with defaults from fury-distribution.
// For infrastructure phase, we need the full spec including infrastructure config.
func (c *ClusterCreator) RenderConfig() (map[string]any, error) {
	// Create phase for infrastructure.
	phase := cluster.NewOperationPhase(
		path.Join(c.paths.WorkDir, cluster.OperationPhaseInfrastructure),
		c.kfdManifest.Tools,
		c.paths.BinPath,
	)

	// Use CreateFuryctlMerger to merge defaults + user config.
	furyctlMerger, err := phase.CreateFuryctlMerger(
		c.paths.DistroPath,
		c.paths.ConfigPath,
		"kfd-v1alpha2",
		"immutable",
	)
	if err != nil {
		return nil, fmt.Errorf("error while creating furyctl merger: %w", err)
	}

	// Create template config without data.
	tfCfg, err := template.NewConfigWithoutData(furyctlMerger, []string{})
	if err != nil {
		return nil, fmt.Errorf("error while creating template config: %w", err)
	}

	// TfCfg.Data already contains the properly structured merged config
	// with "spec", "metadata", etc. from the user config merged with defaults.
	// Convert to map[string]any (including nested maps).
	result := make(map[string]any)

	for k, v := range tfCfg.Data {
		result[k] = convertValue(v)
	}

	return result, nil
}
