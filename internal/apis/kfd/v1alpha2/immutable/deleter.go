// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package immutable

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/samber/lo"
	"github.com/sirupsen/logrus"

	"github.com/sighupio/furyctl/internal/apis/config"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/immutable/create"
	del "github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/immutable/delete"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/immutable/public"
	"github.com/sighupio/furyctl/internal/cluster"
)

var (
	ErrDeletePhaseNotSupported = errors.New(
		"the Immutable delete supports --phase plugins, distribution, kubernetes and infrastructure",
	)
	ErrAPIUnreachable = errors.New(
		"the Kubernetes API is not reachable, nothing was deleted: run the delete with --phase infrastructure " +
			"to erase the nodes without deleting the workloads first",
	)
	ErrDeleteNotSupported = errors.New("does not support Immutable delete")
	ErrPreflightFailed    = errors.New("the delete preflight failed, nothing was changed on any host")
	ErrDeleteStopped      = errors.New("stopped before erasing any disk, nothing was changed on any host")
	ErrWipeFailed         = errors.New(
		"erasing the install disks failed on some hosts: the ansible output above names them, " +
			"the other hosts are erased and restart",
	)

	// The phases of a delete without --phase, in reverse order of apply.
	//
	//nolint:gochecknoglobals // A constant list.
	deletePhases = []string{
		cluster.OperationPhasePlugins, cluster.OperationPhaseDistribution, cluster.OperationPhaseInfrastructure,
	}
)

const (
	// Shipped with the distribution preflight templates.
	deletePlaybook = "delete-playbook.yaml"
	hostsVar       = "furyctl_delete_hosts"

	// The plugins delete script came with the distribution delete script for the Immutable kind.
	pluginsDeleteTemplate = "delete.sh.tpl"

	// The plugins delete script deletes only the plugins that own a StorageClass with this flag, the others without it.
	storageClassProvidersFlag = "--storage-class-providers"

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

	// The tests replace these; nil means the real ones.
	newRunner       func() (playbookRunner, error)
	fetchKubeconfig func() error
	shell           del.ShellRunner
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

// DeleteWarning replaces the generic warning of the delete confirmation and names only the phases that run.
func (c *ClusterDeleter) DeleteWarning() string {
	phases, err := c.phases()
	if err != nil {
		return fmt.Sprintf("WARNING: %v. Nothing will run.", err)
	}

	if slices.Equal(phases, []string{cluster.OperationPhaseKubernetes}) {
		return "WARNING: The Immutable kind has no kubernetes delete: nothing will be deleted."
	}

	var parts []string

	if slices.Contains(phases, cluster.OperationPhasePlugins) {
		parts = append(parts, "You are about to DELETE the plugins of the Immutable cluster: the kapp apps "+
			"kfd-plugin-* and the helm releases of spec.plugins, with their custom resources and CRDs.")
	}

	if slices.Contains(phases, cluster.OperationPhaseDistribution) {
		parts = append(parts, "You are about to DELETE the distribution of the Immutable cluster: the kapp app kfd, "+
			"with its custom resources, CRDs and the PVCs of its StatefulSets.")
	}

	if slices.Contains(phases, cluster.OperationPhaseInfrastructure) {
		hosts := c.hosts()

		parts = append(parts, fmt.Sprintf(
			"You are about to ERASE every node of the Immutable cluster (%d hosts: %s).\n"+
				"The partition table of the install disk of every host is erased, and the host restarts. "+
				"Nothing of the cluster or of the install can be used again.\n"+
				"NOT removed: data on other disks, and resources outside the nodes (LoadBalancer IPs, DNS records, "+
				"external volumes). This is not a secure erase.\n"+
				"With an empty install disk the hosts boot from the network and wait there until furyctl apply "+
				"installs them again.",
			len(hosts), strings.Join(hosts, ", "),
		))
	}

	return "WARNING: " + strings.Join(parts, "\n") +
		"\nThis action is irreversible. Run with --dry-run first to check it without changing anything."
}

// Delete runs the selected phases in reverse order of apply and stops at the first error.
func (c *ClusterDeleter) Delete() error {
	phases, err := c.phases()
	if err != nil {
		return err
	}

	if slices.Equal(phases, []string{cluster.OperationPhaseKubernetes}) {
		logrus.Info("The Immutable kind has no kubernetes delete, nothing was deleted. " +
			"--phase infrastructure returns the nodes to their original state and erases all the OS content.")

		return nil
	}

	if err := c.checkScripts(phases); err != nil {
		return err
	}

	infra := slices.Contains(phases, cluster.OperationPhaseInfrastructure)

	var runner playbookRunner

	if infra {
		if runner, err = c.prepare(); err != nil {
			return err
		}

		if err := c.checkNodes(runner); err != nil {
			return err
		}
	}

	if err := c.deleteWorkloads(phases); err != nil {
		return err
	}

	if c.dryRun {
		logrus.Infof("Dry run completed for the phases %s, nothing was changed", strings.Join(phases, ", "))

		return nil
	}

	if !infra {
		logrus.Infof("Delete completed: %s", strings.Join(phases, ", "))

		return nil
	}

	return c.wipe(runner)
}

// OnDeleteStop keeps a signal from starting the wipe call, or waits for a running one, when a signal stops furyctl.
func (c *ClusterDeleter) OnDeleteStop() {
	if c.step.CompareAndSwap(stepIdle, stepStopped) || c.step.Load() != stepWiping {
		return
	}

	logrus.Warn("The wipe call does not stop, waiting for it to end...")
	<-c.wiped
}

// A workloadStep runs a delete script when its phase is selected.
type workloadStep struct {
	phase   string
	message string
	script  *del.Script
	args    []string
}

// deleteWorkloads runs the plugins and distribution delete scripts of the selected phases; a dry run only renders them.
func (c *ClusterDeleter) deleteWorkloads(phases []string) error {
	plugins := del.NewPlugins(c.kfdManifest, c.paths, c.dryRun, c.shell)

	// The StorageClass providers go after the distribution, whose PVCs they serve.
	steps := lo.Filter([]workloadStep{
		{cluster.OperationPhasePlugins, "Deleting the plugins that own no StorageClass...", plugins, nil},
		{
			cluster.OperationPhaseDistribution, "Deleting SIGHUP Distribution...",
			del.NewDistribution(c.kfdManifest, c.paths, c.dryRun, c.shell), nil,
		},
		{
			cluster.OperationPhasePlugins, "Deleting the plugins that own a StorageClass...",
			plugins,
			[]string{storageClassProvidersFlag},
		},
	}, func(s workloadStep, _ int) bool { return slices.Contains(phases, s.phase) })

	if len(steps) == 0 {
		return nil
	}

	if !c.dryRun {
		if err := c.kubeconfig(); err != nil {
			return fmt.Errorf("%w: %w", ErrAPIUnreachable, err)
		}
	}

	for _, s := range steps {
		logrus.Info(s.message)

		if err := s.script.Exec(s.args...); err != nil {
			return err //nolint:wrapcheck // del names the phase.
		}
	}

	return nil
}

// checkNodes runs the node checks of the delete playbook.
func (c *ClusterDeleter) checkNodes(runner playbookRunner) error {
	hosts := c.hosts()

	logrus.Infof("Checking the %d hosts: MAC address, Flatcar, install disk...", len(hosts))

	if _, err := runner.Playbook(playbookArgs(hosts, "preflight")...); err != nil {
		return fmt.Errorf("%w: %w", ErrPreflightFailed, err)
	}

	return nil
}

// wipe erases the install disk of every host with the distribution delete playbook.
func (c *ClusterDeleter) wipe(runner playbookRunner) error {
	hosts := c.hosts()

	c.wiped = make(chan struct{})
	if !c.step.CompareAndSwap(stepIdle, stepWiping) {
		return ErrDeleteStopped
	}

	logrus.Infof("Erasing the install disk of the %d hosts and restarting them...", len(hosts))

	// Unstoppable: a wipe call cut short erases only part of the cluster.
	out, err := runner.PlaybookUnstoppable(playbookArgs(hosts, "wipe")...)

	// The result of each host is printed before wiped is closed: a stop handler waiting on it exits furyctl.
	logrus.Info(strings.TrimSpace(string(out)))
	close(c.wiped)

	if err != nil {
		return fmt.Errorf("%w: %w", ErrWipeFailed, err)
	}

	logrus.Infof("Delete completed, the install disks of %d hosts are erased: %s", len(hosts), strings.Join(hosts, ", "))
	logrus.Info("The hosts boot from the network and wait there: run furyctl apply to install them again")

	return nil
}

// phases returns the phases of the run in order, or ErrDeletePhaseNotSupported.
func (c *ClusterDeleter) phases() ([]string, error) {
	switch c.phase {
	case cluster.OperationPhaseAll:
		return deletePhases, nil
	case cluster.OperationPhasePlugins, cluster.OperationPhaseDistribution, cluster.OperationPhaseKubernetes,
		cluster.OperationPhaseInfrastructure:
		return []string{c.phase}, nil
	default:
		return nil, fmt.Errorf("%w: --phase %s", ErrDeletePhaseNotSupported, c.phase)
	}
}

// checkScripts stops a plugins or distribution delete when the distribution has no plugins delete script: its
// distribution delete script has no Immutable branch either.
func (c *ClusterDeleter) checkScripts(phases []string) error {
	if !slices.Contains(phases, cluster.OperationPhasePlugins) &&
		!slices.Contains(phases, cluster.OperationPhaseDistribution) {
		return nil
	}

	template := filepath.Join("templates", cluster.OperationPhasePlugins, "scripts", pluginsDeleteTemplate)
	if _, err := os.Stat(filepath.Join(c.paths.DistroPath, template)); err != nil {
		return fmt.Errorf(
			"the distribution version %s %w of the plugins and the distribution: it does not ship %s, "+
				"use a distribution version that does, or run the delete with --phase infrastructure",
			c.kfdManifest.Version, ErrDeleteNotSupported, template,
		)
	}

	return nil
}

// prepare checks that the distribution ships the delete playbook and renders the preflight folder.
func (c *ClusterDeleter) prepare() (playbookRunner, error) {
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

	preflight := c.preflight()

	if err := preflight.Render(); err != nil {
		return nil, fmt.Errorf("error rendering the preflight templates: %w", err)
	}

	return preflight.AnsibleRunner(), nil
}

func (c *ClusterDeleter) preflight() *create.PreFlight {
	return create.NewPreFlight(
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
}

// kubeconfig points KUBECONFIG to the admin.conf of a control plane host and checks that the API answers.
func (c *ClusterDeleter) kubeconfig() error {
	if c.fetchKubeconfig != nil {
		return c.fetchKubeconfig()
	}

	_, err := c.preflight().FetchKubeconfig()

	return err //nolint:wrapcheck // Delete wraps it with ErrAPIUnreachable.
}
