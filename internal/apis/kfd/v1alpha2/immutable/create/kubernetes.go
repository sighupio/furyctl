// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package create

import (
	"fmt"
	"path"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/furyctl/internal/apis/config"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/immutable/public"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/tool/ansible"
	"github.com/sighupio/furyctl/internal/upgrade"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	kubex "github.com/sighupio/furyctl/internal/x/kube"
	templatex "github.com/sighupio/furyctl/pkg/template"
)

const (
	FromSecondsToHalfMinuteRetries = 30

	// The playbook that upgrades one worker node. This is the file name of the playbook,
	// and not the value that the pre-upgrade phase looks for in an upgrade path template.
	workerUpgradePlaybook = "upgrade-worker-nodes.yml"
)

type Kubernetes struct {
	*cluster.OperationPhase

	furyctlConf       public.ImmutableKfdV1Alpha2
	kfdManifest       config.KFD
	paths             cluster.CreatorPaths
	dryRun            bool
	ansibleRunner     *ansible.Runner
	upgrade           *upgrade.Upgrade
	upgradeNode       string
	skipNodesUpgrade  bool
	workerNodes       []string
	force             []string
	podRunningTimeout int
}

func NewKubernetes(
	furyctlConf public.ImmutableKfdV1Alpha2,
	kfdManifest config.KFD,
	paths cluster.CreatorPaths,
	dryRun bool,
	upgr *upgrade.Upgrade,
	upgradeNode string,
	skipNodesUpgrade bool,
	workerNodes []string,
	force []string,
	podRunningTimeout int,
) *Kubernetes {
	phase := cluster.NewOperationPhase(
		path.Join(paths.WorkDir, cluster.OperationPhaseKubernetes),
		kfdManifest.Tools,
		paths.BinPath,
	)

	return &Kubernetes{
		OperationPhase: phase,
		furyctlConf:    furyctlConf,
		kfdManifest:    kfdManifest,
		paths:          paths,
		dryRun:         dryRun,
		ansibleRunner: ansible.NewRunner(
			execx.NewStdExecutor(),
			ansible.PathsForVersion(paths.BinPath, kfdManifest.Tools.Immutable.Ansible.Version, phase.Path),
		),
		upgrade:           upgr,
		upgradeNode:       upgradeNode,
		skipNodesUpgrade:  skipNodesUpgrade,
		workerNodes:       workerNodes,
		force:             force,
		podRunningTimeout: podRunningTimeout,
	}
}

func (k *Kubernetes) Self() *cluster.OperationPhase {
	return k.OperationPhase
}

func (k *Kubernetes) Exec(startFrom string, upgradeState *upgrade.State) error {
	logrus.Info("Configuring SIGHUP Distribution Kubernetes cluster...")

	if err := k.prepare(); err != nil {
		return fmt.Errorf("error preparing kubernetes phase: %w", err)
	}

	if k.dryRun {
		logrus.Info("Kubernetes cluster configured successfully (dry-run mode)")

		return nil
	}

	if k.upgradeNode != "" {
		// Exec already called prepare(), so this goes to the runner and not to
		// UpgradeWorkerNodes.
		return k.runWorkerUpgradePlaybooks([]string{k.upgradeNode}, nil)
	}

	if err := k.preKubernetes(startFrom, upgradeState); err != nil {
		return fmt.Errorf("error running pre-kubernetes phase: %w", err)
	}

	if err := k.coreKubernetes(startFrom, upgradeState); err != nil {
		return fmt.Errorf("error running core kubernetes phase: %w", err)
	}

	if err := k.postKubernetes(upgradeState); err != nil {
		return fmt.Errorf("error running post-kubernetes phase: %w", err)
	}

	logrus.Info("SIGHUP Distribution Kubernetes cluster configured successfully")

	return nil
}

func (k *Kubernetes) SetUpgrade(upgradeEnabled bool) {
	k.upgrade.Enabled = upgradeEnabled
}

// UpgradeWorkerNodes upgrades the given worker nodes. The resume path calls this, and
// that path has no earlier phase to render the folder, so this method calls prepare().
func (k *Kubernetes) UpgradeWorkerNodes(
	nodes []string,
	onResult func(node string, status upgrade.PhaseStatus) error,
) error {
	if err := k.prepare(); err != nil {
		return fmt.Errorf("error preparing kubernetes phase: %w", err)
	}

	if k.dryRun {
		logrus.Infof("Would upgrade %d worker nodes (dry-run mode)", len(nodes))

		return nil
	}

	return k.runWorkerUpgradePlaybooks(nodes, onResult)
}

// runWorkerUpgradePlaybooks upgrades one node for each run of the playbook. The caller
// renders the phase first.
//
// A full upgrade does not come here. That path runs the playbook one time for every
// worker from the pre-kubernetes script, and the serial value of the playbook gives the
// order. This method gives the per-node result that the resume path records.
func (k *Kubernetes) runWorkerUpgradePlaybooks(
	nodes []string,
	onResult func(node string, status upgrade.PhaseStatus) error,
) error {
	for i, node := range nodes {
		logrus.Infof("Upgrading the worker node %s (%d of %d)...", node, i+1, len(nodes))

		if _, err := k.ansibleRunner.Playbook(workerUpgradePlaybook, "--limit", node); err != nil {
			workerErr := fmt.Errorf("error upgrading node %s: %w", node, err)

			if onResult != nil {
				if stateErr := onResult(node, upgrade.PhaseStatusFailed); stateErr != nil {
					return fmt.Errorf("%w, error saving worker upgrade state: %w", workerErr, stateErr)
				}
			}

			return workerErr
		}

		if onResult != nil {
			if err := onResult(node, upgrade.PhaseStatusSuccess); err != nil {
				return fmt.Errorf("error saving successful upgrade state for node %s: %w", node, err)
			}
		}
	}

	return nil
}

func (k *Kubernetes) prepare() error {
	if err := k.CreateRootFolder(); err != nil {
		return fmt.Errorf("error creating kubernetes phase folder: %w", err)
	}

	furyctlMerger, err := k.CreateFuryctlMerger(
		k.paths.DistroPath,
		k.paths.ConfigPath,
		"kfd-v1alpha2",
		"immutable",
	)
	if err != nil {
		return fmt.Errorf("error creating furyctl merger: %w", err)
	}

	mCfg, err := templatex.NewConfigWithoutData(furyctlMerger, []string{})
	if err != nil {
		return fmt.Errorf("error creating template config: %w", err)
	}

	k.CopyPathsToConfig(&mCfg)

	version := k.kfdManifest.Kubernetes.Immutable.Version

	mCfg.Data["kubernetes"] = map[any]any{
		"version": version,
	}

	mCfg.Data["options"]["skipPodsRunningCheck"] = cluster.IsForceEnabledForFeature(
		k.force,
		cluster.ForceFeaturePodsRunningCheck,
	)
	mCfg.Data["options"]["podRunningTimeout"] = k.podRunningTimeout / FromSecondsToHalfMinuteRetries

	// Inject the immutable.yaml version data under "versions" so hosts.yaml renders the pins inline.
	versionVars, err := VersionVarsForPhase(k.Path, version, k.KubectlPath)
	if err != nil {
		return fmt.Errorf("error building version vars: %w", err)
	}

	mCfg.Data["versions"] = versionVars

	if err := k.CopyFromTemplate(
		mCfg,
		"kubernetes",
		path.Join(k.paths.DistroPath, "templates", cluster.OperationPhaseKubernetes, "immutable"),
		k.Path,
		k.paths.ConfigPath,
	); err != nil {
		return fmt.Errorf("error copying from template: %w", err)
	}

	if k.dryRun {
		return nil
	}

	// Check hosts connection.
	logrus.Info("Checking that the hosts are reachable...")

	if _, err := k.ansibleRunner.Exec("all", "-m", "ping"); err != nil {
		return fmt.Errorf("error checking hosts: %w", err)
	}

	return nil
}

func (k *Kubernetes) preKubernetes(
	startFrom string,
	upgradeState *upgrade.State,
) error {
	if startFrom == "" || startFrom == cluster.OperationSubPhasePreKubernetes {
		// Run upgrade script if needed.
		if err := k.upgrade.Exec(k.Path, "pre-kubernetes"); err != nil {
			upgradeState.Phases.PreKubernetes.Status = upgrade.PhaseStatusFailed

			return fmt.Errorf("error running upgrade: %w", err)
		}

		if k.upgrade.Enabled {
			upgradeState.Phases.PreKubernetes.Status = upgrade.PhaseStatusSuccess

			k.stageWorkers(upgradeState)
		}
	}

	return nil
}

// stageWorkers records the workers that this run does not upgrade, so that a later run
// upgrades them one at a time.
//
// The upgrade path must hold a worker upgrade for this to make sense. A path that
// upgrades no worker leaves nothing to stage, and IncludesWorkerUpgrade reports this.
func (k *Kubernetes) stageWorkers(upgradeState *upgrade.State) {
	if !k.skipNodesUpgrade || len(k.workerNodes) == 0 || !k.upgrade.IncludesWorkerUpgrade {
		return
	}

	nodes := make(map[string]upgrade.PhaseStatus, len(k.workerNodes))
	for _, node := range k.workerNodes {
		nodes[node] = upgrade.PhaseStatusPending
	}

	upgradeState.Transition = &upgrade.Transition{
		From: k.upgrade.From,
		To:   k.upgrade.To,
	}
	upgradeState.StagedWorkers = &upgrade.StagedWorkers{Nodes: nodes}

	logrus.Infof("Skipped the upgrade of %d worker nodes, run furyctl apply --upgrade-node for each of them", len(nodes))
}

func (k *Kubernetes) coreKubernetes(
	startFrom string,
	upgradeState *upgrade.State,
) error {
	if startFrom != cluster.OperationSubPhasePostKubernetes {
		logrus.Info("Applying cluster configuration...")

		// Apply create playbook.
		if !k.upgrade.Enabled {
			if _, err := k.ansibleRunner.Playbook("apply.yaml"); err != nil {
				return fmt.Errorf("error applying playbook: %w", err)
			}
		} else {
			upgradeState.Phases.Kubernetes.Status = upgrade.PhaseStatusSuccess
		}

		if err := kubex.SetConfigEnv(path.Join(k.Path, "admin.conf")); err != nil {
			return fmt.Errorf("error setting kubeconfig env: %w", err)
		}

		if err := kubex.CopyToWorkDir(path.Join(k.Path, "admin.conf"), "kubeconfig"); err != nil {
			return fmt.Errorf("error copying kubeconfig: %w", err)
		}

		if k.furyctlConf.Spec.Kubernetes.Advanced != nil && k.furyctlConf.Spec.Kubernetes.Advanced.Users != nil {
			for _, username := range k.furyctlConf.Spec.Kubernetes.Advanced.Users.Names {
				if err := kubex.CopyToWorkDir(
					path.Join(
						k.Path,
						username+".kubeconfig",
					),
					username+".kubeconfig",
				); err != nil {
					return fmt.Errorf("error copying %s.kubeconfig: %w", username, err)
				}
			}
		}
	}

	return nil
}

func (k *Kubernetes) postKubernetes(
	upgradeState *upgrade.State,
) error {
	if err := k.upgrade.Exec(k.Path, "post-kubernetes"); err != nil {
		upgradeState.Phases.PostKubernetes.Status = upgrade.PhaseStatusFailed

		return fmt.Errorf("error running upgrade: %w", err)
	}

	if k.upgrade.Enabled {
		upgradeState.Phases.PostKubernetes.Status = upgrade.PhaseStatusSuccess
	}

	return nil
}
