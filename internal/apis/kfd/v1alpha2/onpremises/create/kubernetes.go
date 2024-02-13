// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package create

import (
	"fmt"
	"path"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/fury-distribution/pkg/apis/onpremises/v1alpha2/public"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/template"
	"github.com/sighupio/furyctl/internal/tool/ansible"
	"github.com/sighupio/furyctl/internal/upgrade"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	kubex "github.com/sighupio/furyctl/internal/x/kube"
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
	force             bool
	podRunningTimeout int
}

func (k *Kubernetes) Self() *cluster.OperationPhase {
	return k.OperationPhase
}

func (k *Kubernetes) Exec(startFrom string, upgradeState *upgrade.State) error {
	logrus.Info("Creating Kubernetes Fury cluster...")

	if err := k.prepare(); err != nil {
		return fmt.Errorf("error preparing kubernetes phase: %w", err)
	}

	if k.dryRun {
		logrus.Info("Kubernetes cluster created successfully (dry-run mode)")

		return nil
	}

	if k.upgradeNode != "" {
		if _, err := k.ansibleRunner.Playbook("56.upgrade-worker-nodes.yml", "--limit", k.upgradeNode); err != nil {
			return fmt.Errorf("error upgrading node %s: %w", k.upgradeNode, err)
		}

		return nil
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

	logrus.Info("Kubernetes cluster created successfully")

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
		"onpremises",
	)
	if err != nil {
		return fmt.Errorf("error creating furyctl merger: %w", err)
	}

	mCfg, err := template.NewConfigWithoutData(furyctlMerger, []string{})
	if err != nil {
		return fmt.Errorf("error creating template config: %w", err)
	}

	k.CopyPathsToConfig(&mCfg)

	mCfg.Data["options"] = map[any]any{
		"dryRun": k.dryRun,
	}
	mCfg.Data["kubernetes"] = map[any]any{
		"version":              k.kfdManifest.Kubernetes.OnPremises.Version,
		"skipPodsRunningCheck": k.force,
		"podRunningTimeout":    k.podRunningTimeout / FromSecondsToHalfMinuteRetries,
	}

	if err := k.CopyFromTemplate(
		mCfg,
		"kubernetes",
		path.Join(k.paths.DistroPath, "templates", cluster.OperationPhaseKubernetes, "onpremises"),
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
		}
	}

	return nil
}

func (k *Kubernetes) coreKubernetes(
	startFrom string,
	upgradeState *upgrade.State,
) error {
	if startFrom != cluster.OperationSubPhasePostKubernetes {
		logrus.Info("Running ansible playbook...")

		// Apply create playbook.
		if !k.upgrade.Enabled {
			if _, err := k.ansibleRunner.Playbook("create-playbook.yaml"); err != nil {
				return fmt.Errorf("error applying playbook: %w", err)
			}
		} else {
			upgradeState.Phases.Kubernetes.Status = upgrade.PhaseStatusSuccess
		}

		if err := kubex.SetConfigEnv(path.Join(k.OperationPhase.Path, "admin.conf")); err != nil {
			return fmt.Errorf("error setting kubeconfig env: %w", err)
		}

		if err := kubex.CopyToWorkDir(path.Join(k.OperationPhase.Path, "admin.conf"), "kubeconfig"); err != nil {
			return fmt.Errorf("error copying kubeconfig: %w", err)
		}

		if k.furyctlConf.Spec.Kubernetes.Advanced != nil && k.furyctlConf.Spec.Kubernetes.Advanced.Users != nil {
			for _, username := range k.furyctlConf.Spec.Kubernetes.Advanced.Users.Names {
				if err := kubex.CopyToWorkDir(
					path.Join(
						k.OperationPhase.Path,
						fmt.Sprintf("%s.kubeconfig", username),
					),
					fmt.Sprintf("%s.kubeconfig", username),
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

func NewKubernetes(
	furyctlConf public.OnpremisesKfdV1Alpha2,
	kfdManifest config.KFD,
	paths cluster.CreatorPaths,
	dryRun bool,
	upgr *upgrade.Upgrade,
	upgradeNode string,
	force bool,
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
			ansible.Paths{
				Ansible:         "ansible",
				AnsiblePlaybook: "ansible-playbook",
				WorkDir:         phase.Path,
			},
		),
		upgrade:           upgr,
		upgradeNode:       upgradeNode,
		force:             force,
		podRunningTimeout: podRunningTimeout,
	}
}
