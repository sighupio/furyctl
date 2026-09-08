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

	"github.com/sighupio/furyctl/internal/apis/config"
	commcreate "github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/common/create"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/onpremises/create"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/onpremises/public"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/onpremises/supported"
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
	KubernetesPhaseSchemaPath                       = ".spec.kubernetes"
	DistributionPhaseSchemaPath                     = ".spec.distribution"
	PluginsPhaseSchemaPath                          = ".spec.plugins"
	AllPhaseSchemaPath                              = ""
	StartFromFlagNotSet                             = ""
	stagedUpgradeProceed        stagedUpgradeAction = 0
	stagedUpgradeResumeBatch    stagedUpgradeAction = 1
	stagedUpgradeResumeNode     stagedUpgradeAction = 2
	stagedUpgradeFinalize       stagedUpgradeAction = 3
	stagedUpgradeNoop           stagedUpgradeAction = 4
)

var (
	ErrUnsupportedPhase = errors.New("unsupported phase")
	ErrAbortedByUser    = errors.New("operation aborted by user")
	errStagedUpgrade    = errors.New("staged worker upgrade")
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

type stagedWorkersUpgrader interface {
	UpgradeWorkerNodes(
		nodes []string,
		onResult func(node string, status upgrade.PhaseStatus) error,
	) error
}

type stagedUpgradeAction uint8

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
	var workerNodes []string
	if c.skipNodesUpgrade && c.upgrade {
		workerNodes = c.workerNodes()
	}

	kubernetes := create.NewKubernetes(
		c.furyctlConf,
		c.kfdManifest,
		c.paths,
		c.dryRun,
		upgr,
		c.upgradeNode,
		c.skipNodesUpgrade,
		workerNodes,
		c.force,
		podRunningCheckTimeout,
	)
	kubernetesPhase := upgrade.NewReducerOperatorPhaseDecorator[reducers.Reducers](
		c.upgradeStateStore,
		kubernetes,
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
	var existingUpgradeState *upgrade.State
	if status.ClusterExists {
		var found bool
		existingUpgradeState, found, err = c.loadUpgradeState()
		if err != nil {
			return err
		}
		if !found {
			existingUpgradeState = nil
		}

		action, err := c.stagedUpgradeDecision(existingUpgradeState, status.Diffs, startFrom)
		if err != nil {
			return err
		}
		if action == stagedUpgradeFinalize {
			if c.dryRun {
				existingUpgradeState.StagedWorkers.ReadyForResume = true
			} else if err := c.persistStagedUpgradeReady(existingUpgradeState, renderedConfig); err != nil {
				return err
			}

			// The target configuration is now persisted (or would be in dry-run),
			// therefore the preflight diff from before reconciliation is stale.
			action, err = c.stagedUpgradeDecision(existingUpgradeState, nil, startFrom)
			if err != nil {
				return err
			}
		}

		switch action {
		case stagedUpgradeProceed:

		case stagedUpgradeResumeBatch:
			return c.resumeStagedWorkerBatch(kubernetes, existingUpgradeState, renderedConfig)

		case stagedUpgradeResumeNode:
			return c.resumeStagedWorkers(
				kubernetes,
				existingUpgradeState,
				[]string{c.upgradeNode},
				renderedConfig,
			)

		case stagedUpgradeNoop:
			logStagedWorkerNextSteps(existingUpgradeState)

			return nil

		default:
			return fmt.Errorf("%w: unsupported staged upgrade action %d", errStagedUpgrade, action)
		}

		if !c.upgrade && c.upgradeNode == "" {
			existingUpgradeState = nil
		}
	}

	r, err := premrules.NewOnPremClusterRulesExtractor(c.paths.DistroPath, renderedConfig, supported.Phases())
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

	appliedUpgradeState, err := c.executePhase(
		startFrom,
		kubernetesPhase,
		distributionPhase,
		pluginsPhase,
		upgr,
		kubeRdcs,
		rdcs,
		unsafeKubeReducers,
		unsafeReducers,
		existingUpgradeState,
	)
	if err != nil {
		return err
	}

	return c.persistAppliedConfig(appliedUpgradeState, upgr, renderedConfig)
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

func (c *ClusterCreator) executePhase(
	startFrom string,
	kubernetesPhase upgrade.ReducersOperatorPhase[reducers.Reducers],
	distributionPhase upgrade.ReducersOperatorPhase[reducers.Reducers],
	pluginsPhase *commcreate.Plugins,
	upgr *upgrade.Upgrade,
	kubeRdcs reducers.Reducers,
	rdcs reducers.Reducers,
	unsafeKubeReducers []premrules.Rule,
	unsafeReducers []premrules.Rule,
	existingUpgradeState *upgrade.State,
) (*upgrade.State, error) {
	switch c.phase {
	case cluster.OperationPhaseKubernetes:
		if err := c.confirmUnsafeReducers(kubeRdcs, unsafeKubeReducers); err != nil {
			return nil, err
		}

		upgradeState := &upgrade.State{Phases: upgrade.Phases{
			PreKubernetes:  &upgrade.Phase{Status: upgrade.PhaseStatusPending},
			Kubernetes:     &upgrade.Phase{Status: upgrade.PhaseStatusPending},
			PostKubernetes: &upgrade.Phase{Status: upgrade.PhaseStatusPending},
		}}
		if err := kubernetesPhase.Exec(kubeRdcs, StartFromFlagNotSet, upgradeState); err != nil {
			return nil, fmt.Errorf("error while executing kubernetes phase: %w", err)
		}

		return upgradeState, nil

	case cluster.OperationPhaseDistribution:
		if err := c.confirmUnsafeReducers(rdcs, unsafeReducers); err != nil {
			return nil, err
		}

		upgradeState := &upgrade.State{Phases: upgrade.Phases{
			PreDistribution:  &upgrade.Phase{Status: upgrade.PhaseStatusPending},
			Distribution:     &upgrade.Phase{Status: upgrade.PhaseStatusPending},
			PostDistribution: &upgrade.Phase{Status: upgrade.PhaseStatusPending},
		}}
		if err := distributionPhase.Exec(rdcs, StartFromFlagNotSet, upgradeState); err != nil {
			return nil, fmt.Errorf("error while executing distribution phase: %w", err)
		}

		return upgradeState, nil

	case cluster.OperationPhasePlugins:
		if !distribution.HasFeature(c.kfdManifest, distribution.FeaturePlugins) {
			return nil, fmt.Errorf("error while executing plugins phase: %w", distribution.ErrPluginsFeatureNotSupported)
		}

		if err := pluginsPhase.Exec(); err != nil {
			return nil, fmt.Errorf("error while executing plugins phase: %w", err)
		}

		return &upgrade.State{}, nil

	case cluster.OperationPhaseAll:
		upgradeState, err := c.allPhases(
			startFrom,
			kubernetesPhase,
			distributionPhase,
			pluginsPhase,
			upgr,
			kubeRdcs,
			rdcs,
			unsafeKubeReducers,
			unsafeReducers,
			existingUpgradeState,
		)
		if err != nil {
			return nil, fmt.Errorf("error while executing cluster creation: %w", err)
		}

		return upgradeState, nil

	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedPhase, c.phase)
	}
}

func (c *ClusterCreator) confirmUnsafeReducers(rdcs reducers.Reducers, unsafe []premrules.Rule) error {
	if len(rdcs) == 0 || len(unsafe) == 0 {
		return nil
	}

	confirm, err := cluster.AskConfirmation(cluster.IsForceEnabledForFeature(c.force, cluster.ForceFeatureMigrations))
	if err != nil {
		return fmt.Errorf("error while asking for confirmation: %w", err)
	}
	if !confirm {
		return ErrAbortedByUser
	}

	return nil
}

func (c *ClusterCreator) persistAppliedConfig(
	appliedUpgradeState *upgrade.State,
	upgr *upgrade.Upgrade,
	renderedConfig map[string]any,
) error {
	if c.dryRun {
		return nil
	}

	if appliedUpgradeState.HasStagedWorkers() {
		switch {
		case appliedUpgradeState.AllTrackedPhasesSucceeded() && appliedUpgradeState.AllStagedWorkersSucceeded():
			if err := c.upgradeStateStore.Delete(); err != nil {
				return fmt.Errorf("error while deleting completed staged worker state: %w", err)
			}

		case appliedUpgradeState.AllTrackedPhasesSucceeded():
			if err := c.persistStagedUpgradeReady(appliedUpgradeState, renderedConfig); err != nil {
				return err
			}

			logStagedWorkerNextSteps(appliedUpgradeState)

			return nil

		default:
			return fmt.Errorf(
				"%w: the staged rollout is incomplete; set spec.distributionVersion to %q and run 'furyctl apply --upgrade'",
				errStagedUpgrade,
				appliedUpgradeState.Transition.To,
			)
		}
	} else if upgr.Enabled {
		if err := c.upgradeStateStore.Delete(); err != nil {
			return fmt.Errorf("error while deleting upgrade state: %w", err)
		}
	}

	return c.storeTargetConfig(renderedConfig)
}

// persistStagedUpgradeReady saves the configuration before marking workers ready.
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

func (c *ClusterCreator) storeTargetConfig(renderedConfig map[string]any) error {
	if err := c.stateStore.StoreConfig(renderedConfig); err != nil {
		return fmt.Errorf("error storing target configuration: %w", err)
	}
	if err := c.stateStore.StoreKFD(); err != nil {
		return fmt.Errorf("error storing target distribution configuration: %w", err)
	}

	return nil
}

func (c *ClusterCreator) workerNodes() []string {
	nodes := make([]string, 0)

	for _, group := range c.furyctlConf.Spec.Kubernetes.Nodes {
		for _, host := range group.Hosts {
			nodes = append(nodes, host.Name)
		}
	}

	return nodes
}

// stagedUpgradeDecision selects the next action for a validated worker upgrade.
func (c *ClusterCreator) stagedUpgradeDecision(
	upgradeState *upgrade.State,
	changes r3diff.Changelog,
	startFrom string,
) (stagedUpgradeAction, error) {
	if upgradeState == nil || !upgradeState.HasStagedWorkers() {
		return stagedUpgradeProceed, nil
	}
	if !c.upgrade && c.upgradeNode == "" {
		return stagedUpgradeProceed, fmt.Errorf(
			"%w: a worker upgrade is pending; run 'furyctl apply --upgrade' to continue",
			errStagedUpgrade,
		)
	}
	if c.upgradeNode != "" {
		if !upgradeState.StagedWorkers.ReadyForResume && upgradeState.AllTrackedPhasesSucceeded() {
			if err := validateStagedTransition(upgradeState, changes); err != nil {
				return stagedUpgradeProceed, err
			}

			return stagedUpgradeFinalize, nil
		}
		if len(changes) != 0 {
			return stagedUpgradeProceed, fmt.Errorf(
				"%w: configuration changed while workers are pending; run 'furyctl apply --upgrade' before using --upgrade-node",
				errStagedUpgrade,
			)
		}
		if !upgradeState.StagedWorkers.ReadyForResume {
			return stagedUpgradeProceed, rejectIncompleteStagedUpgrade(upgradeState, changes)
		}

		return stagedUpgradeResumeNode, nil
	}

	if !upgradeState.StagedWorkers.ReadyForResume {
		if upgradeState.AllTrackedPhasesSucceeded() {
			if err := validateStagedTransition(upgradeState, changes); err != nil {
				return stagedUpgradeProceed, err
			}

			return stagedUpgradeFinalize, nil
		}

		return stagedUpgradeProceed, rejectIncompleteStagedUpgrade(upgradeState, changes)
	}
	phaseScoped := c.phase != "" && c.phase != cluster.OperationPhaseAll
	if c.skipNodesUpgrade || phaseScoped || startFrom != "" || len(c.postApplyPhases) > 0 {
		return stagedUpgradeNoop, nil
	}
	if len(changes) != 0 {
		return stagedUpgradeProceed, fmt.Errorf(
			"%w: configuration changed while workers are pending; "+
				"complete the staged worker upgrade before changing configuration",
			errStagedUpgrade,
		)
	}
	return stagedUpgradeResumeBatch, nil
}

// rejectIncompleteStagedUpgrade requires the recorded version change to continue.
func rejectIncompleteStagedUpgrade(upgradeState *upgrade.State, changes r3diff.Changelog) error {
	if err := validateStagedTransition(upgradeState, changes); err != nil {
		return err
	}
	if len(changes) > 0 {
		return nil
	}

	return fmt.Errorf(
		"%w: the staged rollout is incomplete but the configuration has no distribution version diff; "+
			"set spec.distributionVersion to %q (the staged target) and run 'furyctl apply --upgrade'",
		errStagedUpgrade,
		upgradeState.Transition.To,
	)
}

func (c *ClusterCreator) loadUpgradeState() (*upgrade.State, bool, error) {
	rawState, err := c.upgradeStateStore.Get()
	if errors.Is(err, upgrade.ErrStateNotFound) {
		return &upgrade.State{}, false, nil
	}

	if err != nil {
		return nil, false, fmt.Errorf("%w: error loading state: %w", errStagedUpgrade, err)
	}

	upgradeState := &upgrade.State{}
	if err := yamlx.UnmarshalV3(rawState, upgradeState); err != nil {
		return nil, false, fmt.Errorf("%w: error unmarshalling state: %w", errStagedUpgrade, err)
	}
	if err := validateLoadedStagedUpgradeState(upgradeState); err != nil {
		return nil, false, err
	}

	return upgradeState, true, nil
}

func validateStagedTransition(upgradeState *upgrade.State, changes r3diff.Changelog) error {
	if upgradeState == nil || upgradeState.Transition == nil ||
		upgradeState.Transition.From == "" || upgradeState.Transition.To == "" {
		return fmt.Errorf("%w: state has no valid distribution transition", errStagedUpgrade)
	}

	if len(changes) == 0 {
		return nil
	}

	// A Distribution upgrade can add default values for new fields.
	versionChanges := changes.Filter([]string{"spec", "distributionVersion"})
	if len(versionChanges) != 1 {
		return fmt.Errorf(
			"%w: the configuration changed but does not request the recorded transition; "+
				"set spec.distributionVersion to %q to complete the rollout from %s",
			errStagedUpgrade,
			upgradeState.Transition.To,
			upgradeState.Transition.From,
		)
	}

	from, fromOK := versionChanges[0].From.(string)
	to, toOK := versionChanges[0].To.(string)
	if !fromOK || !toOK || from != upgradeState.Transition.From || to != upgradeState.Transition.To {
		return fmt.Errorf(
			"%w: configuration requests %v to %v but workers are pending for %s to %s",
			errStagedUpgrade,
			versionChanges[0].From,
			versionChanges[0].To,
			upgradeState.Transition.From,
			upgradeState.Transition.To,
		)
	}

	return nil
}

func validateLoadedStagedUpgradeState(upgradeState *upgrade.State) error {
	if upgradeState == nil || !upgradeState.HasStagedWorkers() {
		return nil
	}
	if err := validateStagedTransition(upgradeState, nil); err != nil {
		return err
	}

	for node, status := range upgradeState.StagedWorkers.Nodes {
		if node == "" {
			return fmt.Errorf("%w: state contains an unnamed worker", errStagedUpgrade)
		}

		switch status {
		case upgrade.PhaseStatusPending, upgrade.PhaseStatusFailed, upgrade.PhaseStatusSuccess:
		default:
			return fmt.Errorf("%w: state contains invalid status %q for worker %q", errStagedUpgrade, status, node)
		}
	}

	if upgradeState.StagedWorkers.ReadyForResume && !upgradeState.AllTrackedPhasesSucceeded() {
		return fmt.Errorf(
			"%w: state is ready to resume worker nodes but a tracked upgrade phase is incomplete",
			errStagedUpgrade,
		)
	}

	return nil
}

func logStagedWorkerNextSteps(upgradeState *upgrade.State) {
	remaining := len(upgradeState.PendingStagedWorkers())
	if remaining == 0 {
		return
	}

	logrus.Infof(
		"%d worker nodes remain to be upgraded. Run 'furyctl apply --upgrade' to upgrade all remaining "+
			"workers, or 'furyctl apply --upgrade-node <node-name>' to upgrade one worker.",
		remaining,
	)
}

// pendingStagedWorkersInConfigOrder returns workers in configuration order.
// Workers saved in the state but missing from the configuration are appended.
func (c *ClusterCreator) pendingStagedWorkersInConfigOrder(upgradeState *upgrade.State) []string {
	pendingNodes := upgradeState.PendingStagedWorkers()

	pending := make(map[string]struct{}, len(pendingNodes))
	for _, node := range pendingNodes {
		pending[node] = struct{}{}
	}

	nodes := make([]string, 0, len(pendingNodes))

	for _, node := range c.workerNodes() {
		if _, ok := pending[node]; ok {
			nodes = append(nodes, node)

			delete(pending, node)
		}
	}

	for _, node := range pendingNodes {
		if _, ok := pending[node]; ok {
			nodes = append(nodes, node)
		}
	}

	return nodes
}

func (c *ClusterCreator) resumeStagedWorkerBatch(
	kubernetes stagedWorkersUpgrader,
	upgradeState *upgrade.State,
	renderedConfig map[string]any,
) error {
	nodes := c.pendingStagedWorkersInConfigOrder(upgradeState)
	if len(nodes) == 0 {
		return c.resumeStagedWorkers(kubernetes, upgradeState, nodes, renderedConfig)
	}

	// Do not list worker node names because a cluster can have hundreds of workers.
	message := fmt.Sprintf(
		"\nResuming staged upgrade %s to %s for %d pending worker nodes.",
		upgradeState.Transition.From,
		upgradeState.Transition.To,
		len(nodes),
	)
	confirm, err := cluster.AskConfirmationWithMessage(
		cluster.IsForceEnabledForFeature(c.force, cluster.ForceFeatureUpgrades),
		message,
	)
	if err != nil {
		return fmt.Errorf("%w: error asking for confirmation: %w", errStagedUpgrade, err)
	}
	if !confirm {
		return ErrAbortedByUser
	}

	return c.resumeStagedWorkers(kubernetes, upgradeState, nodes, renderedConfig)
}

func (c *ClusterCreator) resumeStagedWorkers(
	kubernetes stagedWorkersUpgrader,
	upgradeState *upgrade.State,
	nodes []string,
	renderedConfig map[string]any,
) error {
	// The saved state is already validated. Validate only the selected nodes.

	selectedNodes := make([]string, 0, len(nodes))
	for _, node := range nodes {
		status, ok := upgradeState.StagedWorkers.Nodes[node]
		if !ok {
			return fmt.Errorf("%w: worker node %q is not a pending worker node", errStagedUpgrade, node)
		}

		if status == upgrade.PhaseStatusSuccess {
			continue
		}

		selectedNodes = append(selectedNodes, node)
	}

	if len(selectedNodes) > 0 {
		if err := kubernetes.UpgradeWorkerNodes(selectedNodes, func(node string, status upgrade.PhaseStatus) error {
			upgradeState.MarkStagedWorker(node, status)

			if c.dryRun {
				return nil
			}

			return c.upgradeStateStore.Store(upgradeState)
		}); err != nil {
			return fmt.Errorf("%w: %w", errStagedUpgrade, err)
		}
	} else {
		logrus.Infof("The %d selected worker nodes are already upgraded", len(nodes))
	}

	if c.dryRun || !upgradeState.AllStagedWorkersSucceeded() {
		if !c.dryRun {
			logStagedWorkerNextSteps(upgradeState)
		}

		return nil
	}

	if err := c.storeTargetConfig(renderedConfig); err != nil {
		return fmt.Errorf("%w: %w", errStagedUpgrade, err)
	}

	if err := c.upgradeStateStore.Delete(); err != nil {
		return fmt.Errorf("%w: error deleting completed state: %w", errStagedUpgrade, err)
	}

	logrus.Info("Staged worker upgrade completed successfully")

	return nil
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
	existingUpgradeState *upgrade.State,
) (*upgrade.State, error) {
	upgradeState := existingUpgradeState
	if upgradeState == nil {
		upgradeState = &upgrade.State{}
	}

	if upgr.Enabled {
		if existingUpgradeState != nil {
			if startFrom == "" {
				resumableState := c.upgradeStateStore.GetLatestResumablePhase(upgradeState)

				logrus.Infof("An upgrade is already in progress, resuming from %s phase.\n"+
					"If you wish to start from a different phase, you can use the --start-from "+
					"flag to select the desired phase to resume.", resumableState)

				startFrom = resumableState
			}
		} else if !c.dryRun {
			logrus.Debugf("creating a new upgrade state on the cluster...")

			upgradeState = c.initUpgradeState()

			if err := c.upgradeStateStore.Store(upgradeState); err != nil {
				return nil, fmt.Errorf("error while storing upgrade state: %w", err)
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
				return nil, fmt.Errorf("error while asking for confirmation: %w", err)
			}

			if !confirm {
				return nil, ErrAbortedByUser
			}
		}

		if err := kubernetesPhase.Exec(kubeRdcs, c.getKubernetesSubPhase(startFrom), upgradeState); err != nil {
			return nil, fmt.Errorf("error while executing kubernetes phase: %w", err)
		}

		if c.upgradeNode != "" {
			return upgradeState, nil
		}
	}

	if startFrom != cluster.OperationPhasePlugins {
		if len(rdcs) > 0 && len(unsafeReducers) > 0 {
			confirm, err := cluster.AskConfirmation(cluster.IsForceEnabledForFeature(c.force, cluster.ForceFeatureMigrations))
			if err != nil {
				return nil, fmt.Errorf("error while asking for confirmation: %w", err)
			}

			if !confirm {
				return nil, ErrAbortedByUser
			}
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
		logrus.Info("Executing extra phases...")

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
