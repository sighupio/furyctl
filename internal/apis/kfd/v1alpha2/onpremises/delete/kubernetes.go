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

	// Check hosts connection.
	logrus.Info("Checking that the hosts are reachable...")

	if _, err := k.ansibleRunner.Exec("all", "-m", "ping"); err != nil {
		return fmt.Errorf("error checking hosts: %w", err)
	}

	if k.dryRun {
		logrus.Info("Running ansible playbook with check enabled (dry-run)...")

		// Apply delete playbook with --check.
		if _, err := k.ansibleRunner.Playbook("delete-playbook.yaml", "--check"); err != nil {
			return fmt.Errorf("error applying playbook: %w", err)
		}

		return nil
	}

	logrus.Info("Running ansible playbook...")

	// Apply delete playbook.
	if _, err := k.ansibleRunner.Playbook("delete-playbook.yaml"); err != nil {
		return fmt.Errorf("error applying playbook: %w", err)
	}

	return nil
}

func NewKubernetes(
	furyctlConf public.OnpremisesKfdV1Alpha2,
	kfdManifest config.KFD,
	paths cluster.DeleterPaths,
	dryRun bool,
) (*Kubernetes, error) {
	kubeDir := path.Join(paths.WorkDir, cluster.OperationPhaseKubernetes)

	phase, err := cluster.NewOperationPhase(kubeDir, kfdManifest.Tools, paths.BinPath)
	if err != nil {
		return nil, fmt.Errorf("error creating kubernetes phase: %w", err)
	}

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
	}, nil
}
