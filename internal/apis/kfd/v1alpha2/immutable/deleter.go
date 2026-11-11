// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package immutable

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/samber/lo"
	"github.com/sirupsen/logrus"

	"github.com/sighupio/furyctl/internal/apis/config"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/immutable/create"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/immutable/public"
	"github.com/sighupio/furyctl/internal/cluster"
)

var (
	ErrDeletePhaseNotSupported = errors.New("the Immutable delete has no phases, the --phase flag is not supported")
	ErrDeleteNotSupported      = errors.New("does not support Immutable delete")
	ErrPreflightFailed         = errors.New("the delete preflight failed, nothing was changed on any host")
	ErrDeleteStopped           = errors.New("stopped before erasing any disk, nothing was changed on any host")
	ErrWipeFailed              = errors.New(
		"erasing the install disks failed on some hosts: the ansible output above names them, " +
			"the other hosts are erased and restart",
	)
)

const (
	// Shipped with the distribution preflight templates.
	deletePlaybook = "delete-playbook.yaml"
	hostsVar       = "furyctl_delete_hosts"

	stepIdle    int32 = 0
	stepStopped int32 = 1
	stepWiping  int32 = 2
)

// playbookRunner is ansible.Runner in the rendered preflight folder.
type playbookRunner interface {
	Playbook(params ...string) ([]byte, error)
	// PlaybookUnstoppable runs a playbook that a signal does not stop.
	PlaybookUnstoppable(params ...string) ([]byte, error)
}

type ClusterDeleter struct {
	paths       cluster.DeleterPaths
	furyctlConf public.ImmutableKfdV1Alpha2
	kfdManifest config.KFD
	phase       string
	dryRun      bool

	// A signal before the wipe call stops the delete, and during it waits for the call to end.
	step  atomic.Int32
	wiped chan struct{}

	// The tests replace newRunner; nil means the real one.
	newRunner func() (playbookRunner, error)
}

func (c *ClusterDeleter) SetProperties(props []cluster.DeleterProperty) {
	for _, prop := range props {
		c.SetProperty(prop.Name, prop.Value)
	}
}

func (c *ClusterDeleter) SetProperty(name string, value any) {
	switch strings.ToLower(name) {
	case cluster.DeleterPropertyConfigPath:
		cluster.SetPropertyValue(value, &c.paths.ConfigPath)
	case cluster.DeleterPropertyDistroPath:
		cluster.SetPropertyValue(value, &c.paths.DistroPath)
	case cluster.DeleterPropertyWorkDir:
		cluster.SetPropertyValue(value, &c.paths.WorkDir)
	case cluster.DeleterPropertyBinPath:
		cluster.SetPropertyValue(value, &c.paths.BinPath)
	case cluster.DeleterPropertyFuryctlConf:
		cluster.SetPropertyValue(value, &c.furyctlConf)
	case cluster.DeleterPropertyKfdManifest:
		cluster.SetPropertyValue(value, &c.kfdManifest)
	case cluster.DeleterPropertyPhase:
		cluster.SetPropertyValue(value, &c.phase)
	case cluster.DeleterPropertyDryRun:
		cluster.SetPropertyValue(value, &c.dryRun)
	default:
		logrus.Debugf("ignoring unknown property %q", name)
	}
}

// DeleteWarning replaces the generic warning of the delete confirmation.
func (c *ClusterDeleter) DeleteWarning() string {
	hosts := c.hosts()

	return fmt.Sprintf(
		"WARNING: You are about to ERASE every node of the Immutable cluster (%d hosts: %s).\n"+
			"The partition table of the install disk of every host is erased, and the host restarts. "+
			"Nothing of the cluster or of the install can be used again.\n"+
			"NOT removed: data on other disks, and resources outside the nodes (LoadBalancer IPs, DNS records, "+
			"external volumes). This is not a secure erase.\n"+
			"With an empty install disk the hosts boot from the network and wait there until furyctl apply "+
			"installs them again.\n"+
			"This action is irreversible. Run with --dry-run first to check every host without changing anything.",
		len(hosts), strings.Join(hosts, ", "),
	)
}

// Delete erases the install disk of every host with the distribution delete playbook.
func (c *ClusterDeleter) Delete() error {
	runner, err := c.prepare()
	if err != nil {
		return err
	}

	hosts := c.hosts()

	logrus.Infof("Checking the %d hosts: MAC address, Flatcar, install disk...", len(hosts))

	if _, err := runner.Playbook(playbookArgs(hosts, "preflight")...); err != nil {
		return fmt.Errorf("%w: %w", ErrPreflightFailed, err)
	}

	if c.dryRun {
		logrus.Infof("Dry run completed: the %d hosts can be erased, nothing was changed", len(hosts))

		return nil
	}

	c.wiped = make(chan struct{})
	if !c.step.CompareAndSwap(stepIdle, stepWiping) {
		return ErrDeleteStopped
	}

	logrus.Infof("Erasing the install disk of the %d hosts and restarting them...", len(hosts))

	// Unstoppable: a wipe call cut short erases only part of the cluster.
	out, err := runner.PlaybookUnstoppable(playbookArgs(hosts, "wipe")...)
	close(c.wiped)

	if err != nil {
		return fmt.Errorf("%w: %w\n%s", ErrWipeFailed, err, out)
	}

	logrus.Infof("Delete completed, the install disks of %d hosts are erased: %s", len(hosts), strings.Join(hosts, ", "))
	logrus.Info("The hosts boot from the network and wait there: run furyctl apply to install them again")

	return nil
}

// OnDeleteStop keeps a signal from starting the wipe call, or waits for a running one, when a signal stops furyctl.
func (c *ClusterDeleter) OnDeleteStop() {
	if c.step.CompareAndSwap(stepIdle, stepStopped) || c.step.Load() != stepWiping {
		return
	}

	logrus.Warn("The wipe call does not stop, waiting for it to end...")
	<-c.wiped
}

// prepare runs the guards that must fail before any playbook call.
func (c *ClusterDeleter) prepare() (playbookRunner, error) {
	if c.phase != cluster.OperationPhaseAll {
		return nil, fmt.Errorf("%w: --phase %s", ErrDeletePhaseNotSupported, c.phase)
	}

	// The distribution, not the rendered folder: an older render can still hold the playbook.
	template := filepath.Join(
		c.paths.DistroPath, "templates", cluster.OperationPhasePreFlight, "immutable", deletePlaybook,
	)
	if _, err := os.Stat(template); err != nil {
		return nil, fmt.Errorf(
			"the distribution version %s %w: it does not ship %s, use a distribution version that does",
			c.kfdManifest.Version, ErrDeleteNotSupported,
			filepath.Join("templates", cluster.OperationPhasePreFlight, "immutable", deletePlaybook),
		)
	}

	return c.runner()
}

// playbookArgs passes the hosts that the playbook guard compares with the inventory.
func playbookArgs(hosts []string, tags string) []string {
	// A JSON list: hostnames are DNS names, so Go quoting is valid JSON quoting.
	hostsJSON := "[" + strings.Join(lo.Map(hosts, func(h string, _ int) string { return strconv.Quote(h) }), ",") + "]"

	return []string{deletePlaybook, "-e", hostsVar + "=" + hostsJSON, "--tags", tags}
}

// hosts lists every machine of spec.infrastructure.nodes, as the preflight inventory does.
func (c *ClusterDeleter) hosts() []string {
	return lo.Map(
		c.furyctlConf.Spec.Infrastructure.Nodes,
		func(n public.SpecInfrastructureNode, _ int) string { return n.Hostname },
	)
}

func (c *ClusterDeleter) runner() (playbookRunner, error) {
	if c.newRunner != nil {
		return c.newRunner()
	}

	preflight := create.NewPreFlight(
		c.furyctlConf,
		c.kfdManifest,
		cluster.CreatorPaths{
			ConfigPath: c.paths.ConfigPath,
			WorkDir:    c.paths.WorkDir,
			DistroPath: c.paths.DistroPath,
			BinPath:    c.paths.BinPath,
		},
		false,
		nil,
		nil,
		cluster.OperationPhaseAll,
		cluster.OperationPhaseAll,
		false,
	)

	if err := preflight.Render(); err != nil {
		return nil, fmt.Errorf("error rendering the preflight templates: %w", err)
	}

	return preflight.AnsibleRunner(), nil
}
