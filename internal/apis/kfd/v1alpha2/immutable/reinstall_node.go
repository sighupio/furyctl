// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package immutable

import (
	"errors"
	"fmt"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/immutable/create"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/immutable/public"
	"github.com/sighupio/furyctl/internal/cluster"
)

var (
	ErrReinstallNodeUnsupported = errors.New("unsupported --reinstall-node host")
	errReinstallNodeChanges     = errors.New("--reinstall-node installs the host with the applied configuration")
)

// validateNodeFlags checks --upgrade-node and --reinstall-node before the preflight phase.
func (c *ClusterCreator) validateNodeFlags(startFrom string) error {
	if err := c.validateUpgradeNode(startFrom); err != nil {
		return err
	}

	return c.validateReinstallNode(startFrom)
}

// validateReinstallNode checks --reinstall-node before the preflight phase.
func (c *ClusterCreator) validateReinstallNode(startFrom string) error {
	if c.reinstallNode == "" {
		return nil
	}

	if role := c.furyctlConf.NodeRole(c.reinstallNode); role != public.NodeRoleWorker &&
		role != public.NodeRoleLoadBalancer {
		return fmt.Errorf(
			"%w: %s", ErrReinstallNodeUnsupported, nodeRoleProblem(c.reinstallNode, role, "worker and load balancer"),
		)
	}

	if c.phase != cluster.OperationPhaseAll || startFrom != StartFromFlagNotSet || len(c.postApplyPhases) > 0 {
		return fmt.Errorf(
			"%w: %q selects its own steps, so --phase, --start-from and --post-apply-phases cannot be used with it",
			ErrReinstallNodeUnsupported,
			c.reinstallNode,
		)
	}

	if err := create.CheckReinstallShipped(c.paths.DistroPath); err != nil {
		return fmt.Errorf("%w: the distribution version %s: %w", ErrReinstallNodeUnsupported, c.kfdManifest.Version, err)
	}

	return nil
}

// reinstall erases the host, installs it again from the network and joins it back.
func (c *ClusterCreator) reinstall(
	status *create.Status,
	infra *create.Infrastructure,
	kubernetes *create.Kubernetes,
) error {
	host := c.reinstallNode

	if !status.ClusterExists {
		return fmt.Errorf("%w: --reinstall-node needs a running cluster to join %s to", ErrClusterNotFound, host)
	}

	if len(status.Diffs) > 0 {
		return fmt.Errorf(
			"%w: the configuration changed, run 'furyctl apply' to apply the changes, then use --reinstall-node",
			errReinstallNodeChanges,
		)
	}

	if status.DiffSkipped {
		return fmt.Errorf(
			"%w: furyctl could not compare the configuration with the applied one, even with --force",
			errReinstallNodeChanges,
		)
	}

	if !c.dryRun {
		confirm, err := cluster.AskConfirmationWithMessage(
			cluster.IsForceEnabledForFeature(c.force, cluster.ForceFeatureAll),
			c.reinstallWarning(),
		)
		if err != nil {
			return fmt.Errorf("error while asking for confirmation: %w", err)
		}

		if !confirm {
			return ErrAbortedByUser
		}
	}

	if err := c.nodeDeleter().RemoveNode(); err != nil {
		return err
	}

	if c.dryRun {
		return nil
	}

	if err := infra.ReinstallNode(host); err != nil {
		return reinstallError(host, err)
	}

	if c.furyctlConf.NodeRole(host) == public.NodeRoleWorker {
		if err := kubernetes.JoinNode(host); err != nil {
			return reinstallError(host, err)
		}
	}

	logrus.Infof("%s is installed again", host)

	return nil
}

// reinstallError is an error after the erase: --reinstall-node cannot check the host anymore, apply installs it.
func reinstallError(host string, err error) error {
	return fmt.Errorf(
		"error while installing %s again: the install disk of %s is already erased, "+
			"run 'furyctl apply' to install it again: %w",
		host, host, err,
	)
}

func (c *ClusterCreator) reinstallWarning() string {
	steps := "its pods are evicted and its Node object is deleted, then the partition table of its install disk " +
		"is erased and the host restarts"
	if c.furyctlConf.NodeRole(c.reinstallNode) == public.NodeRoleLoadBalancer {
		steps = "the partition table of its install disk is erased and the host restarts: the other load balancers " +
			"keep the virtual IP, and with a single load balancer the API is not reachable until it is back"
	}

	return fmt.Sprintf(
		"\nWARNING: You are about to REINSTALL %s: %s. "+
			"furyctl then serves the boot assets, waits for %s to install from the network, and configures it again.\n"+
			"The data on its install disk is lost. Run with --dry-run first to check it without changing anything.",
		c.reinstallNode, steps, c.reinstallNode,
	)
}

// nodeDeleter gives the steps of delete node that remove the host before its install.
func (c *ClusterCreator) nodeDeleter() *ClusterDeleter {
	return &ClusterDeleter{
		paths: cluster.DeleterPaths{
			ConfigPath: c.paths.ConfigPath,
			WorkDir:    c.paths.WorkDir,
			DistroPath: c.paths.DistroPath,
			BinPath:    c.paths.BinPath,
		},
		furyctlConf: c.furyctlConf,
		kfdManifest: c.kfdManifest,
		dryRun:      c.dryRun,
		deleteNode:  c.reinstallNode,
	}
}
