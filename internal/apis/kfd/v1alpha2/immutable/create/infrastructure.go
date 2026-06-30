// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package create

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strconv"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/fury-distribution/pkg/apis/config"
	"github.com/sighupio/fury-distribution/pkg/apis/immutable/v1alpha2/public"
	"github.com/sighupio/furyctl/cmd/serve"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/tool/ansible"
	"github.com/sighupio/furyctl/internal/upgrade"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	"github.com/sighupio/furyctl/pkg/template"
)

// Infrastructure wraps the common infrastructure phase.
type Infrastructure struct {
	*cluster.OperationPhase
	paths         cluster.CreatorPaths
	upgrade       *upgrade.Upgrade
	kfdManifest   config.KFD
	furyctlConf   public.ImmutableKfdV1Alpha2
	dryRun        bool
	ansibleRunner *ansible.Runner
	force         []string
}

// NewInfrastructure creates a new Infrastructure phase.
func NewInfrastructure(
	phase *cluster.OperationPhase,
	configPath string,
	distroPath string,
	upgr *upgrade.Upgrade,
	furyctlConf public.ImmutableKfdV1Alpha2,
	kfdManifest config.KFD,
	paths cluster.CreatorPaths,
	dryRun bool,
	force []string,
) *Infrastructure {
	return &Infrastructure{
		OperationPhase: phase,
		paths: cluster.CreatorPaths{
			ConfigPath: configPath,
			DistroPath: distroPath,
		},
		upgrade:     upgr,
		furyctlConf: furyctlConf,
		kfdManifest: kfdManifest,
		dryRun:      dryRun,
		ansibleRunner: ansible.NewRunner(
			execx.NewStdExecutor(),
			ansible.Paths{
				Ansible:         "ansible",
				AnsiblePlaybook: "ansible-playbook",
				WorkDir:         filepath.Join(phase.Path, "ansible"),
			},
		),
		force: force,
	}
}

// Exec executes the infrastructure phase.
func (i *Infrastructure) Exec(_ string, upgradeState *upgrade.State) error {
	if err := i.BootstrapNodes(); err != nil {
		return fmt.Errorf("preparing for infrastructure phase failed: %w", err)
	}

	furyctlMerger, err := i.CreateFuryctlMerger(
		i.paths.DistroPath,
		i.paths.ConfigPath,
		"kfd-v1alpha2",
		"immutable",
	)
	if err != nil {
		return fmt.Errorf("error creating furyctl merger: %w", err)
	}

	mCfg, err := template.NewConfigWithoutData(furyctlMerger, []string{})
	if err != nil {
		return fmt.Errorf("error creating template config: %w", err)
	}

	i.CopyPathsToConfig(&mCfg)

	mCfg.Data["kubernetes"] = map[any]any{
		"version": normalizeVersion(i.kfdManifest.Kubernetes.Immutable.Version),
	}

	sourcePath := filepath.Join(
		i.paths.DistroPath,
		"templates",
		"infrastructure",
		"immutable",
		"ansible",
	)
	targetPath := filepath.Join(i.Path, "ansible")

	if err := i.CopyFromTemplate(
		mCfg,
		"infrastructure",
		sourcePath,
		targetPath,
		i.paths.ConfigPath,
	); err != nil {
		return fmt.Errorf("error copying from templates: %w", err)
	}

	// Render the version vars file into the infrastructure ansible workdir too: the install
	// playbook (apply.yaml) configures containerd on load balancers and nodes and reads
	// containerd_sandbox_image (and the other version values) from this single source — the same
	// file the kubernetes phase renders.
	immutableAssets, err := i.getImmutableAssets()
	if err != nil {
		return fmt.Errorf("error selecting immutable assets: %w", err)
	}

	versionVarsTpl := filepath.Join(
		i.paths.DistroPath, "templates", cluster.OperationPhaseKubernetes, "immutable", "group_vars", "all.yml.tpl",
	)

	if err := renderVersionVarsFile(
		targetPath, normalizeVersion(i.kfdManifest.Kubernetes.Immutable.Version), i.KubectlPath,
		versionVarsTpl, immutableAssets,
	); err != nil {
		return fmt.Errorf("error rendering version vars file: %w", err)
	}

	// Struct to keep each node's bootstrap status.
	nodeStatus := make(map[string]string, len(i.furyctlConf.Spec.Infrastructure.Nodes))
	for _, node := range i.furyctlConf.Spec.Infrastructure.Nodes {
		nodeStatus[node.Hostname] = "pending"
	}

	// Serve the downloaded assets to the machines.
	ipxeServer, err := url.Parse(string(i.furyctlConf.Spec.Infrastructure.IpxeServer.Url))
	ipxeServerHost := ""
	ipxeServerPort := ""

	if err != nil {
		return fmt.Errorf("failed to parse ipxe server URL: %w", err)
	}

	if i.furyctlConf.Spec.Infrastructure.IpxeServer.BindAddress != nil {
		ipxeServerHost = *i.furyctlConf.Spec.Infrastructure.IpxeServer.BindAddress
	} else {
		ipxeServerHost = ipxeServer.Hostname()
	}

	if i.furyctlConf.Spec.Infrastructure.IpxeServer.BindPort != nil {
		ipxeServerPort = strconv.Itoa(*i.furyctlConf.Spec.Infrastructure.IpxeServer.BindPort)
	} else {
		ipxeServerPort = ipxeServer.Port()
	}

	if err := serve.Path(ipxeServerHost, ipxeServerPort, filepath.Join(i.Path, "server"), &nodeStatus); err != nil {
		return fmt.Errorf("serving assets failed: %w", err)
	}

	logrus.Info("Applying nodes configuration...")

	// Run apply playbook.
	if !i.upgrade.Enabled {
		if _, err := i.ansibleRunner.Playbook("apply.yaml"); err != nil {
			return fmt.Errorf("error applying playbook: %w", err)
		}
	} else {
		upgradeState.Phases.Kubernetes.Status = upgrade.PhaseStatusSuccess
	}

	return nil
}

// Self returns the operation phase.
func (i *Infrastructure) Self() *cluster.OperationPhase {
	return i.OperationPhase
}

func (i *Infrastructure) SetUpgrade(upgradeEnabled bool) {
	i.upgrade.Enabled = upgradeEnabled
}
