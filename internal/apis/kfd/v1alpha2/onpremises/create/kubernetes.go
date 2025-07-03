// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package create

import (
	"fmt"
	"path"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/fury-distribution/pkg/apis/onpremises/v1alpha2/public"
	"github.com/sirupsen/logrus"

	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/tool/ansible"
	"github.com/sighupio/furyctl/internal/upgrade"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	kubex "github.com/sighupio/furyctl/internal/x/kube"
	"github.com/sighupio/furyctl/pkg/template"
)

const FromSecondsToHalfMinuteRetries = 30

type Kubernetes struct {
	*cluster.OperationPhase

	furyctlConf       public.OnpremisesKfdV1Alpha2
	kfdManifest       config.KFD
	paths             cluster.CreatorPaths
	dryRun            bool
	ansibleRunner     *ansible.Runner
	upgrade           *upgrade.Upgrade
	upgradeNode       string
	force             []string
	podRunningTimeout int
}

func (k *Kubernetes) Self() *cluster.OperationPhase {
	logrus.WithField("k.OperationPhase", k.OperationPhase).Debug("[DEBUG] Self: returning operation phase")
	return k.OperationPhase
}

func (k *Kubernetes) Exec(startFrom string, upgradeState *upgrade.State) error {
	logrus.WithFields(logrus.Fields{
		"startFrom":       startFrom,
		"upgradeState":    upgradeState,
		"k.dryRun":        k.dryRun,
		"k.upgradeNode":   k.upgradeNode,
		"k.upgrade":       k.upgrade,
		"k.ansibleRunner": k.ansibleRunner,
	}).Info("[DEBUG] Kubernetes.Exec: entering method")

	logrus.Info("Configuring SIGHUP Distribution cluster...")

	if err := k.prepare(); err != nil {
		logrus.WithError(err).Error("Kubernetes.Exec: error in prepare")
		return fmt.Errorf("error preparing kubernetes phase: %w", err)
	}

	if k.dryRun {
		logrus.Info("Kubernetes cluster created successfully (dry-run mode)")
		return nil
	}

	if k.upgradeNode != "" {
		logrus.WithField("upgradeNode", k.upgradeNode).Info("Kubernetes.Exec: upgrading specific node")
		if _, err := k.ansibleRunner.Playbook("56.upgrade-worker-nodes.yml", "--limit", k.upgradeNode); err != nil {
			logrus.WithError(err).Error("Kubernetes.Exec: error upgrading node")
			return fmt.Errorf("error upgrading node %s: %w", k.upgradeNode, err)
		}
		return nil
	}

	logrus.Info("Kubernetes.Exec: calling preKubernetes")
	if err := k.preKubernetes(startFrom, upgradeState); err != nil {
		logrus.WithError(err).Error("Kubernetes.Exec: error in preKubernetes")
		return fmt.Errorf("error running pre-kubernetes phase: %w", err)
	}

	logrus.Info("Kubernetes.Exec: calling coreKubernetes")
	if err := k.coreKubernetes(startFrom, upgradeState); err != nil {
		logrus.WithError(err).Error("Kubernetes.Exec: error in coreKubernetes")
		return fmt.Errorf("error running core kubernetes phase: %w", err)
	}

	logrus.Info("Kubernetes.Exec: calling postKubernetes")
	if err := k.postKubernetes(upgradeState); err != nil {
		logrus.WithError(err).Error("Kubernetes.Exec: error in postKubernetes")
		return fmt.Errorf("error running post-kubernetes phase: %w", err)
	}

	logrus.Info("Kubernetes cluster created successfully")
	return nil
}

func (k *Kubernetes) prepare() error {
	logrus.Debug("[DEBUG] prepare: entering method")

	if err := k.CreateRootFolder(); err != nil {
		logrus.WithError(err).Error("prepare: error creating kubernetes phase folder")
		return fmt.Errorf("error creating kubernetes phase folder: %w", err)
	}

	logrus.Debug("[DEBUG] prepare: creating furyctl merger")
	furyctlMerger, err := k.CreateFuryctlMerger(
		k.paths.DistroPath,
		k.paths.ConfigPath,
		"kfd-v1alpha2",
		"onpremises",
	)
	if err != nil {
		logrus.WithError(err).Error("prepare: error creating furyctl merger")
		return fmt.Errorf("error creating furyctl merger: %w", err)
	}

	logrus.Debug("[DEBUG] prepare: creating template config")
	mCfg, err := template.NewConfigWithoutData(furyctlMerger, []string{})
	if err != nil {
		logrus.WithError(err).Error("prepare: error creating template config")
		return fmt.Errorf("error creating template config: %w", err)
	}

	k.CopyPathsToConfig(&mCfg)

	mCfg.Data["kubernetes"] = map[any]any{
		"version": k.kfdManifest.Kubernetes.OnPremises.Version,
	}

	mCfg.Data["options"]["skipPodsRunningCheck"] = cluster.IsForceEnabledForFeature(
		k.force,
		cluster.ForceFeaturePodsRunningCheck,
	)
	mCfg.Data["options"]["podRunningTimeout"] = k.podRunningTimeout / FromSecondsToHalfMinuteRetries

	logrus.Debug("[DEBUG] prepare: copying from template")
	if err := k.CopyFromTemplate(
		mCfg,
		"kubernetes",
		path.Join(k.paths.DistroPath, "templates", cluster.OperationPhaseKubernetes, "onpremises"),
		k.Path,
		k.paths.ConfigPath,
	); err != nil {
		logrus.WithError(err).Error("prepare: error copying from template")
		return fmt.Errorf("error copying from template: %w", err)
	}

	if k.dryRun {
		logrus.Debug("[DEBUG] prepare: dry run mode, skipping host check")
		return nil
	}

	// Check hosts connection.
	logrus.Info("Checking that the hosts are reachable...")

	if _, err := k.ansibleRunner.Exec("all", "-m", "ping"); err != nil {
		logrus.WithError(err).Error("prepare: error checking hosts")
		return fmt.Errorf("error checking hosts: %w", err)
	}

	logrus.Debug("[DEBUG] prepare: method completed successfully")
	return nil
}

func (k *Kubernetes) preKubernetes(
	startFrom string,
	upgradeState *upgrade.State,
) error {
	logrus.WithFields(logrus.Fields{
		"startFrom":    startFrom,
		"upgradeState": upgradeState,
		"k.upgrade":    k.upgrade,
		"k.Path":       k.Path,
	}).Debug("[DEBUG] preKubernetes: entering method")

	if upgradeState == nil {
		logrus.Error("preKubernetes: upgradeState is nil")
		return fmt.Errorf("upgradeState is nil")
	}

	logrus.WithFields(logrus.Fields{
		"upgradeState.Phases": upgradeState.Phases,
	}).Debug("[DEBUG] preKubernetes: upgradeState.Phases value")

	logrus.WithFields(logrus.Fields{
		"upgradeState.Phases.PreKubernetes": upgradeState.Phases.PreKubernetes,
	}).Debug("[DEBUG] preKubernetes: upgradeState.Phases.PreKubernetes value")

	if upgradeState.Phases.PreKubernetes == nil {
		logrus.Error("preKubernetes: upgradeState.Phases.PreKubernetes is nil")
		return fmt.Errorf("upgradeState.Phases.PreKubernetes is nil")
	}

	if k.upgrade == nil {
		logrus.Error("preKubernetes: k.upgrade is nil")
		return fmt.Errorf("k.upgrade is nil")
	}

	if k.Path == "" {
		logrus.Error("preKubernetes: k.Path is empty")
		return fmt.Errorf("k.Path is empty")
	}

	if startFrom == "" || startFrom == cluster.OperationSubPhasePreKubernetes {
		logrus.Info("preKubernetes: running upgrade script")

		// Run upgrade script if needed.
		logrus.WithFields(logrus.Fields{
			"k.Path": k.Path,
			"phase":  "pre-kubernetes",
		}).Debug("[DEBUG] preKubernetes: calling k.upgrade.Exec")

		if err := k.upgrade.Exec(k.Path, "pre-kubernetes"); err != nil {
			logrus.WithError(err).Error("preKubernetes: error running upgrade")
			upgradeState.Phases.PreKubernetes.Status = upgrade.PhaseStatusFailed

			return fmt.Errorf("error running upgrade: %w", err)
		}

		if k.upgrade.Enabled {
			logrus.Info("preKubernetes: upgrade enabled, setting status to success")
			upgradeState.Phases.PreKubernetes.Status = upgrade.PhaseStatusSuccess
		} else {
			logrus.Debug("[DEBUG] preKubernetes: upgrade not enabled")
		}
	} else {
		logrus.WithField("startFrom", startFrom).Debug("[DEBUG] preKubernetes: skipping due to startFrom value")
	}

	logrus.Debug("[DEBUG] preKubernetes: method completed successfully")
	return nil
}

func (k *Kubernetes) coreKubernetes(
	startFrom string,
	upgradeState *upgrade.State,
) error {
	logrus.WithFields(logrus.Fields{
		"startFrom":         startFrom,
		"upgradeState":      upgradeState,
		"k.upgrade":         k.upgrade,
		"k.upgrade.Enabled": k.upgrade != nil && k.upgrade.Enabled,
	}).Debug("[DEBUG] coreKubernetes: entering method")

	if upgradeState == nil {
		logrus.Error("coreKubernetes: upgradeState is nil")
		return fmt.Errorf("upgradeState is nil")
	}

	if startFrom != cluster.OperationSubPhasePostKubernetes {
		logrus.Info("Applying cluster configuration...")

		// Apply create playbook.
		if !k.upgrade.Enabled {
			logrus.Info("coreKubernetes: applying create playbook")
			if _, err := k.ansibleRunner.Playbook("create-playbook.yaml"); err != nil {
				logrus.WithError(err).Error("coreKubernetes: error applying playbook")
				return fmt.Errorf("error applying playbook: %w", err)
			}
		} else {
			logrus.Info("coreKubernetes: upgrade enabled, setting kubernetes status to success")
			upgradeState.Phases.Kubernetes.Status = upgrade.PhaseStatusSuccess
		}

		logrus.Info("coreKubernetes: setting kubeconfig environment")
		if err := kubex.SetConfigEnv(path.Join(k.OperationPhase.Path, "admin.conf")); err != nil {
			logrus.WithError(err).Error("coreKubernetes: error setting kubeconfig env")
			return fmt.Errorf("error setting kubeconfig env: %w", err)
		}

		logrus.Info("coreKubernetes: copying kubeconfig to work dir")
		if err := kubex.CopyToWorkDir(path.Join(k.OperationPhase.Path, "admin.conf"), "kubeconfig"); err != nil {
			logrus.WithError(err).Error("coreKubernetes: error copying kubeconfig")
			return fmt.Errorf("error copying kubeconfig: %w", err)
		}

		if k.furyctlConf.Spec.Kubernetes.Advanced != nil && k.furyctlConf.Spec.Kubernetes.Advanced.Users != nil {
			logrus.WithField("users", k.furyctlConf.Spec.Kubernetes.Advanced.Users.Names).Info("coreKubernetes: copying user kubeconfigs")
			for _, username := range k.furyctlConf.Spec.Kubernetes.Advanced.Users.Names {
				if err := kubex.CopyToWorkDir(
					path.Join(
						k.OperationPhase.Path,
						username+".kubeconfig",
					),
					username+".kubeconfig",
				); err != nil {
					logrus.WithError(err).WithField("username", username).Error("coreKubernetes: error copying user kubeconfig")
					return fmt.Errorf("error copying %s.kubeconfig: %w", username, err)
				}
			}
		}
	} else {
		logrus.WithField("startFrom", startFrom).Debug("[DEBUG] coreKubernetes: skipping due to startFrom value")
	}

	logrus.Debug("[DEBUG] coreKubernetes: method completed successfully")
	return nil
}

func (k *Kubernetes) postKubernetes(
	upgradeState *upgrade.State,
) error {
	logrus.WithFields(logrus.Fields{
		"upgradeState":      upgradeState,
		"k.upgrade":         k.upgrade,
		"k.upgrade.Enabled": k.upgrade != nil && k.upgrade.Enabled,
	}).Debug("[DEBUG] postKubernetes: entering method")

	if upgradeState == nil {
		logrus.Error("postKubernetes: upgradeState is nil")
		return fmt.Errorf("upgradeState is nil")
	}

	if upgradeState.Phases.PostKubernetes == nil {
		logrus.Error("postKubernetes: upgradeState.Phases.PostKubernetes is nil")
		return fmt.Errorf("upgradeState.Phases.PostKubernetes is nil")
	}

	logrus.Info("postKubernetes: running upgrade script")
	if err := k.upgrade.Exec(k.Path, "post-kubernetes"); err != nil {
		logrus.WithError(err).Error("postKubernetes: error running upgrade")
		upgradeState.Phases.PostKubernetes.Status = upgrade.PhaseStatusFailed

		return fmt.Errorf("error running upgrade: %w", err)
	}

	if k.upgrade.Enabled {
		logrus.Info("postKubernetes: upgrade enabled, setting status to success")
		upgradeState.Phases.PostKubernetes.Status = upgrade.PhaseStatusSuccess
	} else {
		logrus.Debug("[DEBUG] postKubernetes: upgrade not enabled")
	}

	logrus.Debug("[DEBUG] postKubernetes: method completed successfully")
	return nil
}

func (k *Kubernetes) SetUpgrade(upgradeEnabled bool) {
	logrus.WithFields(logrus.Fields{
		"upgradeEnabled": upgradeEnabled,
		"k.upgrade":      k.upgrade,
	}).Debug("[DEBUG] SetUpgrade: setting upgrade enabled")

	if k.upgrade == nil {
		logrus.Error("SetUpgrade: k.upgrade is nil, cannot set upgrade enabled")
		return
	}

	k.upgrade.Enabled = upgradeEnabled

	logrus.WithField("k.upgrade.Enabled", k.upgrade.Enabled).Debug("[DEBUG] SetUpgrade: upgrade enabled set")
}

func NewKubernetes(
	furyctlConf public.OnpremisesKfdV1Alpha2,
	kfdManifest config.KFD,
	paths cluster.CreatorPaths,
	dryRun bool,
	upgr *upgrade.Upgrade,
	upgradeNode string,
	force []string,
	podRunningTimeout int,
) *Kubernetes {
	logrus.WithFields(logrus.Fields{
		"dryRun":            dryRun,
		"upgradeNode":       upgradeNode,
		"upgr":              upgr,
		"podRunningTimeout": podRunningTimeout,
		"paths.WorkDir":     paths.WorkDir,
		"paths.BinPath":     paths.BinPath,
	}).Debug("[DEBUG] NewKubernetes: creating new Kubernetes instance")

	phase := cluster.NewOperationPhase(
		path.Join(paths.WorkDir, cluster.OperationPhaseKubernetes),
		kfdManifest.Tools,
		paths.BinPath,
	)

	logrus.WithField("phase.Path", phase.Path).Debug("[DEBUG] NewKubernetes: created operation phase")

	ansibleRunner := ansible.NewRunner(
		execx.NewStdExecutor(),
		ansible.Paths{
			Ansible:         "ansible",
			AnsiblePlaybook: "ansible-playbook",
			WorkDir:         phase.Path,
		},
	)

	logrus.Debug("[DEBUG] NewKubernetes: created ansible runner")

	kubernetes := &Kubernetes{
		OperationPhase:    phase,
		furyctlConf:       furyctlConf,
		kfdManifest:       kfdManifest,
		paths:             paths,
		dryRun:            dryRun,
		ansibleRunner:     ansibleRunner,
		upgrade:           upgr,
		upgradeNode:       upgradeNode,
		force:             force,
		podRunningTimeout: podRunningTimeout,
	}

	logrus.WithFields(logrus.Fields{
		"kubernetes.upgrade":        kubernetes.upgrade,
		"kubernetes.ansibleRunner":  kubernetes.ansibleRunner,
		"kubernetes.OperationPhase": kubernetes.OperationPhase,
	}).Debug("[DEBUG] NewKubernetes: created Kubernetes instance")

	return kubernetes
}
