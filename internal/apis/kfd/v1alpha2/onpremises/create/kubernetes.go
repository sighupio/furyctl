// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package create

import (
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/fury-distribution/pkg/apis/onpremises/v1alpha2/public"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/merge"
	"github.com/sighupio/furyctl/internal/template"
	"github.com/sighupio/furyctl/internal/tool/ansible"
	"github.com/sighupio/furyctl/internal/upgrade"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	kubex "github.com/sighupio/furyctl/internal/x/kube"
	yamlx "github.com/sighupio/furyctl/internal/x/yaml"
)

type Kubernetes struct {
	*cluster.OperationPhase
	furyctlConfPath string
	furyctlConf     public.OnpremisesKfdV1Alpha2
	kfdManifest     config.KFD
	paths           cluster.CreatorPaths
	dryRun          bool
	ansibleRunner   *ansible.Runner
	upgradeStore    upgrade.Storer
	upgrade         *upgrade.Upgrade
}

func (k *Kubernetes) Exec(startFrom string, upgradeState *upgrade.State) error {
	logrus.Info("Creating Kubernetes Fury cluster...")
	logrus.Debug("Create: running kubernetes phase...")

	if err := k.prepare(); err != nil {
		return fmt.Errorf("error preparing kubernetes phase: %w", err)
	}

	if k.dryRun {
		logrus.Info("Kubernetes cluster created successfully (dry-run mode)")

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
	if err := k.CreateFolder(); err != nil {
		return fmt.Errorf("error creating kubernetes phase folder: %w", err)
	}

	furyctlMerger, err := k.createFuryctlMerger()
	if err != nil {
		return err
	}

	mCfg, err := template.NewConfigWithoutData(furyctlMerger, []string{})
	if err != nil {
		return fmt.Errorf("error creating template config: %w", err)
	}

	k.CopyPathsToConfig(&mCfg)

	mCfg.Data["kubernetes"] = map[any]any{
		"version": k.kfdManifest.Kubernetes.OnPremises.Version,
	}

	// Generate playbook and hosts file.
	outYaml, err := yamlx.MarshalV2(mCfg)
	if err != nil {
		return fmt.Errorf("error marshaling template config: %w", err)
	}

	outDirPath1, err := os.MkdirTemp("", "furyctl-dist-")
	if err != nil {
		return fmt.Errorf("error creating temp dir: %w", err)
	}

	confPath := filepath.Join(outDirPath1, "config.yaml")

	logrus.Debugf("config path = %s", confPath)

	if err = os.WriteFile(confPath, outYaml, os.ModePerm); err != nil {
		return fmt.Errorf("error writing config file: %w", err)
	}

	templateModel, err := template.NewTemplateModel(
		path.Join(k.paths.DistroPath, "templates", cluster.OperationPhaseKubernetes, "onpremises"),
		path.Join(k.Path),
		confPath,
		outDirPath1,
		k.furyctlConfPath,
		".tpl",
		false,
		k.dryRun,
	)
	if err != nil {
		return fmt.Errorf("error creating template model: %w", err)
	}

	err = templateModel.Generate()
	if err != nil {
		return fmt.Errorf("error generating from template files: %w", err)
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

			if err := k.upgradeStore.Store(upgradeState); err != nil {
				return fmt.Errorf("error storing upgrade state: %w", err)
			}

			return fmt.Errorf("error running upgrade: %w", err)
		}

		if k.upgrade.Enabled {
			upgradeState.Phases.PreKubernetes.Status = upgrade.PhaseStatusSuccess

			if err := k.upgradeStore.Store(upgradeState); err != nil {
				return fmt.Errorf("error storing upgrade state: %w", err)
			}
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
		if _, err := k.ansibleRunner.Playbook("create-playbook.yaml"); err != nil {
			if k.upgrade.Enabled {
				upgradeState.Phases.Kubernetes.Status = upgrade.PhaseStatusFailed

				if err := k.upgradeStore.Store(upgradeState); err != nil {
					return fmt.Errorf("error storing upgrade state: %w", err)
				}
			}

			return fmt.Errorf("error applying playbook: %w", err)
		}

		if k.upgrade.Enabled {
			upgradeState.Phases.Kubernetes.Status = upgrade.PhaseStatusSuccess

			if err := k.upgradeStore.Store(upgradeState); err != nil {
				return fmt.Errorf("error storing upgrade state: %w", err)
			}
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

		if err := k.upgradeStore.Store(upgradeState); err != nil {
			return fmt.Errorf("error storing upgrade state: %w", err)
		}

		return fmt.Errorf("error running upgrade: %w", err)
	}

	if k.upgrade.Enabled {
		upgradeState.Phases.PostKubernetes.Status = upgrade.PhaseStatusSuccess

		if err := k.upgradeStore.Store(upgradeState); err != nil {
			return fmt.Errorf("error storing upgrade state: %w", err)
		}
	}

	return nil
}

//nolint:dupl // Remove duplicated code in the future.
func (k *Kubernetes) createFuryctlMerger() (*merge.Merger, error) {
	defaultsFilePath := path.Join(k.paths.DistroPath, "defaults", "onpremises-kfd-v1alpha2.yaml")

	defaultsFile, err := yamlx.FromFileV2[map[any]any](defaultsFilePath)
	if err != nil {
		return &merge.Merger{}, fmt.Errorf("%s - %w", defaultsFilePath, err)
	}

	furyctlConf, err := yamlx.FromFileV2[map[any]any](k.paths.ConfigPath)
	if err != nil {
		return &merge.Merger{}, fmt.Errorf("%s - %w", k.paths.ConfigPath, err)
	}

	merger := merge.NewMerger(
		merge.NewDefaultModel(defaultsFile, ".data"),
		merge.NewDefaultModel(furyctlConf, ".spec.distribution"),
	)

	_, err = merger.Merge()
	if err != nil {
		return nil, fmt.Errorf("error merging furyctl config: %w", err)
	}

	reverseMerger := merge.NewMerger(
		*merger.GetCustom(),
		*merger.GetBase(),
	)

	_, err = reverseMerger.Merge()
	if err != nil {
		return nil, fmt.Errorf("error merging furyctl config: %w", err)
	}

	return reverseMerger, nil
}

func NewKubernetes(
	furyctlConf public.OnpremisesKfdV1Alpha2,
	kfdManifest config.KFD,
	paths cluster.CreatorPaths,
	dryRun bool,
	upgr *upgrade.Upgrade,
) (*Kubernetes, error) {
	kubeDir := path.Join(paths.WorkDir, cluster.OperationPhaseKubernetes)

	phase, err := cluster.NewOperationPhase(kubeDir, kfdManifest.Tools, paths.BinPath)
	if err != nil {
		return nil, fmt.Errorf("error creating kubernetes phase: %w", err)
	}

	return &Kubernetes{
		OperationPhase:  phase,
		furyctlConf:     furyctlConf,
		kfdManifest:     kfdManifest,
		paths:           paths,
		dryRun:          dryRun,
		furyctlConfPath: paths.ConfigPath,
		ansibleRunner: ansible.NewRunner(
			execx.NewStdExecutor(),
			ansible.Paths{
				Ansible:         "ansible",
				AnsiblePlaybook: "ansible-playbook",
				WorkDir:         phase.Path,
			},
		),
		upgradeStore: upgrade.NewStateStore(
			paths.WorkDir,
			kfdManifest.Tools.Common.Kubectl.Version,
			paths.BinPath,
		),
		upgrade: upgr,
	}, nil
}
