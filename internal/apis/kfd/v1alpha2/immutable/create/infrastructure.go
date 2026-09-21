// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package create

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"

	"github.com/samber/lo"
	"github.com/sirupsen/logrus"

	"github.com/sighupio/furyctl/cmd/serve"
	"github.com/sighupio/furyctl/internal/apis/config"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/immutable/public"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/tool/ansible"
	"github.com/sighupio/furyctl/internal/upgrade"
	execx "github.com/sighupio/furyctl/internal/x/exec"
	templatex "github.com/sighupio/furyctl/pkg/template"
)

const loadBalancerPlaybook = "upgrade-load-balancers.yml"

// ErrLoadBalancerUpgradeUnsupported is returned when the distribution has no load
// balancer upgrade playbook and the user asked for one load balancer by name.
var ErrLoadBalancerUpgradeUnsupported = errors.New("this distribution version does not upgrade the load balancers")

// loadBalancerPlaybookShipped reports whether the distribution gives the load balancer
// upgrade playbook. A distribution released before this feature does not, and an upgrade
// between two of those versions then skips this work instead of stopping.
//
// The test reads the templates of the distribution, and not the rendered phase folder.
// A render writes the files of the distribution over that folder, but it removes nothing,
// so the folder keeps a playbook that an earlier run with another distribution rendered.
//
// The folder of the templates holds both plain and rendered files, thus the two names.
// Any error other than "the file is absent" stops the run, because a playbook that
// furyctl cannot read must not read as an older distribution.
func loadBalancerPlaybookShipped(templatesPath string) (bool, error) {
	for _, name := range []string{loadBalancerPlaybook + ".tpl", loadBalancerPlaybook} {
		if _, err := os.Stat(filepath.Join(templatesPath, name)); err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return false, fmt.Errorf("checking for %s: %w", name, err)
			}

			continue
		}

		return true, nil
	}

	return false, nil
}

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

		// Only a load balancer belongs to this phase. A worker is upgraded by the
		// kubernetes phase, so there is nothing to do here.
		if i.furyctlConf.NodeRole(i.upgradeNode) != public.NodeRoleLoadBalancer {
			logrus.Debugf("%s is not a load balancer, skipping the infrastructure phase", i.upgradeNode)

			return nil
		}

		// --upgrade-node and --upgrade are mutually exclusive, so there is no upgrade
		// state to record here. This mirrors the kubernetes phase.
		if err := i.renderAnsible(); err != nil {
			return err
		}

		hasPlaybook, err := loadBalancerPlaybookShipped(i.ansibleTemplatesPath())
		if err != nil {
			return err
		}

		if !hasPlaybook {
			return fmt.Errorf("%w: cannot upgrade %s", ErrLoadBalancerUpgradeUnsupported, i.upgradeNode)
		}

		return i.runLoadBalancerPlaybook(i.upgradeNode)
	}

	if i.upgrade.Enabled {
		logrus.Debug("running an upgrade, skipping nodes bootstrap...")

		if err := i.renderAnsible(); err != nil {
			return err
		}

		return i.upgradeLoadBalancers(upgradeState)
	}

	if err := i.BootstrapNodes(); err != nil {
		return fmt.Errorf("preparing for infrastructure phase failed: %w", err)
	}

	if err := i.renderAnsible(); err != nil {
		return err
	}

	// Struct to keep each node's bootstrap status.
	nodeStatus := lo.SliceToMap(
		i.furyctlConf.Spec.Infrastructure.Nodes,
		func(node public.SpecInfrastructureNode) (string, string) {
			return node.Hostname, serve.StatusPending
		},
	)

	// Serve the downloaded assets to the machines.
	ipxeServer, err := url.Parse(string(i.furyctlConf.Spec.Infrastructure.IpxeServer.Url))
	ipxeServerPort := ""

	if err != nil {
		return fmt.Errorf("failed to parse ipxe server URL: %w", err)
	}

	ipxeServerHost := lo.FromPtrOr(i.furyctlConf.Spec.Infrastructure.IpxeServer.BindAddress, ipxeServer.Hostname())

	if i.furyctlConf.Spec.Infrastructure.IpxeServer.BindPort != nil {
		ipxeServerPort = strconv.Itoa(*i.furyctlConf.Spec.Infrastructure.IpxeServer.BindPort)
	} else {
		ipxeServerPort = ipxeServer.Port()
	}

	if err := serve.Path(ipxeServerHost, ipxeServerPort, filepath.Join(i.Path, "server"), nodeStatus); err != nil {
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

// ansibleTemplatesPath gives the folder of the infrastructure ansible templates of the
// distribution.
func (i *Infrastructure) ansibleTemplatesPath() string {
	return filepath.Join(i.paths.DistroPath, "templates", "infrastructure", "immutable", "ansible")
}

// renderAnsible renders the infrastructure ansible templates into the phase folder.
// Both the create path and the upgrade path need them: the upgrade only reads the
// inventory and the playbooks, it never re-provisions a node.
func (i *Infrastructure) renderAnsible() error {
	furyctlMerger, err := i.CreateFuryctlMerger(
		i.paths.DistroPath,
		i.paths.ConfigPath,
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

	i.CopyPathsToConfig(&mCfg)

	mCfg.Data["kubernetes"] = map[any]any{
		"version": i.kfdManifest.Kubernetes.Immutable.Version,
	}

	// Inject the immutable.yaml version data. The infra hosts.yaml renders the pins
	// that apply.yaml reads.
	versionVars, err := VersionVarsForPhase(i.Path, i.kfdManifest.Kubernetes.Immutable.Version, i.KubectlPath)
	if err != nil {
		return fmt.Errorf("error building version vars: %w", err)
	}

	mCfg.Data["versions"] = versionVars

	targetPath := filepath.Join(i.Path, "ansible")

	if err := i.CopyFromTemplate(
		mCfg,
		"infrastructure",
		i.ansibleTemplatesPath(),
		targetPath,
		i.paths.ConfigPath,
	); err != nil {
		return fmt.Errorf("error copying from templates: %w", err)
	}

	return nil
}

// runLoadBalancerPlaybook upgrades the load balancers in place: OS, sysext images and
// configuration. The hosts are never re-provisioned, so the bootstrap and the iPXE
// server stay out of this path. An empty target upgrades every load balancer.
// The caller renders the phase first.
//
// The target goes in as an extra variable, not as --limit. The keepalived.conf template
// renders a unicast peer list from every load balancer. The playbook therefore gathers the
// facts of the whole group in a first play, and --limit also filters that play.
func (i *Infrastructure) runLoadBalancerPlaybook(target string) error {
	args := []string{loadBalancerPlaybook}
	if target != "" {
		args = append(args, "-e", "lb_upgrade_target="+target)

		logrus.Infof("Upgrading the load balancer %s...", target)
	} else {
		logrus.Info("Upgrading the load balancers...")
	}

	if _, err := i.ansibleRunner.Playbook(args...); err != nil {
		return fmt.Errorf("error upgrading the load balancers: %w", err)
	}

	return nil
}

// upgradeLoadBalancers upgrades every load balancer and records the result in the
// upgrade state. The caller renders the phase first.
//
// NOTE: This phase has no --start-from support and it keeps no per-host state, the same
// as the other phases today. An interrupted rollout starts again. Add both when the
// phase needs to continue a rollout that stopped.
func (i *Infrastructure) upgradeLoadBalancers(upgradeState *upgrade.State) error {
	if err := i.upgrade.Exec(i.Path, "pre-infrastructure"); err != nil {
		upgradeState.Phases.PreInfrastructure.Status = upgrade.PhaseStatusFailed

		return fmt.Errorf("error running upgrade: %w", err)
	}

	upgradeState.Phases.PreInfrastructure.Status = upgrade.PhaseStatusSuccess

	hasPlaybook, err := loadBalancerPlaybookShipped(i.ansibleTemplatesPath())
	if err != nil {
		upgradeState.Phases.Infrastructure.Status = upgrade.PhaseStatusFailed

		return err
	}

	if !hasPlaybook {
		logrus.Warn("This distribution version does not upgrade the load balancers, skipping them...")
	} else if err := i.runLoadBalancerPlaybook(""); err != nil {
		upgradeState.Phases.Infrastructure.Status = upgrade.PhaseStatusFailed

		return err
	}

	upgradeState.Phases.Infrastructure.Status = upgrade.PhaseStatusSuccess

	if err := i.upgrade.Exec(i.Path, "post-infrastructure"); err != nil {
		upgradeState.Phases.PostInfrastructure.Status = upgrade.PhaseStatusFailed

		return fmt.Errorf("error running upgrade: %w", err)
	}

	upgradeState.Phases.PostInfrastructure.Status = upgrade.PhaseStatusSuccess

	if hasPlaybook {
		logrus.Info("Load balancers upgraded successfully")
	}

	return nil
}
