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

	"github.com/sighupio/furyctl/cmd/serve"
	"github.com/sighupio/furyctl/internal/apis/config"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/immutable/public"
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
	upgradeNode   string
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
	upgradeNode string,
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
		upgradeNode: upgradeNode,
		furyctlConf: furyctlConf,
		kfdManifest: kfdManifest,
		dryRun:      dryRun,
		ansibleRunner: ansible.NewRunner(
			execx.NewStdExecutor(),
			ansible.PathsForVersion(
				paths.BinPath,
				kfdManifest.Tools.Immutable.Ansible.Version,
				filepath.Join(phase.Path, "ansible"),
			),
		),
		force: force,
	}
}

// Exec executes the infrastructure phase.
func (i *Infrastructure) Exec(_ string, upgradeState *upgrade.State) error {
	if i.dryRun {
		logrus.Info("Infrastructure configured successfully (dry-run mode)")

		return nil
	}

	if i.upgradeNode != "" {
		logrus.Debug("node upgrade requested, skipping nodes bootstrap...")
		logrus.Warn("Upgrade for Infrastructure phase (load balancers) not implemented yet...")

		return nil
	}

	if i.upgrade.Enabled {
		logrus.Debug("running an upgrade, skipping nodes bootstrap...")
		logrus.Warn("Upgrade for Infrastructure phase not implemented yet")

		upgradeState.Phases.Infrastructure.Status = upgrade.PhaseStatusSuccess

		return nil
	}

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
		"version": i.kfdManifest.Kubernetes.Immutable.Version,
	}

	// Inject the immutable.yaml version data; the infra hosts.yaml renders containerd_sandbox_tag,
	// kubernetes_image_registry and the haproxy image/tag inline, consumed by the apply.yaml roles.
	immutableAssets, err := selectImmutableAssets(i.Path, i.kfdManifest.Kubernetes.Immutable.Version)
	if err != nil {
		return fmt.Errorf("error selecting immutable assets: %w", err)
	}

	versionVars := map[any]any{}
	for name, value := range buildVersionVars(i.kfdManifest.Kubernetes.Immutable.Version, i.KubectlPath, immutableAssets) {
		versionVars[name] = value
	}

	mCfg.Data["versions"] = versionVars

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
	if _, err := i.ansibleRunner.Playbook("apply.yaml"); err != nil {
		return fmt.Errorf("error applying playbook: %w", err)
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
