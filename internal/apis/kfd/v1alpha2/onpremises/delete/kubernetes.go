// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//nolint:predeclared // We want to use delete as package name.
package delete

import (
	"fmt"
	"path"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/fury-distribution/pkg/apis/onpremises/v1alpha2/public"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/tool/ansible"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	iox "github.com/sighupio/furyctl/internal/x/io"
)

type Kubernetes struct {
	*cluster.OperationPhase
	furyctlConf   public.OnpremisesKfdV1Alpha2
	kfdManifest   config.KFD
	paths         cluster.DeleterPaths
	dryRun        bool
	ansibleRunner *ansible.Runner
}

func (k *Kubernetes) Exec() error {
	logrus.Info("Deleting Kubernetes Fury cluster...")
	logrus.Debug("Delete: running kubernetes phase...")

	err := iox.CheckDirIsEmpty(k.OperationPhase.Path)
	if err == nil {
		logrus.Info("Kubernetes Fury cluster already deleted")

		return nil
	}

	if k.dryRun {
		logrus.Info("Kubernetes cluster deleted successfully (dry-run mode)")

		return nil
	}

	// Check hosts connection.
	logrus.Info("Checking that the hosts are reachable...")

	if _, err := k.ansibleRunner.Exec("all", "-m", "ping"); err != nil {
		return fmt.Errorf("error checking hosts: %w", err)
	}

	logrus.Info("Running ansible playbook...")

	// Apply delete playbook.
	if _, err := k.ansibleRunner.Playbook("delete-playbook.yaml"); err != nil {
		return fmt.Errorf("error applying playbook: %w", err)
	}

	logrus.Info("Kubernetes cluster deleted successfully")

	return nil
}

func NewKubernetes(
	furyctlConf public.OnpremisesKfdV1Alpha2,
	kfdManifest config.KFD,
	paths cluster.DeleterPaths,
	dryRun bool,
) *Kubernetes {
	kubeDir := path.Join(paths.WorkDir, cluster.OperationPhaseKubernetes)

	phase := cluster.NewOperationPhase(kubeDir, kfdManifest.Tools, paths.BinPath)

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
	}
}
