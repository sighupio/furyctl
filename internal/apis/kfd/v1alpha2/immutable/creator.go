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

	"github.com/samber/lo"
	"github.com/sirupsen/logrus"

	"github.com/sighupio/furyctl/internal/apis/config"
	commcreate "github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/common/create"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/immutable/create"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/immutable/public"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/immutable/supported"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/distribution"
	"github.com/sighupio/furyctl/internal/state"
	"github.com/sighupio/furyctl/internal/upgrade"
	"github.com/sighupio/furyctl/pkg/reducers"
	premrules "github.com/sighupio/furyctl/pkg/rulesextractor"
	templatex "github.com/sighupio/furyctl/pkg/template"
	yamlx "github.com/sighupio/furyctl/pkg/x/yaml"
)

const (
	AllPhaseSchemaPath  = ""
	StartFromFlagNotSet = ""
)

var (
	ErrUnsupportedPhase              = errors.New("unsupported phase")
	ErrAbortedByUser                 = errors.New("operation aborted by user")
	ErrClusterCreationNotImplemented = errors.New("cluster creation not implemented for Immutable kind")
	ErrUpgradeNodeUnsupported        = errors.New("unsupported --upgrade-node host")
	errStagedUpgrade                 = errors.New("staged worker upgrade")
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
	// An empty phase is the "all phases" sentinel (cluster.OperationPhaseAll),
	// used e.g. by `furyctl diff` without the --phase flag. In that case there
	// is no single schema path to scope to, so we return AllPhaseSchemaPath and
	// let the diff be computed against the whole configuration.
	if phase == cluster.OperationPhaseAll {
		return AllPhaseSchemaPath, nil
	}

	schemaPath, ok := supported.GetSchemaPath(phase)
	if !ok {
		return "", fmt.Errorf("%w: %s", ErrUnsupportedPhase, phase)
	}

	return schemaPath, nil
}

func (c *ClusterCreator) Create(startFrom string, _, podRunningCheckTimeout int) error {
	if err := c.validateUpgradeNode(startFrom); err != nil {
		return err
	}

	upgr := upgrade.New(c.paths, string(c.furyctlConf.Kind))

	infra := createInfrastructurePhase(c, upgr)

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
			c.skipNodesUpgrade,
			c.stagedWorkerNodes(),
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
		startFrom,
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

	rulesExtractor, err := c.newRulesExtractor(renderedConfig)
	if err != nil {
		return err
	}

	rdcsInfrastructure := reducers.Build(
		status.Diffs,
		rulesExtractor,
		cluster.OperationPhaseInfrastructure,
	)

	rdcsDistribution := reducers.Build(
		status.Diffs,
		rulesExtractor,
		cluster.OperationPhaseDistribution,
	)

	unsafeReducersInfrastructure := rulesExtractor.UnsafeReducerRulesByDiffs(
		rulesExtractor.GetReducers(
			cluster.OperationPhaseInfrastructure,
		),
		status.Diffs,
	)

	unsafeReducersDistribution := rulesExtractor.UnsafeReducerRulesByDiffs(
		rulesExtractor.GetReducers(
			cluster.OperationPhaseDistribution,
		),
		status.Diffs,
	)

	rdcs := rdcsInfrastructure
	rdcs = append(rdcs, rdcsDistribution...)
	unsafeReducers := unsafeReducersInfrastructure
	unsafeReducers = append(unsafeReducers, unsafeReducersDistribution...)

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
			rdcsDistribution,
			status.Diffs,
			c.externalUpgradesPath,
			c.skipNodesUpgrade,
		)

		if err := preupgradePhase.Exec(); err != nil {
			return fmt.Errorf("error while executing preupgrade phase: %w", err)
		}
	}

	// The finalize step reads this. Every case that tracks an upgrade assigns to it.
	appliedUpgradeState := &upgrade.State{}

	switch c.phase {
	case cluster.OperationPhaseInfrastructure:
		confirm, err := c.confirmInfrastructureChanges(rdcsInfrastructure, unsafeReducers)
		if err != nil {
			return fmt.Errorf("error while asking for confirmation: %w", err)
		}

		if !confirm {
			return ErrAbortedByUser
		}

		appliedUpgradeState.Phases = upgrade.Phases{
			PreInfrastructure:  &upgrade.Phase{Status: upgrade.PhaseStatusPending},
			Infrastructure:     &upgrade.Phase{Status: upgrade.PhaseStatusPending},
			PostInfrastructure: &upgrade.Phase{Status: upgrade.PhaseStatusPending},
		}

		if err := infrastructurePhase.Exec(StartFromFlagNotSet, appliedUpgradeState); err != nil {
			return fmt.Errorf("error while executing infrastructure phase: %w", err)
		}

	case cluster.OperationPhaseKubernetes:
		appliedUpgradeState.Phases = upgrade.Phases{
			PreKubernetes:  &upgrade.Phase{Status: upgrade.PhaseStatusPending},
			Kubernetes:     &upgrade.Phase{Status: upgrade.PhaseStatusPending},
			PostKubernetes: &upgrade.Phase{Status: upgrade.PhaseStatusPending},
		}

		if err := kubernetesPhase.Exec(StartFromFlagNotSet, appliedUpgradeState); err != nil {
			return fmt.Errorf("error while executing kubernetes phase: %w", err)
		}

	case cluster.OperationPhaseDistribution:
		confirm, err := c.confirmDistributionChanges(rdcsDistribution, unsafeReducersDistribution)
		if err != nil {
			return fmt.Errorf("error while confirming distribution changes: %w", err)
		}

		if !confirm {
			return ErrAbortedByUser
		}

		appliedUpgradeState.Phases = upgrade.Phases{
			PreDistribution:  &upgrade.Phase{Status: upgrade.PhaseStatusPending},
			Distribution:     &upgrade.Phase{Status: upgrade.PhaseStatusPending},
			PostDistribution: &upgrade.Phase{Status: upgrade.PhaseStatusPending},
		}

		if err := distributionPhase.Exec(rdcsDistribution, StartFromFlagNotSet, appliedUpgradeState); err != nil {
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
		allPhasesState, err := c.allPhases(
			startFrom,
			infrastructurePhase,
			kubernetesPhase,
			distributionPhase,
			pluginsPhase,
			upgr,
			rdcs,
			unsafeReducers,
		)
		if err != nil {
			return fmt.Errorf("error while executing cluster creation: %w", err)
		}

		appliedUpgradeState = allPhasesState

	default:
		return fmt.Errorf("%w: %s", ErrUnsupportedPhase, c.phase)
	}

	return c.persistAppliedConfig(appliedUpgradeState, upgr, renderedConfig, status)
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
	tfCfg, err := templatex.NewConfigWithoutData(furyctlMerger, []string{})
	if err != nil {
		return nil, fmt.Errorf("error while creating template config: %w", err)
	}

	// TfCfg.Data already contains the properly structured merged config
	// with "spec", "metadata", etc. from the user config merged with defaults.
	// Convert to map[string]any (including nested maps).
	return lo.MapValues(tfCfg.Data, func(v map[any]any, _ string) any {
		return convertValue(v)
	}), nil
}

// persistAppliedConfig stores the configuration of the target version, and it decides what
// happens to the upgrade state.
//
// A run that leaves workers behind must not store that configuration as a complete upgrade.
// The cluster then reports a version that the workers do not run, and the next
// "furyctl apply --upgrade" finds no difference between the two versions and does nothing.
// The upgrade state is the only record of the workers that stay behind, so a run that leaves
// any of them keeps it.
func (c *ClusterCreator) persistAppliedConfig(
	appliedUpgradeState *upgrade.State,
	upgr *upgrade.Upgrade,
	renderedConfig map[string]any,
	status *create.Status,
) error {
	if c.dryRun {
		return nil
	}

	if appliedUpgradeState.HasStagedWorkers() {
		switch {
		// Every phase and every worker succeeded. The upgrade is complete.
		case appliedUpgradeState.AllTrackedPhasesSucceeded() && appliedUpgradeState.AllStagedWorkersSucceeded():
			if err := c.upgradeStateStore.Delete(); err != nil {
				return fmt.Errorf("error while deleting completed staged worker state: %w", err)
			}

		// Every phase succeeded and some worker did not. Keep the state for the next run.
		case appliedUpgradeState.AllTrackedPhasesSucceeded():
			if err := c.persistStagedUpgradeReady(appliedUpgradeState, renderedConfig); err != nil {
				return err
			}

			logStagedWorkerNextSteps(appliedUpgradeState)

			return nil

		// A phase did not succeed. Store no configuration, because the cluster is not at
		// the target version.
		default:
			return fmt.Errorf(
				"%w: the rollout is incomplete, set spec.distributionVersion to %s and run "+
					"'furyctl apply --upgrade'",
				errStagedUpgrade,
				stagedTargetVersion(appliedUpgradeState),
			)
		}
	} else if upgr.Enabled {
		if err := c.upgradeStateStore.Delete(); err != nil {
			return fmt.Errorf("error while deleting upgrade state: %w", err)
		}
	}

	// A cluster that does not exist yet holds no configuration to store. This happens when
	// the infrastructure phase runs on its own, before the kubernetes phase ever ran.
	if !status.ClusterExists && c.phase == cluster.OperationPhaseInfrastructure {
		logrus.Info("skipping saving status to cluster because Kubernetes cluster does not exist yet")

		return nil
	}

	return c.storeTargetConfig(renderedConfig)
}

// stagedTargetVersion gives the version that the staged rollout goes to.
//
// The staging step writes the transition together with the workers, so a state that holds
// workers holds a transition. A state that a user edited breaks that rule, and a read of a
// missing transition panics, therefore this function does not assume one.
func stagedTargetVersion(upgradeState *upgrade.State) string {
	if upgradeState.Transition == nil {
		return "the version of the target distribution"
	}

	return fmt.Sprintf("%q", upgradeState.Transition.To)
}

// persistStagedUpgradeReady stores the configuration, and then marks the staged workers as
// ready to continue. The order matters: a mark without a stored configuration gives a state
// that reports a target version that the cluster does not hold.
func (c *ClusterCreator) persistStagedUpgradeReady(
	upgradeState *upgrade.State,
	renderedConfig map[string]any,
) error {
	if err := c.storeTargetConfig(renderedConfig); err != nil {
		return err
	}

	upgradeState.StagedWorkers.ReadyForResume = true

	if err := c.upgradeStateStore.Store(upgradeState); err != nil {
		return fmt.Errorf("error while marking staged worker upgrade ready to resume: %w", err)
	}

	return nil
}

// storeTargetConfig saves the configuration of the cluster and of the distribution.
func (c *ClusterCreator) storeTargetConfig(renderedConfig map[string]any) error {
	if err := c.stateStore.StoreConfig(renderedConfig); err != nil {
		return fmt.Errorf("error while creating secret with the cluster configuration: %w", err)
	}

	if err := c.stateStore.StoreKFD(); err != nil {
		return fmt.Errorf("error while creating secret with the distribution configuration: %w", err)
	}

	return nil
}

// logStagedWorkerNextSteps tells the user how many workers stay behind, and how to upgrade
// them.
func logStagedWorkerNextSteps(upgradeState *upgrade.State) {
	remaining := len(upgradeState.PendingStagedWorkers())
	if remaining == 0 {
		return
	}

	logrus.Infof(
		"%d worker nodes remain to be upgraded. Run 'furyctl apply --upgrade' to upgrade all "+
			"remaining workers, or 'furyctl apply --upgrade-node <node-name>' to upgrade one worker.",
		remaining,
	)
}

// newRulesExtractor builds the rules extractor of the distribution. A distribution
// without a rules file is not an error: the extractor is then empty.
func (c *ClusterCreator) newRulesExtractor(renderedConfig map[string]any) (*premrules.ImmutableExtractor, error) {
	rulesExtractor, err := premrules.NewImmutableClusterRulesExtractor(
		c.paths.DistroPath,
		renderedConfig,
		supported.Phases(),
	)
	if err != nil && !errors.Is(err, premrules.ErrReadingRulesFile) {
		return nil, fmt.Errorf("error while creating rules builder: %w", err)
	}

	return rulesExtractor, nil
}

// stagedWorkerNodes gives the workers that this run does not upgrade. Only an upgrade
// with --skip-nodes-upgrade stages them, so every other run gets no host and the
// kubernetes phase stages nothing.
func (c *ClusterCreator) stagedWorkerNodes() []string {
	if !c.skipNodesUpgrade || !c.upgrade {
		return nil
	}

	return c.workerNodes()
}

// workerNodes gives every host that the configuration lists under a node group.
//
// The Immutable configuration holds the workers under spec.kubernetes.nodeGroups[].nodes,
// which differs from the OnPremises layout. RoleAssignments hides that difference.
func (c *ClusterCreator) workerNodes() []string {
	nodes := make([]string, 0)

	for _, ra := range c.furyctlConf.RoleAssignments() {
		if ra.Role == public.NodeRoleWorker {
			nodes = append(nodes, ra.Hostname)
		}
	}

	return nodes
}

// validateUpgradeNode rejects a --upgrade-node host that this kind cannot upgrade on its
// own, and a phase selection that does not match the role of that host. The role selects
// the phase: the infrastructure phase upgrades a load balancer and the kubernetes phase
// upgrades a worker node. A phase that the user selects as well either does no work, or
// it runs the playbook of the other role against the host. A run for one host also
// returns before the extra phases, so --post-apply-phases belongs to the same rule.
//
// Only the Immutable kind has this rule, because only here does the phase depend on the
// role. OnPremises upgrades every --upgrade-node host in its kubernetes phase.
func (c *ClusterCreator) validateUpgradeNode(startFrom string) error {
	if _, err := c.upgradeNodeRole(); err != nil {
		return err
	}

	if c.upgradeNode == "" {
		return nil
	}

	if c.phase != cluster.OperationPhaseAll ||
		startFrom != StartFromFlagNotSet ||
		len(c.postApplyPhases) > 0 {
		return fmt.Errorf(
			"%w: %q selects its own phase, so --phase, --start-from and --post-apply-phases "+
				"cannot be used with it",
			ErrUpgradeNodeUnsupported,
			c.upgradeNode,
		)
	}

	return nil
}

// upgradeNodeRole resolves the role of the --upgrade-node host. Only a worker and a
// load balancer have a single-host upgrade path. The control plane and etcd are
// upgraded as a group, with the quorum guarded.
func (c *ClusterCreator) upgradeNodeRole() (string, error) {
	if c.upgradeNode == "" {
		return public.NodeRoleNone, nil
	}

	role := c.furyctlConf.NodeRole(c.upgradeNode)

	switch role {
	case public.NodeRoleWorker, public.NodeRoleLoadBalancer:
		return role, nil

	case public.NodeRoleNone:
		return "", fmt.Errorf(
			"%w: %q is not a host of this cluster configuration",
			ErrUpgradeNodeUnsupported,
			c.upgradeNode,
		)

	default:
		return "", fmt.Errorf(
			"%w: %q is a %s host, and only worker and load balancer hosts can be upgraded one at a time",
			ErrUpgradeNodeUnsupported,
			c.upgradeNode,
			role,
		)
	}
}

func createInfrastructurePhase(c *ClusterCreator, upgr *upgrade.Upgrade) *create.Infrastructure {
	infraPath := filepath.Join(c.paths.WorkDir, "infrastructure")

	phase := cluster.NewOperationPhase(
		infraPath,
		c.kfdManifest.Tools,
		c.paths.BinPath,
	)

	infra := create.NewInfrastructure(
		phase,
		c.paths.ConfigPath,
		c.paths.DistroPath,
		upgr,
		c.upgradeNode,
		c.furyctlConf,
		c.kfdManifest,
		c.paths,
		c.dryRun,
		c.force,
	)

	return infra
}

// convertValue recursively converts any value, handling maps and slices.
func convertValue(v any) any {
	switch val := v.(type) {
	case map[any]any:
		// Convert map[any]any to map[string]any, dropping non-string keys.
		result := make(map[string]any, len(val))

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
		return lo.MapValues(val, func(v any, _ string) any {
			return convertValue(v)
		})

	case []any:
		return lo.Map(val, func(item any, _ int) any {
			return convertValue(item)
		})

	default:
		return val
	}
}

// readUpgradeState reads a stored upgrade state, and gives the phase to resume from.
//
// It reads the state twice, because the two answers need two different readings of it.
// The phase to resume from needs the phases exactly as the earlier run stored them:
// GetLatestResumablePhase skips a phase that the state does not hold, so a phase that
// this version of furyctl adds must stay absent for that decision. Otherwise every
// resumed upgrade starts again from the first phase of the order.
//
// The run itself needs every phase to exist, because a write to a phase that the state
// does not hold panics, and a state that an older furyctl version wrote has no
// infrastructure sub-phases. A read over a complete state keeps every stored status and
// leaves the rest pending.
func (c *ClusterCreator) readUpgradeState(stored []byte, startFrom string) (*upgrade.State, string, error) {
	storedState := &upgrade.State{}
	if err := yamlx.UnmarshalV3(stored, storedState); err != nil {
		return nil, "", fmt.Errorf("error while unmarshalling upgrade state: %w", err)
	}

	if startFrom == "" {
		startFrom = c.upgradeStateStore.GetLatestResumablePhase(storedState)

		logrus.Infof("An upgrade is already in progress, resuming from %s phase.\n"+
			"If you wish to start from a different phase, you can use the --start-from "+
			"flag to select the desired phase to resume.", startFrom)
	}

	upgradeState := c.initUpgradeState()
	if err := yamlx.UnmarshalV3(stored, upgradeState); err != nil {
		return nil, "", fmt.Errorf("error while unmarshalling upgrade state: %w", err)
	}

	return upgradeState, startFrom, nil
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
) (*upgrade.State, error) {
	upgradeState := &upgrade.State{}

	if upgr.Enabled && !c.dryRun {
		s, err := c.upgradeStateStore.Get()
		if err == nil {
			upgradeState, startFrom, err = c.readUpgradeState(s, startFrom)
			if err != nil {
				return nil, err
			}
		} else {
			logrus.Debugf("error while getting upgrade state: %v", err)
			logrus.Debugf("creating a new upgrade state on the cluster...")

			upgradeState = c.initUpgradeState()

			if err := c.upgradeStateStore.Store(upgradeState); err != nil {
				return nil, fmt.Errorf("error while storing upgrade state: %w", err)
			}
		}
	}

	if startFrom == "" ||
		startFrom == cluster.OperationPhaseInfrastructure ||
		startFrom == cluster.OperationSubPhasePreInfrastructure ||
		startFrom == cluster.OperationSubPhasePostInfrastructure {
		confirm, err := c.confirmInfrastructureChanges(rdcs, unsafeReducers)
		if err != nil {
			return nil, fmt.Errorf("error while asking for confirmation: %w", err)
		}

		if !confirm {
			return nil, ErrAbortedByUser
		}

		if err := infrastructurePhase.Exec(c.getInfrastructureSubPhase(startFrom), upgradeState); err != nil {
			return nil, fmt.Errorf("error while executing infrastructure phase: %w", err)
		}

		// A load balancer is upgraded by the infrastructure phase alone. It is not a
		// Kubernetes node, so the phases below have nothing to do with it.
		if c.upgradeNode != "" && c.furyctlConf.NodeRole(c.upgradeNode) == public.NodeRoleLoadBalancer {
			return upgradeState, nil
		}
	}

	if startFrom != cluster.OperationSubPhasePreDistribution &&
		startFrom != cluster.OperationPhaseDistribution &&
		startFrom != cluster.OperationSubPhasePostDistribution &&
		startFrom != cluster.OperationPhasePlugins {
		if err := kubernetesPhase.Exec(c.getKubernetesSubPhase(startFrom), upgradeState); err != nil {
			return nil, fmt.Errorf("error while executing kubernetes phase: %w", err)
		}

		if c.upgradeNode != "" {
			return upgradeState, nil
		}
	}

	if startFrom != cluster.OperationPhasePlugins {
		confirm, err := c.confirmDistributionChanges(rdcs, unsafeReducers)
		if err != nil {
			return nil, fmt.Errorf("error while asking for confirmation: %w", err)
		}

		if !confirm {
			return nil, ErrAbortedByUser
		}

		if err := distributionPhase.Exec(rdcs, c.getDistributionSubPhase(startFrom), upgradeState); err != nil {
			return nil, fmt.Errorf("error while executing distribution phase: %w", err)
		}
	}

	if distribution.HasFeature(c.kfdManifest, distribution.FeaturePlugins) {
		if err := pluginsPhase.Exec(); err != nil {
			return nil, fmt.Errorf("error while executing plugins phase: %w", err)
		}
	}

	if len(c.postApplyPhases) > 0 {
		logrus.Infof("Executing extra phases: %s...", strings.Join(c.postApplyPhases, ", "))

		if err := c.extraPhases(
			kubernetesPhase,
			distributionPhase,
			pluginsPhase,
			upgr,
			upgradeState,
		); err != nil {
			return nil, fmt.Errorf("error while executing extra phases: %w", err)
		}
	}

	return upgradeState, nil
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

		default:
			logrus.Debugf("ignoring unknown post-apply phase %q", phase)
		}
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

func (c *ClusterCreator) confirmInfrastructureChanges(
	rdcs reducers.Reducers,
	unsafeReducers []premrules.Rule,
) (bool, error) {
	if len(rdcs) > 0 && len(unsafeReducers) > 0 {
		askConfirmation := false

		if strings.Contains(rdcs.ToString(), ".spec.infrastructure.") {
			askConfirmation = true

			logrus.Warning("Changes to infrastructure phase configuration " +
				"that could cause data loss or service disruption have been found.")
		}

		if strings.Contains(rdcs.ToString(), ".spec.infrastructure.nodes") {
			askConfirmation = true

			logrus.Warning("Changes to configuration that require nodes reprovisioning have been found. " +
				"Manual intervention will be required to reset the nodes.")
		}

		if askConfirmation {
			confirm, err := cluster.AskConfirmationWithMessage(
				cluster.IsForceEnabledForFeature(c.force, cluster.ForceFeatureMigrations),
				"\nPotentially unsafe changes or that require manual intervention have been detected. Proceed with caution.",
			)
			if err != nil {
				return false, fmt.Errorf("error while asking for confirmation: %w", err)
			}

			return confirm, nil
		}
	}

	return true, nil
}

func (c *ClusterCreator) confirmDistributionChanges(
	rdcs reducers.Reducers,
	unsafeReducers []premrules.Rule,
) (bool, error) {
	if len(rdcs) > 0 && len(unsafeReducers) > 0 {
		if strings.Contains(rdcs.ToString(), ".spec.distribution") {
			confirm, err := cluster.AskConfirmation(cluster.IsForceEnabledForFeature(c.force, cluster.ForceFeatureMigrations))
			if err != nil {
				return false, fmt.Errorf("error while asking for confirmation: %w", err)
			}

			return confirm, nil
		}
	}

	return true, nil
}
