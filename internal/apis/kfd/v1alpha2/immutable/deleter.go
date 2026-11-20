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

	"github.com/samber/lo"
	"github.com/sirupsen/logrus"

	"github.com/sighupio/furyctl/internal/apis/config"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/immutable/create"
	del "github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/immutable/delete"
	"github.com/sighupio/furyctl/internal/apis/kfd/v1alpha2/immutable/public"
	"github.com/sighupio/furyctl/internal/cluster"
	"github.com/sighupio/furyctl/internal/tool/ansible"
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
	ErrWipeFailed         = errors.New("erasing the install disks failed on some hosts: " + wipeRecovery)
	ErrNodeUnsupported    = errors.New("unsupported host")
	ErrNodeWipeFailed     = errors.New(
		"erasing the install disk failed, the ansible output above says why: no other host was changed",
	)
	ErrDrainFailed = errors.New(
		"removing the worker from Kubernetes failed, its install disk was not erased: uncordon it with kubectl " +
			"to give it back its pods",
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
	// The single-host mode of the delete playbook.
	targetVar  = "furyctl_delete_target"
	kubectlVar = "kubectl_bin"

	// The plugins delete script came with the distribution delete script for the Immutable kind.
	pluginsDeleteTemplate = "delete.sh.tpl"

	// The plugins delete script deletes only the plugins that own a StorageClass with this flag, the others without it.
	storageClassProvidersFlag = "--storage-class-providers"

	// Erasing an install disk again changes nothing: the recovery does not need to know which hosts were erased.
	wipeRecovery = "erase by hand the install disk of every host that you can still reach (wipefs -af <install disk>, " +
		"then reboot; it changes nothing on a host that is already erased), then run furyctl apply to install " +
		"the cluster again"
)

type ClusterDeleter struct {
	paths       cluster.DeleterPaths
	furyctlConf public.ImmutableKfdV1Alpha2
	kfdManifest config.KFD
	phase       string
	dryRun      bool
	// The worker of delete node, or the host of apply --reinstall-node.
	deleteNode string

	// The preflight folder, rendered once for the node checks and the kubeconfig.
	pf *create.PreFlight
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
	case cluster.DeleterPropertyDeleteNode:
		cluster.SetPropertyValue(value, &c.deleteNode)
	default:
		logrus.Debugf("ignoring unknown property %q", name)
	}
}

// DeleteWarning replaces the generic warning of the delete confirmation and names only the phases that run.
func (c *ClusterDeleter) DeleteWarning() string {
	if c.deleteNode != "" {
		return fmt.Sprintf(
			"WARNING: You are about to DELETE the worker %s of the Immutable cluster.\n"+
				"Its pods are evicted, its Node object is deleted, the partition table of its install disk is erased, "+
				"and the host restarts. It boots from the network and waits there.\n"+
				"Then remove the host from the configuration file and run furyctl apply.\n"+
				"This action is irreversible. Run with --dry-run first to check it without changing anything.",
			c.deleteNode,
		)
	}

	phases, err := c.phases()
	if err != nil {
		return fmt.Sprintf("WARNING: %v. Nothing will run.", err)
	}

	if slices.Equal(phases, []string{cluster.OperationPhaseKubernetes}) {
		return "WARNING: The Immutable kind has no kubernetes delete: nothing will be deleted."
	}

	var actions []string

	if slices.Contains(phases, cluster.OperationPhasePlugins) {
		actions = append(actions, "- DELETE all the plugins defined in spec.plugins, with their custom resources and CRDs.")
	}

	if slices.Contains(phases, cluster.OperationPhaseDistribution) {
		actions = append(actions, "- DELETE the distribution modules with its custom resources, CRDs and the PVCs "+
			"of its StatefulSets.")
	}

	infra := slices.Contains(phases, cluster.OperationPhaseInfrastructure)
	if infra {
		hosts := c.hosts()

		actions = append(actions, fmt.Sprintf(
			"- ERASE every node of the Immutable cluster (%d hosts: %s). The partition table of the install disk of "+
				"every host is erased, and the host restarts. Nothing of the cluster or of the install can be used again.",
			len(hosts), strings.Join(hosts, ", "),
		))
	}

	warning := "WARNING: You are about to perform the following unrecoverable actions:\n\n" + strings.Join(actions, "\n")

	if infra {
		warning += "\n\nNOT removed: data on other disks, and resources outside the nodes (LoadBalancer IPs, " +
			"DNS records, external volumes). This is not a secure erase."
	}

	if !c.dryRun {
		warning += "\n\nRun with --dry-run first to check it without changing anything."
	}

	return warning
}

// Delete runs the selected phases in reverse order of apply and stops at the first error.
func (c *ClusterDeleter) Delete() error {
	if c.deleteNode != "" {
		return c.deleteWorker()
	}

	phases, err := c.phases()
	if err != nil {
		return err
	}

	if slices.Equal(phases, []string{cluster.OperationPhaseKubernetes}) {
		logrus.Info("The Immutable kind has no kubernetes delete, nothing was deleted. " +
			"Use --phase infrastructure to erase the install disk of the nodes.")

		return nil
	}

	if err := c.checkScripts(phases); err != nil {
		return err
	}

	infra := slices.Contains(phases, cluster.OperationPhaseInfrastructure)

	var runner *ansible.Runner

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

// RemoveNode checks the host, drains a worker, then erases the install disk. A dry run only checks.
func (c *ClusterDeleter) RemoveNode() error {
	host := c.deleteNode

	runner, err := c.prepare()
	if err != nil {
		return err
	}

	if err := c.checkSingleHostMode(); err != nil {
		return err
	}

	logrus.Infof("Checking %s: MAC address, Flatcar, install disk...", host)

	if _, err := runner.Playbook(nodeArgs(host, "preflight")...); err != nil {
		return fmt.Errorf("%w: %w", ErrPreflightFailed, err)
	}

	if c.dryRun {
		logrus.Infof("Dry run completed, %s passed the checks and nothing was changed", host)

		return nil
	}

	if c.furyctlConf.NodeRole(host) == public.NodeRoleWorker {
		logrus.Infof("Draining %s and deleting its Node object...", host)

		args := append(nodeArgs(host, "drain"), "-e", kubectlVar+"="+c.pf.KubectlPath)
		if _, err := runner.Playbook(args...); err != nil {
			return fmt.Errorf("%w: %w", ErrDrainFailed, err)
		}
	}

	logrus.Infof("Erasing the install disk of %s and restarting it...", host)

	if _, err := runner.Playbook(nodeArgs(host, "wipe")...); err != nil {
		return fmt.Errorf("%w: %w", ErrNodeWipeFailed, err)
	}

	logrus.Infof("The install disk of %s is erased, the host restarts, boots from the network and waits there", host)

	return nil
}

// CheckNode refuses a host of delete node that is not a worker, before the confirmation.
func (c *ClusterDeleter) CheckNode() error {
	if role := c.furyctlConf.NodeRole(c.deleteNode); role != public.NodeRoleWorker {
		return fmt.Errorf("%w: %s", ErrNodeUnsupported, nodeRoleProblem(c.deleteNode, role, "worker"))
	}

	return nil
}

// deleteWorker removes the worker and prints what to remove from the configuration file.
func (c *ClusterDeleter) deleteWorker() error {
	if err := c.CheckNode(); err != nil {
		return err
	}

	if err := c.RemoveNode(); err != nil {
		return err
	}

	if !c.dryRun {
		logrus.Info(c.configEdit())
	}

	return nil
}

// configEdit lists the entries to remove from the configuration file: furyctl never edits it.
func (c *ClusterDeleter) configEdit() string {
	lines := []string{
		fmt.Sprintf("To finish, remove %s from the configuration file and run furyctl apply:", c.deleteNode),
		"  - spec.infrastructure.nodes: the entry with hostname " + c.deleteNode,
	}

	for _, ng := range c.furyctlConf.Spec.Kubernetes.NodeGroups {
		if !lo.ContainsBy(ng.Nodes, func(m public.Member) bool { return m.Hostname == c.deleteNode }) {
			continue
		}

		if len(ng.Nodes) == 1 {
			lines = append(lines, fmt.Sprintf(
				"  - spec.kubernetes.nodeGroups: the node group %s, whose only node is %s", ng.Name, c.deleteNode,
			))

			continue
		}

		lines = append(lines, fmt.Sprintf(
			"  - spec.kubernetes.nodeGroups, node group %s: the entry with hostname %s in nodes", ng.Name, c.deleteNode,
		))
	}

	return strings.Join(lines, "\n")
}

// nodeRoleProblem says why a host cannot be deleted or reinstalled alone.
func nodeRoleProblem(host, role, accepted string) string {
	if role == public.NodeRoleNone {
		return fmt.Sprintf("%q is not a host of this cluster configuration", host)
	}

	problem := fmt.Sprintf("%q is a %s host, and only %s hosts can be erased one at a time", host, role, accepted)
	if role == public.NodeRoleControlPlane {
		problem += ": etcd and the control plane change as a group"
	}

	return problem
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
	plugins := del.NewPlugins(c.kfdManifest, c.paths, c.dryRun)

	// The StorageClass providers go after the distribution, whose PVCs they serve.
	steps := lo.Filter([]workloadStep{
		{cluster.OperationPhasePlugins, "Deleting the plugins that own no StorageClass...", plugins, nil},
		{
			cluster.OperationPhaseDistribution, "Deleting SIGHUP Distribution...",
			del.NewDistribution(c.kfdManifest, c.paths, c.dryRun), nil,
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
func (c *ClusterDeleter) checkNodes(runner *ansible.Runner) error {
	hosts := c.hosts()

	logrus.Infof("Checking the %d hosts: MAC address, Flatcar, install disk...", len(hosts))

	if _, err := runner.Playbook(playbookArgs(hosts, "preflight")...); err != nil {
		return fmt.Errorf("%w: %w", ErrPreflightFailed, err)
	}

	return nil
}

// wipe erases the install disk of every host with the distribution delete playbook.
func (c *ClusterDeleter) wipe(runner *ansible.Runner) error {
	hosts := c.hosts()

	logrus.Infof("Erasing the install disk of the %d hosts and restarting them...", len(hosts))

	// One fork per host: every host gets the wipe in the same batch.
	if _, err := runner.Playbook(append(playbookArgs(hosts, "wipe"), "-f", strconv.Itoa(len(hosts)))...); err != nil {
		return fmt.Errorf("%w: %w", ErrWipeFailed, err)
	}

	logrus.Infof("Delete completed, the install disks of %d hosts are erased: %s", len(hosts), strings.Join(hosts, ", "))
	logrus.Info("Run furyctl apply to install them again")

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
func (c *ClusterDeleter) prepare() (*ansible.Runner, error) {
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

// nodeArgs selects one host in the delete playbook; its guard checks that it is a worker or a load balancer.
func nodeArgs(host, tags string) []string {
	return append(playbookArgs([]string{host}, tags), "-e", targetVar+"="+host)
}

// checkSingleHostMode stops when the delete playbook of the distribution has no single-host mode.
func (c *ClusterDeleter) checkSingleHostMode() error {
	rel := filepath.Join("templates", cluster.OperationPhasePreFlight, "immutable", deletePlaybook)

	content, err := os.ReadFile(filepath.Join(c.paths.DistroPath, rel))
	if err != nil {
		return fmt.Errorf("error reading %s: %w", rel, err)
	}

	if !strings.Contains(string(content), targetVar) {
		return fmt.Errorf(
			"the distribution version %s %w of one host: its %s has no %s, use a distribution version that does",
			c.kfdManifest.Version, ErrDeleteNotSupported, rel, targetVar,
		)
	}

	return nil
}

// hosts lists every machine of spec.infrastructure.nodes, as the preflight inventory does.
func (c *ClusterDeleter) hosts() []string {
	return lo.Map(
		c.furyctlConf.Spec.Infrastructure.Nodes,
		func(n public.SpecInfrastructureNode, _ int) string { return n.Hostname },
	)
}

func (c *ClusterDeleter) runner() (*ansible.Runner, error) {
	pf, err := c.preflight()
	if err != nil {
		return nil, err
	}

	return pf.AnsibleRunner(), nil
}

// preflight renders the preflight folder the first time, and returns it.
func (c *ClusterDeleter) preflight() (*create.PreFlight, error) {
	if c.pf != nil {
		return c.pf, nil
	}

	pf := create.NewPreFlight(
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

	if err := pf.Render(); err != nil {
		return nil, fmt.Errorf("error rendering the preflight templates: %w", err)
	}

	c.pf = pf

	return pf, nil
}

// kubeconfig points KUBECONFIG to the admin.conf of a control plane host and checks that the API answers.
func (c *ClusterDeleter) kubeconfig() error {
	pf, err := c.preflight()
	if err != nil {
		return err
	}

	_, err = pf.FetchKubeconfig()

	return err //nolint:wrapcheck // Delete wraps it with ErrAPIUnreachable.
}
